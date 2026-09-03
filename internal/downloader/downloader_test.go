package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
	"tw-prop-mcp/internal/repository/db"
)

// Ensure repository interface satisfied.
var _ repository.SnapshotRepository = (*testRepo)(nil)

// testRepo implements repository.SnapshotRepository for unit tests.
type testRepo struct{}

func (r *testRepo) Create(ctx context.Context, arg repository.CreateSnapshotParams) (domain.DatasetSnapshot, error) {
	return domain.DatasetSnapshot{}, nil
}
func (r *testRepo) GetByID(ctx context.Context, id string) (domain.DatasetSnapshot, error) {
	return domain.DatasetSnapshot{}, nil
}
func (r *testRepo) List(ctx context.Context) ([]domain.DatasetSnapshot, error) { return nil, nil }
func (r *testRepo) UpdateStatus(ctx context.Context, id string, to domain.SnapshotStatus) error {
	return nil
}
func (r *testRepo) Lock(ctx context.Context, id string) error { return nil }
func (r *testRepo) Delete(ctx context.Context, id string) error { return nil }

// ---------- Downloader Mock ----------

func TestDownloader_Mock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		dest := filepath.Join(tmpDir, "out.txt")
		m := &MockDownloader{Content: []byte("hello downloader")}
		got, err := m.Download(context.Background(), "http://example.com/file.csv", dest)
		if err != nil {
			t.Fatalf("mock download failed: %v", err)
		}
		if got != dest {
			t.Fatalf("path mismatch %s vs %s", got, dest)
		}
		b, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(b) != "hello downloader" {
			t.Fatalf("content mismatch: %s", string(b))
		}
		if len(m.Calls) != 1 || m.Calls[0].URL != "http://example.com/file.csv" {
			t.Fatalf("calls not recorded: %+v", m.Calls)
		}
	})
	t.Run("failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		dest := filepath.Join(tmpDir, "out.txt")
		m := &MockDownloader{Err: errors.New("network down")}
		_, err := m.Download(context.Background(), "http://example.com/file.csv", dest)
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "network down") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDownloader_HTTP_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing User-Agent")
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("csv,content\n1,2,3"))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "dl.csv")
	d := NewHTTPDownloader()
	got, err := d.Download(context.Background(), srv.URL, dest)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	b, _ := os.ReadFile(got)
	if string(b) != "csv,content\n1,2,3" {
		t.Fatalf("content mismatch: %s", string(b))
	}
}

func TestDownloader_HTTP_Retry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok after retry"))
	}))
	defer srv.Close()
	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "retry.csv")
	d := NewHTTPDownloader()
	d.InitialBackoff = 5 * 1000000 // 5ms
	got, err := d.Download(context.Background(), srv.URL, dest)
	if err != nil {
		t.Fatalf("download retry failed: %v", err)
	}
	b, _ := os.ReadFile(got)
	if string(b) != "ok after retry" {
		t.Fatalf("content mismatch after retry")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts got %d", attempts)
	}
}

// ---------- Checksum ----------

func TestChecksum_Compute(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "hello.txt")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ComputeFileSHA256(path)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])
	if got != expected {
		t.Fatalf("checksum mismatch: got %s want %s", got, expected)
	}
	got2, err := ComputeSHA256(strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("compute reader: %v", err)
	}
	if got2 != expected {
		t.Fatalf("reader checksum mismatch")
	}
	if len(got) != 64 {
		t.Fatalf("sha256 hex length should be 64, got %d", len(got))
	}
}

func TestChecksum_Verify(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.txt")
	_ = os.WriteFile(path, []byte("abc"), 0o644)
	h := sha256.Sum256([]byte("abc"))
	correct := hex.EncodeToString(h[:])
	if err := VerifyChecksum(path, correct); err != nil {
		t.Fatalf("verify should pass: %v", err)
	}
	if err := VerifyChecksum(path, correct+"00"); err == nil {
		t.Fatalf("verify should fail on mismatch")
	} else if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
	if err := VerifyChecksum(path, ""); err != nil {
		t.Fatalf("empty expected should pass")
	}
}

// ---------- Archive Immutable ----------

