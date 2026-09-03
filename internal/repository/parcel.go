package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository/db"
)

// Parcel repository errors.
var (
	ErrParcelExists   = errors.New("parcel already exists")
	ErrParcelNotFound = errors.New("parcel not found")
)

// ParcelFilter defines search criteria for parcels.
// County and District are required; other fields are optional.
type ParcelFilter struct {
	County   string
	District string
	Section  *string
	MinArea  *float64
	MaxArea  *float64
	Limit    int32
	Offset   int32
}

// ParcelRepository defines persistence operations for parcel.
type ParcelRepository interface {
	Create(ctx context.Context, p *domain.Parcel) error
	GetByID(ctx context.Context, id string) (*domain.Parcel, error)
	GetByLandNumber(ctx context.Context, county, district, section, landNumber string) (*domain.Parcel, error)
	Search(ctx context.Context, filter ParcelFilter) ([]*domain.Parcel, error)
	// GetGeometry4326 fetches the 4326-transformed WKT for a parcel (geometry, centroid, bbox)
	GetGeometry4326(ctx context.Context, id string) (geom4326, centroid4326, bbox4326 string, err error)
}

type parcelRepository struct {
	queries *db.Queries
	db      DBTX
}

// NewParcelRepository creates a repository backed by pgx + sqlc.
func NewParcelRepository(dbt DBTX) ParcelRepository {
	return &parcelRepository{
		queries: db.New(dbt),
		db:      dbt,
	}
}

// Create inserts a new parcel. Geometry is stored as EPSG:3826 via ST_GeomFromText.
// Unique violation on (county,district,section,land_number,source,source_version) => ErrParcelExists.
func (r *parcelRepository) Create(ctx context.Context, p *domain.Parcel) error {
	if p.County == "" || p.District == "" || p.Section == "" || p.LandNumber == "" {
		return fmt.Errorf("parcel missing required location fields")
	}
	if p.Geometry == "" {
		return fmt.Errorf("parcel geometry is required")
	}
	if p.AreaSqm <= 0 {
		return fmt.Errorf("parcel area_sqm must be >0")
	}
	if p.Source == "" || p.SourceVersion == "" {
		return fmt.Errorf("parcel source and source_version are required")
	}

	// Prepare numeric area
	var area pgtype.Numeric
	if err := area.Scan(fmt.Sprintf("%f", p.AreaSqm)); err != nil {
		return fmt.Errorf("scan area_sqm: %w", err)
	}

	var urbanZoning pgtype.Text
	if p.UrbanZoning != "" {
		urbanZoning = pgtype.Text{String: p.UrbanZoning, Valid: true}
	}
	var landUse pgtype.Text
	if p.LandUseCategory != "" {
		landUse = pgtype.Text{String: p.LandUseCategory, Valid: true}
	}

	// Handle ID: if empty, let DB generate; else parse
	var idParam string
	if p.ID != "" {
		if _, err := parseUUID(p.ID); err != nil {
			return fmt.Errorf("invalid parcel id: %w", err)
		}
		idParam = p.ID
	}

	// ImportBatchID nullable uuid
	var ibID pgtype.UUID
	if p.ImportBatchID != "" {
		u, err := parseUUID(p.ImportBatchID)
		if err != nil {
			return fmt.Errorf("invalid import_batch_id: %w", err)
		}
		ibID = u
	}

	// Geometry strings: strip SRID prefix if present for ST_GeomFromText; we pass WKT and SRID separately.
	geomWKT := stripSRID(p.Geometry)
	centroidWKT := stripSRID(p.Centroid)
	bboxWKT := stripSRID(p.BBox)

	// Use a single INSERT ... RETURNING to get the new row.
	// We compute centroid/bbox via PostGIS if not provided.
	// Use ST_Multi to ensure MultiPolygon type.
	query := `
INSERT INTO parcel (id, county, district, section, land_number, area_sqm, urban_zoning, land_use_category, geometry, centroid, bbox, source, source_version, import_batch_id)
VALUES (
  COALESCE(NULLIF($1::text,'')::uuid, gen_random_uuid()),
  $2, $3, $4, $5, $6, $7, $8,
  ST_Multi(ST_GeomFromText($9, 3826)),
  CASE WHEN $10::text <> '' THEN ST_GeomFromText($10, 3826) ELSE ST_Centroid(ST_Multi(ST_GeomFromText($9, 3826))) END,
  CASE WHEN $11::text <> '' THEN ST_GeomFromText($11, 3826) ELSE ST_Envelope(ST_Multi(ST_GeomFromText($9, 3826))) END,
  $12, $13, $14
)
RETURNING id, county, district, section, land_number, area_sqm, urban_zoning, land_use_category,
          ST_AsText(geometry) as geometry, ST_AsText(centroid) as centroid, ST_AsText(bbox) as bbox,
          source, source_version, import_batch_id, created_at, updated_at,
          ST_AsText(ST_Transform(geometry,4326)) as geom4326,
          ST_AsText(ST_Transform(centroid,4326)) as centroid4326,
          ST_AsText(ST_Transform(bbox,4326)) as bbox4326
`

	var row struct {
		ID            pgtype.UUID
		County        string
		District      string
		Section       string
		LandNumber    string
		AreaSqm       pgtype.Numeric
		UrbanZoning   pgtype.Text
		LandUse       pgtype.Text
		Geometry      string
		Centroid      *string
		BBox          *string
		Source        string
		SourceVersion string
		ImportBatchID pgtype.UUID
		CreatedAt     pgtype.Timestamptz
		UpdatedAt     pgtype.Timestamptz
		Geom4326      *string
		Centroid4326  *string
		BBox4326      *string
	}

	err := r.db.QueryRow(ctx, query,
		idParam,
		p.County, p.District, p.Section, p.LandNumber,
		area,
		urbanZoning, landUse,
		geomWKT,
		centroidWKT,
		bboxWKT,
		p.Source, p.SourceVersion,
		ibID,
	).Scan(
		&row.ID, &row.County, &row.District, &row.Section, &row.LandNumber,
		&row.AreaSqm, &row.UrbanZoning, &row.LandUse,
		&row.Geometry, &row.Centroid, &row.BBox,
		&row.Source, &row.SourceVersion, &row.ImportBatchID,
		&row.CreatedAt, &row.UpdatedAt,
		&row.Geom4326, &row.Centroid4326, &row.BBox4326,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrParcelExists
		}
		// Also handle string contains for unique violation fallback
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return ErrParcelExists
		}
		return err
	}

	// Populate back to domain object
	p.ID = uuidToString(row.ID)
	p.County = row.County
	p.District = row.District
	p.Section = row.Section
	p.LandNumber = row.LandNumber
	if v, err := numericToFloat64(row.AreaSqm); err == nil {
		p.AreaSqm = v
	}
	if row.UrbanZoning.Valid {
		p.UrbanZoning = row.UrbanZoning.String
	}
	if row.LandUse.Valid {
		p.LandUseCategory = row.LandUse.String
	}
	p.Geometry = row.Geometry
	if row.Centroid != nil {
		p.Centroid = *row.Centroid
	}
	if row.BBox != nil {
		p.BBox = *row.BBox
	}
	p.Source = row.Source
	p.SourceVersion = row.SourceVersion
	if row.ImportBatchID.Valid {
		p.ImportBatchID = uuidToString(row.ImportBatchID)
	}
	if row.CreatedAt.Valid {
		p.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		p.UpdatedAt = row.UpdatedAt.Time
	}
	if row.Geom4326 != nil {
		p.Geometry4326 = *row.Geom4326
	}
	if row.Centroid4326 != nil {
		p.Centroid4326 = *row.Centroid4326
	}
	if row.BBox4326 != nil {
		p.BBox4326 = *row.BBox4326
	}
	return nil
}

