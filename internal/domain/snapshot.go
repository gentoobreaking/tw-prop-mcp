package domain

import "time"

// SnapshotStatus represents the lifecycle state of a dataset snapshot.
type SnapshotStatus string

const (
	SnapshotStatusPending   SnapshotStatus = "PENDING"
	SnapshotStatusImporting SnapshotStatus = "IMPORTING"
	SnapshotStatusLocked    SnapshotStatus = "LOCKED"
	SnapshotStatusFailed    SnapshotStatus = "FAILED"
)

// DatasetSnapshot is the domain model for dataset_snapshot table.
// It maps 12 core fields (+ CreatedAt) per T003 acceptance criteria.
// P2 Raw Data Immutable: once LOCKED, the snapshot must not be mutated.
type DatasetSnapshot struct {
	ID                string         `json:"id"`
	Source            string         `json:"source"`
	SourceVersion     string         `json:"source_version"`
	DownloadedAt      time.Time      `json:"downloaded_at"`
	PublishedAt       *time.Time     `json:"published_at,omitempty"`
	FileName          string         `json:"file_name"`
	FileSHA256        string         `json:"file_sha256"`
	RecordCount       int64          `json:"record_count"`
	Status            SnapshotStatus `json:"status"`
	SchemaVersion     string         `json:"schema_version"`
	ImportStartedAt   *time.Time     `json:"import_started_at,omitempty"`
	ImportCompletedAt *time.Time     `json:"import_completed_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

// validTransitions defines the allowed state machine edges.
// PENDING -> IMPORTING -> LOCKED / FAILED
// LOCKED and FAILED are terminal.
var validTransitions = map[SnapshotStatus]map[SnapshotStatus]bool{
	SnapshotStatusPending: {
		SnapshotStatusImporting: true,
	},
	SnapshotStatusImporting: {
		SnapshotStatusLocked: true,
		SnapshotStatusFailed: true,
	},
}

// CanTransition reports whether a transition from -> to is allowed.
// LOCKED never allows any outgoing transition (terminal state).
func CanTransition(from, to SnapshotStatus) bool {
	if from == SnapshotStatusLocked {
		return false
	}
	if from == SnapshotStatusFailed {
		return false
	}
	m, ok := validTransitions[from]
	if !ok {
		return false
	}
	return m[to]
}

// IsValidStatus reports whether s is a known SnapshotStatus.
func IsValidStatus(s SnapshotStatus) bool {
	switch s {
	case SnapshotStatusPending, SnapshotStatusImporting, SnapshotStatusLocked, SnapshotStatusFailed:
		return true
	default:
		return false
	}
}
