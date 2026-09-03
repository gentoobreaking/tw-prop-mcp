package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository/db"
)

// ParcelRoadAccessRepository defines persistence operations for parcel road access
type ParcelRoadAccessRepository interface {
	Create(ctx context.Context, arg domain.ParcelRoadAccessCreateParams) (domain.ParcelRoadAccess, error)
	GetByID(ctx context.Context, id string) (domain.ParcelRoadAccess, error)
	GetByParcelID(ctx context.Context, parcelID string) (domain.ParcelRoadAccess, error)
	Search(ctx context.Context, filter domain.ParcelRoadAccessFilter) ([]domain.ParcelRoadAccess, error)
	BatchInsert(ctx context.Context, records []domain.ParcelRoadAccess) (int64, error)
	Delete(ctx context.Context, id string) error
}

// parcelRoadAccessRepository implements ParcelRoadAccessRepository
type parcelRoadAccessRepository struct {
	queries *db.Queries
	db      DBTX
}

const parcelRoadAccessBatchInsertSize = 256

// NewParcelRoadAccessRepository creates a repository backed by pgx + sqlc
func NewParcelRoadAccessRepository(dbt DBTX) ParcelRoadAccessRepository {
	return &parcelRoadAccessRepository{
		queries: db.New(dbt),
		db:      dbt,
	}
}

// Create inserts a new parcel road access row
func (r *parcelRoadAccessRepository) Create(ctx context.Context, arg domain.ParcelRoadAccessCreateParams) (domain.ParcelRoadAccess, error) {
	if err := domain.ValidateAccessType(arg.AccessType); err != nil {
		return domain.ParcelRoadAccess{}, err
	}

	parcelUID, err := parseUUID(arg.ParcelID)
	if err != nil {
		return domain.ParcelRoadAccess{}, err
	}

	var roadUID pgtype.UUID
	if arg.RoadID != "" {
		uid, err := parseUUID(arg.RoadID)
		if err != nil {
			return domain.ParcelRoadAccess{}, err
		}
		roadUID = uid
	}

	var nearestPoint interface{}
	if arg.NearestPoint != "" {
		nearestPoint = arg.NearestPoint
	}

	var roadWidthM pgtype.Numeric
	if arg.RoadWidthM != nil {
		roadWidthM = pgtype.Numeric{Valid: true}
		if err := roadWidthM.Scan(fmt.Sprintf("%f", *arg.RoadWidthM)); err != nil {
			return domain.ParcelRoadAccess{}, fmt.Errorf("road_width_m: %w", err)
		}
	}

	var distanceM pgtype.Numeric
	distanceM.Scan(fmt.Sprintf("%f", arg.DistanceM))

	row, err := r.queries.CreateParcelRoadAccess(ctx, db.CreateParcelRoadAccessParams{
		ParcelID:         parcelUID,
		RoadID:           roadUID,
		DistanceM:        distanceM,
		NearestPoint:     nearestPoint,
		RoadWidthM:       roadWidthM,
		AccessType:       arg.AccessType,
		Source:           arg.Source,
		AlgorithmVersion: arg.AlgorithmVersion,
	})
	if err != nil {
		return domain.ParcelRoadAccess{}, err
	}
	return toDomainParcelRoadAccess(row), nil
}

// GetByID fetches a parcel road access by UUID
func (r *parcelRoadAccessRepository) GetByID(ctx context.Context, id string) (domain.ParcelRoadAccess, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return domain.ParcelRoadAccess{}, err
	}
	row, err := r.queries.GetParcelRoadAccessByID(ctx, uid)
	if err != nil {
		return domain.ParcelRoadAccess{}, err
	}
	return toDomainParcelRoadAccess(row), nil
}

// GetByParcelID fetches parcel road access by parcel ID
func (r *parcelRoadAccessRepository) GetByParcelID(ctx context.Context, parcelID string) (domain.ParcelRoadAccess, error) {
	uid, err := parseUUID(parcelID)
	if err != nil {
		return domain.ParcelRoadAccess{}, err
	}
	row, err := r.queries.GetParcelRoadAccessByParcelID(ctx, uid)
	if err != nil {
		return domain.ParcelRoadAccess{}, err
	}
	return toDomainParcelRoadAccess(row), nil
}

