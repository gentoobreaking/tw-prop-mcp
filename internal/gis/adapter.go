package gis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// RoadSegment is a road or street extracted from an official GIS source.
// Geometry is WKT in EPSG:4326 (WGS84).
type RoadSegment struct {
	ID       string
	Name     string
	Geometry string
}

// GISSource abstracts a data source for official parcel geometry and
// road network data.  Implementations fetch WKT in EPSG:4326.
type GISSource interface {
	// FetchParcelGeometry retrieves the boundary geometry of a parcel
	// identified by county, district, section, and land number.
	// Returns WKT in EPSG:4326.
	FetchParcelGeometry(ctx context.Context, county, district, section, landNumber string) (wkt4326 string, err error)

	// FetchRoadNetwork retrieves road segments within the given bounding box.
	// The bbox is a WKT POLYGON in EPSG:4326, e.g.
	// "POLYGON((lon1 lat1, lon2 lat2, lon1 lat2))".
	FetchRoadNetwork(ctx context.Context, bbox string) ([]RoadSegment, error)
}

// GISService coordinates multiple GISSource implementations with optional
// fallback.  When fallback is true, each source is tried in order; the first
// successful result is returned.  When fallback is false, only the first
// source is tried.
type GISService struct {
	sources  []GISSource
	fallback bool
}

// NewGISService creates a GISService with the given sources and fallback
// behaviour.
func NewGISService(sources []GISSource, fallback bool) *GISService {
	return &GISService{sources: sources, fallback: fallback}
}

// FetchParcelGeometry tries each configured source (unless fallback is false,
// in which case only the first is tried) and returns the first successful
// WKT geometry.
func (s *GISService) FetchParcelGeometry(ctx context.Context, county, district, section, landNumber string) (string, error) {
	for i, src := range s.sources {
		wkt, err := src.FetchParcelGeometry(ctx, county, district, section, landNumber)
		if err == nil {
			return wkt, nil
		}
		if !s.fallback || i == len(s.sources)-1 {
			return "", fmt.Errorf("FetchParcelGeometry: %w", err)
		}
		// Try next source
	}
	return "", fmt.Errorf("FetchParcelGeometry: no sources available")
}

// FetchRoadNetwork tries each configured source and returns road segments.
func (s *GISService) FetchRoadNetwork(ctx context.Context, bbox string) ([]RoadSegment, error) {
	for i, src := range s.sources {
		segments, err := src.FetchRoadNetwork(ctx, bbox)
		if err == nil {
			return segments, nil
		}
		if !s.fallback || i == len(s.sources)-1 {
			return nil, fmt.Errorf("FetchRoadNetwork: %w", err)
		}
	}
	return nil, fmt.Errorf("FetchRoadNetwork: no sources available")
}

// ---------------------------------------------------------------------------
// LandSurveyAdapter — 國土測繪圖資服務雲 (maps.land.gov.tw) WFS adapter
// ---------------------------------------------------------------------------

// LandSurveyAdapter implements GISSource by querying the NLSC WFS GetFeature
// endpoint.  It is configurable with a base URL and HTTP client so it can be
// tested with httptest.Server.
type LandSurveyAdapter struct {
	baseURL string
	client  *http.Client
}