// GetByID fetches a parcel by UUID string.
func (r *parcelRepository) GetByID(ctx context.Context, id string) (*domain.Parcel, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetParcelByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrParcelNotFound
		}
		return nil, err
	}
	p := toParcelDomain(row)
	// Populate 4326 via auxiliary query (best effort)
	if geom4326, centroid4326, bbox4326, err := r.GetGeometry4326(ctx, id); err == nil {
		p.Geometry4326 = geom4326
		p.Centroid4326 = centroid4326
		p.BBox4326 = bbox4326
	}
	return p, nil
}

// GetByLandNumber fetches a parcel by its natural key (county,district,section,land_number).
func (r *parcelRepository) GetByLandNumber(ctx context.Context, county, district, section, landNumber string) (*domain.Parcel, error) {
	row, err := r.queries.GetParcelByLandNumber(ctx, db.GetParcelByLandNumberParams{
		County:     county,
		District:   district,
		Section:    section,
		LandNumber: landNumber,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrParcelNotFound
		}
		return nil, err
	}
	p := toParcelDomain(row)
	// Populate 4326
	if geom4326, centroid4326, bbox4326, err := r.fetch4326ByLandNumber(ctx, county, district, section, landNumber); err == nil {
		p.Geometry4326 = geom4326
		p.Centroid4326 = centroid4326
		p.BBox4326 = bbox4326
	}
	return p, nil
}

