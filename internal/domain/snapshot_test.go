package domain

import (
	"testing"
	"time"
)

func TestSnapshotStateMachine(t *testing.T) {
	tests := []struct {
		name string
		from SnapshotStatus
		to   SnapshotStatus
		want bool
	}{
		{"PENDING->IMPORTING valid", SnapshotStatusPending, SnapshotStatusImporting, true},
		{"IMPORTING->LOCKED valid", SnapshotStatusImporting, SnapshotStatusLocked, true},
		{"IMPORTING->FAILED valid", SnapshotStatusImporting, SnapshotStatusFailed, true},
		{"PENDING->LOCKED invalid", SnapshotStatusPending, SnapshotStatusLocked, false},
		{"PENDING->FAILED invalid", SnapshotStatusPending, SnapshotStatusFailed, false},
		{"PENDING->PENDING invalid", SnapshotStatusPending, SnapshotStatusPending, false},
		{"IMPORTING->PENDING invalid", SnapshotStatusImporting, SnapshotStatusPending, false},
		{"IMPORTING->IMPORTING invalid", SnapshotStatusImporting, SnapshotStatusImporting, false},
		{"LOCKED->any invalid PENDING", SnapshotStatusLocked, SnapshotStatusPending, false},
		{"LOCKED->IMPORTING invalid", SnapshotStatusLocked, SnapshotStatusImporting, false},
		{"LOCKED->LOCKED invalid", SnapshotStatusLocked, SnapshotStatusLocked, false},
		{"LOCKED->FAILED invalid", SnapshotStatusLocked, SnapshotStatusFailed, false},
		{"FAILED->PENDING invalid", SnapshotStatusFailed, SnapshotStatusPending, false},
		{"FAILED->IMPORTING invalid", SnapshotStatusFailed, SnapshotStatusImporting, false},
		{"FAILED->LOCKED invalid", SnapshotStatusFailed, SnapshotStatusLocked, false},
		{"FAILED->FAILED invalid", SnapshotStatusFailed, SnapshotStatusFailed, false},
		{"unknown from invalid", SnapshotStatus("UNKNOWN"), SnapshotStatusPending, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CanTransition(tc.from, tc.to)
			if got != tc.want {
				t.Fatalf("CanTransition(%s->%s)=%v want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestSnapshotStateMachineLockedTerminal(t *testing.T) {
	// LOCKED must block any transition, even to itself.
	for _, to := range []SnapshotStatus{SnapshotStatusPending, SnapshotStatusImporting, SnapshotStatusLocked, SnapshotStatusFailed} {
		if CanTransition(SnapshotStatusLocked, to) {
			t.Fatalf("LOCKED -> %s should be forbidden", to)
		}
	}
}

func TestSnapshotModelFields(t *testing.T) {
	now := time.Now()
	pub := now.Add(-24 * time.Hour)
	start := now.Add(-1 * time.Hour)
	end := now
	s := DatasetSnapshot{
		ID:                "00000000-0000-0000-0000-000000000001",
		Source:            "MOI",
		SourceVersion:     "2024Q1",
		DownloadedAt:      now,
		PublishedAt:       &pub,
		FileName:          "lvr_2024q1.csv",
		FileSHA256:        "abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd",
		RecordCount:       12345,
		Status:            SnapshotStatusPending,
		SchemaVersion:     "v2.0",
		ImportStartedAt:   &start,
		ImportCompletedAt: &end,
		CreatedAt:         now,
	}
	if s.Source != "MOI" || s.SourceVersion != "2024Q1" {
		t.Fatalf("source fields mismatch")
	}
	if s.FileSHA256 != "abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd" {
		t.Fatalf("sha256 mismatch")
	}
	if len(s.FileSHA256) != 64 {
		t.Fatalf("sha256 length should be 64, got %d", len(s.FileSHA256))
	}
	if s.RecordCount != 12345 {
		t.Fatalf("record_count mismatch")
	}
	if s.Status != SnapshotStatusPending {
		t.Fatalf("status mismatch")
	}
	if s.PublishedAt == nil || s.ImportStartedAt == nil || s.ImportCompletedAt == nil {
		t.Fatalf("nullable time fields should be set")
	}
	// Test nullable nil case
	s2 := DatasetSnapshot{
		Source:        "NLSC",
		SourceVersion: "2024Q2",
		FileName:      "parcel.csv",
		FileSHA256:    "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		Status:        SnapshotStatusImporting,
		SchemaVersion: "v2.0",
	}
	if s2.PublishedAt != nil || s2.ImportStartedAt != nil || s2.ImportCompletedAt != nil {
		t.Fatalf("nil nullable fields expected")
	}
}

func TestIsValidStatus(t *testing.T) {
	valid := []SnapshotStatus{SnapshotStatusPending, SnapshotStatusImporting, SnapshotStatusLocked, SnapshotStatusFailed}
	for _, s := range valid {
		if !IsValidStatus(s) {
			t.Fatalf("%s should be valid", s)
		}
	}
	if IsValidStatus("UNKNOWN") {
		t.Fatalf("UNKNOWN should be invalid")
	}
	if IsValidStatus("") {
		t.Fatalf("empty should be invalid")
	}
}
