//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"tw-prop-mcp/internal/domain"
)

func newTestDB(t *testing.T) (context.Context, *pgx.Conn, func()) {
	t.Helper()
	ctx := context.Background()

	// Prefer DATABASE_URL if set (docker-compose running)
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatalf("pgx connect DATABASE_URL: %v", err)
		}
		if err := runMigrations(ctx, conn); err != nil {
			t.Fatalf("run migrations on DATABASE_URL: %v", err)
		}
		// Pre-clean with trigger disabled to remove leftover LOCKED rows from prior runs.
		_, _ = conn.Exec(ctx, "DROP TRIGGER IF EXISTS trg_snapshot_lock ON dataset_snapshot")
		_, _ = conn.Exec(ctx, "DELETE FROM transaction_building")
		_, _ = conn.Exec(ctx, "DELETE FROM transaction_land")
		_, _ = conn.Exec(ctx, "DELETE FROM transaction")
		_, _ = conn.Exec(ctx, "DELETE FROM import_batch")
		_, _ = conn.Exec(ctx, "DELETE FROM dataset_snapshot")
		// Recreate trigger after cleanup.
		_ = runMigrations(ctx, conn)
		cleanup := func() {
			_, _ = conn.Exec(ctx, "DROP TRIGGER IF EXISTS trg_snapshot_lock ON dataset_snapshot")
			_, _ = conn.Exec(ctx, "DELETE FROM transaction_building")
			_, _ = conn.Exec(ctx, "DELETE FROM transaction_land")
			_, _ = conn.Exec(ctx, "DELETE FROM transaction")
			_, _ = conn.Exec(ctx, "DELETE FROM import_batch")
			_, _ = conn.Exec(ctx, "DELETE FROM dataset_snapshot")
			_, _ = conn.Exec(ctx, `CREATE OR REPLACE FUNCTION prevent_locked_snapshot_update() RETURNS trigger AS $$ BEGIN IF OLD.status='LOCKED' THEN RAISE EXCEPTION 'snapshot locked: %', OLD.id; END IF; IF TG_OP = 'DELETE' THEN RETURN OLD; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql; DROP TRIGGER IF EXISTS trg_snapshot_lock ON dataset_snapshot; CREATE TRIGGER trg_snapshot_lock BEFORE UPDATE OR DELETE ON dataset_snapshot FOR EACH ROW EXECUTE FUNCTION prevent_locked_snapshot_update();`)
			conn.Close(ctx)
		}
		return ctx, conn, cleanup
	}

	// Fallback: testcontainers postgres:16-alpine
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
	// Ensure postgis available? Not required for snapshot locking; ignore if missing.
	_, _ = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto;")
	_, _ = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS postgis;")

	if err := runMigrations(ctx, conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	cleanup := func() {
		conn.Close(ctx)
		_ = testcontainers.TerminateContainer(pgC)
	}
	return ctx, conn, cleanup
}

func runMigrations(ctx context.Context, conn *pgx.Conn) error {
	candidates := []string{
		filepath.Join("..", "..", "migrations"),
		filepath.Join("migrations"),
		"../../migrations",
	}
	var migDir string
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "000001_init.up.sql")); err == nil {
			migDir = c
			break
		}
	}
	if migDir == "" {
		migDir = filepath.Join("..", "..", "migrations")
	}
	for _, fname := range []string{"000001_init.up.sql", "000002_snapshot_lock.up.sql"} {
		b, err := os.ReadFile(filepath.Join(migDir, fname))
		if err != nil {
			abs := "/Users/david/Projects/tw-prop-mcp/migrations/" + fname
			b, err = os.ReadFile(abs)
			if err != nil {
				return err
			}
		}
		if _, err := conn.Exec(ctx, string(b)); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			// Fallback for postgres:16-alpine without postgis: create minimal snapshot table
			if fname == "000001_init.up.sql" && strings.Contains(strings.ToLower(err.Error()), "postgis") {
				// Minimal dataset_snapshot schema sufficient for T003 lock tests
				minimal := `
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
				CREATE TABLE IF NOT EXISTS import_batch (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					snapshot_id UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
					status VARCHAR(20) NOT NULL DEFAULT 'RUNNING',
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);
				`
				if _, err2 := conn.Exec(ctx, minimal); err2 != nil {
					return fmt.Errorf("postgis fallback minimal snapshot: %w (orig: %v)", err2, err)
				}
				continue
			}
			return err
		}
	}
	return nil
}

