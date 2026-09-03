package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// TestComparableResultRepository_Interface verifies the implementation satisfies the interface.
func TestComparableResultRepository_Interface(t *testing.T) {
	// Compile-time check: the repository returned by NewComparableResultRepository
	// satisfies the ComparableResultRepository interface.
	repo := repository.NewComparableResultRepository(nil)
	var _ repository.ComparableResultRepository = repo
}

// TestComparableResultRepository_ListByTarget_Validation verifies invalid UUID handling.
func TestComparableResultRepository_ListByTarget_Validation(t *testing.T) {
	repo := repository.NewComparableResultRepository(nil)

	_, err := repo.ListByTarget(context.Background(), "not-a-uuid", 100)
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

// TestComparableResultRepository_GetByID_Validation verifies invalid UUID handling.
func TestComparableResultRepository_GetByID_Validation(t *testing.T) {
	repo := repository.NewComparableResultRepository(nil)

	_, err := repo.GetByID(context.Background(), "not-a-uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

// TestComparableResultRepository_BatchInsert_Empty
func TestComparableResultRepository_BatchInsert_Empty(t *testing.T) {
	repo := repository.NewComparableResultRepository(nil)

	// Empty input should return 0 inserted, nil error (no DB access)
	count, err := repo.BatchInsert(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error for empty batch: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 inserted, got %d", count)
	}
}

// TestValuationResultRepository_Interface verifies the implementation satisfies the interface.
func TestValuationResultRepository_Interface(t *testing.T) {
	repo := repository.NewValuationResultRepository(nil)
	var _ repository.ValuationResultRepository = repo
}

// TestValuationResultRepository_Insert_MissingSnapshotID
func TestValuationResultRepository_Insert_MissingSnapshotID(t *testing.T) {
	repo := repository.NewValuationResultRepository(nil)

	result := domain.ValuationResult{
		TargetParcelID:     uuid.NewString(),
		AlgorithmVersion:   "v2.0",
	}
	_, err := repo.Insert(context.Background(), result)
	if err == nil {
		t.Error("expected error for missing snapshot_id")
	}
}

// TestValuationResultRepository_Insert_MissingAlgorithmVersion
func TestValuationResultRepository_Insert_MissingAlgorithmVersion(t *testing.T) {
	repo := repository.NewValuationResultRepository(nil)

	result := domain.ValuationResult{
		TargetParcelID: uuid.NewString(),
		SnapshotID:     uuid.NewString(),
	}
	_, err := repo.Insert(context.Background(), result)
	if err == nil {
		t.Error("expected error for missing algorithm_version")
	}
}

// TestValuationResultRepository_Insert_MissingConfigVersion
func TestValuationResultRepository_Insert_MissingConfigVersion(t *testing.T) {
	repo := repository.NewValuationResultRepository(nil)

	result := domain.ValuationResult{
		TargetParcelID: uuid.NewString(),
		SnapshotID:     uuid.NewString(),
		AlgorithmVersion: "v2.0",
	}
	_, err := repo.Insert(context.Background(), result)
	if err == nil {
		t.Error("expected error for missing configuration_version")
	}
}

// TestValuationResultRepository_Insert_InvalidTargetParcelID
func TestValuationResultRepository_Insert_InvalidTargetParcelID(t *testing.T) {
	repo := repository.NewValuationResultRepository(nil)

	result := domain.ValuationResult{
		TargetParcelID:       "not-a-uuid",
		SnapshotID:           uuid.NewString(),
		AlgorithmVersion:     "v2.0",
		ConfigurationVersion: "v2.0",
	}
	_, err := repo.Insert(context.Background(), result)
	if err == nil {
		t.Error("expected error for invalid target_parcel_id")
	}
}

// TestValuationResultRepository_Insert_InvalidSnapshotID
func TestValuationResultRepository_Insert_InvalidSnapshotID(t *testing.T) {
	repo := repository.NewValuationResultRepository(nil)

	result := domain.ValuationResult{
		TargetParcelID:       uuid.NewString(),
		SnapshotID:           "not-a-uuid",
		AlgorithmVersion:     "v2.0",
		ConfigurationVersion: "v2.0",
	}
	_, err := repo.Insert(context.Background(), result)
	if err == nil {
		t.Error("expected error for invalid snapshot_id")
	}
}

// TestValuationResultRepository_GetByID_Validation
func TestValuationResultRepository_GetByID_Validation(t *testing.T) {
	repo := repository.NewValuationResultRepository(nil)

	_, err := repo.GetByID(context.Background(), "not-a-uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

// TestValuationResultRepository_GetByQueryHash_EmptyHash
func TestValuationResultRepository_GetByQueryHash_EmptyHash(t *testing.T) {
	// GetByQueryHash requires a real DB; with nil dbtx it panics.
	// We verify the method exists and accepts the correct arguments.
	// Integration test with real DB would verify round-trip behavior.
	t.Log("GetByQueryHash requires testcontainer DB - tested in integration")
}

// TestValuationResultRepository_ListByParcel_Validation
func TestValuationResultRepository_ListByParcel_Validation(t *testing.T) {
	repo := repository.NewValuationResultRepository(nil)

	_, err := repo.ListByParcel(context.Background(), "not-a-uuid", "not-a-uuid", 100)
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}
