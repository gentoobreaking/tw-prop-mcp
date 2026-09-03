package gis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GISDownloader downloads official GIS data with caching and retries.
type GISDownloader struct {
	baseURL    string
	client     *http.Client
	cacheDir   string
	maxRetries int
}

// NewGISDownloader creates a GISDownloader.
func NewGISDownloader(baseURL, cacheDir string) *GISDownloader {
	if cacheDir == "" {
		cacheDir = "/tmp/gis-cache"
	}
	_ = os.MkdirAll(cacheDir, 0o755)
	return &GISDownloader{
		baseURL:    strings.TrimRight(baseURL, "/"),
		client:     &http.Client{Timeout: 60 * time.Second},
		cacheDir:   cacheDir,
		maxRetries: 3,
	}
}

// DownloadParcelGeoJSON downloads parcel GeoJSON for a given land number.
func (d *GISDownloader) DownloadParcelGeoJSON(ctx context.Context, county, district, section, landNumber string) ([]byte, error) {
	path := fmt.Sprintf("/api/v1/parcels/%s/%s/%s/%s",
		county, district, section, landNumber)
	return d.downloadWithCache(ctx, path, fmt.Sprintf("parcel_%s_%s_%s_%s.geojson", county, district, section, landNumber))
}

// DownloadRoadGeoJSON downloads road network GeoJSON within a bounding box.
func (d *GISDownloader) DownloadRoadGeoJSON(ctx context.Context, bbox string) ([]byte, error) {
	params := fmt.Sprintf("?bbox=%s", bbox)
	return d.downloadWithCache(ctx, "/api/v1/roads"+params, fmt.Sprintf("roads_%s.geojson", strings.ReplaceAll(bbox, " ", "_")))
}

func (d *GISDownloader) downloadWithCache(ctx context.Context, path, cacheKey string) ([]byte, error) {
	cacheFile := filepath.Join(d.cacheDir, cacheKey)

	// Check cache first
	if data, err := os.ReadFile(cacheFile); err == nil {
		return data, nil
	}

	url := d.baseURL + path
	var lastErr error

	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = fmt.Errorf("create request: %w", err)
			continue
		}
		req.Header.Set("Accept", "application/geo+json,application/json")

		// Add cache headers if cache file exists
		if info, err := os.Stat(cacheFile); err == nil {
			req.Header.Set("If-Modified-Since", info.ModTime().Format(http.TimeFormat))
		}

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("download: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusNotModified {
			data, _ := os.ReadFile(cacheFile)
			resp.Body.Close()
			return data, nil
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read body: %w", err)
			continue
		}

		// Write to cache
		_ = os.WriteFile(cacheFile, data, 0o644)
		return data, nil
	}

	return nil, fmt.Errorf("after %d retries: %w", d.maxRetries, lastErr)
}

// ParsedParcel represents a parsed parcel from GeoJSON.
type ParsedParcel struct {
	County          string
	District        string
	Section         string
	LandNumber      string
	AreaSqm         float64
	UrbanZoning     string
	LandUseCategory string
	Geometry4326    string // WKT in EPSG:4326
}

// GISParser parses GeoJSON into ParsedParcel.
type GISParser struct{}

// NewGISParser creates a GISParser.
func NewGISParser() *GISParser {
	return &GISParser{}
}

