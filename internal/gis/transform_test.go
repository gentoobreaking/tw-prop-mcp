package gis

import (
	"context"
	"testing"
)

func TestTransformWKTToInternal_ValidInput(t *testing.T) {
	t.Skip("Requires PostGIS database; integration test covers this")
}

func TestTransformWKTToExternal_ValidInput(t *testing.T) {
	t.Skip("Requires PostGIS database; integration test covers this")
}

func TestTransformWKTToInternal_EmptyInput(t *testing.T) {
	ctx := context.Background()
	mockDB := &MockDB{}
	_, err := TransformWKTToInternal(ctx, mockDB, "")
	if err == nil {
		t.Error("expected error for empty WKT")
	}
	if err.Error() != "transform 4326→3826: empty WKT input" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTransformWKTToExternal_EmptyInput(t *testing.T) {
	ctx := context.Background()
	mockDB := &MockDB{}
	_, err := TransformWKTToExternal(ctx, mockDB, "")
	if err == nil {
		t.Error("expected error for empty WKT")
	}
	if err.Error() != "transform 3826→4326: empty WKT input" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStripSRID_RemovesPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SRID=4326;POINT(121.5 25.0)", "POINT(121.5 25.0)"},
		{"SRID=3826;MULTIPOLYGON(((0 0, 1 0, 1 1, 0 1, 0 0)))", "MULTIPOLYGON(((0 0, 1 0, 1 1, 0 1, 0 0)))"},
		{"  SRID=4326;POINT(1 1)  ", "POINT(1 1)"},
		{"POINT(121.5 25.0)", "POINT(121.5 25.0)"},
		{"", ""},
	}
	for _, tt := range tests {
		result := stripSRID(tt.input)
		if result != tt.expected {
			t.Errorf("stripSRID(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestStripSRID_CaseInsensitive(t *testing.T) {
	result := stripSRID("srid=4326;POINT(1 1)")
	if result != "POINT(1 1)" {
		t.Errorf("stripSRID case insensitive failed: %q", result)
	}
}