// Search returns parcel road access records matching the filter
func (r *parcelRoadAccessRepository) Search(ctx context.Context, filter domain.ParcelRoadAccessFilter) ([]domain.ParcelRoadAccess, error) {
	rows, err := r.queries.ListParcelRoadAccess(ctx, db.ListParcelRoadAccessParams{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]domain.ParcelRoadAccess, len(rows))
	for i, row := range rows {
		result[i] = toDomainParcelRoadAccess(row)
	}
	return result, nil
}

// BatchInsert inserts parcel road access records in batches using COPY FROM
func (r *parcelRoadAccessRepository) BatchInsert(ctx context.Context, records []domain.ParcelRoadAccess) (int64, error) {
	if len(records) == 0 {
		return 0, nil
	}

	var total int64

	for i := 0; i < len(records); i += 256 {
		end := i + 256
		if end > len(records) {
			end = len(records)
		}

		chunk := records[i:end]
		params := make([]db.BatchInsertParcelRoadAccessParams, 0, len(chunk))
		for j := range chunk {
			p, err := toBatchInsertParcelRoadAccessParam(chunk[j])
			if err != nil {
				return total, fmt.Errorf("batch insert at row %d: %w", i+j, err)
			}
			params = append(params, p)
		}

		n, err := r.queries.BatchInsertParcelRoadAccess(ctx, params)
		if err != nil {
			return total, fmt.Errorf("batch insert rows %d-%d: %w", i, end, err)
		}
		total += n
	}
	return total, nil
}

// Delete removes a parcel road access record
func (r *parcelRoadAccessRepository) Delete(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	_ = r.queries.DeleteParcelRoadAccess(ctx, uid)
	return nil
}

// toDomainParcelRoadAccess converts a db.ParcelRoadAccess row to domain.ParcelRoadAccess
func toDomainParcelRoadAccess(row db.ParcelRoadAccess) domain.ParcelRoadAccess {
	var roadID string
	if row.RoadID.Valid {
		roadID = row.RoadID.String()
	}
	var nearestPoint string
	if np, ok := row.NearestPoint.(string); ok {
		nearestPoint = np
	}
	var roadWidthM *float64
	if row.RoadWidthM.Valid {
		f, _ := row.RoadWidthM.Float64Value()
		roadWidthM = &f.Float64
	}
	var distanceM float64
	if row.DistanceM.Valid {
		d, _ := row.DistanceM.Float64Value()
		distanceM = d.Float64
	}
	var computedAt time.Time
	if row.ComputedAt.Valid {
		computedAt = row.ComputedAt.Time
	}

	return domain.ParcelRoadAccess{
		ID:              row.ID.String(),
		ParcelID:        row.ParcelID.String(),
		RoadID:          roadID,
		DistanceM:       distanceM,
		NearestPoint:    nearestPoint,
		RoadWidthM:      roadWidthM,
		AccessType:      row.AccessType,
		Source:          row.Source,
		AlgorithmVersion: row.AlgorithmVersion,
		ComputedAt:      computedAt,
	}
}

// toBatchInsertParcelRoadAccessParam converts domain.ParcelRoadAccess to db.BatchInsertParcelRoadAccessParams
func toBatchInsertParcelRoadAccessParam(r domain.ParcelRoadAccess) (db.BatchInsertParcelRoadAccessParams, error) {
	parcelUID, err := parseUUID(r.ParcelID)
	if err != nil {
		return db.BatchInsertParcelRoadAccessParams{}, fmt.Errorf("parcel_id: %w", err)
	}

	var roadUID pgtype.UUID
	if r.RoadID != "" {
		uid, err := parseUUID(r.RoadID)
		if err != nil {
			return db.BatchInsertParcelRoadAccessParams{}, err
		}
		roadUID = uid
	}

	var distanceM pgtype.Numeric
	distanceM.Scan(fmt.Sprintf("%f", r.DistanceM))

	var roadWidthM pgtype.Numeric
	if r.RoadWidthM != nil {
		roadWidthM = pgtype.Numeric{Valid: true}
		if err := roadWidthM.Scan(fmt.Sprintf("%f", *r.RoadWidthM)); err != nil {
			return db.BatchInsertParcelRoadAccessParams{}, fmt.Errorf("road_width_m: %w", err)
		}
	}

	return db.BatchInsertParcelRoadAccessParams{
		ParcelID:         parcelUID,
		RoadID:           roadUID,
		DistanceM:        distanceM,
		NearestPoint:     r.NearestPoint,
		RoadWidthM:       roadWidthM,
		AccessType:       r.AccessType,
		Source:           r.Source,
		AlgorithmVersion: r.AlgorithmVersion,
	}, nil
}