package integration

import (
	"context"
	"embed"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"tw-prop-mcp/internal/importpipeline"
	"tw-prop-mcp/internal/repository"
)

//go:embed testdata/sample.csv
var sampleCSVFS embed.FS

func TestImportPipeline_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Start postgres:16-alpine
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
		t.Logf("postgis not available: %v", err)
		hasPostGIS = false
	}

	// Run migrations
	if err := RunMigrations(ctx, pool, hasPostGIS); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// Create repositories
	txRepo := repository.NewTransactionRepository(pool)
	parcelRepo := repository.NewParcelRepository(pool)
	snapshotRepo := repository.NewSnapshotRepository(pool)

	// Create a test snapshot
	snapshotID := "test-snapshot-" + time.Now().Format("20060102150405")
	_, err = snapshotRepo.Create(ctx, repository.CreateSnapshotParams{
		SourceVersion:  "v2.0",
		FileName:       "sample.csv",
		FileSHA256:     "",
		RecordCount:    0,
		Status:         "PENDING",
		SchemaVersion:  "v2.0",
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	// Read sample CSV
	csvData, err := sampleCSVFS.ReadFile("testdata/sample.csv")
	if err != nil {
		t.Fatalf("read sample csv: %v", err)
	}

	// Write CSV to temp file
	tmpFile, err := os.CreateTemp("", "import_*.csv")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(csvData); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	// Create import pipeline
	p := importpipeline.NewImportPipeline(importpipeline.PipelineConfig{
		SnapshotID:   snapshotID,
		DownloadURL:  "file://" + tmpFile.Name(),
		DownloadDest: tmpFile.Name(),
		MaxRetries:   1,
		RetryDelay:   100 * time.Millisecond,
	}, nil)

	p.SetRepositories(txRepo, parcelRepo, snapshotRepo)

	// Run import
	result, err := p.ImportFromSource(ctx)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if result.TransactionsImported == 0 {
		t.Error("expected transactions to be imported")
	}

	if p.GetStatus() != importpipeline.StatusLocked {
		t.Errorf("expected status LOCKED, got %s", p.GetStatus())
	}

	t.Logf("Import completed: %d transactions, %d parcels in %v",
		result.TransactionsImported, result.ParcelsImported, result.Duration)
}