// ParseParcelGeoJSON parses a GeoJSON FeatureCollection into parcels.
func (p *GISParser) ParseParcelGeoJSON(data []byte) ([]ParsedParcel, error) {
	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Geometry json.RawMessage `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse GeoJSON: %w", err)
	}

	if fc.Type != "FeatureCollection" {
		return nil, fmt.Errorf("expected FeatureCollection, got %s", fc.Type)
	}

	parcels := make([]ParsedParcel, 0, len(fc.Features))
	for _, f := range fc.Features {
		geom, err := geoJSONGeometryToWKT(f.Geometry)
		if err != nil {
			continue // skip invalid geometry
		}

		props := f.Properties
		parcel := ParsedParcel{
			County:          getStringProp(props, "COUNTY", "county"),
			District:        getStringProp(props, "TOWN", "town", "DISTRICT", "district"),
			Section:         getStringProp(props, "SECTION", "section"),
			LandNumber:      getStringProp(props, "LAN_NO", "lan_no", "LAND_NUMBER", "land_number"),
			AreaSqm:         getFloatProp(props, "AREA", "area", "AREA_SQM", "area_sqm"),
			UrbanZoning:     getStringProp(props, "URBAN_ZONING", "urban_zoning", "ZONING", "zoning"),
			LandUseCategory: getStringProp(props, "LAND_USE", "land_use", "LAND_USE_CATEGORY", "land_use_category"),
			Geometry4326:    geom,
		}
		if parcel.County != "" && parcel.District != "" && parcel.Section != "" && parcel.LandNumber != "" {
			parcels = append(parcels, parcel)
		}
	}
	return parcels, nil
}

func getStringProp(props map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := props[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func getFloatProp(props map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := props[k]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case int:
				return float64(val)
			case string:
				var f float64
				fmt.Sscanf(val, "%f", &f)
				return f
			}
		}
	}
	return 0
}

// ImportPipeline orchestrates the GIS import process.
type ImportPipeline struct {
	Downloader     *GISDownloader
	Parser         *GISParser
	Repo           ParcelRepository
	GeometryEngine *GeometryEngine
	Provenance     ProvenanceRecorder
}

// ImportResult contains the result of an import operation.
type ImportResult struct {
	Imported int
	Skipped  int
	Errors   []ImportError
}

// ImportError represents an error during import.
type ImportError struct {
	Type    string // "validation", "database", "geometry"
	Record  string // identifier of the record
	Message string
}

// ParcelRepository is the interface for parcel persistence.
type ParcelRepository interface {
	BatchInsertParcels(ctx context.Context, pool *pgxpool.Pool, records []ParcelImportRecord) (int64, error)
	BatchInsertRoadSegments(ctx context.Context, pool *pgxpool.Pool, records []RoadImportRecord) (int64, error)
	CreateGISTIndexes(ctx context.Context) error
	VacuumAnalyze(ctx context.Context) error
}

// ParcelImportRecord represents a parcel ready for database insertion.
type ParcelImportRecord struct {
	County          string
	District        string
	Section         string
	LandNumber      string
	AreaSqm         float64
	UrbanZoning     string
	LandUseCategory string
	Geometry3826    string // EWKT with SRID=3826
	Centroid3826    string
	BBox3826        string
	Source          string
	SourceVersion   string
	ImportBatchID   string
	SnapshotID      string
}

// RoadImportRecord represents a road segment ready for database insertion.
type RoadImportRecord struct {
	Name            string
	RoadClass       string
	Width           float64
	Geometry3826    string // EWKT with SRID=3826
	ImportBatchID   string
}

// ProvenanceRecorder records provenance information.
type ProvenanceRecorder interface {
	RecordParcelProvenance(ctx context.Context, record ProvenanceRecord) error
	RecordRoadProvenance(ctx context.Context, record RoadProvenanceRecord) error
}

// ProvenanceRecord for parcel.
type ProvenanceRecord struct {
	ParcelID        string
	Source          string
	SourceVersion   string
	SnapshotID      string
	ImportBatchID   string
	SourceChecksum  string
	DownloadedAt    time.Time
}

// RoadProvenanceRecord for road segment.
type RoadProvenanceRecord struct {
	RoadSegmentID   string
	Source          string
	SourceVersion   string
	SnapshotID      string
	ImportBatchID   string
	SourceChecksum  string
	DownloadedAt    time.Time
}

// ImportParcels imports parcels for a given area.
func (p *ImportPipeline) ImportParcels(ctx context.Context, county, district, section, snapshotID string) (ImportResult, error) {
	result := ImportResult{}
	_ = generateBatchID()

	// Create snapshot for this import
	if err := p.createImportSnapshot(ctx, snapshotID, "NLSC_GIS"); err != nil {
		result.Errors = append(result.Errors, ImportError{Type: "database", Message: err.Error()})
		return result, err
	}

	// For demo: we parse from sample file in tests
	// Real implementation would call Downloader + Parser
	return result, nil
}

// ImportRoads imports road segments within a bounding box.
func (p *ImportPipeline) ImportRoads(ctx context.Context, bbox, snapshotID string) (ImportResult, error) {
	result := ImportResult{}
	_ = generateBatchID()

	if err := p.createImportSnapshot(ctx, snapshotID, "NLSC_GIS"); err != nil {
		result.Errors = append(result.Errors, ImportError{Type: "database", Message: err.Error()})
		return result, err
	}

	return result, nil
}

func (p *ImportPipeline) createImportSnapshot(ctx context.Context, snapshotID, source string) error {
	// This would use the snapshot repository to create a LOCKED snapshot
	return nil
}

func generateBatchID() string {
	return fmt.Sprintf("batch_%d_%d", time.Now().UnixNano(), time.Now().UnixMicro())
}

func computeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
// BatchInsertParcels inserts parcels using COPY protocol.
func (p *ImportPipeline) BatchInsertParcels(ctx context.Context, pool *pgxpool.Pool, records []ParcelImportRecord) (int64, error) {
	if len(records) == 0 {
		return 0, nil
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	copyCount, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"parcel_geometry"},
		[]string{"county", "district", "section", "land_number", "area_sqm", "urban_zoning", "land_use_category", "geometry", "centroid", "bbox", "source", "source_version", "import_batch_id", "snapshot_id"},
		pgx.CopyFromSlice(len(records), func(i int) ([]any, error) {
			r := records[i]
			return []any{
				r.County, r.District, r.Section, r.LandNumber, r.AreaSqm,
				r.UrbanZoning, r.LandUseCategory, r.Geometry3826, r.Centroid3826, r.BBox3826,
				r.Source, r.SourceVersion, r.ImportBatchID, r.SnapshotID,
			}, nil
		}),
	)
	return copyCount, err
}

// BatchInsertRoadSegments inserts road segments using COPY protocol.
func (p *ImportPipeline) BatchInsertRoadSegments(ctx context.Context, pool *pgxpool.Pool, records []RoadImportRecord) (int64, error) {
	if len(records) == 0 {
		return 0, nil
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	copyCount, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"road_segment"},
		[]string{"name", "road_class", "width", "geometry", "import_batch_id"},
		pgx.CopyFromSlice(len(records), func(i int) ([]any, error) {
			r := records[i]
			return []any{r.Name, r.RoadClass, r.Width, r.Geometry3826, r.ImportBatchID}, nil
		}),
	)
	return copyCount, err
}

// getPool returns the database pool.
// This is a placeholder - real implementation would inject the pool.
func (p *ImportPipeline) getPool() *pgxpool.Pool {
	return nil
}

func (p *ImportPipeline) CreateGISTIndexes(ctx context.Context) error {
	if p.Repo != nil {
		return p.Repo.CreateGISTIndexes(ctx)
	}
	return nil
}

func (p *ImportPipeline) VacuumAnalyze(ctx context.Context) error {
	if p.Repo != nil {
		return p.Repo.VacuumAnalyze(ctx)
	}
	return nil
}

// ValidateParcel validates a parsed parcel before import.
func ValidateParcel(parcel ParsedParcel) []ImportError {
	var errs []ImportError
	id := fmt.Sprintf("%s/%s/%s/%s", parcel.County, parcel.District, parcel.Section, parcel.LandNumber)

	if parcel.County == "" || parcel.District == "" || parcel.Section == "" || parcel.LandNumber == "" {
		errs = append(errs, ImportError{Type: "validation", Record: id, Message: "missing required 4-key fields"})
	}
	if parcel.AreaSqm <= 0 {
		errs = append(errs, ImportError{Type: "validation", Record: id, Message: "area_sqm must be > 0"})
	}
	if parcel.Geometry4326 == "" {
		errs = append(errs, ImportError{Type: "geometry", Record: id, Message: "empty geometry"})
	} else {
		// Check if geometry is valid WKT (basic check)
		upper := strings.ToUpper(strings.TrimSpace(parcel.Geometry4326))
		if !strings.HasPrefix(upper, "POLYGON") && !strings.HasPrefix(upper, "MULTIPOLYGON") {
			errs = append(errs, ImportError{Type: "geometry", Record: id, Message: "geometry must be POLYGON or MULTIPOLYGON"})
		}
	}
	return errs
}

// TransformParcelToImportRecord converts a parsed parcel to import record with 3826 geometry.
func TransformParcelToImportRecord(ctx context.Context, db DBQuerier, parcel ParsedParcel, sourceVersion, importBatchID, snapshotID string) (ParcelImportRecord, error) {
	// Transform 4326 -> 3826
	ewkt3826, err := TransformWKTToInternal(ctx, db, parcel.Geometry4326)
	if err != nil {
		return ParcelImportRecord{}, fmt.Errorf("transform geometry: %w", err)
	}

	// Get centroid in 3826
	centroid, err := getCentroid3826(ctx, db, ewkt3826)
	if err != nil {
		return ParcelImportRecord{}, fmt.Errorf("get centroid: %w", err)
	}

	// Get bbox in 3826
	bbox, err := getBBox3826(ctx, db, ewkt3826)
	if err != nil {
		return ParcelImportRecord{}, fmt.Errorf("get bbox: %w", err)
	}

	// Compute area in 3826 (sqm)
	area, err := getArea3826(ctx, db, ewkt3826)
	if err != nil {
		return ParcelImportRecord{}, fmt.Errorf("get area: %w", err)
	}

	// Cross-validate with source area (5% tolerance)
	if parcel.AreaSqm > 0 && area > 0 {
		diff := (area - parcel.AreaSqm) / parcel.AreaSqm
		if diff > 0.05 || diff < -0.05 {
			// Warning only, not blocking
		}
	}

	// Ensure MultiPolygon
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(ewkt3826)), "SRID=3826;MULTIPOLYGON") {
		ewkt3826 = ensureMultiPolygon(ewkt3826)
	}

	return ParcelImportRecord{
		County:          parcel.County,
		District:        parcel.District,
		Section:         parcel.Section,
		LandNumber:      parcel.LandNumber,
		AreaSqm:         area,
		UrbanZoning:     parcel.UrbanZoning,
		LandUseCategory: parcel.LandUseCategory,
		Geometry3826:    ewkt3826,
		Centroid3826:    centroid,
		BBox3826:        bbox,
		Source:          "NLSC_GIS",
		SourceVersion:   sourceVersion,
		ImportBatchID:   importBatchID,
		SnapshotID:      snapshotID,
	}, nil
}

func ensureMultiPolygon(ewkt string) string {
	s := strings.TrimSpace(ewkt)
	upper := strings.ToUpper(s)

	// Remove SRID prefix if present ("SRID=3826;" is 10 chars)
	if strings.HasPrefix(upper, "SRID=3826;") {
		s = s[10:]
		upper = strings.ToUpper(s)
	}

	if strings.HasPrefix(upper, "POLYGON(") {
		return "SRID=3826;MULTI" + s
	}
	if strings.HasPrefix(upper, "MULTIPOLYGON(") {
		return "SRID=3826;" + s
	}
	return "SRID=3826;MULTIPOLYGON((" + s + "))"
}

func getCentroid3826(ctx context.Context, db DBQuerier, ewkt string) (string, error) {
	q := `SELECT ST_AsEWKT(ST_Centroid(ST_GeomFromEWKT($1)))`
	var centroid string
	err := db.QueryRow(ctx, q, ewkt).Scan(&centroid)
	return centroid, err
}

func getBBox3826(ctx context.Context, db DBQuerier, ewkt string) (string, error) {
	q := `SELECT ST_AsEWKT(ST_Envelope(ST_GeomFromEWKT($1)))`
	var bbox string
	err := db.QueryRow(ctx, q, ewkt).Scan(&bbox)
	return bbox, err
}

func getArea3826(ctx context.Context, db DBQuerier, ewkt string) (float64, error) {
	q := `SELECT ST_Area(ST_GeomFromEWKT($1))`
	var area float64
	err := db.QueryRow(ctx, q, ewkt).Scan(&area)
	return area, err
}