func TestArchive_Immutable(t *testing.T) {
	tmpRoot := t.TempDir()
	store := NewRawArchiveStore(filepath.Join(tmpRoot, "raw"))
	srcFile := filepath.Join(tmpRoot, "src.csv")
	if err := os.WriteFile(srcFile, []byte("col1,col2\n1,2\n"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	snapshotID := uuid.New().String()
	metadata := map[string]any{
		"source":         "MOI",
		"source_version": "2024Q1",
		"file_name":      "lvr_land_a.csv",
		"url":            "http://example.com/lvr.csv",
	}
	dir, err := store.StoreRawArchive(srcFile, snapshotID, metadata)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	for _, name := range []string{"manifest.json", "checksum.sha256", "downloaded_at.txt", "source_metadata.json", "lvr_land_a.csv"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
	origPath := filepath.Join(dir, "lvr_land_a.csv")
	fi, err := os.Stat(origPath)
	if err != nil {
		t.Fatalf("stat orig: %v", err)
	}
	if fi.Mode().Perm() != 0o444 {
		t.Fatalf("expected 444 got %o", fi.Mode().Perm())
	}
	// Verify checksum file contains actual sha
	actual, _ := ComputeFileSHA256(srcFile)
	b, _ := os.ReadFile(filepath.Join(dir, "checksum.sha256"))
	if !strings.Contains(string(b), actual) {
		t.Fatalf("checksum file doesn't contain expected sha: %s vs %s", string(b), actual)
	}
	// Second store same snapshotID must fail
	_, err = store.StoreRawArchive(srcFile, snapshotID, metadata)
	if err == nil {
		t.Fatalf("second store should fail (immutable)")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already exists error, got %v", err)
	}
	fi2, _ := os.Stat(origPath)
	if fi2.Mode().Perm() != 0o444 {
		t.Fatalf("chmod changed after second attempt")
	}
}

// ---------- Snapshot Idempotent & NetworkError (unit with fake DB) ----------

// fakeDB implements repository.DBTX for unit testing snapshot service without real postgres.
type fakeDB struct {
	snapshots map[string]db.DatasetSnapshot
	batches   map[string]db.ImportBatch
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		snapshots: make(map[string]db.DatasetSnapshot),
		batches:   make(map[string]db.ImportBatch),
	}
}

func (f *fakeDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (f *fakeDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "WHERE source") && strings.Contains(sql, "file_sha256") {
		source := args[0].(string)
		version := args[1].(string)
		sha := args[2].(string)
		key := source + "|" + version + "|" + sha
		if snap, ok := f.snapshots[key]; ok {
			return &fakeRow{scan: func(dest ...any) error {
				if len(dest) < 13 {
					return errors.New("not enough dest")
				}
				*dest[0].(*pgtype.UUID) = snap.ID
				*dest[1].(*string) = snap.Source
				*dest[2].(*string) = snap.SourceVersion
				*dest[3].(*pgtype.Timestamptz) = snap.DownloadedAt
				*dest[4].(*pgtype.Timestamptz) = snap.PublishedAt
				*dest[5].(*string) = snap.FileName
				*dest[6].(*string) = snap.FileSha256
				*dest[7].(*int64) = snap.RecordCount
				*dest[8].(*string) = snap.Status
				*dest[9].(*string) = snap.SchemaVersion
				*dest[10].(*pgtype.Timestamptz) = snap.ImportStartedAt
				*dest[11].(*pgtype.Timestamptz) = snap.ImportCompletedAt
				*dest[12].(*pgtype.Timestamptz) = snap.CreatedAt
				return nil
			}}
		}
		return &fakeRow{err: pgx.ErrNoRows}
	}
	if strings.Contains(sql, "INSERT INTO dataset_snapshot") {
		uid := args[0].(pgtype.UUID)
		source := args[1].(string)
		version := args[2].(string)
		fileName := args[3].(string)
		sha := args[4].(string)
		key := source + "|" + version + "|" + sha
		if _, exists := f.snapshots[key]; exists {
			return &fakeRow{err: &pgconn.PgError{Code: "23505", Message: "duplicate key"}}
		}
		snap := db.DatasetSnapshot{
			ID:            uid,
			Source:        source,
			SourceVersion: version,
			FileName:      fileName,
			FileSha256:    sha,
			RecordCount:   0,
			Status:        "PENDING",
			SchemaVersion: "v2.0",
			DownloadedAt:  pgtype.Timestamptz{Time: pgtype.Timestamptz{}.Time, Valid: true},
			CreatedAt:     pgtype.Timestamptz{Valid: true},
		}
		f.snapshots[key] = snap
		return &fakeRow{scan: func(dest ...any) error {
			if len(dest) < 13 {
				return errors.New("not enough dest")
			}
			*dest[0].(*pgtype.UUID) = snap.ID
			*dest[1].(*string) = snap.Source
			*dest[2].(*string) = snap.SourceVersion
			*dest[3].(*pgtype.Timestamptz) = snap.DownloadedAt
			*dest[4].(*pgtype.Timestamptz) = snap.PublishedAt
			*dest[5].(*string) = snap.FileName
			*dest[6].(*string) = snap.FileSha256
			*dest[7].(*int64) = snap.RecordCount
			*dest[8].(*string) = snap.Status
			*dest[9].(*string) = snap.SchemaVersion
			*dest[10].(*pgtype.Timestamptz) = snap.ImportStartedAt
			*dest[11].(*pgtype.Timestamptz) = snap.ImportCompletedAt
			*dest[12].(*pgtype.Timestamptz) = snap.CreatedAt
			return nil
		}}
	}
	if strings.Contains(sql, "INSERT INTO import_batch") {
		uid := args[0].(pgtype.UUID)
		bid := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		b := db.ImportBatch{
			ID:         bid,
			SnapshotID: uid,
			Status:     "RUNNING",
		}
		f.batches[uuid.UUID(uid.Bytes).String()] = b
		return &fakeRow{scan: func(dest ...any) error {
			if len(dest) < 11 {
				return errors.New("not enough dest for batch")
			}
			*dest[0].(*pgtype.UUID) = b.ID
			*dest[1].(*pgtype.UUID) = b.SnapshotID
			*dest[2].(*pgtype.Timestamptz) = b.StartedAt
			*dest[3].(*pgtype.Timestamptz) = b.CompletedAt
			*dest[4].(*string) = b.Status
			*dest[5].(*int64) = b.RecordsProcessed
			*dest[6].(*int64) = b.RecordsImported
			*dest[7].(*int64) = b.RecordsFailed
			*dest[8].(*int64) = b.RecordCount
			*dest[9].(*pgtype.Text) = b.ErrorMessage
			*dest[10].(*pgtype.Timestamptz) = b.CreatedAt
			return nil
		}}
	}
	return &fakeRow{err: errors.New("unhandled query: " + sql)}
}

func (f *fakeDB) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

type fakeRow struct {
	err  error
	scan func(dest ...any) error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.scan != nil {
		return r.scan(dest...)
	}
	return nil
}

func TestSnapshot_Idempotent(t *testing.T) {
	tmpRoot := t.TempDir()
	store := NewRawArchiveStore(filepath.Join(tmpRoot, "raw"))
	fdb := newFakeDB()
	repo := &testRepo{}
	downloader := &MockDownloader{Content: []byte("same content for idempotent")}
	svc := NewSnapshotService(downloader, store, repo, fdb)

	ctx := context.Background()
	snap1, err := svc.CreateFromDownload(ctx, "MOI", "2024Q1", "http://example.com/lvr.csv", "lvr.csv")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if snap1.ID == "" {
		t.Fatalf("snap1 ID empty")
	}
	if snap1.FileSHA256 == "" {
		t.Fatalf("sha empty")
	}
	snap2, err := svc.CreateFromDownload(ctx, "MOI", "2024Q1", "http://example.com/lvr.csv", "lvr.csv")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if snap1.ID != snap2.ID {
		t.Fatalf("idempotent failed: %s vs %s", snap1.ID, snap2.ID)
	}
	if snap1.FileSHA256 != snap2.FileSHA256 {
		t.Fatalf("sha mismatch")
	}
	snap3, err := svc.CreateFromDownload(ctx, "MOI", "2024Q2", "http://example.com/lvr.csv", "lvr.csv")
	if err != nil {
		t.Fatalf("third create diff version: %v", err)
	}
	if snap3.ID == snap1.ID {
		t.Fatalf("different version should create new snapshot")
	}
}

func TestSnapshot_NetworkError(t *testing.T) {
	tmpRoot := t.TempDir()
	store := NewRawArchiveStore(filepath.Join(tmpRoot, "raw"))
	fdb := newFakeDB()
	repo := &testRepo{}
	downloader := &MockDownloader{Err: errors.New("connection refused")}
	svc := NewSnapshotService(downloader, store, repo, fdb)
	_, err := svc.CreateFromDownload(context.Background(), "MOI", "2024Q1", "http://example.com/lvr.csv", "lvr.csv")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrDownloadFailed) && !strings.Contains(err.Error(), "download failed") {
		t.Fatalf("expected download failed error, got %v", err)
	}
	// Checksum mismatch
	downloader2 := &MockDownloader{Content: []byte("some content")}
	svc2 := NewSnapshotService(downloader2, store, repo, fdb)
	_, err = svc2.CreateFromDownloadWithChecksum(context.Background(), "MOI", "2024Q1", "http://example.com/other.csv", "other.csv", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	if err == nil {
		t.Fatalf("expected checksum mismatch")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
	// Context canceled download
	d := NewHTTPDownloader()
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer slowSrv.Close()
	_, err = d.Download(canceledCtx, slowSrv.URL, filepath.Join(tmpRoot, "tmpfile"))
	if err == nil {
		t.Fatalf("expected context canceled error")
	}
}
