package importpipeline

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/normalizer"
	"tw-prop-mcp/internal/repository"
	"tw-prop-mcp/internal/validator"
)

// mockDownloader implements downloader.Downloader for testing.
type mockDownloader struct {
	path string
	err  error
}

func (m *mockDownloader) Download(ctx context.Context, url, dest string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.path, nil
}

// mockSnapshotRepo implements repository.SnapshotRepository for testing.
type mockSnapshotRepo struct {
	snapshots map[string]*repository.CreateSnapshotParams
	locks     map[string]bool
}

func (m *mockSnapshotRepo) Create(ctx context.Context, arg repository.CreateSnapshotParams) (domain.DatasetSnapshot, error) {
	m.snapshots[arg.FileName] = &arg
	return domain.DatasetSnapshot{}, nil
}

func (m *mockSnapshotRepo) GetByID(ctx context.Context, id string) (domain.DatasetSnapshot, error) {
	for _, s := range m.snapshots {
		if s.FileSHA256 == id || s.FileName == id {
			return domain.DatasetSnapshot{}, nil
		}
	}
	return domain.DatasetSnapshot{}, errors.New("not found")
}

func (m *mockSnapshotRepo) List(ctx context.Context) ([]domain.DatasetSnapshot, error) {
	return nil, nil
}

func (m *mockSnapshotRepo) UpdateStatus(ctx context.Context, id string, to domain.SnapshotStatus) error {
	return nil
}

func (m *mockSnapshotRepo) Lock(ctx context.Context, id string) error {
	m.locks[id] = true
	return nil
}

func (m *mockSnapshotRepo) Delete(ctx context.Context, id string) error {
	return nil
}

// mockTxRepo implements repository.TransactionRepository for testing.
type mockTxRepo struct {
	inserted []domain.Transaction
}

func (m *mockTxRepo) GetByID(ctx context.Context, id string) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}

func (m *mockTxRepo) Search(ctx context.Context, filter repository.SearchFilter) ([]domain.Transaction, error) {
	return nil, nil
}

func (m *mockTxRepo) BatchInsert(ctx context.Context, txns []domain.Transaction) (int64, error) {
	m.inserted = append(m.inserted, txns...)
	return int64(len(txns)), nil
}

func (m *mockTxRepo) GetStatistics(ctx context.Context, county, district, section string) (repository.StatisticsResult, error) {
	return repository.StatisticsResult{}, nil
}

// mockParcelRepo implements repository.ParcelRepository for testing.
type mockParcelRepo struct {
	inserted []domain.Parcel
}

func (m *mockParcelRepo) GetByID(ctx context.Context, id string) (domain.Parcel, error) {
	return domain.Parcel{}, nil
}

func (m *mockParcelRepo) GetByLandNumber(ctx context.Context, county, district, section, landNumber string) (domain.Parcel, error) {
	return domain.Parcel{}, nil
}

func (m *mockParcelRepo) Search(ctx context.Context, filter repository.ParcelFilter) ([]domain.Parcel, error) {
	return nil, nil
}

func (m *mockParcelRepo) BatchInsert(ctx context.Context, parcels []domain.Parcel) (int64, error) {
	m.inserted = append(m.inserted, parcels...)
	return int64(len(parcels)), nil
}
func (m *mockParcelRepo) GetGeometry4326(ctx context.Context, id string) (string, string, string, error) {
	return "", "", "", nil
}
func TestImportPipeline_InitSnapshot(t *testing.T) {

	p := NewImportPipeline(PipelineConfig{
		SnapshotID:  "test-snapshot",
		DownloadURL: "http://example.com/test.csv",
	}, nil)

	// Can't test without real repo, but verify config
	if p.Config.SnapshotID != "test-snapshot" {
		t.Errorf("expected snapshot ID test-snapshot, got %s", p.Config.SnapshotID)
	}
}

func TestImportPipeline_StatusTransitions(t *testing.T) {
	p := NewImportPipeline(PipelineConfig{
		SnapshotID: "test-snapshot",
	}, nil)

	stages := []ImportPipelineStatus{
		StatusPending,
		StatusDownloading,
		StatusParsing,
		StatusNormalizing,
		StatusValidating,
		StatusImporting,
		StatusLocked,
	}

	for _, stage := range stages {
		p.setStatus(stage)
		if p.GetStatus() != stage {
			t.Errorf("expected status %s, got %s", stage, p.GetStatus())
		}
	}
}

