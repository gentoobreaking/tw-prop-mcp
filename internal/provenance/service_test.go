package provenance

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// mockSnapshotRepo implements repository.SnapshotRepository for testing.
type mockSnapshotRepo struct {
	snapshots map[string]domain.DatasetSnapshot
	err       error
}

func (m *mockSnapshotRepo) Create(ctx context.Context, arg repository.CreateSnapshotParams) (domain.DatasetSnapshot, error) {
	return domain.DatasetSnapshot{}, nil
}

func (m *mockSnapshotRepo) GetByID(ctx context.Context, id string) (domain.DatasetSnapshot, error) {
	if m.err != nil {
		return domain.DatasetSnapshot{}, m.err
	}
	snap, ok := m.snapshots[id]
	if !ok {
		return domain.DatasetSnapshot{}, fmt.Errorf("snapshot %s not found", id)
	}
	return snap, nil
}

func (m *mockSnapshotRepo) List(ctx context.Context) ([]domain.DatasetSnapshot, error) {
	return nil, nil
}

func (m *mockSnapshotRepo) UpdateStatus(ctx context.Context, id string, to domain.SnapshotStatus) error {
	return nil
}

func (m *mockSnapshotRepo) Lock(ctx context.Context, id string) error {
	return nil
}

func (m *mockSnapshotRepo) Delete(ctx context.Context, id string) error {
	return nil
}

// mockTxRepo implements repository.TransactionRepository for testing.
type mockTxRepo struct {
	transactions map[uuid.UUID]domain.Transaction
	err          error
}

func (m *mockTxRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error) {
	if m.err != nil {
		return domain.Transaction{}, m.err
	}
	tx, ok := m.transactions[id]
	if !ok {
		return domain.Transaction{}, fmt.Errorf("transaction %s not found", id.String())
	}
	return tx, nil
}

func (m *mockTxRepo) Search(ctx context.Context, filter repository.SearchFilter) ([]domain.Transaction, error) {
	return nil, nil
}

func (m *mockTxRepo) BatchInsert(ctx context.Context, txns []domain.Transaction) (int64, error) {
	return int64(len(txns)), nil
}

func (m *mockTxRepo) GetStatistics(ctx context.Context, county, district, section string) (repository.StatisticsResult, error) {
	return repository.StatisticsResult{}, nil
}

// mockParcelRepo implements repository.ParcelRepository for testing.
type mockParcelRepo struct{}

func (m *mockParcelRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Parcel, error) {
	return domain.Parcel{}, nil
}

func (m *mockParcelRepo) GetByLandNumber(ctx context.Context, county, district, section, landNumber string) (domain.Parcel, error) {
	return domain.Parcel{}, nil
}

func (m *mockParcelRepo) Search(ctx context.Context, filter repository.ParcelFilter) ([]domain.Parcel, error) {
	return nil, nil
}

func (m *mockParcelRepo) BatchInsert(ctx context.Context, parcels []domain.Parcel) (int64, error) {
	return int64(len(parcels)), nil
}

func (m *mockParcelRepo) GetGeometry4326(ctx context.Context, id string) (string, string, string, error) {
	return "", "", "", nil
}

// TestHashQuery_Deterministic verifies same input produces same hash.
func TestHashQuery_Deterministic(t *testing.T) {
	input := map[string]interface{}{
		"county":   "台北市",
		"district": "中正區",
		"limit":    10,
	}

	for i := 0; i < 100; i++ {
		hash1 := HashQuery(input, "comparable-v2.0", "v2.0", "snap-123")
		hash2 := HashQuery(input, "comparable-v2.0", "v2.0", "snap-123")
		if hash1 != hash2 {
			t.Fatalf("hash not deterministic on iteration %d: %v != %v", i, hash1, hash2)
		}
	}
}

// TestHashQuery_DifferentInput_DifferentHash verifies different input produces different hash.
func TestHashQuery_DifferentInput_DifferentHash(t *testing.T) {
	input1 := map[string]interface{}{"county": "台北市"}
	input2 := map[string]interface{}{"county": "新北市"}

	hash1 := HashQuery(input1, "comparable-v2.0", "v2.0", "snap")
	hash2 := HashQuery(input2, "comparable-v2.0", "v2.0", "snap")

	if hash1 == hash2 {
		t.Error("hash should differ for different input parameters")
	}
}

