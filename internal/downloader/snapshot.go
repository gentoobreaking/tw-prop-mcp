package downloader

import (
	"context"
	"errors"
	"fmt"
	"os"
		"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
	"tw-prop-mcp/internal/repository/db"
)

// Err fields for error classification.
var (
	ErrNetworkTimeout = errors.New("network timeout")
)

// SnapshotService orchestrates download → checksum → archive → snapshot → import_batch.
// Implements idempotency: same (source, sourceVersion, fileSHA256) returns existing snapshot.
type SnapshotService struct {
	Downloader Downloader
	Archive    *RawArchiveStore
	Repo       repository.SnapshotRepository
	DB         repository.DBTX
}

// NewSnapshotService creates a SnapshotService.
func NewSnapshotService(downloader Downloader, archive *RawArchiveStore, repo repository.SnapshotRepository, db repository.DBTX) *SnapshotService {
	return &SnapshotService{
		Downloader: downloader,
		Archive:    archive,
		Repo:       repo,
		DB:         db,
	}
}

// CreateFromDownload downloads url, verifies, archives and creates a PENDING snapshot.
// Idempotent: if a snapshot with same source+sourceVersion+sha256 exists, it is returned directly.
func (s *SnapshotService) CreateFromDownload(ctx context.Context, source, sourceVersion, url, fileName string) (domain.DatasetSnapshot, error) {
	return s.CreateFromDownloadWithChecksum(ctx, source, sourceVersion, url, fileName, "")
}

// CreateFromDownloadWithChecksum allows caller to provide expected SHA256 for verification.
func (s *SnapshotService) CreateFromDownloadWithChecksum(ctx context.Context, source, sourceVersion, url, fileName, expectedSHA256 string) (domain.DatasetSnapshot, error) {
	if source == "" || sourceVersion == "" {
		return domain.DatasetSnapshot{}, fmt.Errorf("source and sourceVersion required")
	}
	if url == "" {
		return domain.DatasetSnapshot{}, fmt.Errorf("%w: empty url", ErrDownloadFailed)
	}
	if s.Downloader == nil {
		return domain.DatasetSnapshot{}, fmt.Errorf("downloader not configured")
	}
	if s.Archive == nil {
		return domain.DatasetSnapshot{}, fmt.Errorf("archive not configured")
	}
	if s.Repo == nil || s.DB == nil {
		return domain.DatasetSnapshot{}, fmt.Errorf("repository not configured")
	}

	// Resolve fileName if empty from URL.
	if fileName == "" {
		// basename of URL path
		parts := strings.Split(url, "/")
		fileName = parts[len(parts)-1]
		if fileName == "" {
			fileName = "download"
		}
	}

	// 1. Download to temp file.
	tmpFile, err := os.CreateTemp("", "downloader-*")
	if err != nil {
		return domain.DatasetSnapshot{}, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	// download dest is tmpPath
	// Ensure cleanup after.
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	downloadedPath, err := s.Downloader.Download(ctx, url, tmpPath)
	if err != nil {
		// Classify network timeout vs generic download failure.
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "deadline") {
			return domain.DatasetSnapshot{}, fmt.Errorf("%w: %w", ErrNetworkTimeout, err)
		}
		return domain.DatasetSnapshot{}, fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}
	// If downloader returned different path (e.g., dest was dir), use that.
	if downloadedPath != "" && downloadedPath != tmpPath {
		tmpPath = downloadedPath
	}

	// 2. Compute SHA256.
	actualSHA, err := ComputeFileSHA256(tmpPath)
	if err != nil {
		return domain.DatasetSnapshot{}, fmt.Errorf("compute checksum: %w", err)
	}

	// 3. Verify expected if provided.
	if expectedSHA256 != "" {
		if !strings.EqualFold(actualSHA, expectedSHA256) {
			return domain.DatasetSnapshot{}, fmt.Errorf("%w: expected %s got %s", ErrChecksumMismatch, expectedSHA256, actualSHA)
		}
	}

	// 4. Idempotency lookup: existing snapshot by (source, sourceVersion, fileSHA256)
	existing, err := s.findBySourceSHA(ctx, source, sourceVersion, actualSHA)
	if err == nil && existing != nil {
		return *existing, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		// Log but continue to create? Non-NoRows error.
		// If error is not NoRows, return it.
		// We treat any unexpected DB error as failure.
		// However if it's "no rows", we proceed to create.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// real pg error
		}
		// Check if error is pgx.ErrNoRows wrapped.
		if !isNoRows(err) {
			return domain.DatasetSnapshot{}, fmt.Errorf("lookup existing snapshot: %w", err)
		}
	}

	// 5. Prepare snapshot ID upfront for archive linkage (spec requires Store before Create).
	newID := uuid.New().String()
	metadata := map[string]any{
		"source":         source,
		"source_version": sourceVersion,
		"url":            url,
		"file_name":      fileName,
		"file_sha256":    actualSHA,
	}

	// 6. Store raw archive (atomic, immutable). Uses generated newID.
	archiveDir, err := s.Archive.StoreRawArchive(tmpPath, newID, metadata)
	if err != nil {
		// If archive already exists race, try lookup again.
		if strings.Contains(err.Error(), "already exists") {
			existing2, err2 := s.findBySourceSHA(ctx, source, sourceVersion, actualSHA)
			if err2 == nil && existing2 != nil {
				return *existing2, nil
			}
		}
		return domain.DatasetSnapshot{}, fmt.Errorf("store archive: %w", err)
	}
	_ = archiveDir

	// 7. Create snapshot with explicit ID to match archive.
	snap, err := s.createSnapshotWithID(ctx, newID, source, sourceVersion, fileName, actualSHA)
	if err != nil {
		// Unique violation => fetch existing (concurrent race)
		if isUniqueViolation(err) {
			// Cleanup archive we just created? Keep it if race? But archive with newID is orphan.
			// Try to remove orphan archive.
			_ = os.RemoveAll(archiveDir)
			existing3, err2 := s.findBySourceSHA(ctx, source, sourceVersion, actualSHA)
			if err2 == nil && existing3 != nil {
				return *existing3, nil
			}
		}
		// Cleanup archive on failure
		_ = os.RemoveAll(archiveDir)
		return domain.DatasetSnapshot{}, fmt.Errorf("create snapshot: %w", err)
	}

	// 8. Create import_batch.
	if err := s.createImportBatch(ctx, snap.ID); err != nil {
		// Snapshot created but import_batch failed. Return snapshot anyway? Spec says create ImportBatch then return.
		// Log error but do not rollback snapshot.
		return snap, fmt.Errorf("snapshot created %s but import_batch failed: %w", snap.ID, err)
	}

	return snap, nil
}