// Search returns parcels matching filter. County and District are required.
func (r *parcelRepository) Search(ctx context.Context, filter ParcelFilter) ([]*domain.Parcel, error) {
	if filter.County == "" || filter.District == "" {
		return nil, fmt.Errorf("county and district are required for search")
	}
	// Default limit
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Build pgtype numerics for min/max area
	var minArea pgtype.Numeric
	var maxArea pgtype.Numeric
	hasMin := filter.MinArea != nil
	hasMax := filter.MaxArea != nil
	if hasMin {
		if err := minArea.Scan(fmt.Sprintf("%f", *filter.MinArea)); err != nil {
			return nil, fmt.Errorf("invalid MinArea: %w", err)
		}
	}
	if hasMax {
		if err := maxArea.Scan(fmt.Sprintf("%f", *filter.MaxArea)); err != nil {
			return nil, fmt.Errorf("invalid MaxArea: %w", err)
		}
	}

	sectionVal := ""
	if filter.Section != nil {
		sectionVal = *filter.Section
	}

	// Note: sqlc SearchParcels has Column3 as string for section filter using "$3::text IS NULL"
	// Passing empty string would not be null; we need to handle empty as no filter.
	// We work around by using custom query when section is empty to get correct semantics,
	// otherwise delegate to sqlc.
	if sectionVal == "" && !hasMin && !hasMax {
		// Simple search without optional filters -> use custom query for correct NULL handling
		// We still use the same semantics: county,district only
		return r.searchCustom(ctx, filter.County, filter.District, nil, nil, nil, limit, offset)
	}
	// If section is empty but has area filters, we need custom as well because sqlc's
	// Column3 string empty != NULL
	if sectionVal == "" {
		return r.searchCustom(ctx, filter.County, filter.District, nil, filter.MinArea, filter.MaxArea, limit, offset)
	}

	// Section non-empty: we can use sqlc with section equality + area filters.
	// However sqlc's handling of numeric null via pgtype.Numeric.Valid is correct:
	// if Numeric not Valid, the query's "$4::numeric IS NULL" should be true.
	// We need to ensure Valid flag is set correctly.
	if !hasMin {
		minArea = pgtype.Numeric{} // Valid false -> NULL
	}
	if !hasMax {
		maxArea = pgtype.Numeric{}
	}

	rows, err := r.queries.SearchParcels(ctx, db.SearchParcelsParams{
		County:   filter.County,
		District: filter.District,
		Column3:  sectionVal,
		Column4:  minArea,
		Column5:  maxArea,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Parcel, 0, len(rows))
	for _, row := range rows {
		p := toParcelDomain(row)
		out = append(out, p)
	}
	// Populate 4326 for each result (batch fetch could be optimized but per-row is fine for tests)
	for _, p := range out {
		if geom4326, centroid4326, bbox4326, err := r.GetGeometry4326(ctx, p.ID); err == nil {
			p.Geometry4326 = geom4326
			p.Centroid4326 = centroid4326
			p.BBox4326 = bbox4326
		}
	}
	return out, nil
}

func (r *parcelRepository) searchCustom(ctx context.Context, county, district string, section *string, minArea, maxArea *float64, limit, offset int32) ([]*domain.Parcel, error) {
	// Build dynamic query with correct null handling
	args := []any{county, district}
	// idx for placeholders
	ph := 3
	query := `SELECT id, county, district, section, land_number, area_sqm, urban_zoning, land_use_category, ST_AsText(geometry) as geometry, ST_AsText(centroid) as centroid, ST_AsText(bbox) as bbox, source, source_version, import_batch_id, created_at, updated_at FROM parcel WHERE county=$1 AND district=$2`
	if section != nil && *section != "" {
		query += fmt.Sprintf(" AND section=$%d", ph)
		args = append(args, *section)
		ph++
	}
	if minArea != nil {
		query += fmt.Sprintf(" AND area_sqm >= $%d", ph)
		var n pgtype.Numeric
		_ = n.Scan(fmt.Sprintf("%f", *minArea))
		args = append(args, n)
		ph++
	}
	if maxArea != nil {
		query += fmt.Sprintf(" AND area_sqm <= $%d", ph)
		var n pgtype.Numeric
		_ = n.Scan(fmt.Sprintf("%f", *maxArea))
		args = append(args, n)
		ph++
	}
	query += fmt.Sprintf(" ORDER BY section, land_number LIMIT $%d OFFSET $%d", ph, ph+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Parcel
	for rows.Next() {
		var id pgtype.UUID
		var county, district, section, landNumber, source, sourceVersion string
		var area pgtype.Numeric
		var urbanZoning, landUse pgtype.Text
		var geometry, centroid, bbox *string
		var importBatchID pgtype.UUID
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(
			&id, &county, &district, &section, &landNumber,
			&area, &urbanZoning, &landUse,
			&geometry, &centroid, &bbox,
			&source, &sourceVersion, &importBatchID,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		p := &domain.Parcel{
			ID:            uuidToString(id),
			County:        county,
			District:      district,
			Section:       section,
			LandNumber:    landNumber,
			Source:        source,
			SourceVersion: sourceVersion,
		}
		if v, err := numericToFloat64(area); err == nil {
			p.AreaSqm = v
		}
		if urbanZoning.Valid {
			p.UrbanZoning = urbanZoning.String
		}
		if landUse.Valid {
			p.LandUseCategory = landUse.String
		}
		if geometry != nil {
			p.Geometry = *geometry
		}
		if centroid != nil {
			p.Centroid = *centroid
		}
		if bbox != nil {
			p.BBox = *bbox
		}
		if importBatchID.Valid {
			p.ImportBatchID = uuidToString(importBatchID)
		}
		if createdAt.Valid {
			p.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			p.UpdatedAt = updatedAt.Time
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Populate 4326
	for _, p := range out {
		if geom4326, centroid4326, bbox4326, err := r.GetGeometry4326(ctx, p.ID); err == nil {
			p.Geometry4326 = geom4326
			p.Centroid4326 = centroid4326
			p.BBox4326 = bbox4326
		}
	}
	return out, nil
}

// GetGeometry4326 returns transformed WKT in EPSG:4326.
func (r *parcelRepository) GetGeometry4326(ctx context.Context, id string) (string, string, string, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return "", "", "", err
	}
	query := `SELECT ST_AsText(ST_Transform(geometry,4326)), ST_AsText(ST_Transform(centroid,4326)), ST_AsText(ST_Transform(bbox,4326)) FROM parcel WHERE id=$1`
	var geom, centroid, bbox *string
	err = r.db.QueryRow(ctx, query, uid).Scan(&geom, &centroid, &bbox)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", ErrParcelNotFound
		}
		return "", "", "", err
	}
	var g, c, b string
	if geom != nil {
		g = *geom
	}
	if centroid != nil {
		c = *centroid
	}
	if bbox != nil {
		b = *bbox
	}
	return g, c, b, nil
}

func (r *parcelRepository) fetch4326ByLandNumber(ctx context.Context, county, district, section, landNumber string) (string, string, string, error) {
	query := `SELECT ST_AsText(ST_Transform(geometry,4326)), ST_AsText(ST_Transform(centroid,4326)), ST_AsText(ST_Transform(bbox,4326)) FROM parcel WHERE county=$1 AND district=$2 AND section=$3 AND land_number=$4`
	var geom, centroid, bbox *string
	err := r.db.QueryRow(ctx, query, county, district, section, landNumber).Scan(&geom, &centroid, &bbox)
	if err != nil {
		return "", "", "", err
	}
	var g, c, b string
	if geom != nil {
		g = *geom
	}
	if centroid != nil {
		c = *centroid
	}
	if bbox != nil {
		b = *bbox
	}
	return g, c, b, nil
}

func toParcelDomain(row db.Parcel) *domain.Parcel {
	p := &domain.Parcel{
		ID:            uuidToString(row.ID),
		County:        row.County,
		District:      row.District,
		Section:       row.Section,
		LandNumber:    row.LandNumber,
		Source:        row.Source,
		SourceVersion: row.SourceVersion,
	}
	if v, err := numericToFloat64(row.AreaSqm); err == nil {
		p.AreaSqm = v
	}
	if row.UrbanZoning.Valid {
		p.UrbanZoning = row.UrbanZoning.String
	}
	if row.LandUseCategory.Valid {
		p.LandUseCategory = row.LandUseCategory.String
	}
	// Geometry is string due to sqlc override
	p.Geometry = row.Geometry
	// Centroid/BBox are interface{} in generated model
	if row.Centroid != nil {
		p.Centroid = interfaceToString(row.Centroid)
	}
	if row.Bbox != nil {
		p.BBox = interfaceToString(row.Bbox)
	}
	if row.ImportBatchID.Valid {
		p.ImportBatchID = uuidToString(row.ImportBatchID)
	}
	if row.CreatedAt.Valid {
		p.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		p.UpdatedAt = row.UpdatedAt.Time
	}
	return p
}

func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func numericToFloat64(n pgtype.Numeric) (float64, error) {
	if !n.Valid {
		return 0, fmt.Errorf("numeric not valid")
	}
	f, err := n.Float64Value()
	if err == nil && f.Valid {
		return f.Float64, nil
	}
	if err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("numeric not convertible to float64")
}

func stripSRID(wkt string) string {
	if wkt == "" {
		return ""
	}
	// Remove SRID=...; prefix for ST_GeomFromText
	if idx := strings.Index(wkt, ";"); idx != -1 {
		prefix := wkt[:idx]
		if strings.HasPrefix(strings.ToUpper(prefix), "SRID=") {
			return wkt[idx+1:]
		}
	}
	return wkt
}

func nullableUUIDString(u *pgtype.UUID) any {
	if u == nil || !u.Valid {
		return nil
	}
	// Return string uuid
	return uuidToString(*u)
}
