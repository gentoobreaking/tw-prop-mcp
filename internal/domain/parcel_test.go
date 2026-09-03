package domain

import (
	"strings"
	"testing"
	"time"
)

func TestParcelModelFields(t *testing.T) {
	now := time.Now()
	p := Parcel{
		ID:              "11111111-1111-1111-1111-111111111111",
		County:          "台北市",
		District:        "中山區",
		Section:         "中山段二小段",
		LandNumber:      "00120000",
		AreaSqm:         123.45,
		UrbanZoning:     "住",
		LandUseCategory: "住宅",
		Geometry:        "MULTIPOLYGON(((121.5 25.0,121.6 25.0,121.6 25.1,121.5 25.1,121.5 25.0)))",
		Centroid:        "POINT(121.55 25.05)",
		BBox:            "POLYGON((121.5 25.0,121.6 25.0,121.6 25.1,121.5 25.1,121.5 25.0))",
		Source:          "NLSC",
		SourceVersion:   "2024Q1",
		ImportBatchID:   "22222222-2222-2222-2222-222222222222",
		CreatedAt:       now,
	}
	if p.County != "台北市" || p.District != "中山區" || p.Section != "中山段二小段" || p.LandNumber != "00120000" {
		t.Fatalf("location fields mismatch")
	}
	if p.AreaSqm != 123.45 {
		t.Fatalf("area mismatch")
	}
	if p.UrbanZoning != "住" || p.LandUseCategory != "住宅" {
		t.Fatalf("zoning mismatch")
	}
	if p.Geometry == "" || p.Centroid == "" || p.BBox == "" {
		t.Fatalf("geometry fields should be non-empty")
	}
	if p.Source != "NLSC" || p.SourceVersion != "2024Q1" {
		t.Fatalf("source fields mismatch")
	}
	if p.CreatedAt.IsZero() {
		t.Fatalf("created_at should be set")
	}
	// Ensure 3826 marker: geometry should be treatable as 3826
	if strings.Contains(p.Geometry, "4326") {
		t.Fatalf("internal geometry should be 3826, not 4326")
	}
}

func TestParcelTransformSQL(t *testing.T) {
	sql := TransformSQL("geometry")
	if !strings.Contains(sql, "ST_Transform") || !strings.Contains(sql, "4326") {
		t.Fatalf("TransformSQL should contain ST_Transform and 4326, got %q", sql)
	}
	if !strings.Contains(sql, "geometry") {
		t.Fatalf("TransformSQL should reference column, got %q", sql)
	}
	sql2 := TransformSQL("centroid")
	if !strings.Contains(sql2, "centroid") {
		t.Fatalf("TransformSQL column name not propagated")
	}
}

func TestParcelTo4326(t *testing.T) {
	p := Parcel{
		ID:           "test-id",
		Centroid4326: "POINT(121.55 25.05)",
	}
	lat, lon, err := p.To4326()
	if err != nil {
		t.Fatalf("To4326 failed: %v", err)
	}
	if lat != 25.05 || lon != 121.55 {
		t.Fatalf("To4326 mismatch: got lat=%v lon=%v", lat, lon)
	}

	// With SRID prefix
	p2 := Parcel{
		ID:           "test-id-2",
		Centroid4326: "SRID=4326;POINT(121.6 24.9)",
	}
	lat, lon, err = p2.To4326()
	if err != nil {
		t.Fatalf("To4326 with SRID failed: %v", err)
	}
	if lat != 24.9 || lon != 121.6 {
		t.Fatalf("To4326 with SRID mismatch")
	}

	// No centroid4326 should error
	p3 := Parcel{ID: "empty"}
	_, _, err = p3.To4326()
	if err == nil {
		t.Fatalf("expected error when no centroid4326")
	}
	if !strings.Contains(err.Error(), "no Centroid4326") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// Invalid WKT
	p4 := Parcel{ID: "bad", Centroid4326: "INVALID(1 2)"}
	_, _, err = p4.To4326()
	if err == nil {
		t.Fatalf("expected error for invalid WKT")
	}
}

func TestParcelEPSGConstants(t *testing.T) {
	if EPSG3826 != 3826 {
		t.Fatalf("EPSG3826 should be 3826")
	}
	if EPSG4326 != 4326 {
		t.Fatalf("EPSG4326 should be 4326")
	}
}

func TestParcelGeometry4326WKT(t *testing.T) {
	p := Parcel{
		Geometry4326: "MULTIPOLYGON(((121.5 25.0,121.6 25.0,121.6 25.1,121.5 25.1,121.5 25.0)))",
	}
	if p.Geometry4326WKT() != p.Geometry4326 {
		t.Fatalf("Geometry4326WKT mismatch")
	}
}
