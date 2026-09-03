//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// txTestDB sets up a PostgreSQL 16-alpine container with the transaction
// schema only (no GIS tables). Returns a ready *pgx.Conn and cleanup.
func txTestDB(t *testing.T) (context.Context, *pgx.Conn, uuid.UUID, uuid.UUID, func()) {
	t.Helper()
	ctx := context.Background()

	// Allow overriding with a live DATABASE_URL for faster local feedback.
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatalf("pgx connect DATABASE_URL: %v", err)
		}
		_, _ = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto;")
		if err := runTxMigrations(ctx, conn); err != nil {
			t.Fatalf("run migrations: %v", err)
		}
		snapshotID, batchID := createTxTestData(ctx, t, conn)
		cleanup := func() {
			_, _ = conn.Exec(ctx, "DELETE FROM transaction; DELETE FROM import_batch; DELETE FROM dataset_snapshot;")
			conn.Close(ctx)
		}
		_, _ = conn.Exec(ctx, "DELETE FROM transaction; DELETE FROM import_batch; DELETE FROM dataset_snapshot;")
		return ctx, conn, snapshotID, batchID, cleanup
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
	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS postgis;"); err != nil {
		t.Logf("postgis not available (expected for postgres:16-alpine): %v", err)
	}
	if err := runTxMigrations(ctx, conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	snapshotID, batchID := createTxTestData(ctx, t, conn)
	cleanup := func() {
		conn.Close(ctx)
		_ = testcontainers.TerminateContainer(pgC)
	}
	return ctx, conn, snapshotID, batchID, cleanup
}

// runTxMigrations executes the full migration if postgis is available,
// otherwise falls back to a minimal schema containing all transaction columns.
func runTxMigrations(ctx context.Context, conn *pgx.Conn) error {
	migDir := filepath.Join("..", "..", "migrations")
	upSQL, err := os.ReadFile(filepath.Join(migDir, "000001_init.up.sql"))
	if err == nil {
		if _, err := conn.Exec(ctx, string(upSQL)); err != nil && !isAlreadyExists(err) {
			// Full migration failed (likely missing postgis). Fall back to minimal schema.
			minimal := createMinimalTxSchema()
			if _, err2 := conn.Exec(ctx, minimal); err2 != nil && !isAlreadyExists(err2) {
				return fmt.Errorf("create minimal schema: %w", err2)
			}
		}
		return nil
	}
	// Migration file not found — create minimal schema directly.
	minimal := createMinimalTxSchema()
	if _, err := conn.Exec(ctx, minimal); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("create minimal schema: %w", err)
	}
	return nil
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate")
}

// createMinimalTxSchema returns a DDL with dataset_snapshot, import_batch, and
// transaction tables — no GIS columns, no GIS-dependent tables.
func createMinimalTxSchema() string {
	return `
CREATE TABLE IF NOT EXISTS dataset_snapshot (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source              VARCHAR(50) NOT NULL,
    source_version      VARCHAR(50) NOT NULL,
    downloaded_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at        TIMESTAMPTZ,
    file_name           VARCHAR(255) NOT NULL,
    file_sha256         CHAR(64) NOT NULL,
    record_count        BIGINT NOT NULL DEFAULT 0,
    status              VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','IMPORTING','LOCKED','FAILED')),
    schema_version      VARCHAR(20) NOT NULL DEFAULT 'v2.0',
    import_started_at   TIMESTAMPTZ,
    import_completed_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source, source_version, file_sha256)
);
CREATE INDEX IF NOT EXISTS idx_snapshot_status ON dataset_snapshot(status);
CREATE INDEX IF NOT EXISTS idx_snapshot_source_version ON dataset_snapshot(source, source_version);

CREATE TABLE IF NOT EXISTS import_batch (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id         UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    status              VARCHAR(20) NOT NULL DEFAULT 'RUNNING' CHECK (status IN ('RUNNING','COMPLETED','FAILED')),
    records_processed   BIGINT NOT NULL DEFAULT 0,
    records_imported    BIGINT NOT NULL DEFAULT 0,
    records_failed      BIGINT NOT NULL DEFAULT 0,
    record_count        BIGINT NOT NULL DEFAULT 0,
    error_message       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_import_batch_snapshot ON import_batch(snapshot_id);

CREATE TABLE IF NOT EXISTS transaction (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id             UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
    import_batch_id         UUID NOT NULL REFERENCES import_batch(id) ON DELETE RESTRICT,
    transaction_id          VARCHAR(50) NOT NULL,
    transaction_date        DATE NOT NULL,
    transaction_type        VARCHAR(20) NOT NULL,
    county                  VARCHAR(20) NOT NULL,
    district                VARCHAR(20) NOT NULL,
    section                 VARCHAR(50),
    land_number             VARCHAR(50),
    transaction_target      VARCHAR(50),
    total_price             BIGINT NOT NULL CHECK (total_price > 0),
    unit_price              BIGINT NOT NULL CHECK (unit_price > 0),
    land_area_sqm           NUMERIC(12,4) CHECK (land_area_sqm IS NULL OR land_area_sqm > 0),
    building_area_sqm       NUMERIC(12,4) CHECK (building_area_sqm IS NULL OR building_area_sqm > 0),
    urban_zoning            VARCHAR(50),
    non_urban_zoning        VARCHAR(50),
    land_use_category       VARCHAR(50),
    building_type           VARCHAR(50),
    floor                   VARCHAR(20),
    age                     INTEGER CHECK (age IS NULL OR age >= 0),
    parking_area_sqm        NUMERIC(10,4) CHECK (parking_area_sqm IS NULL OR parking_area_sqm >= 0),
    parking_price           BIGINT CHECK (parking_price IS NULL OR parking_price >= 0),
    source_record_hash      CHAR(64) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (snapshot_id, source_record_hash),
    UNIQUE (county, district, section, land_number, snapshot_id, transaction_id)
);
CREATE INDEX IF NOT EXISTS idx_txn_snapshot ON transaction(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_txn_location ON transaction(county, district, section);
CREATE INDEX IF NOT EXISTS idx_txn_date ON transaction(transaction_date);
CREATE INDEX IF NOT EXISTS idx_txn_land_number ON transaction(county, district, section, land_number);
CREATE INDEX IF NOT EXISTS idx_txn_import_batch ON transaction(import_batch_id);
`
}

