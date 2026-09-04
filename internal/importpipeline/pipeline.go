package importpipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/downloader"
	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/mcp"
	"tw-prop-mcp/internal/normalizer"
	"tw-prop-mcp/internal/parser"
	"tw-prop-mcp/internal/repository"
	"tw-prop-mcp/internal/validator"
)

// ImportPipelineError represents an error in the import pipeline.
type ImportPipelineError struct {
	Stage string
	Err   error
}

func (e *ImportPipelineError) Error() string {
	return fmt.Sprintf("import pipeline [%s]: %v", e.Stage, e.Err)
}

func (e *ImportPipelineError) Unwrap() error {
	return e.Err
}

// ImportResult holds the result of an import operation.
type ImportResult struct {
	TransactionsImported int
	ParcelsImported      int
	Errors               []ImportPipelineError
	Duration             time.Duration
	SnapshotID           string
}

// ImportPipelineStatus represents the current status of the import pipeline.
type ImportPipelineStatus string

const (
	StatusPending     ImportPipelineStatus = "PENDING"
	StatusDownloading ImportPipelineStatus = "DOWNLOADING"
	StatusParsing     ImportPipelineStatus = "PARSING"
	StatusNormalizing ImportPipelineStatus = "NORMALIZING"
	StatusValidating  ImportPipelineStatus = "VALIDATING"
	StatusImporting   ImportPipelineStatus = "IMPORTING"
	StatusLocked      ImportPipelineStatus = "LOCKED"
	StatusFailed      ImportPipelineStatus = "FAILED"
)

// PipelineConfig holds configuration for the import pipeline.
type PipelineConfig struct {
	SnapshotID       string
	DownloadURL      string
	DownloadDest     string
	ExpectedChecksum string // Optional SHA256 checksum
	MaxRetries       int
	RetryDelay       time.Duration
}

// ImportPipeline orchestrates the complete data import flow.
type ImportPipeline struct {
	Downloader     *downloader.HTTPDownloader
	Parser         *parser.Parser
	Normalizer     *normalizer.Normalizer
	Validator      *validator.Validator
	TxRepo         repository.TransactionRepository
	ParcelRepo     repository.ParcelRepository
	SnapshotRepo   repository.SnapshotRepository
	Config         PipelineConfig
	Status         ImportPipelineStatus
	CurrentStage   string
	Logger         *slog.Logger
}

// NewImportPipeline creates a new ImportPipeline with default components.
func NewImportPipeline(config PipelineConfig, logger *slog.Logger) *ImportPipeline {
	if logger == nil {
		logger = slog.Default()
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 5 * time.Second
	}

	return &ImportPipeline{
		Downloader:    downloader.NewHTTPDownloader(),
		Parser:        parser.NewParser(),
		Normalizer:    normalizer.New(),
		Validator:     validator.New(nil), // uses default clock
		Config:        config,
		Status:        StatusPending,
		Logger:        logger.With("snapshot_id", config.SnapshotID),
	}
}

// SetRepositories sets the repository dependencies (called after DB connection established).
func (p *ImportPipeline) SetRepositories(
	txRepo repository.TransactionRepository,
	parcelRepo repository.ParcelRepository,
	snapshotRepo repository.SnapshotRepository,
) {
	p.TxRepo = txRepo
	p.ParcelRepo = parcelRepo
	p.SnapshotRepo = snapshotRepo
}

// ImportFromSource executes the complete import pipeline.
func (p *ImportPipeline) ImportFromSource(ctx context.Context) (ImportResult, error) {
	start := time.Now()
	result := ImportResult{SnapshotID: p.Config.SnapshotID}
	p.Status = StatusPending

	// Initialize snapshot if not exists
	if err := p.initSnapshot(ctx); err != nil {
		return result, p.wrapError("init_snapshot", err)
	}

	// Stage 1: Download
	p.setStatus(StatusDownloading)
	archivePath, err := p.download(ctx)
	if err != nil {
		p.setStatus(StatusFailed)
		return result, p.wrapError("download", err)
	}

	// Stage 2: Verify checksum
	if p.Config.ExpectedChecksum != "" {
		if err := p.verifyChecksum(archivePath); err != nil {
			p.setStatus(StatusFailed)
			return result, p.wrapError("checksum", err)
		}
	}

	// Stage 3: Parse
	p.setStatus(StatusParsing)
	rawRows, err := p.parse(ctx, archivePath)
	if err != nil {
		p.setStatus(StatusFailed)
		return result, p.wrapError("parse", err)
	}

	// Stage 4: Normalize
	p.setStatus(StatusNormalizing)
	transactions, parcels, err := p.normalize(rawRows)
	if err != nil {
		p.setStatus(StatusFailed)
		return result, p.wrapError("normalize", err)
	}

	// Stage 5: Validate
	p.setStatus(StatusValidating)
	validTxns, validParcels, err := p.validate(transactions, parcels)
	if err != nil {
		p.setStatus(StatusFailed)
		return result, p.wrapError("validate", err)
	}

	// Stage 6: Deduplicate
	p.setStatus(StatusImporting)

	// Update snapshot status to IMPORTING before import
	if p.SnapshotRepo != nil {
		if err := p.SnapshotRepo.UpdateStatus(ctx, p.Config.SnapshotID, domain.SnapshotStatusImporting); err != nil {
			p.setStatus(StatusFailed)
			return result, p.wrapError("update_snapshot_status", err)
		}
	}

	dedupedTxns, dedupedParcels := p.deduplicate(validTxns, validParcels)

	// Create import batch
	importBatchID := uuid.NewString()

	// Stage 7: Import
	if err := p.importData(ctx, dedupedTxns, dedupedParcels, importBatchID); err != nil {
		p.setStatus(StatusFailed)
		return result, p.wrapError("import", err)
	}
	result.TransactionsImported = len(dedupedTxns)
	result.ParcelsImported = len(dedupedParcels)

	// Stage 8: Lock snapshot
	if err := p.lockSnapshot(ctx); err != nil {
		p.setStatus(StatusFailed)
		return result, p.wrapError("lock_snapshot", err)
	}

	p.setStatus(StatusLocked)
	result.Duration = time.Since(start)
	mcp.IncDataImport(true)
	return result, nil
}

