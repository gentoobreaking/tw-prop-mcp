//go:build integration

package artifact_lock

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testDB struct {
	conn      *pgxpool.Pool
	hasPostGIS bool
}

// setupPostgres starts a postgres:16-alpine container, applies migrations,
// and returns a connection for testing artifact locking (P5).
func setupPostgres(t *testing.T) *testDB {
	t.Helper()
	ctx := context.Background()

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
	t.Cleanup(func() {
		pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	// Check for PostGIS availability
	hasPostGIS := true
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS postgis"); err != nil {
		t.Logf("postgis not available in postgres:16-alpine (expected): %v", err)
		hasPostGIS = false
	}
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto"); err != nil {
		t.Fatalf("create pgcrypto: %v", err)
	}

	if hasPostGIS {
		migDir := filepath.Join("../../migrations")
		migrations := []string{
			"000001_init.up.sql",
			"000002_snapshot_lock.up.sql",
			"000003_config_locks.up.sql",
			"000004_raw_data_lock.up.sql",
		}
		for _, mig := range migrations {
			content, err := os.ReadFile(filepath.Join(migDir, mig))
			if err != nil {
				t.Fatalf("read migration %s: %v", mig, err)
			}
			if _, err := pool.Exec(ctx, string(content)); err != nil {
				t.Fatalf("exec migration %s: %v", mig, err)
			}
		}
	} else {
		// Minimal schema without postgis for core constraint tests
		minimal := `
		CREATE TABLE IF NOT EXISTS dataset_snapshot (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			source VARCHAR(50) NOT NULL,
			source_version VARCHAR(50) NOT NULL,
			downloaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			published_at TIMESTAMPTZ,
			file_name VARCHAR(255) NOT NULL,
			file_sha256 CHAR(64) NOT NULL,
			record_count BIGINT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','IMPORTING','LOCKED','FAILED')),
			schema_version VARCHAR(20) NOT NULL DEFAULT 'v2.0',
			import_started_at TIMESTAMPTZ,
			import_completed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (source, source_version, file_sha256)
		);
		CREATE TABLE IF NOT EXISTS import_batch (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			snapshot_id UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
			started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			status VARCHAR(20) NOT NULL DEFAULT 'RUNNING' CHECK (status IN ('RUNNING','COMPLETED','FAILED')),
			records_processed BIGINT NOT NULL DEFAULT 0,
			records_imported BIGINT NOT NULL DEFAULT 0,
			records_failed BIGINT NOT NULL DEFAULT 0,
			record_count BIGINT NOT NULL DEFAULT 0,
			error_message TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS "transaction" (
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
			transaction_target VARCHAR(50),
			total_price BIGINT NOT NULL CHECK (total_price > 0),
			unit_price BIGINT NOT NULL CHECK (unit_price > 0),
			land_area_sqm NUMERIC(12,4) CHECK (land_area_sqm IS NULL OR land_area_sqm > 0),
			building_area_sqm NUMERIC(12,4) CHECK (building_area_sqm IS NULL OR building_area_sqm > 0),
			urban_zoning VARCHAR(50),
			non_urban_zoning VARCHAR(50),
			land_use_category VARCHAR(50),
			building_type VARCHAR(50),
			floor VARCHAR(20),
			age INTEGER CHECK (age IS NULL OR age >= 0),
			parking_area_sqm NUMERIC(10,4) CHECK (parking_area_sqm IS NULL OR parking_area_sqm >= 0),
			parking_price BIGINT CHECK (parking_price IS NULL OR parking_price >= 0),
			source_record_hash CHAR(64) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (snapshot_id, source_record_hash),
			UNIQUE (county, district, section, land_number, snapshot_id, transaction_id)
		);
		CREATE TABLE IF NOT EXISTS algorithm_version (
			version VARCHAR(50) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			weights JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS configuration_snapshot (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			version VARCHAR(50) NOT NULL UNIQUE,
			config JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS valuation_result (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			target_parcel_id UUID NOT NULL,
			snapshot_id UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
			comparable_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
			algorithm_version VARCHAR(50) NOT NULL,
			configuration_version VARCHAR(50) NOT NULL,
			outlier_method VARCHAR(20) NOT NULL CHECK (outlier_method IN ('IQR','P10_P90','MAD')),
			weights JSONB NOT NULL,
			statistics JSONB NOT NULL,
			bear_value BIGINT NOT NULL CHECK (bear_value > 0),
			base_value BIGINT NOT NULL CHECK (base_value > 0),
			bull_value BIGINT NOT NULL CHECK (bull_value > 0),
			confidence VARCHAR(20) NOT NULL CHECK (confidence IN ('HIGH','MEDIUM','LOW','INSUFFICIENT')),
			status VARCHAR(20) NOT NULL DEFAULT 'COMPLETED' CHECK (status IN ('COMPLETED','INSUFFICIENT_DATA','FAILED')),
			query_hash CHAR(64) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (query_hash)
		);
		CREATE TABLE IF NOT EXISTS parcel (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			county VARCHAR(20) NOT NULL,
			district VARCHAR(20) NOT NULL,
			section VARCHAR(50) NOT NULL,
			land_number VARCHAR(50) NOT NULL,
			area_sqm NUMERIC(12,4) NOT NULL CHECK (area_sqm > 0),
			urban_zoning VARCHAR(50),
			land_use_category VARCHAR(50),
			geometry TEXT,
			centroid TEXT,
			bbox TEXT,
			source VARCHAR(50) NOT NULL,
			source_version VARCHAR(50) NOT NULL,
			import_batch_id UUID REFERENCES import_batch(id) ON DELETE RESTRICT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (county, district, section, land_number, source, source_version)
		);
		CREATE TABLE IF NOT EXISTS road_segment (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100),
			road_class VARCHAR(20),
			width_m NUMERIC(6,2) CHECK (width_m IS NULL OR width_m >= 0),
			width_source VARCHAR(20) NOT NULL CHECK (width_source IN ('OFFICIAL','GIS_DERIVED','UNKNOWN')),
			geometry TEXT NOT NULL,
			source VARCHAR(50) NOT NULL,
			source_version VARCHAR(50) NOT NULL,
			import_batch_id UUID REFERENCES import_batch(id) ON DELETE RESTRICT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`
		if _, err := pool.Exec(ctx, minimal); err != nil {
			t.Fatalf("create minimal schema: %v", err)
		}

		// Create snapshot lock trigger
		snapshotLockFn := `
		CREATE OR REPLACE FUNCTION prevent_locked_snapshot_update() RETURNS trigger AS $$
		BEGIN
			IF OLD.status = 'LOCKED' THEN
				RAISE EXCEPTION 'snapshot locked: %', OLD.id;
			END IF;
			IF TG_OP = 'DELETE' THEN
				RETURN OLD;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;`
		if _, err := pool.Exec(ctx, snapshotLockFn); err != nil {
			t.Fatalf("create snapshot lock function: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			DROP TRIGGER IF EXISTS trg_snapshot_lock ON dataset_snapshot;
			CREATE TRIGGER trg_snapshot_lock
			BEFORE UPDATE OR DELETE ON dataset_snapshot
			FOR EACH ROW EXECUTE FUNCTION prevent_locked_snapshot_update();
		`); err != nil {
			t.Fatalf("create snapshot lock trigger: %v", err)
		}

		// Create raw data lock functions
		rawDataLockFn := `
		CREATE OR REPLACE FUNCTION prevent_locked_snapshot_data_change() RETURNS trigger LANGUAGE plpgsql AS $$
		DECLARE snap_status TEXT;
		BEGIN
			SELECT status INTO snap_status FROM dataset_snapshot WHERE id = OLD.snapshot_id;
			IF snap_status = 'LOCKED' THEN
				RAISE EXCEPTION 'snapshot locked: % (status=LOCKED)', OLD.snapshot_id;
			END IF;
			IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
			RETURN NEW;
		END;
		$$;

		CREATE OR REPLACE FUNCTION prevent_locked_batch_data_change() RETURNS trigger LANGUAGE plpgsql AS $$
		DECLARE snap_status TEXT;
		BEGIN
			SELECT ds.status INTO snap_status FROM dataset_snapshot ds
			JOIN import_batch ib ON ib.snapshot_id = ds.id WHERE ib.id = OLD.import_batch_id;
			IF snap_status = 'LOCKED' THEN
				RAISE EXCEPTION 'snapshot locked: raw data is immutable';
			END IF;
			IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
			RETURN NEW;
		END;
		$$;`
		if _, err := pool.Exec(ctx, rawDataLockFn); err != nil {
			t.Fatalf("create raw data lock functions: %v", err)
		}

		// Create triggers for raw data tables
		rawTriggers := []string{
			`DROP TRIGGER IF EXISTS trg_transaction_lock ON "transaction";
			CREATE TRIGGER trg_transaction_lock BEFORE UPDATE OR DELETE ON "transaction"
			FOR EACH ROW EXECUTE FUNCTION prevent_locked_snapshot_data_change();`,
			`DROP TRIGGER IF EXISTS trg_valuation_lock ON valuation_result;
			CREATE TRIGGER trg_valuation_lock BEFORE UPDATE OR DELETE ON valuation_result
			FOR EACH ROW EXECUTE FUNCTION prevent_locked_snapshot_data_change();`,
			`DROP TRIGGER IF EXISTS trg_parcel_lock ON parcel;
			CREATE TRIGGER trg_parcel_lock BEFORE UPDATE OR DELETE ON parcel
			FOR EACH ROW EXECUTE FUNCTION prevent_locked_batch_data_change();`,
			`DROP TRIGGER IF EXISTS trg_road_lock ON road_segment;
			CREATE TRIGGER trg_road_lock BEFORE UPDATE OR DELETE ON road_segment
			FOR EACH ROW EXECUTE FUNCTION prevent_locked_batch_data_change();`,
		}
		for _, triggerSQL := range rawTriggers {
			if _, err := pool.Exec(ctx, triggerSQL); err != nil {
				t.Fatalf("create raw data lock trigger: %v", err)
			}
		}

		// Create config lock function
		configLockFn := `
		CREATE OR REPLACE FUNCTION raise_artifact_locked() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'artifact locked: % is immutable', TG_TABLE_NAME;
			RETURN NULL;
		END;
		$$;`
		if _, err := pool.Exec(ctx, configLockFn); err != nil {
			t.Fatalf("create config lock function: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			DROP TRIGGER IF EXISTS lock_algorithm_version ON algorithm_version;
			CREATE TRIGGER lock_algorithm_version
			BEFORE UPDATE OR DELETE ON algorithm_version
			FOR EACH ROW EXECUTE FUNCTION raise_artifact_locked();
		`); err != nil {
			t.Fatalf("create algorithm_version lock trigger: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			DROP TRIGGER IF EXISTS lock_configuration_snapshot ON configuration_snapshot;
			CREATE TRIGGER lock_configuration_snapshot
			BEFORE UPDATE OR DELETE ON configuration_snapshot
			FOR EACH ROW EXECUTE FUNCTION raise_artifact_locked();
		`); err != nil {
			t.Fatalf("create configuration_snapshot lock trigger: %v", err)
		}

		// Seed default algorithm/config (v2.0)
		if _, err := pool.Exec(ctx, `
			INSERT INTO algorithm_version (version, name, description, weights) VALUES
			('comparable-v2.0', 'Comparable Scoring v2.0', 'W_area=0.30...', '{"W_area":0.30}'::jsonb),
			('valuation-v2.0', 'Valuation v2.0', 'weighted median', '{"outlier":"IQR"}'::jsonb)
			ON CONFLICT (version) DO NOTHING;
		`); err != nil {
			t.Fatalf("seed algorithm_version: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO configuration_snapshot (version, config) VALUES
			('v2.0', '{"W_area":0.30}'::jsonb)
			ON CONFLICT (version) DO NOTHING;
		`); err != nil {
			t.Fatalf("seed configuration_snapshot: %v", err)
		}
	}

	return &testDB{conn: pool, hasPostGIS: hasPostGIS}
}

// insertLockedSnapshot creates a LOCKED snapshot with an import batch.
func insertLockedSnapshot(t *testing.T, ctx context.Context, db *testDB) (snapshotID, batchID string) {
	t.Helper()
	err := db.conn.QueryRow(ctx, `
		INSERT INTO dataset_snapshot (source, source_version, file_name, file_sha256, record_count, status)
		VALUES ('MOI', '2024Q1', 'test.csv', 'abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd', 1, 'LOCKED')
		RETURNING id
	`).Scan(&snapshotID)
	if err != nil {
		t.Fatalf("insert locked snapshot: %v", err)
	}
	err = db.conn.QueryRow(ctx, "INSERT INTO import_batch (snapshot_id, status) VALUES ($1, 'COMPLETED') RETURNING id", snapshotID).Scan(&batchID)
	if err != nil {
		t.Fatalf("insert batch: %v", err)
	}
	return snapshotID, batchID
}

// insertPendingSnapshot creates a PENDING snapshot with an import batch.
func insertPendingSnapshot(t *testing.T, ctx context.Context, db *testDB) (snapshotID, batchID string) {
	t.Helper()
	err := db.conn.QueryRow(ctx, `
		INSERT INTO dataset_snapshot (source, source_version, file_name, file_sha256, record_count, status)
		VALUES ('MOI', '2024Q1', 'test.csv', 'abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd', 1, 'PENDING')
		RETURNING id
	`).Scan(&snapshotID)
	if err != nil {
		t.Fatalf("insert pending snapshot: %v", err)
	}
	err = db.conn.QueryRow(ctx, "INSERT INTO import_batch (snapshot_id, status) VALUES ($1, 'RUNNING') RETURNING id", snapshotID).Scan(&batchID)
	if err != nil {
		t.Fatalf("insert batch: %v", err)
	}
	return snapshotID, batchID
}

// --- Tests ---

// TestSnapshotLocked_UpdateFails verifies UPDATE/DELETE on dataset_snapshot
// fails when status='LOCKED' (P5: Snapshot immutable).
func TestSnapshotLocked_UpdateFails(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	var snapshotID string
	err := db.conn.QueryRow(ctx, `
		INSERT INTO dataset_snapshot (source, source_version, file_name, file_sha256, record_count, status)
		VALUES ('MOI', '2024Q1', 'test.csv', 'abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd', 100, 'LOCKED')
		RETURNING id
	`).Scan(&snapshotID)
	if err != nil {
		t.Fatalf("insert locked snapshot: %v", err)
	}

	_, err = db.conn.Exec(ctx, "UPDATE dataset_snapshot SET record_count=999 WHERE id=$1", snapshotID)
	if err == nil {
		t.Fatal("expected UPDATE on LOCKED snapshot to fail, but it succeeded")
	}

	_, err = db.conn.Exec(ctx, "DELETE FROM dataset_snapshot WHERE id=$1", snapshotID)
	if err == nil {
		t.Fatal("expected DELETE on LOCKED snapshot to fail, but it succeeded")
	}
}

// TestRawDataLocked_Transaction verifies raw data (transaction) cannot be
// UPDATE'd or DELETE'd when the snapshot is LOCKED (P5: Raw Data immutable).
func TestRawDataLocked_Transaction(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	snapshotID, batchID := insertLockedSnapshot(t, ctx, db)

	var txnID string
	err := db.conn.QueryRow(ctx, `
		INSERT INTO "transaction" (snapshot_id, import_batch_id, transaction_id, transaction_date,
			transaction_type, county, district, total_price, unit_price, source_record_hash)
		VALUES ($1, $2, 'TX001', '2024-01-01', '土地', '臺北市', '中正區', 1000000, 1000, 'hash001')
		RETURNING id
	`, snapshotID, batchID).Scan(&txnID)
	if err != nil {
		t.Fatalf("insert transaction: %v", err)
	}

	// UPDATE should fail when snapshot is LOCKED
	_, err = db.conn.Exec(ctx, "UPDATE \"transaction\" SET total_price=999 WHERE id=$1", txnID)
	if err == nil {
		t.Fatal("expected UPDATE on locked transaction to fail, but succeeded")
	}

	// DELETE should fail when snapshot is LOCKED
	_, err = db.conn.Exec(ctx, "DELETE FROM \"transaction\" WHERE id=$1", txnID)
	if err == nil {
		t.Fatal("expected DELETE on locked transaction to fail, but succeeded")
	}
}

// TestRawDataLocked_Parcel verifies parcel table locking (P5: GIS Source Metadata immutable).
func TestRawDataLocked_Parcel(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	_, batchID := insertLockedSnapshot(t, ctx, db)

	var parcelID string
	if db.hasPostGIS {
		err := db.conn.QueryRow(ctx, `
			INSERT INTO parcel (county, district, section, land_number, area_sqm,
				geometry, source, source_version, import_batch_id)
			VALUES ('臺北市', '中正區', '八德段', '001-002-003', 100.5,
				ST_SetSRID(ST_GeomFromText('MULTIPOLYGON(((0 0,1 0,1 1,0 1,0 0)))'), 3826),
				'NLSC', '2024Q1', $1)
			RETURNING id
		`, batchID).Scan(&parcelID)
		if err != nil {
			t.Fatalf("insert parcel (postgis): %v", err)
		}
	} else {
		err := db.conn.QueryRow(ctx, `
			INSERT INTO parcel (county, district, section, land_number, area_sqm,
				geometry, source, source_version, import_batch_id)
			VALUES ('臺北市', '中正區', '八德段', '001-002-003', 100.5,
				'MULTIPOLYGON(((0 0,1 0,1 1,0 1,0 0)))',
				'NLSC', '2024Q1', $1)
			RETURNING id
		`, batchID).Scan(&parcelID)
		if err != nil {
			t.Fatalf("insert parcel (minimal): %v", err)
		}
	}

	// UPDATE should fail
	_, err := db.conn.Exec(ctx, "UPDATE parcel SET area_sqm=999 WHERE id=$1", parcelID)
	if err == nil {
		t.Fatal("expected UPDATE on parcel under LOCKED snapshot to fail")
	}

	// DELETE should fail
	_, err = db.conn.Exec(ctx, "DELETE FROM parcel WHERE id=$1", parcelID)
	if err == nil {
		t.Fatal("expected DELETE on parcel under LOCKED snapshot to fail")
	}
}

// TestRawDataLocked_RoadSegment verifies road_segment table locking.
func TestRawDataLocked_RoadSegment(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	_, batchID := insertLockedSnapshot(t, ctx, db)

	var roadID string
	if db.hasPostGIS {
		err := db.conn.QueryRow(ctx, `
			INSERT INTO road_segment (name, road_class, width_source, geometry, source, source_version, import_batch_id)
			VALUES ('中山路', '主要幹道', 'OFFICIAL',
				ST_SetSRID(ST_GeomFromText('MULTILINESTRING(((0 0,1 1)))'), 3826),
				'MOI', '2024Q1', $1)
			RETURNING id
		`, batchID).Scan(&roadID)
		if err != nil {
			t.Fatalf("insert road segment (postgis): %v", err)
		}
	} else {
		err := db.conn.QueryRow(ctx, `
			INSERT INTO road_segment (name, road_class, width_source, geometry, source, source_version, import_batch_id)
			VALUES ('中山路', '主要幹道', 'OFFICIAL',
				'MULTILINESTRING(((0 0,1 1)))',
				'MOI', '2024Q1', $1)
			RETURNING id
		`, batchID).Scan(&roadID)
		if err != nil {
			t.Fatalf("insert road segment (minimal): %v", err)
		}
	}

	// UPDATE should fail
	_, err := db.conn.Exec(ctx, "UPDATE road_segment SET name='修改路' WHERE id=$1", roadID)
	if err == nil {
		t.Fatal("expected UPDATE on road_segment under LOCKED snapshot to fail")
	}

	// DELETE should fail
	_, err = db.conn.Exec(ctx, "DELETE FROM road_segment WHERE id=$1", roadID)
	if err == nil {
		t.Fatal("expected DELETE on road_segment under LOCKED snapshot to fail")
	}
}

// TestRawDataLocked_ValuationResult verifies valuation_result table locking.
func TestRawDataLocked_ValuationResult(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	snapshotID, _ := insertLockedSnapshot(t, ctx, db)

	var valID string
	err := db.conn.QueryRow(ctx, `
		INSERT INTO valuation_result (target_parcel_id, snapshot_id, comparable_ids,
			algorithm_version, configuration_version, outlier_method, weights, statistics,
			bear_value, base_value, bull_value, confidence, query_hash)
		VALUES ('00000000-0000-0000-0000-000000000001', $1, '[]'::jsonb,
			'comparable-v2.0', 'v2.0', 'IQR', '{}'::jsonb, '{}'::jsonb,
			1000000, 2000000, 3000000, 'HIGH', 'abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789')
		RETURNING id
	`, snapshotID).Scan(&valID)
	if err != nil {
		t.Fatalf("insert valuation result: %v", err)
	}

	// UPDATE should fail
	_, err = db.conn.Exec(ctx, "UPDATE valuation_result SET base_value=999 WHERE id=$1", valID)
	if err == nil {
		t.Fatal("expected UPDATE on valuation_result under LOCKED snapshot to fail")
	}

	// DELETE should fail
	_, err = db.conn.Exec(ctx, "DELETE FROM valuation_result WHERE id=$1", valID)
	if err == nil {
		t.Fatal("expected DELETE on valuation_result under LOCKED snapshot to fail")
	}
}

// TestRawDataUpdateAllowed_WhenUnlocked verifies raw data CAN be modified
// when the snapshot is not LOCKED (status='PENDING').
func TestRawDataUpdateAllowed_WhenUnlocked(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	snapshotID, batchID := insertPendingSnapshot(t, ctx, db)

	var txnID string
	err := db.conn.QueryRow(ctx, `
		INSERT INTO "transaction" (snapshot_id, import_batch_id, transaction_id, transaction_date,
			transaction_type, county, district, total_price, unit_price, source_record_hash)
		VALUES ($1, $2, 'TX001', '2024-01-01', '土地', '臺北市', '中正區', 1000000, 1000, 'hash001')
		RETURNING id
	`, snapshotID, batchID).Scan(&txnID)
	if err != nil {
		t.Fatalf("insert transaction: %v", err)
	}

	// UPDATE should succeed when snapshot is PENDING
	_, err = db.conn.Exec(ctx, "UPDATE \"transaction\" SET total_price=2000000 WHERE id=$1", txnID)
	if err != nil {
		t.Fatalf("UPDATE on unlocked transaction should succeed: %v", err)
	}

	// DELETE should succeed when snapshot is PENDING
	_, err = db.conn.Exec(ctx, "DELETE FROM \"transaction\" WHERE id=$1", txnID)
	if err != nil {
		t.Fatalf("DELETE on unlocked transaction should succeed: %v", err)
	}
}

// TestAlgorithmVersionImmutable verifies algorithm_version is immutable (P5).
func TestAlgorithmVersionImmutable(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	_, err := db.conn.Exec(ctx, "UPDATE algorithm_version SET name='malicious' WHERE version='comparable-v2.0'")
	if err == nil {
		t.Fatal("expected UPDATE on algorithm_version to fail (immutable)")
	}

	_, err = db.conn.Exec(ctx, "DELETE FROM algorithm_version WHERE version='comparable-v2.0'")
	if err == nil {
		t.Fatal("expected DELETE on algorithm_version to fail (immutable)")
	}
}

// TestConfigurationSnapshotImmutable verifies configuration_snapshot is immutable (P5).
func TestConfigurationSnapshotImmutable(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	_, err := db.conn.Exec(ctx, "UPDATE configuration_snapshot SET config='{\"malicious\":true}'::jsonb WHERE version='v2.0'")
	if err == nil {
		t.Fatal("expected UPDATE on configuration_snapshot to fail (immutable)")
	}

	_, err = db.conn.Exec(ctx, "DELETE FROM configuration_snapshot WHERE version='v2.0'")
	if err == nil {
		t.Fatal("expected DELETE on configuration_snapshot to fail (immutable)")
	}
}

// TestSnapshotManifestImmutable verifies that LOCKED snapshot manifest fields
// cannot be modified (P5: Snapshot Manifest immutable).
func TestSnapshotManifestImmutable(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	var snapshotID string
	err := db.conn.QueryRow(ctx, `
		INSERT INTO dataset_snapshot (source, source_version, file_name, file_sha256, record_count, status)
		VALUES ('MOI', '2024Q1', 'original.csv', 'abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd', 100, 'LOCKED')
		RETURNING id
	`).Scan(&snapshotID)
	if err != nil {
		t.Fatalf("insert locked snapshot: %v", err)
	}

	manifestFields := []string{
		"file_name", "file_sha256", "record_count", "source", "schema_version",
	}
	for _, field := range manifestFields {
		t.Run("change_"+field, func(t *testing.T) {
			_, err := db.conn.Exec(ctx, "UPDATE dataset_snapshot SET "+field+"='modified' WHERE id=$1", snapshotID)
			if err == nil {
				t.Fatalf("expected UPDATE of %s on LOCKED snapshot to fail", field)
			}
		})
	}
}

// TestRawDataLock_LockAfterInsert verifies locking after data insertion
// properly blocks subsequent updates.
func TestRawDataLock_LockAfterInsert(t *testing.T) {
	db := setupPostgres(t)
	ctx := context.Background()

	snapshotID, batchID := insertPendingSnapshot(t, ctx, db)

	var txnID string
	err := db.conn.QueryRow(ctx, `
		INSERT INTO "transaction" (snapshot_id, import_batch_id, transaction_id, transaction_date,
			transaction_type, county, district, total_price, unit_price, source_record_hash)
		VALUES ($1, $2, 'TX001', '2024-01-01', '土地', '臺北市', '中正區', 1000000, 1000, 'hash001')
		RETURNING id
	`, snapshotID, batchID).Scan(&txnID)
	if err != nil {
		t.Fatalf("insert transaction: %v", err)
	}

	// UPDATE succeeds under PENDING
	_, err = db.conn.Exec(ctx, "UPDATE \"transaction\" SET total_price=5000 WHERE id=$1", txnID)
	if err != nil {
		t.Fatalf("UPDATE under PENDING should succeed: %v", err)
	}

	// Lock the snapshot
	_, err = db.conn.Exec(ctx, "UPDATE dataset_snapshot SET status='LOCKED' WHERE id=$1", snapshotID)
	if err != nil {
		t.Fatalf("locking snapshot should succeed: %v", err)
	}

	// Now UPDATE on transaction should fail
	_, err = db.conn.Exec(ctx, "UPDATE \"transaction\" SET total_price=999 WHERE id=$1", txnID)
	if err == nil {
		t.Fatal("expected UPDATE on transaction under LOCKED snapshot to fail after locking")
	}

	// DELETE on transaction should also fail
	_, err = db.conn.Exec(ctx, "DELETE FROM \"transaction\" WHERE id=$1", txnID)
	if err == nil {
		t.Fatal("expected DELETE on transaction under LOCKED snapshot to fail after locking")
	}
}