func TestSnapshotLockTrigger(t *testing.T) {
	ctx, conn, cleanup := newTestDB(t)
	defer cleanup()

	repo := NewSnapshotRepository(conn)

	// Create PENDING snapshot
	snap, err := repo.Create(ctx, CreateSnapshotParams{
		Source:        "MOI",
		SourceVersion: "2024Q1-locktest",
		FileName:      "locktest.csv",
		FileSHA256:    strings.Repeat("a", 64),
		RecordCount:   10,
		Status:        domain.SnapshotStatusPending,
		SchemaVersion: "v2.0",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if snap.Status != domain.SnapshotStatusPending {
		t.Fatalf("status want PENDING got %s", snap.Status)
	}

	// PENDING -> IMPORTING should succeed
	if err := repo.UpdateStatus(ctx, snap.ID, domain.SnapshotStatusImporting); err != nil {
		t.Fatalf("update to IMPORTING: %v", err)
	}
	got, err := repo.GetByID(ctx, snap.ID)
	if err != nil {
		t.Fatalf("get after importing: %v", err)
	}
	if got.Status != domain.SnapshotStatusImporting {
		t.Fatalf("status want IMPORTING got %s", got.Status)
	}

	// Lock (IMPORTING -> LOCKED) should succeed and set import_completed_at
	if err := repo.Lock(ctx, snap.ID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	got, err = repo.GetByID(ctx, snap.ID)
	if err != nil {
		t.Fatalf("get after lock: %v", err)
	}
	if got.Status != domain.SnapshotStatusLocked {
		t.Fatalf("status want LOCKED got %s", got.Status)
	}
	if got.ImportCompletedAt == nil {
		t.Fatalf("import_completed_at should be set after lock")
	}

	// Attempt UPDATE via repository should fail: state machine blocks LOCKED -> any
	if err := repo.UpdateStatus(ctx, snap.ID, domain.SnapshotStatusFailed); err == nil {
		t.Fatalf("expected UpdateStatus on LOCKED to fail")
	} else if !strings.Contains(err.Error(), "illegal transition") && !strings.Contains(err.Error(), "snapshot locked") {
		t.Fatalf("unexpected error for locked update: %v", err)
	}

	// Direct SQL UPDATE should be blocked by DB trigger
	_, err = conn.Exec(ctx, `UPDATE dataset_snapshot SET file_name='hacked.csv' WHERE id=$1`, snap.ID)
	if err == nil {
		t.Fatalf("expected DB trigger to block UPDATE on LOCKED snapshot")
	}
	if !strings.Contains(err.Error(), "snapshot locked") {
		t.Fatalf("expected snapshot locked error, got: %v", err)
	}

	// Direct SQL DELETE should be blocked by DB trigger
	_, err = conn.Exec(ctx, `DELETE FROM dataset_snapshot WHERE id=$1`, snap.ID)
	if err == nil {
		t.Fatalf("expected DB trigger to block DELETE on LOCKED snapshot")
	}
	if !strings.Contains(err.Error(), "snapshot locked") {
		t.Fatalf("expected snapshot locked error on delete, got: %v", err)
	}

	// Repository Delete should also be blocked (trigger)
	if err := repo.Delete(ctx, snap.ID); err == nil {
		t.Fatalf("expected repo Delete on LOCKED to fail")
	} else if !strings.Contains(err.Error(), "snapshot locked") {
		t.Fatalf("expected snapshot locked on repo delete, got: %v", err)
	}

	// Verify snapshot still exists and still LOCKED
	got, err = repo.GetByID(ctx, snap.ID)
	if err != nil {
		t.Fatalf("get after failed delete: %v", err)
	}
	if got.Status != domain.SnapshotStatusLocked {
		t.Fatalf("still want LOCKED, got %s", got.Status)
	}

	// Verify illegal direct transition PENDING->LOCKED via state machine is blocked
	snap2, err := repo.Create(ctx, CreateSnapshotParams{
		Source:        "MOI",
		SourceVersion: "2024Q1-locktest2",
		FileName:      "locktest2.csv",
		FileSHA256:    strings.Repeat("b", 64),
		RecordCount:   5,
		Status:        domain.SnapshotStatusPending,
		SchemaVersion: "v2.0",
	})
	if err != nil {
		t.Fatalf("create2: %v", err)
	}
	if err := repo.Lock(ctx, snap2.ID); err == nil {
		t.Fatalf("expected Lock on PENDING to fail (needs IMPORTING)")
	}
	// Via UpdateStatus PENDING->LOCKED should also fail via state machine
	if err := repo.UpdateStatus(ctx, snap2.ID, domain.SnapshotStatusLocked); err == nil {
		t.Fatalf("expected PENDING->LOCKED via UpdateStatus to fail")
	}
	// Cleanup non-locked snapshot via repo Delete (should succeed)
	if err := repo.Delete(ctx, snap2.ID); err != nil {
		t.Fatalf("delete non-locked snapshot should succeed: %v", err)
	}
	// List should contain at least the locked one
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, v := range list {
		if v.ID == snap.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list should contain locked snapshot %s", snap.ID)
	}
}

func TestSnapshotLockRowCount(t *testing.T) {
	ctx, conn, cleanup := newTestDB(t)
	defer cleanup()
	repo := NewSnapshotRepository(conn)

	snap, err := repo.Create(ctx, CreateSnapshotParams{
		Source:        "MOI",
		SourceVersion: "2024Q1-rowcount",
		FileName:      "rowcount.csv",
		FileSHA256:    strings.Repeat("c", 64),
		RecordCount:   3,
		Status:        domain.SnapshotStatusPending,
		SchemaVersion: "v2.0",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Lock without IMPORTING should fail with rowCount check
	if err := repo.Lock(ctx, snap.ID); err == nil {
		t.Fatalf("lock without IMPORTING should fail")
	}
	// Move to IMPORTING then FAILED, then try lock should still fail
	if err := repo.UpdateStatus(ctx, snap.ID, domain.SnapshotStatusImporting); err != nil {
		t.Fatalf("to importing: %v", err)
	}
	if err := repo.UpdateStatus(ctx, snap.ID, domain.SnapshotStatusFailed); err != nil {
		t.Fatalf("to failed: %v", err)
	}
	if err := repo.Lock(ctx, snap.ID); err == nil {
		t.Fatalf("lock on FAILED should fail")
	}
}