func TestNormalizeAndValidate(t *testing.T) {
	n := normalizer.New()
	v := validator.New(nil)

	// Test valid transaction
	row := map[string]string{
		"transaction_id":         "TXN001",
		"transaction_date":       "2023-01-15",
		"transaction_type":       "A",
		"county":                 "台北市",
		"district":               "大安區",
		"section":                "忠孝段",
		"land_number":            "0001",
		"transaction_target":     "土地",
		"total_price":            "10000000",
		"unit_price":             "300000",
		"land_area_sqm":          "33.33",
		"building_area_sqm":      "0",
		"urban_zoning":           "住宅區",
		"non_urban_zoning":       "",
		"land_use_category":      "住宅",
		"building_type":          "",
		"floor":                  "",
		"age":                    "0",
		"parking_area_sqm":       "0",
		"parking_price":          "0",
		"source_record_hash":     "abc123",
	}

	txn, err := n.NormalizeTransaction(row, "snapshot-1")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	issues := v.ValidateTransaction(txn)
	if v.HasBlockingErrors(issues) {
		t.Errorf("valid transaction should pass validation: %v", issues)
	}

	// Test invalid transaction (negative price)
	row["total_price"] = "-100"
	txn2, _ := n.NormalizeTransaction(row, "snapshot-1")
	issues2 := v.ValidateTransaction(txn2)
	if !v.HasBlockingErrors(issues2) {
		t.Error("negative price should fail validation")
	}
}

func TestImportPipeline_Deduplicate(t *testing.T) {
	p := NewImportPipeline(PipelineConfig{}, nil)

	// Test transaction deduplication
	txns := []domain.Transaction{
		{SourceRecordHash: "hash1", County: "台北市", District: "大安區", Section: "段1", LandNumber: "001"},
		{SourceRecordHash: "hash1", County: "台北市", District: "大安區", Section: "段1", LandNumber: "001"}, // duplicate
		{SourceRecordHash: "hash2", County: "台北市", District: "大安區", Section: "段1", LandNumber: "002"},
	}

	parcels := []domain.Parcel{
		{County: "台北市", District: "大安區", Section: "段1", LandNumber: "001"},
		{County: "台北市", District: "大安區", Section: "段1", LandNumber: "001"}, // duplicate
		{County: "台北市", District: "大安區", Section: "段1", LandNumber: "003"},
	}

	dedupedTxns, dedupedParcels := p.deduplicate(txns, parcels)

	if len(dedupedTxns) != 2 {
		t.Errorf("expected 2 deduped transactions, got %d", len(dedupedTxns))
	}
	if len(dedupedParcels) != 2 {
		t.Errorf("expected 2 deduped parcels, got %d", len(dedupedParcels))
	}
}

func TestImportPipeline_VerifyChecksum(t *testing.T) {
	p := NewImportPipeline(PipelineConfig{
		ExpectedChecksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // SHA256 of empty
	}, nil)

	// Create empty temp file
	tmpFile, err := os.CreateTemp("", "checksum_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	err = p.verifyChecksum(tmpFile.Name())
	if err != nil {
		t.Errorf("empty file checksum should match: %v", err)
	}
}

func TestImportPipeline_FailedChecksum(t *testing.T) {
	p := NewImportPipeline(PipelineConfig{
		ExpectedChecksum: "wrong_checksum",
	}, nil)

	tmpFile, err := os.CreateTemp("", "checksum_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test data")
	tmpFile.Close()

	err = p.verifyChecksum(tmpFile.Name())
	if err == nil {
		t.Error("expected checksum mismatch error")
	}
}

func TestImportPipeline_RetryableError(t *testing.T) {
	p := NewImportPipeline(PipelineConfig{}, nil)

	// Test retryable errors
	retryable := p.RetryableError(&ImportPipelineError{
		Stage: "download",
		Err:   context.DeadlineExceeded,
	})
	if !retryable {
		t.Error("DeadlineExceeded should be retryable")
	}

	retryable = p.RetryableError(&ImportPipelineError{
		Stage: "download",
		Err:   io.EOF,
	})
	if !retryable {
		t.Error("EOF should be retryable")
	}

	// Test non-retryable
	nonRetryable := p.RetryableError(&ImportPipelineError{
		Stage: "validate",
		Err:   errors.New("validation failed"),
	})
	if nonRetryable {
		t.Error("validation error should not be retryable")
	}
}