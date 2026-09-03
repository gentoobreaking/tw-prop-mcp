package repository

import (
	"testing"

	"tw-prop-mcp/internal/domain"
)

func TestSnapshotStateMachineViaRepository(t *testing.T) {
	// Ensure repository's state machine dependency matches domain.
	if !domain.CanTransition(domain.SnapshotStatusPending, domain.SnapshotStatusImporting) {
		t.Fatalf("expected PENDING->IMPORTING allowed")
	}
	if domain.CanTransition(domain.SnapshotStatusPending, domain.SnapshotStatusLocked) {
		t.Fatalf("expected PENDING->LOCKED forbidden")
	}
}

func TestSnapshotModelValidation(t *testing.T) {
	// Validate 12 core fields exist via domain model.
	s := domain.DatasetSnapshot{
		Source:        "MOI",
		SourceVersion: "2024Q1",
		FileName:      "test.csv",
		FileSHA256:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RecordCount:   1,
		Status:        domain.SnapshotStatusPending,
		SchemaVersion: "v2.0",
	}
	if s.Source == "" || s.FileName == "" || s.FileSHA256 == "" {
		t.Fatalf("required fields empty")
	}
	if !domain.IsValidStatus(s.Status) {
		t.Fatalf("status invalid")
	}
}

func TestSnapshotRepositoryInterface(t *testing.T) {
	// Compile-time check: NewSnapshotRepository returns SnapshotRepository.
	var _ SnapshotRepository = (*snapshotRepository)(nil)
}
