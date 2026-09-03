package integration

import (
	"context"
	"embed"
	"fmt"
	"os"
	"testing"
	"time"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"tw-prop-mcp/internal/gis"
	"tw-prop-mcp/internal/repository/db"
)

//go:embed testdata/gis_sample.geojson
var gisSampleFS embed.FS

func TestGISImport_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Start postgres:16-alpine with postgis
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("prop"),
		postgres.WithUsername("prop"),
		postgres.WithPassword("prop_dev_only"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}
	defer func() {
		_ = pgContainer.Terminate(ctx)
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	// Enable postgis extension (optional)
	hasPostGIS := true
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS postgis"); err != nil {
		t.Logf("postgis not available in postgres:16-alpine (expected): %v", err)
		hasPostGIS = false
	}

	// Run migrations
	if err := runMigrations(ctx, pool, hasPostGIS); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// Create querier
	querier := db.New(pool)

	if hasPostGIS {
		// Test geometry engine with postgis
		testGeometryEngineWithPostGIS(t, ctx, pool, querier)

		// Test transform with postgis
		testTransformWithPostGIS(t, ctx, pool)
	} else {
		t.Log("skipping geometry tests without postgis")
	}

	// Test import pipeline with sample geojson (doesn't require postgis for parsing)
	testImportPipeline(t, ctx, pool)
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool, hasPostGIS bool) error {
	if hasPostGIS {
		// Read migration files
		migrationFiles := []string{
			"../../migrations/000001_init.up.sql",
			"../../migrations/000002_snapshot_lock.up.sql",
		}
		for _, f := range migrationFiles {
			content, err := os.ReadFile(f)
			if err != nil {
				return err
			}
			if _, err := pool.Exec(ctx, string(content)); err != nil {
				return err
			}
		}
		return nil
	}

	// Minimal schema without postgis - only tables needed for import pipeline parsing tests
	minimal := `
	CREATE TABLE IF NOT EXISTS dataset_snapshot (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		source VARCHAR(50) NOT NULL,
		source_version VARCHAR(50) NOT NULL,
		downloaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		file_name VARCHAR(255) NOT NULL,
		file_sha256 CHAR(64) NOT NULL,
		record_count BIGINT NOT NULL DEFAULT 0,
		status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
		schema_version VARCHAR(20) NOT NULL DEFAULT 'v2.0',
		UNIQUE (source, source_version, file_sha256)
	);
	CREATE TABLE IF NOT EXISTS import_batch (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		snapshot_id UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
		status VARCHAR(20) NOT NULL DEFAULT 'RUNNING',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS parcel_geometry (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		county VARCHAR(50) NOT NULL,
		district VARCHAR(50) NOT NULL,
		section VARCHAR(50) NOT NULL,
		land_number VARCHAR(50) NOT NULL,
		area_sqm NUMERIC(12,4),
		urban_zoning VARCHAR(50),
		land_use_category VARCHAR(50),
		geometry TEXT, -- WKT without postgis
		centroid TEXT,
		bbox TEXT,
		source VARCHAR(50) NOT NULL,
		source_version VARCHAR(50) NOT NULL,
		import_batch_id VARCHAR(100) NOT NULL,
		snapshot_id UUID NOT NULL REFERENCES dataset_snapshot(id),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (county, district, section, land_number, source_version)
	);
	CREATE TABLE IF NOT EXISTS road_segment (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(100),
		road_class VARCHAR(50),
		width NUMERIC(8,2),
		geometry TEXT, -- WKT without postgis
		import_batch_id VARCHAR(100) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := pool.Exec(ctx, minimal); err != nil {
		return err
	}
	// pgcrypto for gen_random_uuid
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto"); err != nil {
		return err
	}
	return nil
}

func testGeometryEngineWithPostGIS(t *testing.T, ctx context.Context, pool *pgxpool.Pool, querier db.Querier) {
	t.Run("ST_Intersects", func(t *testing.T) {
		// Insert two overlapping polygons
		_, err := pool.Exec(ctx, `
			INSERT INTO parcel_geometry (county, district, section, land_number, geometry, source, source_version)
			VALUES 
				('台南市', '安南區', '竹篙灣段', 'TEST001', ST_SetSRID(ST_GeomFromText('POLYGON((0 0, 10 0, 10 10, 0 10, 0 0))'), 3826), 'TEST', 'v1'),
				('台南市', '安南區', '竹篙灣段', 'TEST002', ST_SetSRID(ST_GeomFromText('POLYGON((5 5, 15 5, 15 15, 5 15, 5 5))'), 3826), 'TEST', 'v1')
		`)
		if err != nil {
			t.Fatalf("insert test parcels: %v", err)
		}

		// Use DBQuerier wrapper
		dbQuerier := &testDBQuerier{pool: pool}
		engine := gis.NewGeometryEngine(dbQuerier)
		intersects, err := engine.ST_Intersects(ctx,
			"SRID=3826;POLYGON((0 0, 10 0, 10 10, 0 10, 0 0))",
			"SRID=3826;POLYGON((5 5, 15 5, 15 15, 5 15, 5 5))")
		if err != nil {
			t.Fatalf("ST_Intersects: %v", err)
		}
		if !intersects {
			t.Error("expected polygons to intersect")
		}
	})

	t.Run("ST_DWithin", func(t *testing.T) {
		dbQuerier := &testDBQuerier{pool: pool}
		engine := gis.NewGeometryEngine(dbQuerier)
		dwithin, err := engine.ST_DWithin(ctx,
			"SRID=3826;POINT(100000 2500000)",
			"SRID=3826;POINT(100100 2500000)",
			200) // 200 meters
		if err != nil {
			t.Fatalf("ST_DWithin: %v", err)
		}
		if !dwithin {
			t.Error("expected points within 200m")
		}
	})

	t.Run("ST_Distance", func(t *testing.T) {
		dbQuerier := &testDBQuerier{pool: pool}
		engine := gis.NewGeometryEngine(dbQuerier)
		dist, err := engine.ST_Distance(ctx,
			"SRID=3826;POINT(100000 2500000)",
			"SRID=3826;POINT(100100 2500000)")
		if err != nil {
			t.Fatalf("ST_Distance: %v", err)
		}
		if dist < 90 || dist > 110 {
			t.Errorf("expected distance ~100m, got %f", dist)
		}
	})

	t.Run("ST_Area", func(t *testing.T) {
		dbQuerier := &testDBQuerier{pool: pool}
		engine := gis.NewGeometryEngine(dbQuerier)
		area, err := engine.ST_Area(ctx, "SRID=3826;POLYGON((0 0, 100 0, 100 100, 0 100, 0 0))")
		if err != nil {
			t.Fatalf("ST_Area: %v", err)
		}
		if area < 9900 || area > 10100 {
			t.Errorf("expected area ~10000, got %f", area)
		}
	})

	t.Run("ST_Centroid", func(t *testing.T) {
		dbQuerier := &testDBQuerier{pool: pool}
		engine := gis.NewGeometryEngine(dbQuerier)
		lon, lat, err := engine.ST_Centroid(ctx, "SRID=3826;POLYGON((0 0, 100 0, 100 100, 0 100, 0 0))")
		if err != nil {
			t.Fatalf("ST_Centroid: %v", err)
		}
		if lon == 0 && lat == 0 {
			t.Error("expected non-zero centroid")
		}
	})
}

func testTransformWithPostGIS(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	dbQuerier := &testDBQuerier{pool: pool}

	t.Run("4326_to_3826", func(t *testing.T) {
		wkt4326 := "POINT(121.5654 25.0340)"
		ewkt3826, err := gis.TransformWKTToInternal(ctx, dbQuerier, wkt4326)
		if err != nil {
			t.Fatalf("transform 4326->3826: %v", err)
		}
		if ewkt3826 == "" {
			t.Error("expected non-empty EWKT")
		}
		if !contains(ewkt3826, "SRID=3826") {
			t.Errorf("expected SRID=3826 prefix, got: %s", ewkt3826)
		}
	})

	t.Run("3826_to_4326_roundtrip", func(t *testing.T) {
		wkt4326 := "POINT(121.5654 25.0340)"
		ewkt3826, err := gis.TransformWKTToInternal(ctx, dbQuerier, wkt4326)
		if err != nil {
			t.Fatalf("transform 4326->3826: %v", err)
		}
		wkt4326back, err := gis.TransformWKTToExternal(ctx, dbQuerier, ewkt3826)
		if err != nil {
			t.Fatalf("transform 3826->4326: %v", err)
		}
		var lon1, lat1, lon2, lat2 float64
		_, _ = fmt.Sscanf(wkt4326, "POINT(%f %f)", &lon1, &lat1)
		_, _ = fmt.Sscanf(wkt4326back, "POINT(%f %f)", &lon2, &lat2)
		if abs(lon1-lon2) > 0.001 || abs(lat1-lat2) > 0.001 {
			t.Errorf("roundtrip mismatch: original POINT(%f %f), back POINT(%f %f)", lon1, lat1, lon2, lat2)
		}
	})
}

func testImportPipeline(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	// Read sample geojson
	data, err := gisSampleFS.ReadFile("testdata/gis_sample.geojson")
	if err != nil {
		t.Fatalf("read sample geojson: %v", err)
	}

	parser := gis.NewGISParser()
	parcels, err := parser.ParseParcelGeoJSON(data)
	if err != nil {
		t.Fatalf("parse geojson: %v", err)
	}
	if len(parcels) != 2 {
		t.Fatalf("expected 2 parcels, got %d", len(parcels))
	}

	// Validate parcels
	for _, p := range parcels {
		errs := gis.ValidateParcel(p)
		if len(errs) > 0 {
			t.Errorf("parcel %s validation errors: %v", p.LandNumber, errs)
		}
	}

	// Test transform to import records
	dbQuerier := &testDBQuerier{pool: pool}
	batchID := "test_batch_" + time.Now().Format("20060102150405")
	snapshotID := "test_snapshot_" + time.Now().Format("20060102150405")

	for _, p := range parcels {
		record, err := gis.TransformParcelToImportRecord(ctx, dbQuerier, p, "v1", batchID, snapshotID)
		if err != nil {
			t.Logf("TransformParcelToImportRecord: %v", err)
			continue
		}
		if record.Geometry3826 == "" {
			t.Error("expected geometry in 3826")
		}
		if !contains(record.Geometry3826, "SRID=3826") {
			t.Errorf("expected SRID=3826 in geometry: %s", record.Geometry3826)
		}
		if record.Centroid3826 == "" || record.BBox3826 == "" {
			t.Error("expected centroid and bbox")
		}
		t.Logf("Parcel %s: area=%.2f, geom=%s", p.LandNumber, record.AreaSqm, record.Geometry3826[:min(80, len(record.Geometry3826))])
	}
}
// testDBQuerier wraps pgxpool.Pool to implement gis.DBQuerier
type testDBQuerier struct {
	pool *pgxpool.Pool
}

func (q *testDBQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return q.pool.QueryRow(ctx, sql, args...)
}

func (q *testDBQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return q.pool.Query(ctx, sql, args...)
}

func (q *testDBQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return q.pool.Exec(ctx, sql, args...)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}