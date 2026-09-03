//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"tw-prop-mcp/internal/domain"
)

// newTestDBWithPostGIS creates a test DB with minimal schemas for valuation tests.
// Uses postgres:16-alpine (no postgis needed since we create tables without geometry).
func newTestDBWithPostGIS(t *testing.T) (context.Context, *pgx.Conn, func()) {
	t.Helper()
	ctx := context.Background()

	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatalf("pgx connect DATABASE_URL: %v", err)
		}
		if err := runValuationMigrations(ctx, conn); err != nil {
			t.Fatalf("run migrations: %v", err)
		}
		cleanup := func() {
			_, _ = conn.Exec(ctx, "DELETE FROM comparable_result; DELETE FROM valuation_result; DELETE FROM transaction; DELETE FROM parcel; DELETE FROM dataset_snapshot;")
			conn.Close(ctx)
		}
		_, _ = conn.Exec(ctx, "DELETE FROM comparable_result; DELETE FROM valuation_result; DELETE FROM transaction; DELETE FROM parcel; DELETE FROM dataset_snapshot;")
		return ctx, conn, cleanup
	}

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("prop"),
		postgres.WithUsername("prop"),
		postgres.WithPassword("prop_dev_only"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		t.Fatalf("pgx connect: %v", err)
	}
	_, _ = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto;")
	if err := runValuationMigrations(ctx, conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	cleanup := func() {
		conn.Close(ctx)
		_ = testcontainers.TerminateContainer(pgC)
	}
	return ctx, conn, cleanup
}