// NewLandSurveyAdapter creates a LandSurveyAdapter with the given base WFS
// URL and HTTP client.
func NewLandSurveyAdapter(baseURL string, client *http.Client) *LandSurveyAdapter {
	if client == nil {
		client = &http.Client{}
	}
	return &LandSurveyAdapter{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

// FetchParcelGeometry queries the WFS GetFeature endpoint at
// maps.land.gov.tw for a parcel matching the given identifier and
// converts the GeoJSON response to WKT in EPSG:4326.
func (a *LandSurveyAdapter) FetchParcelGeometry(ctx context.Context, county, district, section, landNumber string) (string, error) {
	typeName := "TWIS_NLSC_Parcel"
	cqlFilter := fmt.Sprintf(
		"COUNTY='%s' AND TOWN='%s' AND SECTION='%s' AND LAN_NO='%s'",
		county, district, section, landNumber,
	)

	params := url.Values{}
	params.Set("service", "WFS")
	params.Set("version", "2.0.0")
	params.Set("request", "GetFeature")
	params.Set("typeNames", typeName)
	params.Set("outputFormat", "json")
	params.Set("CQL_FILTER", cqlFilter)
	params.Set("maxFeatures", "1")

	endpoint := a.baseURL + "? " + params.Encode()
	// Fix double-space typo above (kept for readability, cleaned here):
	endpoint = strings.Replace(endpoint, "? ", "?", 1)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("land survey request create: %w", err)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("land survey fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("land survey: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("land survey read body: %w", err)
	}

	return geoJSONToWKT(body)
}

// FetchRoadNetwork queries the WFS endpoint for road segments within the
// given bounding box (WKT POLYGON in EPSG:4326).
func (a *LandSurveyAdapter) FetchRoadNetwork(ctx context.Context, bbox string) ([]RoadSegment, error) {
	coords, err := parseBBoxCoords(bbox)
	if err != nil {
		return nil, fmt.Errorf("land survey road network: invalid bbox: %w", err)
	}
	bboxStr := fmt.Sprintf(
		"%.8f,%.8f,%.8f,%.8f",
		coords[0], coords[1], coords[2], coords[3],
	)

	params := url.Values{}
	params.Set("service", "WFS")
	params.Set("version", "2.0.0")
	params.Set("request", "GetFeature")
	params.Set("typeNames", "TWIS_NLSC_Road")
	params.Set("outputFormat", "json")
	params.Set("bbox", bboxStr)

	endpoint := a.baseURL + "?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("land survey road request create: %w", err)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("land survey road fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("land survey road: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("land survey road read body: %w", err)
	}

	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			ID         string          `json:"id"`
			Geometry   json.RawMessage `json:"geometry"`
			Properties map[string]any  `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("land survey road parse: %w", err)
	}

	segments := make([]RoadSegment, 0, len(fc.Features))
	for _, f := range fc.Features {
		wkt, err := geoJSONGeometryToWKT(f.Geometry)
		if err != nil {
			// skip features with unparseable geometry
			continue
		}
		name := ""
		if n, ok := f.Properties["Name"]; ok {
			if s, _ := n.(string); s != "" {
				name = s
			}
		}
		segments = append(segments, RoadSegment{
			ID:       f.ID,
			Name:     name,
			Geometry: wkt,
		})
	}
	return segments, nil
}

// ---------------------------------------------------------------------------
// LandRegistryAdapter — 地籍圖資網路便民服務系統 REST API adapter
// ---------------------------------------------------------------------------

// LandRegistryAdapter implements GISSource by querying the MoI land registry
// REST API.  Configurable with a base URL and HTTP client for testability.
type LandRegistryAdapter struct {
	baseURL string
	client  *http.Client
}

// NewLandRegistryAdapter creates a LandRegistryAdapter with the given base
// REST URL and HTTP client.
func NewLandRegistryAdapter(baseURL string, client *http.Client) *LandRegistryAdapter {
	if client == nil {
		client = &http.Client{}
	}
	return &LandRegistryAdapter{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

// FetchParcelGeometry queries the land registry REST API for a parcel
// matching the given identifier and converts the response to WKT in
// EPSG:4326.
func (a *LandRegistryAdapter) FetchParcelGeometry(ctx context.Context, county, district, section, landNumber string) (string, error) {
	path := fmt.Sprintf("/api/v1/parcels/%s/%s/%s/%s",
		url.PathEscape(county), url.PathEscape(district),
		url.PathEscape(section), url.PathEscape(landNumber),
	)

	endpoint := a.baseURL + path

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("land registry request create: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("land registry fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("land registry: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("land registry read body: %w", err)
	}

	return geoJSONToWKT(body)
}

// FetchRoadNetwork queries the land registry REST API for road segments
// within the given bounding box.
func (a *LandRegistryAdapter) FetchRoadNetwork(ctx context.Context, bbox string) ([]RoadSegment, error) {
	coords, err := parseBBoxCoords(bbox)
	if err != nil {
		return nil, fmt.Errorf("land registry road network: invalid bbox: %w", err)
	}

	params := url.Values{}
	params.Set("bbox", fmt.Sprintf("%.8f,%.8f,%.8f,%.8f",
		coords[0], coords[1], coords[2], coords[3]))

	endpoint := a.baseURL + "/api/v1/roads?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("land registry road request create: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("land registry road fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("land registry road: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("land registry road read body: %w", err)
	}

	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			ID         string          `json:"id"`
			Geometry   json.RawMessage `json:"geometry"`
			Properties map[string]any  `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("land registry road parse: %w", err)
	}

	segments := make([]RoadSegment, 0, len(fc.Features))
	for _, f := range fc.Features {
		wkt, err := geoJSONGeometryToWKT(f.Geometry)
		if err != nil {
			continue
		}
		name := ""
		if n, ok := f.Properties["name"]; ok {
			if s, _ := n.(string); s != "" {
				name = s
			}
		}
		segments = append(segments, RoadSegment{
			ID:       f.ID,
			Name:     name,
			Geometry: wkt,
		})
	}
	return segments, nil
}

// ---------------------------------------------------------------------------
// Shared GeoJSON → WKT conversion
// ---------------------------------------------------------------------------

// geoJSONToWKT parses a GeoJSON body and returns the WKT representation of
// the first geometry found.  Supports FeatureCollection, Feature, and bare
// geometry objects.  The resulting WKT is in the same CRS as the input
// (GeoJSON is implicitly EPSG:4326).
func geoJSONToWKT(data []byte) (string, error) {
	var raw struct {
		Type     string          `json:"type"`
		Geometry json.RawMessage `json:"geometry"`
		Features []struct {
			Geometry json.RawMessage `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("geoJSONToWKT: parse: %w", err)
	}

	var geom json.RawMessage
	switch strings.ToUpper(raw.Type) {
	case "FEATURECOLLECTION":
		if len(raw.Features) == 0 {
			return "", fmt.Errorf("geoJSONToWKT: empty feature collection")
		}
		geom = raw.Features[0].Geometry
	case "FEATURE":
		geom = raw.Geometry
	default:
		// Treat the whole body as a bare geometry object.
		geom = data
	}

	return geoJSONGeometryToWKT(geom)
}

// geoJSONGeometryToWKT converts a single GeoJSON geometry object (as raw JSON)
// to WKT.
func geoJSONGeometryToWKT(data []byte) (string, error) {
	var geom struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(data, &geom); err != nil {
		return "", fmt.Errorf("geoJSONGeometryToWKT: parse geometry: %w", err)
	}

	var coords interface{}
	if err := json.Unmarshal(geom.Coordinates, &coords); err != nil {
		return "", fmt.Errorf("geoJSONGeometryToWKT: parse coordinates: %w", err)
	}

	wktCoords, err := coordsToWKT(coords)
	if err != nil {
		return "", err
	}

	switch strings.ToUpper(geom.Type) {
	case "POINT":
		return "POINT(" + wktCoords + ")", nil
	case "MULTIPOINT":
		return "MULTIPOINT(" + wktCoords + ")", nil
	case "LINESTRING":
		return "LINESTRING(" + wktCoords + ")", nil
	case "MULTILINESTRING":
		return "MULTILINESTRING(" + wktCoords + ")", nil
	case "POLYGON":
		return "POLYGON(" + wktCoords + ")", nil
	case "MULTIPOLYGON":
		return "MULTIPOLYGON(" + wktCoords + ")", nil
	default:
		return "", fmt.Errorf("geoJSONGeometryToWKT: unsupported type %q", geom.Type)
	}
}

// coordsToWKT recursively converts decoded JSON coordinate arrays into the
// WKT coordinate string format.  GeoJSON coordinates are [lon, lat] pairs
// (or nested arrays of them).
func coordsToWKT(coords interface{}) (string, error) {
	arr, ok := coords.([]interface{})
	if !ok {
		return "", fmt.Errorf("coordsToWKT: expected array, got %T", coords)
	}
	if len(arr) == 0 {
		return "", fmt.Errorf("coordsToWKT: empty coordinates")
	}

	// Leaf-level: [lon, lat] for a Point.
	if _, isNum := arr[0].(float64); isNum {
		parts := make([]string, 0, len(arr))
		for _, v := range arr {
			f, ok := v.(float64)
			if !ok {
				return "", fmt.Errorf("coordsToWKT: non-numeric coordinate %T", v)
			}
			parts = append(parts, formatCoord(f))
		}
		return strings.Join(parts, " "), nil
	}

	// Recursive: array of arrays.
	inner := make([]string, 0, len(arr))
	for _, v := range arr {
		s, err := coordsToWKT(v)
		if err != nil {
			return "", err
		}
		inner = append(inner, "("+s+")")
	}
	return strings.Join(inner, ","), nil
}

// formatCoord formats a float coordinate for WKT output, trimming
// unnecessary trailing zeros while preserving precision.
func formatCoord(f float64) string {
	if f == math.Floor(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// parseBBoxCoords extracts the (minX, minY, maxX, maxY) coordinates from a
// WKT POLYGON string like "POLYGON((x1 y1, x2 y2, x3 y3, x1 y1))".
func parseBBoxCoords(bbox string) ([]float64, error) {
	s := strings.TrimSpace(bbox)
	// Strip SRID prefix if present.
	if idx := strings.Index(s, ";"); idx != -1 {
		if strings.HasPrefix(strings.ToUpper(s[:idx]), "SRID=") {
			s = strings.TrimSpace(s[idx+1:])
		}
	}

	s = strings.ToUpper(s)
	if !strings.HasPrefix(s, "POLYGON") {
		return nil, fmt.Errorf("parseBBoxCoords: expected POLYGON, got %q", bbox)
	}

	start := strings.Index(s, "((")
	end := strings.LastIndex(s, "))")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("parseBBoxCoords: malformed polygon: %q", bbox)
	}

	inner := bbox[start+2 : end]
	parts := strings.Split(inner, ",")
	if len(parts) < 3 {
		return nil, fmt.Errorf("parseBBoxCoords: need at least 3 points, got %d", len(parts))
	}

	var coords []float64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		fields := strings.Fields(p)
		if len(fields) < 2 {
			continue
		}
		x, errX := strconv.ParseFloat(fields[0], 64)
		y, errY := strconv.ParseFloat(fields[1], 64)
		if errX != nil || errY != nil {
			return nil, fmt.Errorf("parseBBoxCoords: parse coord: %v %v", errX, errY)
		}
		coords = append(coords, x, y)
	}

	if len(coords) < 4 {
		return nil, fmt.Errorf("parseBBoxCoords: not enough coordinates")
	}

	minX, maxX := coords[0], coords[0]
	minY, maxY := coords[1], coords[1]
	for i := 2; i+1 < len(coords); i += 2 {
		x, y := coords[i], coords[i+1]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	return []float64{minX, minY, maxX, maxY}, nil
}

// Ensure bytes import is used (used by formatCoord path indirectly).
var _ = bytes.MinRead
