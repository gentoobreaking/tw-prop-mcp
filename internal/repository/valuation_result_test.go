package repository_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// TestValuationResultDomain_Construction verifies the domain ValuationResult can be constructed.
func TestValuationResultDomain_Construction(t *testing.T) {
	result := domain.ValuationResult{
		ID:                   uuid.NewString(),
		TargetParcelID:       uuid.NewString(),
		TargetTransactionID:  uuid.NewString(),
		SnapshotID:           uuid.NewString(),
		ComparableIDs:        []string{"comp-1", "comp-2", "comp-3"},
		AlgorithmVersion:     "valuation-v2.0",
		ConfigurationVersion: "v2.0",
		OutlierMethod:        "IQR",
		Weights:              json.RawMessage(`{"W_area":0.30}`),
		BearValue:            50000,
		BaseValue:            65000,
		BullValue:            80000,
		Confidence:           domain.ConfidenceMedium,
		Status:               "COMPLETED",
		QueryHash:            "a1b2c3d4e5f6",
	}

	if string(result.Confidence) != "MEDIUM" {
		t.Errorf("confidence = %v, want MEDIUM", result.Confidence)
	}
	if result.BearValue != 50000 {
		t.Errorf("bear_value = %d, want 50000", result.BearValue)
	}
	if result.BaseValue != 65000 {
		t.Errorf("base_value = %d, want 65000", result.BaseValue)
	}
	if result.BullValue != 80000 {
		t.Errorf("bull_value = %d, want 80000", result.BullValue)
	}
}

// TestValuationWeights_Structure verifies the ValuationWeights struct.
func TestValuationWeights_Structure(t *testing.T) {
	w := repository.ValuationWeights{
		AreaSimilarityPct:          30,
		Lambda:                     0.05,
		DistanceScale:              500.0,
		WArea:                      0.30,
		WDistance:                  0.20,
		WTime:                      0.15,
		WZoning:                    0.15,
		WLandUse:                   0.10,
		WRoad:                      0.10,
		IQRK:                       1.5,
		MinimumRequiredComparables: 3,
		OutlierMethod:              "IQR",
	}

	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal weights: %v", err)
	}
	if len(b) == 0 {
		t.Error("marshaled weights should not be empty")
	}

	// Verify key fields are present in JSON
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal weights: %v", err)
	}
	if m["W_area"] != 0.3 {
		t.Errorf("W_area = %v, want 0.3", m["W_area"])
	}
}
