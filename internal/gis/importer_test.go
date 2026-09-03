package gis

import (
	"testing"
	"time"
)

func TestGISDownloader_Cache(t *testing.T) {
	// This test requires a test server; skip for now
	t.Skip("Requires test HTTP server")
}

func TestGISParser_ParseParcelGeoJSON(t *testing.T) {
	parser := NewGISParser()

	geojson := []byte(`{
		"type": "FeatureCollection",
		"features": [{
			"type": "Feature",
			"geometry": {
				"type": "Polygon",
				"coordinates": [[[121.5, 25.0], [121.51, 25.0], [121.51, 25.01], [121.5, 25.01], [121.5, 25.0]]]
			},
			"properties": {
				"COUNTY": "台南市",
				"TOWN": "安南區",
				"SECTION": "竹篙灣段",
				"LAN_NO": "0001",
				"AREA": 1000,
				"URBAN_ZONING": "工業區",
				"LAND_USE": "工業"
			}
		}]
	}`)

	parcels, err := parser.ParseParcelGeoJSON(geojson)
	if err != nil {
		t.Fatalf("ParseParcelGeoJSON error: %v", err)
	}
	if len(parcels) != 1 {
		t.Fatalf("expected 1 parcel, got %d", len(parcels))
	}
	p := parcels[0]
	if p.County != "台南市" || p.District != "安南區" || p.Section != "竹篙灣段" || p.LandNumber != "0001" {
		t.Errorf("unexpected parcel fields: %+v", p)
	}
	if p.AreaSqm != 1000 {
		t.Errorf("expected area 1000, got %f", p.AreaSqm)
	}
	if p.Geometry4326 == "" {
		t.Error("expected geometry")
	}
}

func TestGISParser_ParseParcelGeoJSON_Multiple(t *testing.T) {
	parser := NewGISParser()

	geojson := []byte(`{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"geometry": {"type": "Polygon", "coordinates": [[[121.5, 25.0], [121.51, 25.0], [121.51, 25.01], [121.5, 25.01], [121.5, 25.0]]]},
				"properties": {"COUNTY": "台南市", "TOWN": "安南區", "SECTION": "竹篙灣段", "LAN_NO": "0001", "AREA": 1000}
			},
			{
				"type": "Feature",
				"geometry": {"type": "Polygon", "coordinates": [[[121.52, 25.0], [121.53, 25.0], [121.53, 25.01], [121.52, 25.01], [121.52, 25.0]]]},
				"properties": {"COUNTY": "台南市", "TOWN": "安南區", "SECTION": "竹篙灣段", "LAN_NO": "0002", "AREA": 2000}
			}
		]
	}`)

	parcels, err := parser.ParseParcelGeoJSON(geojson)
	if err != nil {
		t.Fatalf("ParseParcelGeoJSON error: %v", err)
	}
	if len(parcels) != 2 {
		t.Fatalf("expected 2 parcels, got %d", len(parcels))
	}
	if parcels[0].LandNumber != "0001" || parcels[1].LandNumber != "0002" {
		t.Errorf("unexpected land numbers: %s, %s", parcels[0].LandNumber, parcels[1].LandNumber)
	}
}

func TestGISParser_ParseParcelGeoJSON_MissingFields(t *testing.T) {
	parser := NewGISParser()

	geojson := []byte(`{
		"type": "FeatureCollection",
		"features": [{
			"type": "Feature",
			"geometry": {"type": "Polygon", "coordinates": [[[121.5, 25.0], [121.51, 25.0], [121.51, 25.01], [121.5, 25.01], [121.5, 25.0]]]},
			"properties": {"COUNTY": "台南市"}
		}]
	}`)

	parcels, err := parser.ParseParcelGeoJSON(geojson)
	if err != nil {
		t.Fatalf("ParseParcelGeoJSON error: %v", err)
	}
	// Missing district/section/land_number -> skipped
	if len(parcels) != 0 {
		t.Errorf("expected 0 parcels (missing required fields), got %d", len(parcels))
	}
}

func TestValidateParcel(t *testing.T) {
	valid := ParsedParcel{
		County: "台南市", District: "安南區", Section: "竹篙灣段", LandNumber: "0001",
		AreaSqm: 1000, Geometry4326: "POLYGON((121.5 25.0, 121.51 25.0, 121.51 25.01, 121.5 25.01, 121.5 25.0))",
	}
	errs := ValidateParcel(valid)
	if len(errs) != 0 {
		t.Errorf("valid parcel should have no errors: %v", errs)
	}

	invalid := ParsedParcel{County: "台南市"}
	errs = ValidateParcel(invalid)
	if len(errs) == 0 {
		t.Error("expected validation errors for missing fields")
	}
	found := false
	for _, e := range errs {
		if e.Type == "validation" {
			found = true
		}
	}
	if !found {
		t.Error("expected validation error type")
	}

	zeroArea := valid
	zeroArea.AreaSqm = 0
	errs = ValidateParcel(zeroArea)
	found = false
	for _, e := range errs {
		if e.Type == "validation" && e.Message == "area_sqm must be > 0" {
			found = true
		}
	}
	if !found {
		t.Error("expected area validation error")
	}

	badGeom := valid
	badGeom.Geometry4326 = "POINT(121.5 25.0)"
	errs = ValidateParcel(badGeom)
	found = false
	for _, e := range errs {
		if e.Type == "geometry" {
			found = true
		}
	}
	if !found {
		t.Error("expected geometry error for non-polygon")
	}
}

func TestGenerateBatchID(t *testing.T) {
	id1 := generateBatchID()
	time.Sleep(1 * time.Microsecond) // Ensure different timestamps
	id2 := generateBatchID()
	if id1 == id2 {
		t.Error("batch IDs should be unique")
	}
	if len(id1) < 10 {
		t.Errorf("batch ID too short: %s", id1)
	}
}

func TestComputeChecksum(t *testing.T) {
	data := []byte("test data")
	sum1 := computeChecksum(data)
	sum2 := computeChecksum(data)
	if sum1 != sum2 {
		t.Error("checksum should be deterministic")
	}
	if len(sum1) != 64 { // sha256 hex = 64 chars
		t.Errorf("checksum length should be 64, got %d", len(sum1))
	}
}

func TestEnsureMultiPolygon(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SRID=3826;POLYGON((0 0, 1 0, 1 1, 0 1, 0 0))", "SRID=3826;MULTIPOLYGON((0 0, 1 0, 1 1, 0 1, 0 0))"},
		{"POLYGON((0 0, 1 0, 1 1, 0 1, 0 0))", "SRID=3826;MULTIPOLYGON((0 0, 1 0, 1 1, 0 1, 0 0))"},
		{"SRID=3826;MULTIPOLYGON(((0 0, 1 0, 1 1, 0 1, 0 0)))", "SRID=3826;MULTIPOLYGON(((0 0, 1 0, 1 1, 0 1, 0 0)))"},
	}

	for _, tt := range tests {
		result := ensureMultiPolygon(tt.input)
		if result != tt.expected {
			t.Errorf("ensureMultiPolygon(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}