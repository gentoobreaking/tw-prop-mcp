//go:build integration

package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"tw-prop-mcp/internal/repository"
)

func newIntegrationDB(t *testing.T) (context.Context, *pgx.Conn, func()) {
	t.Helper()
	ctx := context.Background()
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatalf("pgx connect DATABASE_URL: %v", err)
		}
		if err := runMigrations(ctx, conn); err != nil {
			t.Fatalf("run migrations DATABASE_URL: %v", err)
		}
		_, _ = conn.Exec(ctx, "DROP TRIGGER IF EXISTS trg_snapshot_lock ON dataset_snapshot")
		_, _ = conn.Exec(ctx, "DELETE FROM transaction_building")
		_, _ = conn.Exec(ctx, "DELETE FROM transaction_land")
		_, _ = conn.Exec(ctx, "DELETE FROM transaction")
		_, _ = conn.Exec(ctx, "DELETE FROM import_batch")
		_, _ = conn.Exec(ctx, "DELETE FROM dataset_snapshot")
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
			if fname == "000001_init.up.sql" && strings.Contains(strings.ToLower(err.Error()), "postgis") {
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

func TestDownloader_Integration_FullFlow(t *testing.T) {
	ctx, conn, cleanup := newIntegrationDB(t)
	defer cleanup()

	// Mock official server returning small CSV
	csvContent := "transaction_id,price\nTX001,1000000\nTX002,2000000\n"
	expectedSHA := sha256.Sum256([]byte(csvContent))
	expectedHex := hex.EncodeToString(expectedSHA[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(csvContent))
	}))
	defer srv.Close()

	tmpRoot := t.TempDir()
	rawRoot := filepath.Join(tmpRoot, "raw")
	store := NewRawArchiveStore(rawRoot)
	repo := repository.NewSnapshotRepository(conn)
	downloader := NewHTTPDownloader()
	svc := NewSnapshotService(downloader, store, repo, conn)

	snap, err := svc.CreateFromDownload(ctx, "MOI", "2024Q1-integration", srv.URL+"/lvr.csv", "lvr_land_a.csv")
	if err != nil {
		t.Fatalf("CreateFromDownload: %v", err)
	}
	if snap.ID == "" {
		t.Fatalf("snapshot ID empty")
	}
	if snap.Source != "MOI" || snap.SourceVersion != "2024Q1-integration" {
		t.Fatalf("source mismatch: %+v", snap)
	}
	if snap.FileSHA256 != expectedHex {
		t.Fatalf("sha mismatch: got %s want %s", snap.FileSHA256, expectedHex)
	}
	if snap.Status != "PENDING" {
		t.Fatalf("status want PENDING got %s", snap.Status)
	}
	// Verify raw files exist and checksum correct
	archiveDir := filepath.Join(rawRoot, "source_snapshot", snap.ID)
	if _, err := os.Stat(archiveDir); err != nil {
		t.Fatalf("archive dir missing: %v", err)
	}
	for _, name := range []string{"manifest.json", "checksum.sha256", "downloaded_at.txt", "source_metadata.json", "lvr_land_a.csv"} {
		if _, err := os.Stat(filepath.Join(archiveDir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
	// Verify checksum file content
	b, _ := os.ReadFile(filepath.Join(archiveDir, "checksum.sha256"))
	if !strings.Contains(string(b), expectedHex) {
		t.Fatalf("checksum file mismatch: %s", string(b))
	}
	// Verify original file content matches CSV and is 444
	origPath := filepath.Join(archiveDir, "lvr_land_a.csv")
	data, _ := os.ReadFile(origPath)
	if string(data) != csvContent {
		t.Fatalf("original file content mismatch")
	}
	fi, _ := os.Stat(origPath)
	if fi.Mode().Perm() != 0o444 {
		t.Fatalf("expected 444 got %o", fi.Mode().Perm())
	}
	// Verify DB snapshot correct via repo
	got, err := repo.GetByID(ctx, snap.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.FileSHA256 != expectedHex {
		t.Fatalf("db sha mismatch")
	}
	// Verify import_batch was created
	var batchCount int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM import_batch WHERE snapshot_id=$1`, snap.ID).Scan(&batchCount); err != nil {
		t.Fatalf("count import_batch: %v", err)
	}
	if batchCount != 1 {
		t.Fatalf("expected 1 import_batch got %d", batchCount)
	}
	// Idempotency: second call with same content should return same ID and not create new snapshot/batch
	snap2, err := svc.CreateFromDownload(ctx, "MOI", "2024Q1-integration", srv.URL+"/lvr.csv", "lvr_land_a.csv")
	if err != nil {
		t.Fatalf("second CreateFromDownload: %v", err)
	}
	if snap2.ID != snap.ID {
		t.Fatalf("idempotent failed: %s vs %s", snap2.ID, snap.ID)
	}
	var snapCount int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM dataset_snapshot WHERE source='MOI' AND source_version='2024Q1-integration'`).Scan(&snapCount); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if snapCount != 1 {
		t.Fatalf("expected 1 snapshot after idempotent call, got %d", snapCount)
	}
}

func TestDownloader_Integration_ChecksumMismatch(t *testing.T) {
	ctx, conn, cleanup := newIntegrationDB(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("real content"))
	}))
	defer srv.Close()

	tmpRoot := t.TempDir()
	store := NewRawArchiveStore(filepath.Join(tmpRoot, "raw"))
	repo := repository.NewSnapshotRepository(conn)
	downloader := NewHTTPDownloader()
	svc := NewSnapshotService(downloader, store, repo, conn)

	_, err := svc.CreateFromDownloadWithChecksum(ctx, "MOI", "2024Q1-bad", srv.URL+"/bad.csv", "bad.csv", strings.Repeat("0", 64))
	if err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "checksum") {
		t.Fatalf("expected checksum error, got %v", err)
	}
}