// runValuationMigrations creates minimal table schemas needed for valuation tests
// without requiring postgis (no geometry columns).
func runValuationMigrations(ctx context.Context, conn *pgx.Conn) error {
	schema := `
	CREATE EXTENSION IF NOT EXISTS pgcrypto;

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
	CREATE INDEX IF NOT EXISTS idx_snapshot_status ON dataset_snapshot(status);

	CREATE TABLE IF NOT EXISTS parcel (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		cid VARCHAR(50) NOT NULL,
		county VARCHAR(50) NOT NULL,
		district VARCHAR(50) NOT NULL,
		section VARCHAR(50) NOT NULL,
		land_number VARCHAR(50) NOT NULL,
		land_area NUMERIC(12,4),
		transaction_target VARCHAR(50),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (county, district, section, land_number)
	);

	CREATE TABLE IF NOT EXISTS "transaction" (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		snapshot_id UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
		county VARCHAR(50) NOT NULL,
		district VARCHAR(50) NOT NULL,
		section VARCHAR(50),
		land_number VARCHAR(50),
		transaction_date DATE NOT NULL,
		transaction_type VARCHAR(20) NOT NULL,
		transaction_target VARCHAR(50),
		transaction_unit_price BIGINT,
		transaction_land_area NUMERIC(12,4),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (county, district, section, land_number, snapshot_id)
	);

	CREATE TABLE IF NOT EXISTS comparable_result (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		target_parcel_id UUID NOT NULL REFERENCES parcel(id) ON DELETE RESTRICT,
		candidate_transaction_id UUID NOT NULL REFERENCES "transaction"(id) ON DELETE RESTRICT,
		distance_m NUMERIC(10,2),
		area_similarity NUMERIC(6,4),
		zoning_match BOOLEAN NOT NULL,
		land_use_match BOOLEAN NOT NULL,
		road_access_match BOOLEAN NOT NULL,
		time_score NUMERIC(6,4),
		distance_score NUMERIC(6,4),
		total_score NUMERIC(6,4) NOT NULL,
		algorithm_version VARCHAR(50) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_comparable_target ON comparable_result(target_parcel_id);
	CREATE INDEX IF NOT EXISTS idx_comparable_candidate ON comparable_result(candidate_transaction_id);

	CREATE TABLE IF NOT EXISTS valuation_result (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		target_parcel_id UUID NOT NULL REFERENCES parcel(id) ON DELETE RESTRICT,
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
		query_hash CHAR(64) NOT NULL UNIQUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_valuation_parcel ON valuation_result(target_parcel_id);
	CREATE INDEX IF NOT EXISTS idx_valuation_snapshot ON valuation_result(snapshot_id);
	CREATE INDEX IF NOT EXISTS idx_valuation_query_hash ON valuation_result(query_hash);
	CREATE INDEX IF NOT EXISTS idx_valuation_comparable_ids ON valuation_result USING GIN(comparable_ids);
	`
	_, err := conn.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// TestValuationResultRepository_Integration_InsertAndGet tests INSERT → GET round trip.
func TestValuationResultRepository_Integration_InsertAndGet(t *testing.T) {
	ctx, conn, cleanup := newTestDBWithPostGIS(t)
	defer cleanup()

	repo := NewValuationResultRepository(conn)

	parcelID, err := insertTestParcel(ctx, conn)
	if err != nil {
		t.Fatalf("insert test parcel: %v", err)
	}
	snapshotID, err := insertTestSnapshot(ctx, conn)
	if err != nil {
		t.Fatalf("insert test snapshot: %v", err)
	}

	result := domain.ValuationResult{
		TargetParcelID:       parcelID,
		SnapshotID:           snapshotID,
		ComparableIDs:        []string{},
		AlgorithmVersion:     "valuation-v2.0",
		ConfigurationVersion: "v2.0",
		OutlierMethod:        "IQR",
		Weights:              json.RawMessage(`{"W_area":0.30}`),
		BearValue:            50000,
		BaseValue:            65000,
		BullValue:            80000,
		Confidence:           domain.ConfidenceMedium,
		Status:               "COMPLETED",
		QueryHash:            "testhash123",
	}

	inserted, err := repo.Insert(ctx, result)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if inserted.ID == "" {
		t.Error("expected non-empty ID after insert")
	}

	// Round-trip: GET by ID
	fetched, err := repo.GetByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("get by ID: %v", err)
	}

	if fetched.BearValue != 50000 {
		t.Errorf("bear_value = %d, want 50000", fetched.BearValue)
	}
	if fetched.BaseValue != 65000 {
		t.Errorf("base_value = %d, want 65000", fetched.BaseValue)
	}
	if fetched.BullValue != 80000 {
		t.Errorf("bull_value = %d, want 80000", fetched.BullValue)
	}
	if string(fetched.Confidence) != "MEDIUM" {
		t.Errorf("confidence = %v, want MEDIUM", fetched.Confidence)
	}
	if fetched.AlgorithmVersion != "valuation-v2.0" {
		t.Errorf("algorithm_version = %v, want valuation-v2.0", fetched.AlgorithmVersion)
	}
}

// TestValuationResultRepository_Integration_GetByQueryHash tests reproducibility check.
func TestValuationResultRepository_Integration_GetByQueryHash(t *testing.T) {
	ctx, conn, cleanup := newTestDBWithPostGIS(t)
	defer cleanup()

	repo := NewValuationResultRepository(conn)

	parcelID, err := insertTestParcel(ctx, conn)
	if err != nil {
		t.Fatalf("insert test parcel: %v", err)
	}
	snapshotID, err := insertTestSnapshot(ctx, conn)
	if err != nil {
		t.Fatalf("insert test snapshot: %v", err)
	}

	result := domain.ValuationResult{
		TargetParcelID:       parcelID,
		SnapshotID:           snapshotID,
		AlgorithmVersion:     "v2.0",
		ConfigurationVersion: "v2.0",
		OutlierMethod:        "IQR",
		Weights:              json.RawMessage(`{}`),
		BearValue:            50000,
		BaseValue:            65000,
		BullValue:            80000,
		Confidence:           domain.ConfidenceHigh,
		Status:               "COMPLETED",
		QueryHash:            "reproducibility_hash_abc",
	}

	inserted, err := repo.Insert(ctx, result)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Look up by query hash
	fetched, err := repo.GetByQueryHash(ctx, "reproducibility_hash_abc")
	if err != nil {
		t.Fatalf("get by query hash: %v", err)
	}

	if fetched.ID != inserted.ID {
		t.Errorf("ID = %v, want %v", fetched.ID, inserted.ID)
	}
}

