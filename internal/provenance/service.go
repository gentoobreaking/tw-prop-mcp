package provenance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// ProvenanceService provides end-to-end data provenance queries.
// P6 Provenance Required: every transaction/valuation result must answer
// "where did this data come from?"
type ProvenanceService struct {
	txRepo     repository.TransactionRepository
	parcelRepo repository.ParcelRepository
	snapRepo   repository.SnapshotRepository
	dbtx       repository.DBTX
}

// ProvenanceServiceConfig holds dependencies for the ProvenanceService.
type ProvenanceServiceConfig struct {
	TxRepo     repository.TransactionRepository
	ParcelRepo repository.ParcelRepository
	SnapRepo   repository.SnapshotRepository
	DBTX       repository.DBTX
}

// NewProvenanceService creates a new ProvenanceService.
func NewProvenanceService(config ProvenanceServiceConfig) *ProvenanceService {
	return &ProvenanceService{
		txRepo:     config.TxRepo,
		parcelRepo: config.ParcelRepo,
		snapRepo:   config.SnapRepo,
		dbtx:       config.DBTX,
	}
}

// GetSnapshot returns the full dataset_snapshot record for a given snapshot ID.
// Implements the get_data_snapshot tool logic (T016 acceptance criterion).
func (s *ProvenanceService) GetSnapshot(ctx context.Context, snapshotID string) (*domain.DatasetSnapshot, error) {
	snap, err := s.snapRepo.GetByID(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	if snap.ID == "" {
		return nil, ErrSnapshotNotFound
	}
	return &snap, nil
}

// GetProvenanceByTransaction returns the full provenance chain for a transaction:
// Transaction → Snapshot → Official Source
func (s *ProvenanceService) GetProvenanceByTransaction(ctx context.Context, transactionID string) (*domain.ProvenanceInfo, error) {
	txUID, err := parseUUID(transactionID)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction ID: %w", err)
	}

	tx, err := s.txRepo.GetByID(ctx, txUID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransactionNotFound, err)
	}

	snap, err := s.snapRepo.GetByID(ctx, tx.SnapshotID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot for transaction: %w", err)
	}

	return &domain.ProvenanceInfo{
		Source:           snap.Source,
		DatasetSnapshot:  snap.ID,
		SourceFile:       snap.FileName,
		RecordHash:       tx.SourceRecordHash,
		ImportBatchID:    tx.ImportBatchID,
		AlgorithmVersion: "v2.0",
	}, nil
}

// GetProvenanceByValuation returns the full provenance chain for a valuation:
// Valuation → Comparables → Transactions → Snapshot → Official Source
func (s *ProvenanceService) GetProvenanceByValuation(ctx context.Context, valuationID string) (*domain.ProvenanceChain, error) {
	result, err := s.getValuationResult(ctx, valuationID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValuationNotFound, err)
	}

	// Fetch each comparable transaction's provenance
	comparableProvenance := make([]domain.ProvenanceInfo, 0, len(result.ComparableIDs))
	for _, compID := range result.ComparableIDs {
		info, err := s.GetProvenanceByTransaction(ctx, compID)
		if err != nil {
			// If any comparable's provenance is missing, return DATA_NOT_AVAILABLE
			return &domain.ProvenanceChain{
				ValuationID:           valuationID,
				TargetParcel:          result.TargetParcelID,
				TargetTransactionID:   result.TargetTransactionID,
				Source:                result.SnapshotSource,
				DatasetSnapshot:       result.SnapshotID,
				SourceFile:            result.SnapshotFileName,
				AlgorithmVersion:      result.AlgorithmVersion,
				ConfigurationVersion:  result.ConfigurationVersion,
				OutlierMethod:         result.OutlierMethod,
				Confidence:            result.Confidence,
				ComparableIDs:         result.ComparableIDs,
				Statistics:            result.Statistics,
				Status:                "DATA_NOT_AVAILABLE",
				Error:                 fmt.Sprintf("comparable %s provenance unavailable", compID),
			}, nil
		}
		comparableProvenance = append(comparableProvenance, *info)
	}

	return &domain.ProvenanceChain{
		ValuationID:           valuationID,
		ValuationStatus:       result.Status,
		BearValue:             result.BearValue,
		BaseValue:             result.BaseValue,
		BullValue:             result.BullValue,
		TargetParcel:          result.TargetParcelID,
		TargetParcelLocation:  fmt.Sprintf("%s/%s/%s/%s", result.County, result.District, result.Section, result.LandNumber),
		TargetTransactionID:   result.TargetTransactionID,
		AlgorithmVersion:      result.AlgorithmVersion,
		ConfigurationVersion:  result.ConfigurationVersion,
		OutlierMethod:         result.OutlierMethod,
		Confidence:            result.Confidence,
		ComparableIDs:         result.ComparableIDs,
		ComparableProvenance:  comparableProvenance,
		Source:                result.SnapshotSource,
		DatasetSnapshot:       result.SnapshotID,
		SourceFile:            result.SnapshotFileName,
		SnapshotSHA256:        result.SnapshotSHA256,
		SnapshotStatus:        result.SnapshotStatus,
		Statistics:            result.Statistics,
		Weights:               result.Weights,
		Status:                "AVAILABLE",
		CreatedAt:             result.CreatedAt,
	}, nil
}

