package domain

import (
	"time"
)

// ConfidenceLevel represents the confidence of a valuation
type ConfidenceLevel string

const (
	ConfidenceHigh       ConfidenceLevel = "HIGH"
	ConfidenceMedium     ConfidenceLevel = "MEDIUM"
	ConfidenceLow        ConfidenceLevel = "LOW"
	ConfidenceInsufficient ConfidenceLevel = "INSUFFICIENT"
)

// ValuationResult represents a complete land valuation result.
// Deterministic per P3: same snapshot + config + algorithm → same result.
type ValuationResult struct {
	ID                  string             `json:"id"`
	TargetParcelID      string             `json:"target_parcel_id"`
	TargetTransactionID string             `json:"target_transaction_id"`
	SnapshotID          string             `json:"snapshot_id"`
	ComparableIDs       []string           `json:"comparable_ids"`
	AlgorithmVersion    string             `json:"algorithm_version"`
	ConfigurationVersion string            `json:"configuration_version"`
	OutlierMethod       string             `json:"outlier_method"`

	// Valuation range (per ping, 元/坪)
	BearValue  int64   `json:"bear_value"`   // P25 adjusted
	BaseValue  int64   `json:"base_value"`   // P50 adjusted (weighted median)
	BullValue  int64   `json:"bull_value"`   // P75 adjusted

	// Raw statistics
	RawStatistics *StatisticsResult `json:"raw_statistics,omitempty"`

	// Confidence
	Confidence ConfidenceLevel `json:"confidence"`
	Status     string          `json:"status"` // "COMPLETED" or "INSUFFICIENT_DATA"

	// Provenance
	QueryHash    string    `json:"query_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

// ValuationConfig holds the weights and parameters for valuation
type ValuationConfig struct {
	MinimumRequiredComparables int
	OutlierMethod              string
	IQRK                       float64
}

// DefaultValuationConfig returns default valuation configuration
func DefaultValuationConfig() ValuationConfig {
	return ValuationConfig{
		MinimumRequiredComparables: 3,
		OutlierMethod:              "IQR",
		IQRK:                       1.5,
	}
}
