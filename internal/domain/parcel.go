package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Parcel is the domain model for the parcel table.
// Geometry fields are stored internally as EPSG:3826 (TWD97 / TM2 zone 121).
// External API output should use EPSG:4326 via PostGIS ST_Transform.
// Geometry/Centroid/BBox are WKT or EWKT strings, e.g. "SRID=3826;MULTIPOLYGON(((...)))".
type Parcel struct {
	ID              string    `json:"id"`
	County          string    `json:"county"`
	District        string    `json:"district"`
	Section         string    `json:"section"`
	LandNumber      string    `json:"land_number"`
	AreaSqm         float64   `json:"area_sqm"`
	UrbanZoning     string    `json:"urban_zoning,omitempty"`
	LandUseCategory string    `json:"land_use_category,omitempty"`
	Geometry        string    `json:"geometry"`           // WKT/EWKT in EPSG:3826
	Centroid        string    `json:"centroid,omitempty"` // WKT POINT in 3826
	BBox            string    `json:"bbox,omitempty"`     // WKT POLYGON in 3826
	Geometry4326    string    `json:"geometry_4326,omitempty"` // WKT in EPSG:4326 via ST_Transform
	Centroid4326    string    `json:"centroid_4326,omitempty"` // WKT POINT in 4326
	BBox4326        string    `json:"bbox_4326,omitempty"`     // WKT POLYGON in 4326
	Source          string    `json:"source"`
	SourceVersion   string    `json:"source_version"`
	ImportBatchID   string    `json:"import_batch_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// EPSG constants for documentation and SQL generation.
const (
	EPSG3826 = 3826 // Internal storage: TWD97 / TM2 zone 121
	EPSG4326 = 4326 // External API: WGS84
)

// To4326 returns the centroid as (lat, lon) in EPSG:4326.
// The conversion itself is performed by PostGIS via ST_Transform(geometry,4326) / ST_AsText.
// This method only parses the already-transformed Centroid4326 (or Geometry4326) WKT.
// WKT POINT format: "POINT(lon lat)" or "SRID=4326;POINT(lon lat)".
// Returns an error if no 4326 centroid is available.
func (p *Parcel) To4326() (lat, lon float64, err error) {
	wkt := p.Centroid4326
	if wkt == "" {
		// Fallback: try to extract point from Geometry4326 if centroid missing
		// For simplicity, return error instructing caller to use repository's 4326 fetch
		return 0, 0, fmt.Errorf("parcel %s: no Centroid4326 available; fetch via ST_AsText(ST_Transform(centroid,4326))", p.ID)
	}
	lon, lat, err = parsePointWKT(wkt)
	if err != nil {
		return 0, 0, fmt.Errorf("parse Centroid4326: %w", err)
	}
	return lat, lon, nil
}

// Centroid4326LatLon is an alias for To4326 for API symmetry.
func (p *Parcel) Centroid4326LatLon() (lat, lon float64, err error) {
	return p.To4326()
}

// Geometry4326WKT returns the transformed geometry WKT (4326) if populated.
func (p *Parcel) Geometry4326WKT() string {
	return p.Geometry4326
}

// TransformSQL returns the SQL fragment for transforming a geometry column to 4326.
// For use in query builders; actual execution is in PostGIS.
// Example: TransformSQL("geometry") => "ST_AsText(ST_Transform(geometry,4326))"
func TransformSQL(column string) string {
	return fmt.Sprintf("ST_AsText(ST_Transform(%s,%d))", column, EPSG4326)
}

// parsePointWKT parses "POINT(lon lat)" with optional SRID prefix.
func parsePointWKT(wkt string) (lon, lat float64, err error) {
	s := strings.TrimSpace(wkt)
	// Strip SRID prefix: "SRID=4326;POINT(...)"
	if idx := strings.Index(s, ";"); idx != -1 {
		s = s[idx+1:]
	}
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(strings.ToUpper(s), "POINT") {
		return 0, 0, fmt.Errorf("not a POINT WKT: %q", wkt)
	}
	// Extract content between parentheses
	start := strings.Index(s, "(")
	end := strings.LastIndex(s, ")")
	if start == -1 || end == -1 || end <= start {
		return 0, 0, fmt.Errorf("invalid POINT WKT: %q", wkt)
	}
	inner := strings.TrimSpace(s[start+1 : end])
	parts := strings.Fields(inner)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("POINT should have 2 coords, got %q", inner)
	}
	lon, err = strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse lon %q: %w", parts[0], err)
	}
	lat, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse lat %q: %w", parts[1], err)
	}
	return lon, lat, nil
}