// TestHashQuery_DifferentAlgorithm_DifferentHash
func TestHashQuery_DifferentAlgorithm_DifferentHash(t *testing.T) {
	input := map[string]interface{}{"county": "台北市"}

	hash1 := HashQuery(input, "comparable-v2.0", "v2.0", "snap")
	hash2 := HashQuery(input, "comparable-v3.0", "v2.0", "snap")

	if hash1 == hash2 {
		t.Error("hash should differ when algorithm version changes")
	}
}

// TestHashQuery_DifferentConfig_DifferentHash
func TestHashQuery_DifferentConfig_DifferentHash(t *testing.T) {
	input := map[string]interface{}{"county": "台北市"}

	hash1 := HashQuery(input, "comparable-v2.0", "v2.0", "snap")
	hash2 := HashQuery(input, "comparable-v2.0", "v3.0", "snap")

	if hash1 == hash2 {
		t.Error("hash should differ when configuration version changes")
	}
}

// TestHashQuery_DifferentSnapshot_DifferentHash
func TestHashQuery_DifferentSnapshot_DifferentHash(t *testing.T) {
	input := map[string]interface{}{"county": "台北市"}

	hash1 := HashQuery(input, "comparable-v2.0", "v2.0", "snap-1")
	hash2 := HashQuery(input, "comparable-v2.0", "v2.0", "snap-2")

	if hash1 == hash2 {
		t.Error("hash should differ when snapshot ID changes")
	}
}

// TestHashQueryNoTimeDependency verifies hash does not depend on time.
func TestHashQuery_NoTimeDependency(t *testing.T) {
	input := map[string]interface{}{"county": "台北市", "time": time.Now()}

	// Two hashes generated at different times should be the same
	// because the input map is the same (time in map is same object)
	hash1 := HashQuery(input, "v2.0", "v2.0", "snap")
	hash2 := HashQuery(input, "v2.0", "v2.0", "snap")

	if hash1 != hash2 {
		t.Error("hash should not depend on time when input is the same")
	}
}

// TestHashQuery_SHA256Length verifies hash is 64 chars (SHA256 hex).
func TestHashQuery_SHA256Length(t *testing.T) {
	hash := HashQuery(map[string]interface{}{"key": "value"}, "v2.0", "v2.0", "snap")
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64 (SHA256 hex)", len(hash))
	}
}

// TestHashQuerySorted_Deterministic verifies sorted-key hashing is deterministic.
func TestHashQuerySorted_Deterministic(t *testing.T) {
	input := map[string]interface{}{
		"z_key": 1,
		"a_key": 2,
		"m_key": 3,
	}

	hash1 := HashQuerySorted(input, "v2.0", "v2.0", "snap")
	hash2 := HashQuerySorted(input, "v2.0", "v2.0", "snap")

	if hash1 != hash2 {
		t.Errorf("sorted hash not deterministic: %v != %v", hash1, hash2)
	}
	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}
}

// TestHashQuerySorted_MapOrderIndependent verifies that map key order doesn't matter.
func TestHashQuerySorted_MapOrderIndependent(t *testing.T) {
	input1 := map[string]interface{}{"a": 1, "b": 2, "c": 3}
	input2 := map[string]interface{}{"c": 3, "a": 1, "b": 2}

	hash1 := HashQuerySorted(input1, "v2.0", "v2.0", "snap")
	hash2 := HashQuerySorted(input2, "v2.0", "v2.0", "snap")

	if hash1 != hash2 {
		t.Errorf("sorted hash should be map-order independent: %v != %v", hash1, hash2)
	}
}

// TestGetSnapshot_Success
func TestGetSnapshot_Success(t *testing.T) {
	snapID := "test-snapshot-id"
	snapRepo := &mockSnapshotRepo{
		snapshots: map[string]domain.DatasetSnapshot{
			snapID: {
				ID:          snapID,
				Source:      "MOI_PLVR",
				SourceVersion: "2026-09-01",
				FileName:    "MANIFEST.CSV",
				FileSHA256:  "abc123",
				RecordCount: 1000,
				Status:      domain.SnapshotStatusLocked,
			},
		},
	}

	svc := NewProvenanceService(ProvenanceServiceConfig{
		SnapRepo: snapRepo,
	})

	snap, err := svc.GetSnapshot(context.Background(), snapID)
	if err != nil {
		t.Fatalf("GetSnapshot error: %v", err)
	}

	if snap.Source != "MOI_PLVR" {
		t.Errorf("source = %v, want MOI_PLVR", snap.Source)
	}
	if snap.FileName != "MANIFEST.CSV" {
		t.Errorf("file_name = %v, want MANIFEST.CSV", snap.FileName)
	}
	if snap.Status != domain.SnapshotStatusLocked {
		t.Errorf("status = %v, want LOCKED", snap.Status)
	}
}