// getValuationResult fetches a valuation_result with JOINs to parcel and dataset_snapshot.
func (s *ProvenanceService) getValuationResult(ctx context.Context, valuationID string) (*ValuationResultRow, error) {
	if s.dbtx == nil {
		return nil, errors.New("database not available")
	}

	query := `
SELECT
	v.target_parcel_id, v.snapshot_id, v.comparable_ids, v.algorithm_version,
	v.configuration_version, v.outlier_method, v.confidence, v.status,
	v.query_hash, v.bear_value, v.base_value, v.bull_value,
	v.statistics, v.weights, v.created_at,
	s.source AS snapshot_source, s.file_name AS snapshot_file_name,
	s.file_sha256 AS snapshot_sha256, s.status AS snapshot_status,
	p.county, p.district, p.section, p.land_number
FROM valuation_result v
JOIN parcel p ON v.target_parcel_id = p.id
JOIN dataset_snapshot s ON v.snapshot_id = s.id
WHERE v.id = $1
`

	var row ValuationResultRow
	var comparableIDsJSON []byte
	var statisticsJSON []byte
	var weightsJSON []byte

	err := s.dbtx.QueryRow(ctx, query, valuationID).Scan(
		&row.TargetParcelID,
		&row.SnapshotID,
		&comparableIDsJSON,
		&row.AlgorithmVersion,
		&row.ConfigurationVersion,
		&row.OutlierMethod,
		&row.Confidence,
		&row.Status,
		&row.QueryHash,
		&row.BearValue,
		&row.BaseValue,
		&row.BullValue,
		&statisticsJSON,
		&weightsJSON,
		&row.CreatedAt,
		&row.SnapshotSource,
		&row.SnapshotFileName,
		&row.SnapshotSHA256,
		&row.SnapshotStatus,
		&row.County,
		&row.District,
		&row.Section,
		&row.LandNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("query valuation provenance: %w", err)
	}

	// Parse JSONB arrays
	if len(comparableIDsJSON) > 0 {
		if err := json.Unmarshal(comparableIDsJSON, &row.ComparableIDs); err != nil {
			return nil, fmt.Errorf("parse comparable_ids: %w", err)
		}
	}
	row.Statistics = statisticsJSON
	row.Weights = weightsJSON

	return &row, nil
}

// BuildProvenanceResponse builds a domain.DataProvenance from a transaction and snapshot.
func (s *ProvenanceService) BuildProvenanceResponse(tx domain.Transaction, snap domain.DatasetSnapshot) domain.DataProvenance {
	return domain.DataProvenance{
		Source:               snap.Source,
		DatasetSnapshot:      snap.ID,
		SourceFile:           snap.FileName,
		RecordHash:           tx.SourceRecordHash,
		ImportBatchID:        tx.ImportBatchID,
		AlgorithmVersion:     "v2.0",
		ConfigurationVersion: "v2.0",
	}
}

// BuildValuationProvenanceForResponse builds DataProvenance for a valuation result.
func (s *ProvenanceService) BuildValuationProvenanceForResponse(ctx context.Context, valuationID string) (domain.DataProvenance, error) {
	result, err := s.getValuationResult(ctx, valuationID)
	if err != nil {
		return domain.DataProvenance{}, err
	}

	dp := domain.DataProvenance{
		Source:               result.SnapshotSource,
		DatasetSnapshot:      result.SnapshotID,
		SourceFile:           result.SnapshotFileName,
		AlgorithmVersion:     result.AlgorithmVersion,
		ConfigurationVersion: result.ConfigurationVersion,
	}
	return dp, nil
}

// BuildEnvelope creates a ResponseEnvelope with metadata and provenance.
// ProvenanceMiddleware calls this before returning each MCP tool response (T016).
func (s *ProvenanceService) BuildEnvelope(queryHash, algoVer, configVer, snapshotID string) domain.ResponseEnvelope {
	return domain.ResponseEnvelope{
		Metadata: domain.ResponseMetadata{
			AlgorithmVersion:     algoVer,
			SnapshotID:           snapshotID,
			GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
			QueryHash:            queryHash,
			ConfigurationVersion: configVer,
		},
	}
}

// parseUUID parses a UUID string.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// ValuationResultRow represents a valuation_result row with joined data
// from parcel and dataset_snapshot for provenance tracing.
type ValuationResultRow struct {
	TargetParcelID       string `json:"target_parcel_id"`
	SnapshotID            string          `json:"snapshot_id"`
	ComparableIDs         []string        `json:"comparable_ids"`
	AlgorithmVersion      string          `json:"algorithm_version"`
	ConfigurationVersion  string          `json:"configuration_version"`
	OutlierMethod         string          `json:"outlier_method"`
	Confidence            string          `json:"confidence"`
	Status                string          `json:"status"`
	QueryHash             string          `json:"query_hash"`
	BearValue             int64           `json:"bear_value"`
	BaseValue             int64           `json:"base_value"`
	BullValue             int64           `json:"bull_value"`
	Statistics            json.RawMessage `json:"statistics"`
	Weights               json.RawMessage `json:"weights"`
	CreatedAt             time.Time       `json:"created_at"`

	// Joined from dataset_snapshot
	SnapshotSource   string `json:"snapshot_source"`
	SnapshotFileName string `json:"snapshot_file_name"`
	SnapshotSHA256   string `json:"snapshot_sha256"`
	SnapshotStatus   string `json:"snapshot_status"`

	// Joined from parcel
	County     string `json:"county"`
	District   string `json:"district"`
	Section    string `json:"section"`
	LandNumber string `json:"land_number"`

	// Joined from transaction
	TargetTransactionID string `json:"target_transaction_id"`
}

// Errors
var (
	ErrSnapshotNotFound    = errors.New("snapshot not found")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrValuationNotFound   = errors.New("valuation not found")
	ErrDataNotAvailable    = errors.New("DATA_NOT_AVAILABLE")
)
