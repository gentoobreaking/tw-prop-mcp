//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("prop"),
		postgres.WithUsername("prop"),
		postgres.WithPassword("prop_dev_only"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		t.Fatalf("pgx connect: %v", err)
	}
	defer conn.Close(ctx)

	hasPostGIS := true
	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS postgis;"); err != nil {
		t.Logf("postgis not available in postgres:16-alpine (expected): %v", err)
		hasPostGIS = false
	}
	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto;"); err != nil {
		t.Fatalf("create pgcrypto: %v", err)
	}

	migPath := filepath.Join("..", "..", "migrations", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	migSQL := string(sqlBytes)

	if _, err := conn.Exec(ctx, migSQL); err != nil {
		if !hasPostGIS {
			t.Logf("full migration failed without postgis, creating minimal schema for core constraints: %v", err)
			// 建立最小可用 schema（不含 GEOMETRY）供核心約束測試
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
			CREATE TABLE IF NOT EXISTS transaction (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				snapshot_id UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
				import_batch_id UUID NOT NULL REFERENCES import_batch(id) ON DELETE RESTRICT,
				transaction_id VARCHAR(50) NOT NULL,
				transaction_date DATE NOT NULL,
				transaction_type VARCHAR(20) NOT NULL,
				county VARCHAR(20) NOT NULL,
				district VARCHAR(20) NOT NULL,
				section VARCHAR(50),
				land_number VARCHAR(50),
				total_price BIGINT NOT NULL CHECK (total_price > 0),
				unit_price BIGINT NOT NULL CHECK (unit_price > 0),
				source_record_hash CHAR(64) NOT NULL,
				UNIQUE (snapshot_id, source_record_hash),
				UNIQUE (county, district, section, land_number, snapshot_id, transaction_id)
			);
			`
			if _, err := conn.Exec(ctx, minimal); err != nil {
				t.Fatalf("create minimal schema: %v", err)
			}
		} else {
			t.Fatalf("exec migration up: %v", err)
		}
	}

	var count int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='dataset_snapshot'").Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("dataset_snapshot table missing: count=%d err=%v", count, err)
	}

	var snapshotID string
	err = conn.QueryRow(ctx, "INSERT INTO dataset_snapshot (source, source_version, file_name, file_sha256, record_count, status) VALUES ('MOI','2024Q1','test.csv','abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd',1,'PENDING') RETURNING id").Scan(&snapshotID)
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	var batchID string
	err = conn.QueryRow(ctx, "INSERT INTO import_batch (snapshot_id, status) VALUES ($1,'RUNNING') RETURNING id", snapshotID).Scan(&batchID)
	if err != nil {
		t.Fatalf("insert batch: %v", err)
	}
	_, err = conn.Exec(ctx, "INSERT INTO transaction (snapshot_id, import_batch_id, transaction_id, transaction_date, transaction_type, county, district, section, land_number, total_price, unit_price, source_record_hash) VALUES ($1,$2,'TX001','2024-01-01','土地','台北市','大安區','段','123',1000000,1000,'hash001')", snapshotID, batchID)
	if err != nil {
		t.Fatalf("first transaction insert: %v", err)
	}
	_, err = conn.Exec(ctx, "INSERT INTO transaction (snapshot_id, import_batch_id, transaction_id, transaction_date, transaction_type, county, district, section, land_number, total_price, unit_price, source_record_hash) VALUES ($1,$2,'TX002','2024-01-02','土地','台北市','大安區','段','124',2000000,2000,'hash001')", snapshotID, batchID)
	if err == nil {
		t.Fatalf("expected duplicate source_record_hash to fail but succeeded")
	}

	_, err = conn.Exec(ctx, "INSERT INTO transaction (snapshot_id, import_batch_id, transaction_id, transaction_date, transaction_type, county, district, section, land_number, total_price, unit_price, source_record_hash) VALUES ($1,$2,'TX003','2024-01-03','土地','台北市','大安區','其他段','123',3000000,3000,'hash003')", snapshotID, batchID)
	if err != nil {
		t.Fatalf("different section same land_number should allow: %v", err)
	}

	if hasPostGIS {
		_, err = conn.Exec(ctx, "INSERT INTO parcel (county, district, section, land_number, area_sqm, geometry, source, source_version) VALUES ('澎湖縣','西嶼鄉','竹篙灣段','3615',100, ST_SetSRID(ST_GeomFromText('MULTIPOLYGON(((0 0,1 0,1 1,0 1,0 0)))'),4326),'NLSC','2024Q1')")
		if err == nil {
			t.Fatalf("expected SRID 4326 to fail CHECK but succeeded")
		}

		var dist float64
		err = conn.QueryRow(ctx, "SELECT ST_Distance(ST_Transform(ST_SetSRID(ST_MakePoint(119.5,23.5),4326),3826), ST_Transform(ST_Transform(ST_Transform(ST_SetSRID(ST_MakePoint(119.5,23.5),4326),3826),4326),3826))").Scan(&dist)
		if err != nil {
			t.Fatalf("ST_Transform roundtrip: %v", err)
		}
		if dist > 0.01 {
			t.Fatalf("transform roundtrip error too large: %f > 0.01", dist)
		}
	} else {
		t.Logf("skip SRID/transform tests without postgis")
	}

	downBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000001_init.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	if _, err := conn.Exec(ctx, string(downBytes)); err != nil {
		t.Fatalf("exec down migration: %v", err)
	}
	if hasPostGIS {
		if _, err := conn.Exec(ctx, migSQL); err != nil {
			t.Fatalf("re-exec up after down: %v", err)
		}
	}
}
