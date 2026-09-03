package gis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
)

// GeometryEngine performs all spatial computations via PostGIS queries.
// Every method accepts WKT in EPSG:3826 (internal).  EWKT input with an
// "SRID=3826;" prefix is automatically stripped.
type GeometryEngine struct {
	db DBQuerier
}

// NewGeometryEngine creates a GeometryEngine backed by the given DBQuerier.
func NewGeometryEngine(db DBQuerier) *GeometryEngine {
	return &GeometryEngine{db: db}
}

// ST_Intersects reports whether two geometries in EPSG:3826 intersect.
func (e *GeometryEngine) ST_Intersects(ctx context.Context, a, b string) (bool, error) {
	return e.boolQuery(ctx, "ST_Intersects", a, b)
}

// ST_Within reports whether geometry *a* is completely inside geometry *b*.
func (e *GeometryEngine) ST_Within(ctx context.Context, a, b string) (bool, error) {
	return e.boolQuery(ctx, "ST_Within", a, b)
}

// ST_Contains reports whether geometry *a* completely contains geometry *b*.
func (e *GeometryEngine) ST_Contains(ctx context.Context, a, b string) (bool, error) {
	return e.boolQuery(ctx, "ST_Contains", a, b)
}

// ST_DWithin reports whether two geometries are within *distanceMeters*
// (measured in EPSG:3826, a metre-based projection) of each other.
func (e *GeometryEngine) ST_DWithin(ctx context.Context, a, b string, distanceMeters float64) (bool, error) {
	const q = `SELECT ST_DWithin(
		ST_SetSRID(ST_GeomFromText($1), 3826),
		ST_SetSRID(ST_GeomFromText($2), 3826),
		$3
	)`
	var result bool
	err := e.db.QueryRow(ctx, q, stripSRID(a), stripSRID(b), distanceMeters).Scan(&result)
	if err != nil {
		return false, fmt.Errorf("ST_DWithin: %w", err)
	}
	return result, nil
}

// ST_Distance returns the 2-D distance between two geometries in metres
// (EPSG:3826 is metre-based).  Returns 0 for identical geometries.
func (e *GeometryEngine) ST_Distance(ctx context.Context, a, b string) (float64, error) {
	const q = `SELECT ST_Distance(
		ST_SetSRID(ST_GeomFromText($1), 3826),
		ST_SetSRID(ST_GeomFromText($2), 3826)
	)`
	var result float64
	err := e.db.QueryRow(ctx, q, stripSRID(a), stripSRID(b)).Scan(&result)
	if err != nil {
		return 0, fmt.Errorf("ST_Distance: %w", err)
	}
	return result, nil
}

// ST_Area returns the area of a polygon/multipolygon in square metres.
func (e *GeometryEngine) ST_Area(ctx context.Context, wkt string) (float64, error) {
	const q = `SELECT ST_Area(ST_SetSRID(ST_GeomFromText($1), 3826))`
	var result float64
	err := e.db.QueryRow(ctx, q, stripSRID(wkt)).Scan(&result)
	if err != nil {
		return 0, fmt.Errorf("ST_Area: %w", err)
	}
	return result, nil
}

// ST_Centroid returns the centroid of a geometry, transformed to
// EPSG:4326 (lon, lat).  lon/lat are returned in WGS84 coordinates.
func (e *GeometryEngine) ST_Centroid(ctx context.Context, wkt string) (lon, lat float64, err error) {
	const q = `SELECT
		ST_X(ST_Transform(ST_Centroid(ST_SetSRID(ST_GeomFromText($1), 3826)), 4326)),
		ST_Y(ST_Transform(ST_Centroid(ST_SetSRID(ST_GeomFromText($1), 3826)), 4326))
	)`
	err = e.db.QueryRow(ctx, q, stripSRID(wkt)).Scan(&lon, &lat)
	if err != nil {
		return 0, 0, fmt.Errorf("ST_Centroid: %w", err)
	}
	return lon, lat, nil
}

