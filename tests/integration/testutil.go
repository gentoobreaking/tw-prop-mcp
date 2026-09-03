package integration

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations runs the database migrations for integration tests.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, hasPostGIS bool) error {
	if hasPostGIS {
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

	// Minimal schema without postgis
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
		status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
		schema_version VARCHAR(20) NOT NULL DEFAULT 'v2.0',
		import_started_at TIMESTAMPTZ,
		import_completed_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
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
		import_batch_id UUID,
		transaction_id VARCHAR(50) NOT NULL,
		transaction_date DATE NOT NULL,
		transaction_type VARCHAR(10) NOT NULL,
		county VARCHAR(50) NOT NULL,
		district VARCHAR(50) NOT NULL,
		section VARCHAR(50) NOT NULL,
		land_number VARCHAR(50) NOT NULL,
		transaction_target VARCHAR(50),
		total_price BIGINT NOT NULL,
		unit_price BIGINT NOT NULL,
		land_area_sqm NUMERIC(12,4),
		building_area_sqm NUMERIC(12,4),
		urban_zoning VARCHAR(50),
		non_urban_zoning VARCHAR(50),
		land_use_category VARCHAR(50),
		building_type VARCHAR(50),
		floor VARCHAR(50),
		age INT,
		parking_area_sqm NUMERIC(12,4),
		parking_price BIGINT,
		source_record_hash CHAR(64) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (snapshot_id, source_record_hash)
	);
	CREATE TABLE IF NOT EXISTS transaction_land (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		transaction_id UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
		land_type VARCHAR(20) NOT NULL,
		area_sqm NUMERIC(12,4),
		price BIGINT,
		section VARCHAR(50),
		land_number VARCHAR(50)
	);
	CREATE TABLE IF NOT EXISTS transaction_building (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		transaction_id UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
		building_type VARCHAR(50),
		area_sqm NUMERIC(12,4),
		floor VARCHAR(50),
		age INT,
		room_count INT,
		hall_count INT,
		bath_count INT
	);
	CREATE TABLE IF NOT EXISTS parcel (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		county VARCHAR(50) NOT NULL,
		district VARCHAR(50) NOT NULL,
		section VARCHAR(50) NOT NULL,
		land_number VARCHAR(50) NOT NULL,
		area_sqm NUMERIC(12,4),
		urban_zoning VARCHAR(50),
		land_use_category VARCHAR(50),
		geometry TEXT,
		centroid TEXT,
		bbox TEXT,
		source VARCHAR(50) NOT NULL,
		source_version VARCHAR(50) NOT NULL,
		import_batch_id VARCHAR(100),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (county, district, section, land_number, source, source_version)
	);
	CREATE TABLE IF NOT EXISTS parcel_geometry (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		parcel_id UUID NOT NULL REFERENCES parcel(id) ON DELETE CASCADE,
		geometry TEXT NOT NULL,
		centroid TEXT,
		bbox TEXT,
		srid INT NOT NULL DEFAULT 3826,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS road_segment (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(100),
		road_class VARCHAR(50),
		width NUMERIC(8,2),
		geometry TEXT,
		import_batch_id VARCHAR(100),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS parcel_road_access (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		parcel_id UUID NOT NULL REFERENCES parcel(id) ON DELETE CASCADE,
		road_segment_id UUID NOT NULL REFERENCES road_segment(id) ON DELETE CASCADE,
		distance_m NUMERIC(12,4),
		access_type VARCHAR(20),
		algorithm_version VARCHAR(50),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (parcel_id, road_segment_id, algorithm_version)
	);
	CREATE TABLE IF NOT EXISTS comparable_result (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		target_transaction_id UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
		comparable_transaction_id UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
		similarity_score NUMERIC(6,4),
		area_weight NUMERIC(4,3),
		distance_weight NUMERIC(4,3),
		time_weight NUMERIC(4,3),
		zoning_weight NUMERIC(4,3),
		land_use_weight NUMERIC(4,3),
		road_weight NUMERIC(4,3),
		adjusted_price BIGINT,
		algorithm_version VARCHAR(50),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS statistics_engine (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		county VARCHAR(50) NOT NULL,
		district VARCHAR(50) NOT NULL,
		section VARCHAR(50),
		period_start DATE NOT NULL,
		period_end DATE NOT NULL,
		count BIGINT NOT NULL,
		min_price BIGINT,
		max_price BIGINT,
		avg_price BIGINT,
		median_price BIGINT,
		p25_price BIGINT,
		p75_price BIGINT,
		algorithm_version VARCHAR(50),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (county, district, section, period_start, period_end, algorithm_version)
	);
	CREATE TABLE IF NOT EXISTS valuation_result (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		target_transaction_id UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
		estimated_price BIGINT NOT NULL,
		confidence_low BIGINT,
		confidence_high BIGINT,
		configuration_version INT NOT NULL,
		algorithm_version VARCHAR(50),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS algorithm_version (
		name VARCHAR(50) PRIMARY KEY,
		description TEXT,
		weights JSONB NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS configuration_snapshot (
		version INT PRIMARY KEY,
		config JSONB NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := pool.Exec(ctx, minimal); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto"); err != nil {
		return err
	}
	return nil
}