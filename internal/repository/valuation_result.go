package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository/db"
)
// ValuationResult errors.
var (
	ErrValuationResultNotFound = fmt.Errorf("valuation result not found")
)

type ValuationResultRepository interface {
	Insert(ctx context.Context, result domain.ValuationResult) (domain.ValuationResult, error)
	GetByID(ctx context.Context, id string) (domain.ValuationResult, error)
	ListByParcel(ctx context.Context, parcelID, snapshotID string, limit int32) ([]domain.ValuationResult, error)
	GetByQueryHash(ctx context.Context, queryHash string) (domain.ValuationResult, error)
	// PersistValuation atomically inserts a valuation result and its comparables
	// in a single transaction. If any insert fails, the transaction rolls back.
	// Forces provenance validation on both valuation and comparables.
	PersistValuation(ctx context.Context, valuation domain.ValuationResult, comparables []domain.ComparableResult) (domain.ValuationResult, error)
}

func NewValuationResultRepository(dbt DBTX) ValuationResultRepository {
	return &valuationResultRepository{
		db:      dbt,
		queries: db.New(dbt),
	}
}

type valuationResultRepository struct {
	db      DBTX
	queries *db.Queries
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
// PersistValuation atomically inserts a valuation result and its comparable results.
// Uses a database transaction: if any insert fails, the transaction rolls back.
// Forces provenance validation on both valuation and comparables.
func (r *valuationResultRepository) PersistValuation(ctx context.Context, valuation domain.ValuationResult, comparables []domain.ComparableResult) (domain.ValuationResult, error) {
	// Validate valuation provenance
	if valuation.SnapshotID == "" {
		return domain.ValuationResult{}, fmt.Errorf("INVALID_ARGUMENT: missing snapshot_id")
	}
	if valuation.AlgorithmVersion == "" {
		return domain.ValuationResult{}, fmt.Errorf("INVALID_ARGUMENT: missing algorithm_version")
	}
	if valuation.ConfigurationVersion == "" {
		return domain.ValuationResult{}, fmt.Errorf("INVALID_ARGUMENT: missing configuration_version")
	}
	if valuation.TargetParcelID == "" {
		return domain.ValuationResult{}, fmt.Errorf("INVALID_ARGUMENT: missing target_parcel_id")
	}

	// Validate comparables provenance
	for i, c := range comparables {
		if c.TargetTransactionID == "" {
			return domain.ValuationResult{}, fmt.Errorf("comparable at index %d missing target_transaction_id", i)
		}
		if c.AlgorithmVersion == "" {
			return domain.ValuationResult{}, fmt.Errorf("comparable at index %d missing algorithm_version", i)
		}
	}

	var result domain.ValuationResult

	// Use pgx.BeginFunc for atomic transaction: if any insert fails, the transaction rolls back.
	beginer, ok := r.db.(interface{ Begin(ctx context.Context) (pgx.Tx, error) })
	if !ok {
		return domain.ValuationResult{}, fmt.Errorf("dbtx does not support transactions")
	}
	err := pgx.BeginFunc(ctx, beginer, func(tx pgx.Tx) error {
		txQueries := r.queries.WithTx(tx)

		// Parse UUIDs
		targetParcelID, err := parseUUID(valuation.TargetParcelID)
		if err != nil {
			return fmt.Errorf("invalid target_parcel_id: %w", err)
		}
		snapshotID, err := parseUUID(valuation.SnapshotID)
		if err != nil {
			return fmt.Errorf("invalid snapshot_id: %w", err)
		}

		comparableIDsJSON, err := json.Marshal(valuation.ComparableIDs)
		if err != nil {
			return fmt.Errorf("marshal comparable_ids: %w", err)
		}
		weightsJSON, err := json.Marshal(valuation.Weights)
		if err != nil {
			return fmt.Errorf("marshal weights: %w", err)
		}
		statsJSON, err := json.Marshal(valuation.RawStatistics)
		if err != nil {
			return fmt.Errorf("marshal statistics: %w", err)
		}

		valuationParams := db.InsertValuationResultParams{
			TargetParcelID:       targetParcelID,
			SnapshotID:           snapshotID,
			ComparableIds:        comparableIDsJSON,
			AlgorithmVersion:     valuation.AlgorithmVersion,
			ConfigurationVersion: valuation.ConfigurationVersion,
			OutlierMethod:        valuation.OutlierMethod,
			Weights:              weightsJSON,
			Statistics:           statsJSON,
			BearValue:            valuation.BearValue,
			BaseValue:            valuation.BaseValue,
			BullValue:            valuation.BullValue,
			Confidence:           string(valuation.Confidence),
			QueryHash:            valuation.QueryHash,
		}

		// Insert the valuation result first
		valuationRow, err := txQueries.InsertValuationResult(ctx, valuationParams)
		if err != nil {
			return fmt.Errorf("insert valuation result: %w", err)
		}

		// Insert comparable results, linked by target_parcel_id (same as valuation)
		for _, c := range comparables {
			targetParcelID, err := parseUUID(c.TargetTransactionID)
			if err != nil {
				return fmt.Errorf("invalid comparable target_parcel_id: %w", err)
			}
			candidateTxnID, err := parseUUID(c.CandidateTransactionID)
			if err != nil {
				return fmt.Errorf("invalid candidate transaction ID: %w", err)
			}

			distanceM, err := toNumeric(c.DistanceM)
			if err != nil {
				return fmt.Errorf("scan distance_m: %w", err)
			}
			areaSim, err := toNumeric(c.AreaSimilarity)
			if err != nil {
				return fmt.Errorf("scan area_similarity: %w", err)
			}
			timeScore, err := toNumeric(c.TimeScore)
			if err != nil {
				return fmt.Errorf("scan time_score: %w", err)
			}
			distanceScore, err := toNumeric(c.DistanceScore)
			if err != nil {
				return fmt.Errorf("scan distance_score: %w", err)
			}
			totalScore, err := toNumeric(c.TotalScore)
			if err != nil {
				return fmt.Errorf("scan total_score: %w", err)
			}

			compParams := db.InsertComparableResultParams{
				TargetParcelID:         targetParcelID,
				CandidateTransactionID: candidateTxnID,
				DistanceM:              distanceM,
				AreaSimilarity:         areaSim,
				ZoningMatch:            c.ZoningMatch,
				LandUseMatch:           c.LandUseMatch,
				RoadAccessMatch:        c.RoadAccessMatch,
				TimeScore:              timeScore,
				DistanceScore:          distanceScore,
				TotalScore:             totalScore,
				AlgorithmVersion:       c.AlgorithmVersion,
			}

			_, err = txQueries.InsertComparableResult(ctx, compParams)
			if err != nil {
				return fmt.Errorf("insert comparable result: %w", err)
			}
		}

		result = toDomainValuationResult(valuationRow)
		return nil
	})
	if err != nil {
		return domain.ValuationResult{}, fmt.Errorf("persist valuation: %w", err)
	}

	return result, nil
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