// createTxTestData inserts a dataset_snapshot and import_batch, returning their UUIDs.
func createTxTestData(ctx context.Context, t *testing.T, conn *pgx.Conn) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var snapshotID uuid.UUID
	err := conn.QueryRow(ctx,
		`INSERT INTO dataset_snapshot (source, source_version, file_name, file_sha256, record_count, status)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		"MOI", "2024Q1", "test.csv", "abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd", 5, "LOCKED",
	).Scan(&snapshotID)
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	var batchID uuid.UUID
	err = conn.QueryRow(ctx,
		`INSERT INTO import_batch (snapshot_id, status) VALUES ($1, $2) RETURNING id`,
		snapshotID, "COMPLETED",
	).Scan(&batchID)
	if err != nil {
		t.Fatalf("insert import_batch: %v", err)
	}
	return snapshotID, batchID
}

// makeTestTransactions creates 5 deterministic domain.Transaction rows for the
// given snapshot/batch IDs, all in the same county/district/section.
func makeTestTransactions(snapshotID, batchID uuid.UUID) []domain.Transaction {
	prices := []int64{1000000, 2000000, 3000000, 4000000, 5000000}
	landAreas := []float64{50.0, 60.0, 70.0, 80.0, 90.0}
	buildingAreas := []float64{30.0, 40.0, 50.0, 60.0, 70.0}

	txns := make([]domain.Transaction, len(prices))
	for i := range prices {
		txns[i] = domain.Transaction{
			SnapshotID:        snapshotID.String(),
			ImportBatchID:     batchID.String(),
			TransactionID:     fmt.Sprintf("TX%04d", i+1),
			TransactionDate:   time.Date(2024, 1, 1+i, 0, 0, 0, 0, time.UTC),
			TransactionType:   "土地",
			County:            "台北市",
			District:          "中山區",
			Section:           "中山段",
			LandNumber:        fmt.Sprintf("0001-00%02d", i+1),
			TransactionTarget: "土地",
			TotalPrice:        prices[i],
			UnitPrice:         prices[i] / 50,
			LandAreaSqm:       landAreas[i],
			BuildingAreaSqm:   buildingAreas[i],
			UrbanZoning:       "住",
			LandUseCategory:   "住宅",
			BuildingType:      "鋼筋混凸",
			Floor:             "5",
			Age:               10 + i,
			ParkingAreaSqm:    20.0 + float64(i),
			ParkingPrice:      int64(50000 + i*10000),
			SourceRecordHash:  fmt.Sprintf("%064d", i+1),
		}
	}
	return txns
}

func TestTransactionRepo_Integration_BatchInsert_GetByID_Search_Stats(t *testing.T) {
	ctx, conn, snapshotID, batchID, cleanup := txTestDB(t)
	defer cleanup()

	repo := repository.NewTransactionRepository(conn)
	txns := makeTestTransactions(snapshotID, batchID)

	// 1. BatchInsert
	inserted, err := repo.BatchInsert(ctx, txns)
	if err != nil {
		t.Fatalf("BatchInsert failed: %v", err)
	}
	if inserted != int64(len(txns)) {
		t.Fatalf("expected %d inserted, got %d", len(txns), inserted)
	}

	// 2. Search — should return all 5 in descending date order
	got, err := repo.Search(ctx, repository.SearchFilter{
		County:   "台北市",
		District: "中山區",
		Section:  strPtr("中山段"),
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(got) != len(txns) {
		t.Fatalf("expected %d search results, got %d", len(txns), len(got))
	}

	// Verify deterministic ordering (date DESC, id ASC)
	if got[0].TransactionID != "TX0005" || got[4].TransactionID != "TX0001" {
		t.Fatalf("unexpected search order: first=%s, last=%s", got[0].TransactionID, got[4].TransactionID)
	}

	// 3. GetByID — use the first search result's ID
	byID, err := repo.GetByID(ctx, uuid.MustParse(got[0].ID))
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if byID.TotalPrice != got[0].TotalPrice {
		t.Fatalf("GetByID price mismatch: %d vs %d", byID.TotalPrice, got[0].TotalPrice)
	}
	if byID.County != "台北市" {
		t.Fatalf("expected county 台北市, got %s", byID.County)
	}

	// 4. GetStatistics — verify percentiles and aggregates for 5 transactions
	stats, err := repo.GetStatistics(ctx, "台北市", "中山區", "中山段")
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}
	if stats.Count != 5 {
		t.Fatalf("expected count 5, got %d", stats.Count)
	}
	if stats.MinPrice != 1000000 {
		t.Fatalf("expected min price 1000000, got %d", stats.MinPrice)
	}
	if stats.MaxPrice != 5000000 {
		t.Fatalf("expected max price 5000000, got %d", stats.MaxPrice)
	}
	if stats.AvgPrice != 3000000 {
		t.Fatalf("expected avg price 3000000, got %d", stats.AvgPrice)
	}
	// percentile_cont with N=5: positions are (N-1)*p = 4*p, all integer
	// p25 → index 1 → 2000000, p50 → index 2 → 3000000, p75 → index 3 → 4000000
	if stats.P25Price != 2000000 {
		t.Fatalf("expected p25 price 2000000, got %f", stats.P25Price)
	}
	if stats.MedianPrice != 3000000 {
		t.Fatalf("expected median price 3000000, got %f", stats.MedianPrice)
	}
	if stats.P75Price != 4000000 {
		t.Fatalf("expected p75 price 4000000, got %f", stats.P75Price)
	}
	if stats.MedianLandArea != 70.0 {
		t.Fatalf("expected median land area 70, got %f", stats.MedianLandArea)
	}
	if stats.MedianBuildingArea != 50.0 {
		t.Fatalf("expected median building area 50, got %f", stats.MedianBuildingArea)
	}

	// 5. Determinism — repeat queries and compare
	got2, err := repo.Search(ctx, repository.SearchFilter{
		County:   "台北市",
		District: "中山區",
		Section:  strPtr("中山段"),
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("Search (determinism) failed: %v", err)
	}
	if len(got2) != len(got) {
		t.Fatalf("determinism: length mismatch %d vs %d", len(got2), len(got))
	}
	for i := range got {
		if got[i].ID != got2[i].ID {
			t.Fatalf("determinism: ID mismatch at index %d: %s vs %s", i, got[i].ID, got2[i].ID)
		}
		if got[i].TotalPrice != got2[i].TotalPrice {
			t.Fatalf("determinism: price mismatch at index %d", i)
		}
	}

	stats2, err := repo.GetStatistics(ctx, "台北市", "中山區", "中山段")
	if err != nil {
		t.Fatalf("GetStatistics (determinism) failed: %v", err)
	}
	if stats != stats2 {
		t.Fatalf("determinism: statistics mismatch: %+v vs %+v", stats, stats2)
	}

	// 6. Search without section filter should return all 5
	gotAll, err := repo.Search(ctx, repository.SearchFilter{
		County:   "台北市",
		District: "中山區",
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("Search (no section) failed: %v", err)
	}
	if len(gotAll) != len(txns) {
		t.Fatalf("expected %d results without section filter, got %d", len(txns), len(gotAll))
	}
}

func strPtr(s string) *string { return &s }
