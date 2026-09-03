package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository/db"
)

// ValuationResult errors.
var (
	ErrValuationResultNotFound = fmt.Errorf("valuation result not found")
)

// ValuationResultRepository defines persistence operations for valuation_result.
// T029 acceptance criteria:
// - Insert(result)
// - GetByID(valuation_id)
// - ListByParcel(parcel_id, snapshot_id)
type ValuationResultRepository interface {
	Insert(ctx context.Context, result domain.ValuationResult) (domain.ValuationResult, error)
	GetByID(ctx context.Context, id string) (domain.ValuationResult, error)
	ListByParcel(ctx context.Context, parcelID, snapshotID string, limit int32) ([]domain.ValuationResult, error)
	GetByQueryHash(ctx context.Context, queryHash string) (domain.ValuationResult, error)
}

type valuationResultRepository struct {
	queries *db.Queries
}

// NewValuationResultRepository creates a repository backed by pgx + sqlc.
func NewValuationResultRepository(dbt DBTX) ValuationResultRepository {
	return &valuationResultRepository{
		queries: db.New(dbt),
	}
}

// Insert inserts a valuation result with forced provenance.
// Forces: snapshot_id, algorithm_version, configuration_version must be set.
// Automatically writes created_at (DB default).
func (r *valuationResultRepository) Insert(ctx context.Context, result domain.ValuationResult) (domain.ValuationResult, error) {
	// Validate required provenance fields
	if result.SnapshotID == "" {
		return domain.ValuationResult{}, fmt.Errorf("INVALID_ARGUMENT: missing snapshot_id")
	}
	if result.AlgorithmVersion == "" {
		return domain.ValuationResult{}, fmt.Errorf("INVALID_ARGUMENT: missing algorithm_version")
	}
	if result.ConfigurationVersion == "" {
		return domain.ValuationResult{}, fmt.Errorf("INVALID_ARGUMENT: missing configuration_version")
	}
	if result.TargetParcelID == "" {
		return domain.ValuationResult{}, fmt.Errorf("INVALID_ARGUMENT: missing target_parcel_id")
	}

	// Parse UUIDs
	targetParcelID, err := parseUUID(result.TargetParcelID)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("invalid target_parcel_id: %w", err)
	}
	snapshotID, err := parseUUID(result.SnapshotID)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("invalid snapshot_id: %w", err)
	}

	// Serialize JSONB fields
	comparableIDsJSON, err := json.Marshal(result.ComparableIDs)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("marshal comparable_ids: %w", err)
	}
	weightsJSON, err := json.Marshal(result.Weights)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("marshal weights: %w", err)
	}
	statsJSON, err := json.Marshal(result.RawStatistics)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("marshal statistics: %w", err)
	}

	params := db.InsertValuationResultParams{
		TargetParcelID:       targetParcelID,
		SnapshotID:           snapshotID,
		ComparableIds:        comparableIDsJSON,
		AlgorithmVersion:     result.AlgorithmVersion,
		ConfigurationVersion: result.ConfigurationVersion,
		OutlierMethod:        result.OutlierMethod,
		Weights:              weightsJSON,
		Statistics:           statsJSON,
		BearValue:            result.BearValue,
		BaseValue:            result.BaseValue,
		BullValue:            result.BullValue,
		Confidence:           string(result.Confidence),
		QueryHash:            result.QueryHash,
	}

	row, err := r.queries.InsertValuationResult(ctx, params)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("insert valuation result: %w", err)
	}

	return toDomainValuationResult(row), nil
}

// GetByID fetches a valuation result by UUID.
func (r *valuationResultRepository) GetByID(ctx context.Context, id string) (domain.ValuationResult, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("invalid ID: %w", err)
	}

	row, err := r.queries.GetValuationResult(ctx, uid)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("get valuation result: %w", err)
	}

	return toDomainValuationResult(row), nil
}

// ListByParcel lists valuation results for a parcel, ordered by created_at DESC.
func (r *valuationResultRepository) ListByParcel(ctx context.Context, parcelID, snapshotID string, limit int32) ([]domain.ValuationResult, error) {
	if limit <= 0 {
		limit = 100
	}

	parcelUUID, err := parseUUID(parcelID)
	if err != nil {
		return nil, fmt.Errorf("invalid parcel_id: %w", err)
	}
	snapUUID, err := parseUUID(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("invalid snapshot_id: %w", err)
	}

	rows, err := r.queries.ListValuationResultsByParcel(ctx, db.ListValuationResultsByParcelParams{
		TargetParcelID: parcelUUID,
		SnapshotID:     snapUUID,
		Limit:          limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list valuation results: %w", err)
	}

	out := make([]domain.ValuationResult, len(rows))
	for i, row := range rows {
		out[i] = toDomainValuationResult(row)
	}
	return out, nil
}

func (r *valuationResultRepository) GetByQueryHash(ctx context.Context, queryHash string) (domain.ValuationResult, error) {
	row, err := r.queries.GetValuationResultByQueryHash(ctx, queryHash)
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("get valuation by query hash: %w", err)
	}

	return toDomainValuationResult(row), nil
}

// toDomainValuationResult converts db.ValuationResult to domain.ValuationResult.
func toDomainValuationResult(row db.ValuationResult) domain.ValuationResult {
	var comparableIDs []string
	if len(row.ComparableIds) > 0 {
		_ = json.Unmarshal(row.ComparableIds, &comparableIDs)
	}

	var stats *domain.StatisticsResult
	if len(row.Statistics) > 0 {
		_ = json.Unmarshal(row.Statistics, &stats)
	}

	var weights map[string]interface{}
	if len(row.Weights) > 0 {
		_ = json.Unmarshal(row.Weights, &weights)
	}

	return domain.ValuationResult{
		ID:                   uuidToString(row.ID),
		TargetParcelID:       uuidToString(row.TargetParcelID),
		SnapshotID:           uuidToString(row.SnapshotID),
		ComparableIDs:        comparableIDs,
		AlgorithmVersion:     row.AlgorithmVersion,
		ConfigurationVersion: row.ConfigurationVersion,
		OutlierMethod:        row.OutlierMethod,
		BearValue:            row.BearValue,
		BaseValue:            row.BaseValue,
		BullValue:            row.BullValue,
		Confidence:           domain.ConfidenceLevel(row.Confidence),
		Status:               row.Status,
		QueryHash:            row.QueryHash,
		RawStatistics:        stats,
		CreatedAt:            row.CreatedAt.Time,
	}
}

// ValuationWeights represents the weights stored as JSONB in valuation_result.
type ValuationWeights struct {
	AreaSimilarityPct          int     `json:"area_similarity_pct"`
	Lambda                     float64 `json:"lambda"`
	DistanceScale              float64 `json:"distance_scale"`
	WArea                      float64 `json:"W_area"`
	WDistance                  float64 `json:"W_distance"`
	WTime                      float64 `json:"W_time"`
	WZoning                    float64 `json:"W_zoning"`
	WLandUse                   float64 `json:"W_land_use"`
	WRoad                      float64 `json:"W_road"`
	IQRK                       float64 `json:"IQR_k"`
	MinimumRequiredComparables int     `json:"minimum_required_comparables"`
	OutlierMethod              string  `json:"outlier_method"`
}