func (s *SnapshotService) findBySourceSHA(ctx context.Context, source, sourceVersion, sha string) (*domain.DatasetSnapshot, error) {
	row := s.DB.QueryRow(ctx,
		`SELECT id, source, source_version, downloaded_at, published_at, file_name, file_sha256, record_count, status, schema_version, import_started_at, import_completed_at, created_at FROM dataset_snapshot WHERE source=$1 AND source_version=$2 AND file_sha256=$3 LIMIT 1`,
		source, sourceVersion, sha,
	)
	var r db.DatasetSnapshot
	err := row.Scan(
		&r.ID, &r.Source, &r.SourceVersion, &r.DownloadedAt, &r.PublishedAt,
		&r.FileName, &r.FileSha256, &r.RecordCount, &r.Status, &r.SchemaVersion,
		&r.ImportStartedAt, &r.ImportCompletedAt, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	ds := toDomainSnapshot(r)
	return &ds, nil
}

func (s *SnapshotService) createSnapshotWithID(ctx context.Context, id, source, sourceVersion, fileName, sha string) (domain.DatasetSnapshot, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return domain.DatasetSnapshot{}, err
	}
	// Direct insert with explicit ID to tie archive dir.
	row := s.DB.QueryRow(ctx,
		`INSERT INTO dataset_snapshot (id, source, source_version, file_name, file_sha256, record_count, status, schema_version)
		 VALUES ($1,$2,$3,$4,$5,0,'PENDING','v2.0') RETURNING id, source, source_version, downloaded_at, published_at, file_name, file_sha256, record_count, status, schema_version, import_started_at, import_completed_at, created_at`,
		uid, source, sourceVersion, fileName, sha,
	)
	var r db.DatasetSnapshot
	if err := row.Scan(
		&r.ID, &r.Source, &r.SourceVersion, &r.DownloadedAt, &r.PublishedAt,
		&r.FileName, &r.FileSha256, &r.RecordCount, &r.Status, &r.SchemaVersion,
		&r.ImportStartedAt, &r.ImportCompletedAt, &r.CreatedAt,
	); err != nil {
		return domain.DatasetSnapshot{}, err
	}
	return toDomainSnapshot(r), nil
}

func (s *SnapshotService) createImportBatch(ctx context.Context, snapshotID string) error {
	uid, err := parseUUID(snapshotID)
	if err != nil {
		return err
	}
	queries := db.New(s.DB)
	_, err = queries.CreateImportBatch(ctx, uid)
	return err
}

func toDomainSnapshot(row db.DatasetSnapshot) domain.DatasetSnapshot {
	ds := domain.DatasetSnapshot{
		Source:        row.Source,
		SourceVersion: row.SourceVersion,
		FileName:      row.FileName,
		FileSHA256:    row.FileSha256,
		RecordCount:   row.RecordCount,
		Status:        domain.SnapshotStatus(row.Status),
		SchemaVersion: row.SchemaVersion,
	}
	if row.ID.Valid {
		ds.ID = uuidToString(row.ID)
	}
	if row.DownloadedAt.Valid {
		ds.DownloadedAt = row.DownloadedAt.Time
	}
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		ds.PublishedAt = &t
	}
	if row.ImportStartedAt.Valid {
		t := row.ImportStartedAt.Time
		ds.ImportStartedAt = &t
	}
	if row.ImportCompletedAt.Valid {
		t := row.ImportCompletedAt.Time
		ds.ImportCompletedAt = &t
	}
	if row.CreatedAt.Valid {
		ds.CreatedAt = row.CreatedAt.Time
	}
	return ds
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	if !u.Valid {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q", s)
	}
	return u, nil
}

func uuidToString(u pgtype.UUID) string {
	id := uuid.UUID(u.Bytes)
	return id.String()
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505")
}

