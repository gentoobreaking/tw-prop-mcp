package domain

import (
	"encoding/json"
	"time"
)

// Provenance tracks the source and version of data
type Provenance struct {
	Source        string    `json:"source"`
	SourceVersion string    `json:"source_version"`
	RetrievedAt   time.Time `json:"retrieved_at"`
}

// ProvenanceInfo holds the full provenance chain for a transaction or valuation.
// P6 Provenance Required: every result must answer "where did this data come from?"
type ProvenanceInfo struct {
	Source           string `json:"source"`
	DatasetSnapshot  string `json:"dataset_snapshot"`
	SourceFile       string `json:"source_file"`
	RecordHash       string `json:"record_hash"`
	ImportBatchID    string `json:"import_batch_id"`
	AlgorithmVersion string `json:"algorithm_version"`
}

// DataProvenance is the provenance block injected into MCP responses.
// Required by P6: every transaction result must contain this.
type DataProvenance struct {
	Source               string `json:"source"`
	DatasetSnapshot      string `json:"dataset_snapshot"`
	SourceFile           string `json:"source_file"`
	RecordHash           string `json:"record_hash"`
	ImportBatchID        string `json:"import_batch_id"`
	AlgorithmVersion     string `json:"algorithm_version"`
	ConfigurationVersion string `json:"configuration_version"`
}

// ResponseMetadata holds metadata injected into every MCP tool response.
type ResponseMetadata struct {
	AlgorithmVersion     string `json:"algorithm_version"`
	SnapshotID           string `json:"snapshot_id"`
	GeneratedAt          string `json:"generated_at"`           // RFC3339
	QueryHash            string `json:"query_hash"`             // SHA256 hex, deterministic
	ConfigurationVersion string `json:"configuration_version,omitempty"`
	OutlierMethod        string `json:"outlier_method,omitempty"`
}

// ResponseEnvelope wraps every service/MCP response with metadata and provenance.
// ProvenanceMiddleware injects these before returning to the MCP layer.
type ResponseEnvelope struct {
	Metadata       ResponseMetadata `json:"metadata"`
	DataProvenance DataProvenance   `json:"data_provenance"`
}

// ProvenanceChain represents the full traceability chain from valuation
// back to the official source. Valuation → Comparables → Transactions → Snapshot.
type ProvenanceChain struct {
	// Valuation metadata
	ValuationID          string `json:"valuation_id"`
	ValuationStatus      string `json:"valuation_status"`
	BearValue            int64  `json:"bear_value"`
	BaseValue            int64  `json:"base_value"`
	BullValue            int64  `json:"bull_value"`

	// Target
	TargetParcel         string `json:"target_parcel"`
	TargetParcelLocation string `json:"target_parcel_location"`
	TargetTransactionID  string `json:"target_transaction_id,omitempty"`

	// Algorithm provenance
	AlgorithmVersion     string `json:"algorithm_version"`
	ConfigurationVersion string `json:"configuration_version"`
	OutlierMethod        string `json:"outlier_method"`
	Confidence           string `json:"confidence"`

	// Comparables
	ComparableIDs          []string         `json:"comparable_ids"`
	ComparableProvenance   []ProvenanceInfo `json:"comparable_provenance"`

	// Chain: transaction -> snapshot -> source
	Source           string `json:"source"`
	DatasetSnapshot  string `json:"dataset_snapshot"`
	SourceFile       string `json:"source_file"`
	SnapshotSHA256   string `json:"snapshot_sha256"`
	SnapshotStatus   string `json:"snapshot_status"`

	// Raw statistics and weights snapshots
	Statistics json.RawMessage `json:"statistics,omitempty"`
	Weights    json.RawMessage `json:"weights,omitempty"`

	// Chain status
	Status    string    `json:"status"`    // "AVAILABLE" or "DATA_NOT_AVAILABLE"
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}
