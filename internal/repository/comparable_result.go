package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository/db"
)

// ComparableResult errors.
var (
	ErrComparableResultNotFound = fmt.Errorf("comparable result not found")
)

// ComparableResultRepository defines persistence operations for comparable_result.
// T029 acceptance criteria:
// - BatchInsert(results)
// - ListByTarget(target_parcel_id, snapshot_id)
// - GetByID(id)
type ComparableResultRepository interface {
	BatchInsert(ctx context.Context, results []domain.ComparableResult) (int64, error)
	ListByTarget(ctx context.Context, targetParcelID string, limit int32) ([]domain.ComparableResult, error)
	GetByID(ctx context.Context, id string) (domain.ComparableResult, error)
	GetByQueryHash(ctx context.Context, queryHash string) (domain.ComparableResult, error)
}

type comparableResultRepository struct {
	queries *db.Queries
}

// NewComparableResultRepository creates a repository backed by pgx + sqlc.
func NewComparableResultRepository(dbt DBTX) ComparableResultRepository {
	return &comparableResultRepository{
		queries: db.New(dbt),
	}
}

// BatchInsert inserts comparable results in a batch.
// Forces provenance: target_parcel_id, algorithm_version must be set.
func (r *comparableResultRepository) BatchInsert(ctx context.Context, results []domain.ComparableResult) (int64, error) {
	if len(results) == 0 {
		return 0, nil
	}

	// Validate required provenance fields
	for i, res := range results {
		if res.TargetTransactionID == "" {
			return 0, fmt.Errorf("comparable result at index %d missing target_transaction_id", i)
		}
		if res.AlgorithmVersion == "" {
			return 0, fmt.Errorf("comparable result at index %d missing algorithm_version", i)
		}
	}

	var inserted int64
	for _, res := range results {
		// Parse UUIDs
		targetParcelID, err := parseUUID(res.TargetTransactionID)
		if err != nil {
			return inserted, fmt.Errorf("invalid target parcel ID: %w", err)
		}
		candidateTxnID, err := parseUUID(res.CandidateTransactionID)
		if err != nil {
			return inserted, fmt.Errorf("invalid candidate transaction ID: %w", err)
		}

		// Convert floats to pgtype.Numeric
		distanceM, err := toNumeric(res.DistanceM)
		if err != nil {
			return inserted, fmt.Errorf("scan distance_m: %w", err)
		}
		areaSim, err := toNumeric(res.AreaSimilarity)
		if err != nil {
			return inserted, fmt.Errorf("scan area_similarity: %w", err)
		}
		timeScore, err := toNumeric(res.TimeScore)
		if err != nil {
			return inserted, fmt.Errorf("scan time_score: %w", err)
		}
		distanceScore, err := toNumeric(res.DistanceScore)
		if err != nil {
			return inserted, fmt.Errorf("scan distance_score: %w", err)
		}
		totalScore, err := toNumeric(res.TotalScore)
		if err != nil {
			return inserted, fmt.Errorf("scan total_score: %w", err)
		}

		params := db.InsertComparableResultParams{
			TargetParcelID:         targetParcelID,
			CandidateTransactionID: candidateTxnID,
			DistanceM:              distanceM,
			AreaSimilarity:         areaSim,
			ZoningMatch:            res.ZoningMatch,
			LandUseMatch:           res.LandUseMatch,
			RoadAccessMatch:        res.RoadAccessMatch,
			TimeScore:              timeScore,
			DistanceScore:          distanceScore,
			TotalScore:             totalScore,
			AlgorithmVersion:       res.AlgorithmVersion,
		}

		_, err = r.queries.InsertComparableResult(ctx, params)
		if err != nil {
			return inserted, fmt.Errorf("insert comparable result: %w", err)
		}
		inserted++
	}

	return inserted, nil
}

// ListByTarget lists comparable results for a target parcel,
// ordered by total_score DESC, distance_m ASC, candidate_transaction_id ASC (deterministic).
func (r *comparableResultRepository) ListByTarget(ctx context.Context, targetParcelID string, limit int32) ([]domain.ComparableResult, error) {
	if limit <= 0 {
		limit = 100
	}

	parcelID, err := parseUUID(targetParcelID)
	if err != nil {
		return nil, fmt.Errorf("invalid target parcel ID: %w", err)
	}

	rows, err := r.queries.ListComparableResults(ctx, db.ListComparableResultsParams{
		TargetParcelID: parcelID,
		Limit:          limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list comparable results: %w", err)
	}

	return toDomainComparableResults(rows), nil
}

// GetByID fetches a comparable result by UUID.
func (r *comparableResultRepository) GetByID(ctx context.Context, id string) (domain.ComparableResult, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return domain.ComparableResult{}, fmt.Errorf("invalid ID: %w", err)
	}

	row, err := r.queries.GetComparableResultByID(ctx, uid)
	if err != nil {
		return domain.ComparableResult{}, fmt.Errorf("get comparable result: %w", err)
	}

	return toDomainComparableResult(row), nil
}

// GetByQueryHash fetches a comparable result by query hash.
// comparable_result does not have a query_hash column; returns ErrComparableResultNotFound.
func (r *comparableResultRepository) GetByQueryHash(ctx context.Context, queryHash string) (domain.ComparableResult, error) {
	return domain.ComparableResult{}, ErrComparableResultNotFound
}

// toDomainComparableResult converts a db.ComparableResult to domain.ComparableResult.
func toDomainComparableResult(row db.ComparableResult) domain.ComparableResult {
	return domain.ComparableResult{
		ID:                    uuidToString(row.ID),
		TargetTransactionID:     uuidToString(row.TargetParcelID),
		CandidateTransactionID: uuidToString(row.CandidateTransactionID),
		DistanceM:             fromNumeric(row.DistanceM),
		AreaSimilarity:        fromNumeric(row.AreaSimilarity),
		ZoningMatch:           row.ZoningMatch,
		LandUseMatch:          row.LandUseMatch,
		RoadAccessMatch:       row.RoadAccessMatch,
		TimeScore:             fromNumeric(row.TimeScore),
		DistanceScore:         fromNumeric(row.DistanceScore),
		TotalScore:            fromNumeric(row.TotalScore),
		AlgorithmVersion:      row.AlgorithmVersion,
		CreatedAt:             row.CreatedAt.Time,
	}
}

// toDomainComparableResults converts a slice of db.ComparableResult to domain.
func toDomainComparableResults(rows []db.ComparableResult) []domain.ComparableResult {
	out := make([]domain.ComparableResult, len(rows))
	for i, row := range rows {
		out[i] = toDomainComparableResult(row)
	}
	return out
}

// toNumeric converts float64 to pgtype.Numeric via string scan.
func toNumeric(f float64) (pgtype.Numeric, error) {
	if f == 0 {
		return pgtype.Numeric{}, nil
	}
	var n pgtype.Numeric
	err := n.Scan(fmt.Sprintf("%f", f))
	return n, err
}

// fromNumeric converts pgtype.Numeric to float64.
func fromNumeric(n pgtype.Numeric) float64 {
	f, err := numericToFloat64(n)
	if err != nil {
		return 0
	}
	return f
}
