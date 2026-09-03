package gis

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGeometryEngine_ST_Intersects(t *testing.T) {
	resultVal := true
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{ScanFunc: func(dest ...any) error {
				if len(dest) > 0 {
					if b, ok := dest[0].(*bool); ok {
						*b = resultVal
					}
				}
				return nil
			}}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	result, err := engine.ST_Intersects(ctx, "POINT(1 1)", "POINT(1 1)")
	if err != nil {
		t.Fatalf("ST_Intersects error: %v", err)
	}
	if !result {
		t.Errorf("expected true for intersecting points")
	}
}

func TestGeometryEngine_ST_Within(t *testing.T) {
	resultVal := false
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{ScanFunc: func(dest ...any) error {
				if len(dest) > 0 {
					if b, ok := dest[0].(*bool); ok {
						*b = resultVal
					}
				}
				return nil
			}}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	result, err := engine.ST_Within(ctx, "POINT(1 1)", "POLYGON((0 0, 2 0, 2 2, 0 2, 0 0))")
	if err != nil {
		t.Fatalf("ST_Within error: %v", err)
	}
	if result {
		t.Errorf("expected false for point not within polygon")
	}
}

func TestGeometryEngine_ST_Contains(t *testing.T) {
	resultVal := true
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{ScanFunc: func(dest ...any) error {
				if len(dest) > 0 {
					if b, ok := dest[0].(*bool); ok {
						*b = resultVal
					}
				}
				return nil
			}}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	result, err := engine.ST_Contains(ctx, "POLYGON((0 0, 2 0, 2 2, 0 2, 0 0))", "POINT(1 1)")
	if err != nil {
		t.Fatalf("ST_Contains error: %v", err)
	}
	if !result {
		t.Errorf("expected true for polygon containing point")
	}
}

func TestGeometryEngine_ST_DWithin(t *testing.T) {
	resultVal := true
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{ScanFunc: func(dest ...any) error {
				if len(dest) > 0 {
					if b, ok := dest[0].(*bool); ok {
						*b = resultVal
					}
				}
				return nil
			}}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	result, err := engine.ST_DWithin(ctx, "POINT(1 1)", "POINT(2 2)", 1000.0)
	if err != nil {
		t.Fatalf("ST_DWithin error: %v", err)
	}
	if !result {
		t.Errorf("expected true for points within 1000m")
	}
}

func TestGeometryEngine_ST_Distance(t *testing.T) {
	resultVal := 1414.21
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{ScanFunc: func(dest ...any) error {
				if len(dest) > 0 {
					if f, ok := dest[0].(*float64); ok {
						*f = resultVal
					}
				}
				return nil
			}}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	dist, err := engine.ST_Distance(ctx, "POINT(1 1)", "POINT(2 2)")
	if err != nil {
		t.Fatalf("ST_Distance error: %v", err)
	}
	if dist < 0 {
		t.Errorf("expected positive distance, got %f", dist)
	}
}

func TestGeometryEngine_ST_Area(t *testing.T) {
	resultVal := 10000.0
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{ScanFunc: func(dest ...any) error {
				if len(dest) > 0 {
					if f, ok := dest[0].(*float64); ok {
						*f = resultVal
					}
				}
				return nil
			}}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	area, err := engine.ST_Area(ctx, "POLYGON((0 0, 100 0, 100 100, 0 100, 0 0))")
	if err != nil {
		t.Fatalf("ST_Area error: %v", err)
	}
	if area <= 0 {
		t.Errorf("expected positive area, got %f", area)
	}
}

func TestGeometryEngine_ST_Centroid(t *testing.T) {
	lonVal := 121.5
	latVal := 25.0
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{ScanFunc: func(dest ...any) error {
				if len(dest) >= 2 {
					if f, ok := dest[0].(*float64); ok {
						*f = lonVal
					}
					if f, ok := dest[1].(*float64); ok {
						*f = latVal
					}
				}
				return nil
			}}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	lon, lat, err := engine.ST_Centroid(ctx, "POLYGON((0 0, 100 0, 100 100, 0 100, 0 0))")
	if err != nil {
		t.Fatalf("ST_Centroid error: %v", err)
	}
	if lon == 0 && lat == 0 {
		t.Errorf("expected non-zero centroid")
	}
}

func TestGeometryEngine_ImmutableInput(t *testing.T) {
	originalA := "POINT(1 1)"
	originalB := "POINT(2 2)"
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{ScanFunc: func(dest ...any) error { return nil }}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	_, _ = engine.ST_Intersects(ctx, originalA, originalB)
	if originalA != "POINT(1 1)" || originalB != "POINT(2 2)" {
		t.Errorf("inputs were modified: a=%q b=%q", originalA, originalB)
	}
}

func TestGeometryEngine_StripSRIDFromInput(t *testing.T) {
	var capturedA, capturedB string
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if len(args) >= 2 {
				capturedA = args[0].(string)
				capturedB = args[1].(string)
			}
			return &MockRow{ScanFunc: func(dest ...any) error {
				if len(dest) > 0 {
					if b, ok := dest[0].(*bool); ok {
						*b = true
					}
				}
				return nil
			}}
		},
	}
	engine := NewGeometryEngine(db)
	ctx := context.Background()
	_, _ = engine.ST_Intersects(ctx, "SRID=3826;POINT(1 1)", "SRID=3826;POINT(2 2)")
	if capturedA != "POINT(1 1)" || capturedB != "POINT(2 2)" {
		t.Errorf("SRID not stripped: a=%q b=%q", capturedA, capturedB)
	}
}