// TestGetSnapshot_NotFound
func TestGetSnapshot_NotFound(t *testing.T) {
	snapRepo := &mockSnapshotRepo{
		snapshots: map[string]domain.DatasetSnapshot{},
	}

	svc := NewProvenanceService(ProvenanceServiceConfig{
		SnapRepo: snapRepo,
	})

	_, err := svc.GetSnapshot(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

// TestGetProvenanceByTransaction_Success
func TestGetProvenanceByTransaction_Success(t *testing.T) {
	txID := uuid.New()
	snapID := uuid.New()

	txRepo := &mockTxRepo{
		transactions: map[uuid.UUID]domain.Transaction{
			txID: {
				ID:                txID.String(),
				SnapshotID:        snapID.String(),
				ImportBatchID:     "batch-1",
				SourceRecordHash:  "hash-abc123",
				County:            "台北市",
				District:          "中正區",
			},
		},
	}

	snapRepo := &mockSnapshotRepo{
		snapshots: map[string]domain.DatasetSnapshot{
			snapID.String(): {
				ID:              snapID.String(),
				Source:          "MOI_PLVR",
				SourceVersion:   "2026-09-01",
				FileName:        "MANIFEST.CSV",
				FileSHA256:      "abc123",
				Status:          domain.SnapshotStatusLocked,
			},
		},
	}

	svc := NewProvenanceService(ProvenanceServiceConfig{
		TxRepo:   txRepo,
		SnapRepo: snapRepo,
	})

	info, err := svc.GetProvenanceByTransaction(context.Background(), txID.String())
	if err != nil {
		t.Fatalf("GetProvenanceByTransaction error: %v", err)
	}

	if info.Source != "MOI_PLVR" {
		t.Errorf("source = %v, want MOI_PLVR", info.Source)
	}
	if info.DatasetSnapshot != snapID.String() {
		t.Errorf("dataset_snapshot = %v, want %v", info.DatasetSnapshot, snapID.String())
	}
	if info.SourceFile != "MANIFEST.CSV" {
		t.Errorf("source_file = %v, want MANIFEST.CSV", info.SourceFile)
	}
	if info.RecordHash != "hash-abc123" {
		t.Errorf("record_hash = %v, want hash-abc123", info.RecordHash)
	}
	if info.ImportBatchID != "batch-1" {
		t.Errorf("import_batch_id = %v, want batch-1", info.ImportBatchID)
	}
}

// TestGetProvenanceByTransaction_NotFound
func TestGetProvenanceByTransaction_NotFound(t *testing.T) {
	txRepo := &mockTxRepo{
		transactions: map[uuid.UUID]domain.Transaction{},
	}
	snapRepo := &mockSnapshotRepo{snapshots: map[string]domain.DatasetSnapshot{}}

	svc := NewProvenanceService(ProvenanceServiceConfig{
		TxRepo:   txRepo,
		SnapRepo: snapRepo,
	})

	_, err := svc.GetProvenanceByTransaction(context.Background(), uuid.New().String())
	if err == nil {
		t.Error("expected error for nonexistent transaction")
	}
}

// TestGetProvenanceByTransaction_InvalidID
func TestGetProvenanceByTransaction_InvalidID(t *testing.T) {
	txRepo := &mockTxRepo{}
	svc := NewProvenanceService(ProvenanceServiceConfig{
		TxRepo:   txRepo,
		SnapRepo: &mockSnapshotRepo{},
	})

	_, err := svc.GetProvenanceByTransaction(context.Background(), "not-a-uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

// TestBuildProvenanceResponse
func TestBuildProvenanceResponse(t *testing.T) {
	snapRepo := &mockSnapshotRepo{}
	svc := NewProvenanceService(ProvenanceServiceConfig{
		SnapRepo: snapRepo,
	})

	tx := domain.Transaction{
		SourceRecordHash: "hash-123",
		ImportBatchID:    "batch-42",
		County:           "台北市",
	}
	snap := domain.DatasetSnapshot{
		ID:              "snap-1",
		Source:          "MOI_PLVR",
		FileName:        "data.csv",
		Status:          domain.SnapshotStatusLocked,
	}

	dp := svc.BuildProvenanceResponse(tx, snap)

	if dp.Source != "MOI_PLVR" {
		t.Errorf("source = %v, want MOI_PLVR", dp.Source)
	}
	if dp.DatasetSnapshot != "snap-1" {
		t.Errorf("dataset_snapshot = %v, want snap-1", dp.DatasetSnapshot)
	}
	if dp.RecordHash != "hash-123" {
		t.Errorf("record_hash = %v, want hash-123", dp.RecordHash)
	}
	if dp.ImportBatchID != "batch-42" {
		t.Errorf("import_batch_id = %v, want batch-42", dp.ImportBatchID)
	}
}

// TestBuildEnvelope
func TestBuildEnvelope(t *testing.T) {
	snapRepo := &mockSnapshotRepo{}
	svc := NewProvenanceService(ProvenanceServiceConfig{
		SnapRepo: snapRepo,
	})

	env := svc.BuildEnvelope("hash-abc", "v2.0", "config-v1", "snap-123")

	if env.Metadata.AlgorithmVersion != "v2.0" {
		t.Errorf("algorithm_version = %v, want v2.0", env.Metadata.AlgorithmVersion)
	}
	if env.Metadata.SnapshotID != "snap-123" {
		t.Errorf("snapshot_id = %v, want snap-123", env.Metadata.SnapshotID)
	}
	if env.Metadata.QueryHash != "hash-abc" {
		t.Errorf("query_hash = %v, want hash-abc", env.Metadata.QueryHash)
	}
	if env.Metadata.ConfigurationVersion != "config-v1" {
		t.Errorf("configuration_version = %v, want config-v1", env.Metadata.ConfigurationVersion)
	}
	if env.Metadata.GeneratedAt == "" {
		t.Error("generated_at should be set")
	}
}

// TestGetProvenanceByValuation_NoDBTX
func TestGetProvenanceByValuation_NoDBTX(t *testing.T) {
	svc := NewProvenanceService(ProvenanceServiceConfig{
		TxRepo:   &mockTxRepo{},
		SnapRepo: &mockSnapshotRepo{},
	})

	_, err := svc.GetProvenanceByValuation(context.Background(), "some-id")
	if err == nil {
		t.Error("expected error when dbtx is nil")
	}
}

// TestResponseEnvelope_Structure verifies the envelope has required fields.
func TestResponseEnvelope_Structure(t *testing.T) {
	env := domain.ResponseEnvelope{
		Metadata: domain.ResponseMetadata{
			AlgorithmVersion:     "v2.0",
			SnapshotID:           "snap-1",
			GeneratedAt:          "2026-09-04T00:00:00Z",
			QueryHash:            "abc123",
			ConfigurationVersion: "v2.0",
		},
		DataProvenance: domain.DataProvenance{
			Source:               "MOI_PLVR",
			DatasetSnapshot:      "snap-1",
			SourceFile:           "MANIFEST.CSV",
			RecordHash:           "hash-def456",
			ImportBatchID:        "batch-1",
			AlgorithmVersion:     "v2.0",
			ConfigurationVersion: "v2.0",
		},
	}

	if env.Metadata.QueryHash == "" {
		t.Error("query_hash should not be empty")
	}
	if env.DataProvenance.Source == "" {
		t.Error("data_provenance source should not be empty")
	}
}

// TestParseUUID_Valid
func TestParseUUID_Valid(t *testing.T) {
	uid := uuid.New().String()
	parsed, err := parseUUID(uid)
	if err != nil {
		t.Fatalf("parseUUID error: %v", err)
	}
	if parsed.String() != uid {
		t.Errorf("parsed UUID = %v, want %v", parsed.String(), uid)
	}
}

// TestParseUUID_Invalid
func TestParseUUID_Invalid(t *testing.T) {
	_, err := parseUUID("not-a-uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

// TestSortStable ensures sorted key order doesn't affect HashQuerySorted.
func TestSortStable(t *testing.T) {
	// Create two maps with same content but potentially different iteration order
	input := map[string]interface{}{
		"b": 2,
		"a": 1,
		"c": 3,
		"d": 4,
	}

	// Generate hash multiple times — should always be the same
	hashes := make([]string, 10)
	for i := range hashes {
		hashes[i] = HashQuerySorted(input, "v2.0", "v2.0", "snap")
	}

	// All should be the same (deterministic regardless of map iteration order)
	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("hash at iteration %d differs: %v != %v", i, hashes[i], hashes[0])
		}
	}
}

// Ensure sort import is used
var _ = sort.Strings