// initSnapshot creates the snapshot record if it doesn't exist.
func (p *ImportPipeline) initSnapshot(ctx context.Context) error {
	if p.SnapshotRepo == nil {
		return errors.New("snapshot repository not set")
	}

	// Check if snapshot exists
	_, err := p.SnapshotRepo.GetByID(ctx, p.Config.SnapshotID)
	if err == nil {
		return nil // already exists
	}

	// Create new snapshot with the configured SnapshotID
	_, err = p.SnapshotRepo.Create(ctx, repository.CreateSnapshotParams{
		ID:             p.Config.SnapshotID,
		Source:         "OFFICIAL_CSV",
		SourceVersion:  "v2.0",
		FileName:       filepath.Base(p.Config.DownloadURL),
		FileSHA256:     p.Config.ExpectedChecksum,
		RecordCount:    0,
		Status:         domain.SnapshotStatusPending,
		SchemaVersion:  "v2.0",
	})
	return err
}

// download downloads the source file.
func (p *ImportPipeline) download(ctx context.Context) (string, error) {
	dest := p.Config.DownloadDest
	if dest == "" {
		dest = filepath.Join(os.TempDir(), fmt.Sprintf("import_%s", p.Config.SnapshotID))
	}

	var lastErr error
	for attempt := 0; attempt <= p.Config.MaxRetries; attempt++ {
		if attempt > 0 {
			p.Logger.Warn("download retry", "attempt", attempt, "delay", p.Config.RetryDelay)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(p.Config.RetryDelay):
			}
		}

		path, err := p.Downloader.Download(ctx, p.Config.DownloadURL, dest)
		if err == nil {
			p.Logger.Info("download completed", "path", path)
			return path, nil
		}
		lastErr = err
		p.Logger.Warn("download failed", "attempt", attempt, "error", err)
	}
	return "", fmt.Errorf("after %d retries: %w", p.Config.MaxRetries, lastErr)
}

// verifyChecksum verifies the SHA256 checksum of the downloaded file.
func (p *ImportPipeline) verifyChecksum(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("compute hash: %w", err)
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != p.Config.ExpectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", p.Config.ExpectedChecksum, actual)
	}
	p.Logger.Info("checksum verified", "checksum", actual)
	return nil
}

// parse extracts and parses CSV from the archive.
func (p *ImportPipeline) parse(ctx context.Context, archivePath string) ([]map[string]string, error) {
	// For now, assume direct CSV file (archive handling can be extended)
	rows, err := p.Parser.ParseOfficialCSV(archivePath)
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	p.Logger.Info("parsing completed", "rows", len(rows))
	return rows, nil
}

// normalize converts raw rows to domain objects.
func (p *ImportPipeline) normalize(rows []map[string]string) ([]domain.Transaction, []domain.Parcel, error) {
	transactions := make([]domain.Transaction, 0, len(rows))
	parcels := make([]domain.Parcel, 0, len(rows))

	for i, row := range rows {
		// Normalize transaction
		txn, err := p.Normalizer.NormalizeTransaction(row, p.Config.SnapshotID)
		if err != nil {
			p.Logger.Warn("normalize transaction failed", "row", i, "error", err)
			continue
		}
		transactions = append(transactions, *txn)

		// Normalize parcel (if land data present)
		parcel, err := p.Normalizer.NormalizeParcel(row)
		if err == nil && parcel != nil {
			parcels = append(parcels, *parcel)
		}
	}

	p.Logger.Info("normalization completed", "transactions", len(transactions), "parcels", len(parcels))
	return transactions, parcels, nil
}