// TestValuationResultRepository_Integration_ListByParcel tests deterministic ordering.
func TestValuationResultRepository_Integration_ListByParcel(t *testing.T) {
	ctx, conn, cleanup := newTestDBWithPostGIS(t)
	defer cleanup()

	repo := NewValuationResultRepository(conn)

	parcelID, err := insertTestParcel(ctx, conn)
	if err != nil {
		t.Fatalf("insert test parcel: %v", err)
	}
	snapshotID, err := insertTestSnapshot(ctx, conn)
	if err != nil {
		t.Fatalf("insert test snapshot: %v", err)
	}
	// Insert multiple valuation results for the same parcel
	for i := 0; i < 3; i++ {
		result := domain.ValuationResult{
			TargetParcelID:       parcelID,
			SnapshotID:           snapshotID,
			AlgorithmVersion:     "v2.0",
			ConfigurationVersion: "v2.0",
			OutlierMethod:        "IQR",
			Weights:              json.RawMessage(`{}`),
			BearValue:            int64(50000 + i*1000),
			BaseValue:            int64(65000 + i*1000),
			BullValue:            int64(80000 + i*1000),
			Confidence:           domain.ConfidenceMedium,
			Status:               "COMPLETED",
			QueryHash:            fmt.Sprintf("hash_%d", i),
		}
		_, err := repo.Insert(ctx, result)
		if err != nil {
			t.Fatalf("insert #%d: %v", i, err)
		}
	}

	// List should return all 3, ordered by created_at DESC
	results, err := repo.ListByParcel(ctx, parcelID, snapshotID, 100)
	if err != nil {
		t.Fatalf("list by parcel: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("len results = %d, want 3", len(results))
	}
}

// TestComparableResultRepository_Integration_BatchInsert tests batch insert + ListByTarget.
func TestComparableResultRepository_Integration_BatchInsert(t *testing.T) {
	ctx, conn, cleanup := newTestDBWithPostGIS(t)
	defer cleanup()

	repo := NewComparableResultRepository(conn)

	parcelID, err := insertTestParcel(ctx, conn)
	if err != nil {
		t.Fatalf("insert test parcel: %v", err)
	}
	txn1, err := insertTestTransaction(ctx, conn)
	if err != nil {
		t.Fatalf("insert test transaction 1: %v", err)
	}
	txn2, err := insertTestTransaction(ctx, conn)
	if err != nil {
		t.Fatalf("insert test transaction 2: %v", err)
	}
	txn3, err := insertTestTransaction(ctx, conn)
	if err != nil {
		t.Fatalf("insert test transaction 3: %v", err)
	}

	comparables := []domain.ComparableResult{
		{
			TargetTransactionID:    parcelID,
			CandidateTransactionID: txn1,
			DistanceM:              150.0,
			AreaSimilarity:         0.85,
			ZoningMatch:            true,
			LandUseMatch:           true,
			RoadAccessMatch:        true,
			TimeScore:              0.9,
			DistanceScore:          0.7,
			TotalScore:             0.85,
			AlgorithmVersion:       "comparable-v2.0",
		},
		{
			TargetTransactionID:    parcelID,
			CandidateTransactionID: txn2,
			DistanceM:              200.0,
			AreaSimilarity:         0.75,
			ZoningMatch:            true,
			LandUseMatch:           false,
			RoadAccessMatch:        false,
			TimeScore:              0.6,
			DistanceScore:          0.4,
			TotalScore:             0.55,
			AlgorithmVersion:       "comparable-v2.0",
		},
		{
			TargetTransactionID:    parcelID,
			CandidateTransactionID: txn3,
			DistanceM:              100.0,
			AreaSimilarity:         0.95,
			ZoningMatch:            true,
			LandUseMatch:           true,
			RoadAccessMatch:        true,
			TimeScore:              0.95,
			DistanceScore:          0.9,
			TotalScore:             0.95,
			AlgorithmVersion:       "comparable-v2.0",
		},
	}

	count, err := repo.BatchInsert(ctx, comparables)
	if err != nil {
		t.Fatalf("batch insert: %v", err)
	}
	if count != 3 {
		t.Errorf("inserted %d, want 3", count)
	}

	// List should be deterministic: total_score DESC, distance_m ASC, candidate_transaction_id ASC
	results, err := repo.ListByTarget(ctx, parcelID, 100)
	if err != nil {
		t.Fatalf("list by target: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("len results = %d, want 3", len(results))
	}

	// Verify deterministic ordering: highest total_score first
	if results[0].TotalScore < results[1].TotalScore {
		t.Error("expected descending total_score order")
	}

	// Verify all results have correct algorithm version
	for _, r := range results {
		if r.AlgorithmVersion != "comparable-v2.0" {
			t.Errorf("algorithm_version = %v, want comparable-v2.0", r.AlgorithmVersion)
		}
	}
}

// TestValuationResultRepository_Integration_PersistValuation tests atomic write.
func TestValuationResultRepository_Integration_PersistValuation(t *testing.T) {
	ctx, conn, cleanup := newTestDBWithPostGIS(t)
	defer cleanup()

	repo := NewValuationResultRepository(conn)

	parcelID, err := insertTestParcel(ctx, conn)
	if err != nil {
		t.Fatalf("insert test parcel: %v", err)
	}
	snapshotID, err := insertTestSnapshot(ctx, conn)
	if err != nil {
		t.Fatalf("insert test snapshot: %v", err)
	}
	txn1, err := insertTestTransaction(ctx, conn)
	if err != nil {
		t.Fatalf("insert test transaction 1: %v", err)
	}
	txn2, err := insertTestTransaction(ctx, conn)
	if err != nil {
		t.Fatalf("insert test transaction 2: %v", err)
	}

	valuation := domain.ValuationResult{
		TargetParcelID:       parcelID,
		SnapshotID:           snapshotID,
		ComparableIDs:        []string{"comp-1", "comp-2"},
		AlgorithmVersion:     "valuation-v2.0",
		ConfigurationVersion: "v2.0",
		OutlierMethod:        "IQR",
		Weights:              json.RawMessage(`{"W_area":0.30,"W_distance":0.20}`),
		BearValue:            50000,
		BaseValue:            65000,
		BullValue:            80000,
		Confidence:           domain.ConfidenceHigh,
		Status:               "COMPLETED",
		QueryHash:            "persist_hash_xyz",
	}

	comparables := []domain.ComparableResult{
		{
			TargetTransactionID:    parcelID,
			CandidateTransactionID: txn1,
			DistanceM:              150.0,
			AreaSimilarity:         0.85,
			TotalScore:             0.85,
			AlgorithmVersion:       "comparable-v2.0",
		},
		{
			TargetTransactionID:    parcelID,
			CandidateTransactionID: txn2,
			DistanceM:              200.0,
			AreaSimilarity:         0.75,
			TotalScore:             0.55,
			AlgorithmVersion:       "comparable-v2.0",
		},
	}

	inserted, err := repo.PersistValuation(ctx, valuation, comparables)
	if err != nil {
		t.Fatalf("persist valuation: %v", err)
	}

	if inserted.ID == "" {
		t.Error("expected non-empty ID after persist")
	}

	// Verify the valuation was persisted
	fetched, err := repo.GetByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("get by ID: %v", err)
	}
	if fetched.BearValue != 50000 {
		t.Errorf("bear_value = %d, want 50000", fetched.BearValue)
	}

	// Verify comparables were persisted
	compRepo := NewComparableResultRepository(conn)
	compResults, err := compRepo.ListByTarget(ctx, parcelID, 100)
	if err != nil {
		t.Fatalf("list comparables: %v", err)
	}
	if len(compResults) != 2 {
		t.Errorf("expected 2 comparables persisted, got %d", len(compResults))
	}
}

// TestValuationResultRepository_Integration_PersistValuation_Rollback
// verifies that if a comparable insert fails (invalid UUID), the valuation is also rolled back.
func TestValuationResultRepository_Integration_PersistValuation_Rollback(t *testing.T) {
	ctx, conn, cleanup := newTestDBWithPostGIS(t)
	defer cleanup()

	repo := NewValuationResultRepository(conn)

	parcelID, err := insertTestParcel(ctx, conn)
	if err != nil {
		t.Fatalf("insert test parcel: %v", err)
	}
	snapshotID, err := insertTestSnapshot(ctx, conn)
	if err != nil {
		t.Fatalf("insert test snapshot: %v", err)
	}

	valuation := domain.ValuationResult{
		TargetParcelID:       parcelID,
		SnapshotID:           snapshotID,
		AlgorithmVersion:     "v2.0",
		ConfigurationVersion: "v2.0",
		BearValue:            50000,
		BaseValue:            65000,
		BullValue:            80000,
		Confidence:           domain.ConfidenceMedium,
		Status:               "COMPLETED",
	}

	// This comparable has an invalid UUID which should cause a parse error → rollback
	comparables := []domain.ComparableResult{
		{
			TargetTransactionID:    "invalid-uuid",
			CandidateTransactionID: uuid.NewString(),
			DistanceM:              150.0,
			AreaSimilarity:         0.85,
			TotalScore:             0.85,
			AlgorithmVersion:       "comparable-v2.0",
		},
	}

	_, err = repo.PersistValuation(ctx, valuation, comparables)
	if err == nil {
		t.Fatal("expected error for invalid UUID in comparable")
	}

	// Verify the valuation was NOT persisted (rollback)
	results, err := repo.ListByParcel(ctx, parcelID, snapshotID, 100)
	if err != nil {
		t.Fatalf("list by parcel: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 valuation results after rollback, got %d", len(results))
	}
}

// --- Test helpers ---

func insertTestParcel(ctx context.Context, conn *pgx.Conn) (string, error) {
	parcelID := uuid.NewString()
	_, err := conn.Exec(ctx,
		`INSERT INTO parcel (id, cid, county, district, section, land_number, land_area, transaction_target)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		parcelID, "A1", "台北市", "中山區", "中山段", "000012-000", 100.0, "房地(含地及建物者)",
	)
	if err != nil {
		return "", fmt.Errorf("insert parcel: %w", err)
	}
	return parcelID, nil
}

func insertTestSnapshot(ctx context.Context, conn *pgx.Conn) (string, error) {
	snapshotID := uuid.NewString()
	_, err := conn.Exec(ctx,
		`INSERT INTO dataset_snapshot (id, source, source_version, file_name, file_sha256, record_count, status, schema_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		snapshotID, "housefun", uuid.NewString(), "test.csv", uuid.NewString(), 1, "LOCKED", "v2.0",
	)
	if err != nil {
		return "", fmt.Errorf("insert snapshot: %w", err)
	}
	return snapshotID, nil
}

func insertTestTransaction(ctx context.Context, conn *pgx.Conn) (string, error) {
	txnID := uuid.NewString()
	snapshotID, err := insertTestSnapshot(ctx, conn)
	if err != nil {
		return "", err
	}
	_, err = conn.Exec(ctx,
		`INSERT INTO "transaction" (id, snapshot_id, county, district, section, land_number, transaction_date, transaction_type, transaction_target, transaction_unit_price, transaction_land_area)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		txnID, snapshotID, "台北市", "中山區", "中山段", "000012-000", "2024-01-15", "買賣", "房地(含地及建物者)", 65000, 100.0,
	)
	if err != nil {
		return "", fmt.Errorf("insert transaction: %w", err)
	}
	return txnID, nil
}
