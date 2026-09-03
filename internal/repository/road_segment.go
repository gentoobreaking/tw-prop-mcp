package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository/db"
)

// RoadSegmentRepository defines persistence operations for road segments
type RoadSegmentRepository interface {
	Create(ctx context.Context, arg domain.RoadSegmentCreateParams) (domain.RoadSegment, error)
	GetByID(ctx context.Context, id string) (domain.RoadSegment, error)
	GetByName(ctx context.Context, name string) ([]domain.RoadSegment, error)
	Search(ctx context.Context, filter domain.RoadSegmentFilter) ([]domain.RoadSegment, error)
	BatchInsert(ctx context.Context, roads []domain.RoadSegment) (int64, error)
}

// roadSegmentRepository implements RoadSegmentRepository
type roadSegmentRepository struct {
	queries *db.Queries
	db      DBTX
}

const roadBatchInsertSize = 256

// NewRoadSegmentRepository creates a repository backed by pgx + sqlc
func NewRoadSegmentRepository(dbt DBTX) RoadSegmentRepository {
	return &roadSegmentRepository{
		queries: db.New(dbt),
		db:      dbt,
	}
}

// Create inserts a new road segment row
func (r *roadSegmentRepository) Create(ctx context.Context, arg domain.RoadSegmentCreateParams) (domain.RoadSegment, error) {
	if arg.WidthSource != "" && arg.WidthSource != domain.WidthSourceOfficial && arg.WidthSource != domain.WidthSourceGISDerived && arg.WidthSource != domain.WidthSourceUnknown {
		return domain.RoadSegment{}, fmt.Errorf("invalid width_source: %s", arg.WidthSource)
	}

	var widthM pgtype.Numeric
	if arg.WidthM != nil {
		widthM = pgtype.Numeric{Valid: true}
		if err := widthM.Scan(fmt.Sprintf("%f", *arg.WidthM)); err != nil {
			return domain.RoadSegment{}, fmt.Errorf("width_m: %w", err)
		}
	}

	var importBatchID pgtype.UUID
	if arg.ImportBatchID != "" {
		uid, err := parseUUID(arg.ImportBatchID)
		if err != nil {
			return domain.RoadSegment{}, fmt.Errorf("import_batch_id: %w", err)
		}
		importBatchID = uid
	}

	row, err := r.queries.CreateRoadSegment(ctx, db.CreateRoadSegmentParams{
		Name:          pgtype.Text{String: arg.Name, Valid: true},
		RoadClass:     pgtype.Text{String: arg.RoadClass, Valid: true},
		WidthM:        widthM,
		WidthSource:   arg.WidthSource,
		Geometry:      arg.Geometry,
		Source:        arg.Source,
		SourceVersion: arg.SourceVersion,
		ImportBatchID: importBatchID,
	})
	if err != nil {
		return domain.RoadSegment{}, err
	}
	return toDomainRoadSegment(row), nil
}

// GetByID fetches a road segment by UUID
func (r *roadSegmentRepository) GetByID(ctx context.Context, id string) (domain.RoadSegment, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return domain.RoadSegment{}, err
	}
	row, err := r.queries.GetRoadSegmentByID(ctx, uid)
	if err != nil {
		return domain.RoadSegment{}, err
	}
	return toDomainRoadSegment(row), nil
}

// GetByName fetches road segments by name
func (r *roadSegmentRepository) GetByName(ctx context.Context, name string) ([]domain.RoadSegment, error) {
	rows, err := r.queries.GetRoadSegmentsByName(ctx, pgtype.Text{String: name, Valid: true})
	if err != nil {
		return nil, err
	}
	result := make([]domain.RoadSegment, len(rows))
	for i, row := range rows {
		result[i] = toDomainRoadSegment(row)
	}
	return result, nil
}

// Search returns road segments matching the filter
func (r *roadSegmentRepository) Search(ctx context.Context, filter domain.RoadSegmentFilter) ([]domain.RoadSegment, error) {
	rows, err := r.queries.ListRoadSegments(ctx, db.ListRoadSegmentsParams{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]domain.RoadSegment, len(rows))
	for i, row := range rows {
		result[i] = toDomainRoadSegment(row)
	}
	return result, nil
}

// BatchInsert inserts road segments in batches using COPY FROM
func (r *roadSegmentRepository) BatchInsert(ctx context.Context, roads []domain.RoadSegment) (int64, error) {
	if len(roads) == 0 {
		return 0, nil
	}

	batchSize := 256
	var total int64

	for i := 0; i < len(roads); i += batchSize {
		end := i + batchSize
		if end > len(roads) {
			end = len(roads)
		}

		chunk := roads[i:end]
		params := make([]db.BatchInsertRoadSegmentsParams, 0, len(chunk))
		for j := range chunk {
			p, err := toBatchInsertRoadParam(chunk[j])
			if err != nil {
				return total, fmt.Errorf("batch insert at row %d: %w", i+j, err)
			}
			params = append(params, p)
		}

		n, err := r.queries.BatchInsertRoadSegments(ctx, params)
		if err != nil {
			return total, fmt.Errorf("batch insert rows %d-%d: %w", i, end, err)
		}
		total += n
	}
	return total, nil
}

// toDomainRoadSegment converts a db.RoadSegment row to domain.RoadSegment
func toDomainRoadSegment(row db.RoadSegment) domain.RoadSegment {
	return domain.RoadSegment{
		ID:            row.ID.String(),
		Name:          row.Name.String,
		RoadClass:     row.RoadClass.String,
		WidthM:        numericToFloat64Ptr(row.WidthM),
		WidthSource:   row.WidthSource,
		Geometry:      row.Geometry,
		Source:        row.Source,
		SourceVersion: row.SourceVersion,
		ImportBatchID: row.ImportBatchID.String(),
		CreatedAt:     row.CreatedAt.Time,
	}
}

// numericToFloat64Ptr converts pgtype.Numeric to *float64
func numericToFloat64Ptr(n pgtype.Numeric) *float64 {
	if n.Valid {
		f, _ := n.Float64Value()
		v := f.Float64
		return &v
	}
	return nil
}

// toBatchInsertRoadParam converts domain.RoadSegment to db.BatchInsertRoadSegmentsParams
func toBatchInsertRoadParam(r domain.RoadSegment) (db.BatchInsertRoadSegmentsParams, error) {
	id, err := parseUUID(r.ID)
	if err != nil {
		return db.BatchInsertRoadSegmentsParams{}, fmt.Errorf("id: %w", err)
	}

	var widthM pgtype.Numeric
	if r.WidthM != nil {
		widthM = pgtype.Numeric{Valid: true}
		if err := widthM.Scan(fmt.Sprintf("%f", *r.WidthM)); err != nil {
			return db.BatchInsertRoadSegmentsParams{}, fmt.Errorf("width_m: %w", err)
		}
	}

	var importBatchID pgtype.UUID
	if r.ImportBatchID != "" {
		uid, err := parseUUID(r.ImportBatchID)
		if err != nil {
			return db.BatchInsertRoadSegmentsParams{}, fmt.Errorf("import_batch_id: %w", err)
		}
		importBatchID = uid
	}

	return db.BatchInsertRoadSegmentsParams{
		ID:             id,
		Name:           pgtype.Text{String: r.Name, Valid: true},
		RoadClass:      pgtype.Text{String: r.RoadClass, Valid: true},
		WidthM:         widthM,
		WidthSource:    r.WidthSource,
		Geometry:       r.Geometry,
		Source:         r.Source,
		SourceVersion:  r.SourceVersion,
		ImportBatchID:  importBatchID,
	}, nil
}