// validate validates normalized objects.
func (p *ImportPipeline) validate(transactions []domain.Transaction, parcels []domain.Parcel) ([]domain.Transaction, []domain.Parcel, error) {
	validTxns := make([]domain.Transaction, 0, len(transactions))
	validParcels := make([]domain.Parcel, 0, len(parcels))

	for _, txn := range transactions {
		issues := p.Validator.ValidateTransaction(&txn)
		if p.Validator.HasBlockingErrors(issues) {
			p.Logger.Warn("transaction validation failed", "id", txn.ID, "issues", issues)
			mcp.IncDataImport(false)
			continue
		}
		validTxns = append(validTxns, txn)
	}

	for _, parcel := range parcels {
		issues := p.Validator.ValidateParcel(&parcel)
		if p.Validator.HasBlockingErrors(issues) {
			p.Logger.Warn("parcel validation failed", "id", parcel.ID, "issues", issues)
			mcp.IncDataImport(false)
			continue
		}
		validParcels = append(validParcels, parcel)
	}

	p.Logger.Info("validation completed", "valid_transactions", len(validTxns), "valid_parcels", len(validParcels))
	return validTxns, validParcels, nil
}

// deduplicate removes duplicates based on source_record_hash within the same snapshot.
func (p *ImportPipeline) deduplicate(transactions []domain.Transaction, parcels []domain.Parcel) ([]domain.Transaction, []domain.Parcel) {
	seenTxns := make(map[string]bool)
	seenParcels := make(map[string]bool)
	dedupedTxns := make([]domain.Transaction, 0, len(transactions))
	dedupedParcels := make([]domain.Parcel, 0, len(parcels))

	for _, txn := range transactions {
		key := txn.SourceRecordHash
		if key == "" {
			// Generate hash from key fields if not present
			key = fmt.Sprintf("%s|%s|%s|%s|%s", txn.County, txn.District, txn.Section, txn.LandNumber, txn.TransactionDate.Format("2006-01-02"))
		}
		if !seenTxns[key] {
			seenTxns[key] = true
			dedupedTxns = append(dedupedTxns, txn)
		}
	}

	for _, parcel := range parcels {
		key := fmt.Sprintf("%s|%s|%s|%s", parcel.County, parcel.District, parcel.Section, parcel.LandNumber)
		if !seenParcels[key] {
			seenParcels[key] = true
			dedupedParcels = append(dedupedParcels, parcel)
		}
	}

	p.Logger.Info("deduplication completed", "transactions", len(dedupedTxns), "parcels", len(dedupedParcels))
	return dedupedTxns, dedupedParcels
}

// importData inserts data into the database.
func (p *ImportPipeline) importData(ctx context.Context, transactions []domain.Transaction, parcels []domain.Parcel, importBatchID string) error {
	if p.TxRepo == nil || p.ParcelRepo == nil {
		return errors.New("repositories not set")
	}

	// Import transactions
	if len(transactions) > 0 {
		for i := range transactions {
			transactions[i].ImportBatchID = importBatchID
		}
		inserted, err := p.TxRepo.BatchInsert(ctx, transactions)
		if err != nil {
			return fmt.Errorf("batch insert transactions: %w", err)
		}
		p.Logger.Info("transactions inserted", "count", inserted)
	}

	// Import parcels
	if len(parcels) > 0 {
		inserted, err := p.ParcelRepo.BatchInsert(ctx, parcels)
		if err != nil {
			return fmt.Errorf("batch insert parcels: %w", err)
		}
		p.Logger.Info("parcels inserted", "count", inserted)
	}

	return nil
}

// lockSnapshot transitions the snapshot to LOCKED status.
func (p *ImportPipeline) lockSnapshot(ctx context.Context) error {
	if p.SnapshotRepo == nil {
		return errors.New("snapshot repository not set")
	}
	if err := p.SnapshotRepo.Lock(ctx, p.Config.SnapshotID); err != nil {
		return err
	}
	mcp.IncSnapshotLocked()
	return nil
}

// setStatus updates the pipeline status.
func (p *ImportPipeline) setStatus(status ImportPipelineStatus) {
	p.Status = status
	p.CurrentStage = string(status)
	p.Logger.Info("stage", "status", status)
}

// wrapError wraps an error with stage context.
func (p *ImportPipeline) wrapError(stage string, err error) error {
	return &ImportPipelineError{Stage: stage, Err: err}
}

// RetryableError checks if an error is retryable.
func (p *ImportPipeline) RetryableError(err error) bool {
	var pipelineErr *ImportPipelineError
	if errors.As(err, &pipelineErr) {
		// Network errors and transient DB errors are retryable
		return errors.Is(pipelineErr.Err, context.DeadlineExceeded) ||
			errors.Is(pipelineErr.Err, io.EOF) ||
			errors.Is(pipelineErr.Err, http.ErrHandlerTimeout)
	}
	return false
}

// GetStatus returns the current pipeline status.
func (p *ImportPipeline) GetStatus() ImportPipelineStatus {
	return p.Status
}

// GetCurrentStage returns the current stage name.
func (p *ImportPipeline) GetCurrentStage() string {
	return p.CurrentStage
}