// boolQuery is a shared helper for the ST_Intersects / ST_Within / ST_Contains
// methods that return a single boolean.
func (e *GeometryEngine) boolQuery(ctx context.Context, fn, a, b string) (bool, error) {
	q := fmt.Sprintf(`SELECT %s(
		ST_SetSRID(ST_GeomFromText($1), 3826),
		ST_SetSRID(ST_GeomFromText($2), 3826)
	)`, fn)
	var result bool
	err := e.db.QueryRow(ctx, q, stripSRID(a), stripSRID(b)).Scan(&result)
	if err != nil {
		return false, fmt.Errorf("%s: %w", fn, err)
	}
	return result, nil
}

// parcelGeometryRow is the scan target for GetParcelGeometryWithCentroid.
type parcelGeometryRow struct {
	ID              string    `db:"id"`
	County          string    `db:"county"`
	District        string    `db:"district"`
	Section         string    `db:"section"`
	LandNumber      string    `db:"land_number"`
	AreaSqm         float64   `db:"area_sqm"`
	UrbanZoning     *string   `db:"urban_zoning"`
	LandUseCategory *string   `db:"land_use_category"`
	Geometry        string    `db:"geometry"`
	Centroid        *string   `db:"centroid"`
	BBox            *string   `db:"bbox"`
	Geometry4326    *string   `db:"geometry_4326"`
	Centroid4326    *string   `db:"centroid_4326"`
	BBox4326        *string   `db:"bbox_4326"`
	Source          string    `db:"source"`
	SourceVersion   string    `db:"source_version"`
	ImportBatchID   *string   `db:"import_batch_id"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// GetParcelGeometryWithCentroid fetches a parcel by UUID and returns it as a
// domain.Parcel with geometry columns exposed in both EPSG:3826 (internal)
// and EPSG:4326 (external, via PostGIS ST_Transform).
func (e *GeometryEngine) GetParcelGeometryWithCentroid(ctx context.Context, id string) (*domain.Parcel, error) {
	const q = `SELECT
		p.id, p.county, p.district, p.section, p.land_number, p.area_sqm,
		p.urban_zoning, p.land_use_category,
		ST_AsText(p.geometry)                 AS geometry,
		ST_AsText(p.centroid)                 AS centroid,
		ST_AsText(p.bbox)                     AS bbox,
		ST_AsText(ST_Transform(p.geometry, 4326))  AS geometry_4326,
		ST_AsText(ST_Transform(p.centroid, 4326))  AS centroid_4326,
		ST_AsText(ST_Transform(p.bbox, 4326))      AS bbox_4326,
		p.source, p.source_version, p.import_batch_id,
		p.created_at, p.updated_at
	FROM parcel p
	WHERE p.id = $1::uuid`

	var row parcelGeometryRow
	err := e.db.QueryRow(ctx, q, id).Scan(
		&row.ID, &row.County, &row.District, &row.Section, &row.LandNumber, &row.AreaSqm,
		&row.UrbanZoning, &row.LandUseCategory,
		&row.Geometry, &row.Centroid, &row.BBox,
		&row.Geometry4326, &row.Centroid4326, &row.BBox4326,
		&row.Source, &row.SourceVersion, &row.ImportBatchID,
		&row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("GetParcelGeometryWithCentroid: %w", err)
	}

	// Convert UUID bytes to canonical string form if needed.
	idStr := row.ID
	if !strings.Contains(idStr, "-") {
		// Could be raw bytes; attempt pgtype.UUID decode
		var u pgtype.UUID
		_ = u.Scan(row.ID)
		if u.Valid {
			idStr = fmt.Sprintf("%x-%x-%x-%x-%x", u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:16])
		}
	}

	return &domain.Parcel{
		ID:              idStr,
		County:          row.County,
		District:        row.District,
		Section:         row.Section,
		LandNumber:      row.LandNumber,
		AreaSqm:         row.AreaSqm,
		UrbanZoning:     derefString(row.UrbanZoning),
		LandUseCategory: derefString(row.LandUseCategory),
		Geometry:        row.Geometry,
		Centroid:        derefString(row.Centroid),
		BBox:            derefString(row.BBox),
		Geometry4326:    derefString(row.Geometry4326),
		Centroid4326:    derefString(row.Centroid4326),
		BBox4326:        derefString(row.BBox4326),
		Source:          row.Source,
		SourceVersion:   row.SourceVersion,
		ImportBatchID:   derefString(row.ImportBatchID),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
