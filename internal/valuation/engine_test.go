package valuation

import (
	"context"
	"testing"

	"tw-prop-mcp/internal/domain"
)

func TestNewValuationEngine_Defaults(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{})
	if eng.config.MinimumRequiredComparables != 3 {
		t.Errorf("default min comparables = %d, want 3", eng.config.MinimumRequiredComparables)
	}
	if eng.config.OutlierMethod != "IQR" {
		t.Errorf("default outlier method = %v, want IQR", eng.config.OutlierMethod)
	}
	if eng.config.IQRK != 1.5 {
		t.Errorf("default IQRK = %v, want 1.5", eng.config.IQRK)
	}
}

func TestValuate_InsufficientComparables(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	params := ValuationParams{
		TargetTransactionID:   "target-1",
		SnapshotID:            "snapshot-1",
		AlgorithmVersion:      "v2.0",
		ConfigurationVersion:  "v2.0",
		OutlierMethod:         "IQR",
	}

	result, err := eng.Valuate(ctx, params, []domain.ComparableResult{}, nil)
	if err != nil {
		t.Fatalf("Valuate returned error: %v", err)
	}

	if result.Status != "INSUFFICIENT_DATA" {
		t.Errorf("status = %v, want INSUFFICIENT_DATA", result.Status)
	}
	if result.Confidence != domain.ConfidenceInsufficient {
		t.Errorf("confidence = %v, want %v", result.Confidence, domain.ConfidenceInsufficient)
	}
}

func TestValuate_SufficientComparables(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	params := ValuationParams{
		TargetTransactionID:   "target-1",
		SnapshotID:            "snapshot-1",
		AlgorithmVersion:      "v2.0",
		ConfigurationVersion:  "v2.0",
		OutlierMethod:         "IQR",
	}

	comparables := make([]domain.ComparableResult, 5)
	for i := range comparables {
		comparables[i] = domain.ComparableResult{
			CandidateTransactionID: "candidate-" + string(rune('A'+i)),
			TotalScore:            0.8,
			AreaSimilarity:        0.9,
			DistanceM:             100.0,
			ZoningMatch:           true,
			LandUseMatch:          true,
			RoadAccessMatch:       true,
		}
	}

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{
				Count:   5,
				Min:     50000,
				P10:     55000,
				P25:     60000,
				Median:  65000,
				Mean:    65000,
				P75:     70000,
				P90:     75000,
				MaxVal:  80000,
			},
		},
		AlgorithmVersion: "v2.0",
	}

	result, err := eng.Valuate(ctx, params, comparables, stats)
	if err != nil {
		t.Fatalf("Valuate returned error: %v", err)
	}

	if result.Status != "COMPLETED" {
		t.Errorf("status = %v, want COMPLETED", result.Status)
	}
	if result.Confidence != domain.ConfidenceHigh {
		t.Errorf("confidence = %v, want %v", result.Confidence, domain.ConfidenceHigh)
	}
	if result.BaseValue == 0 {
		t.Error("base value should not be zero")
	}
	if len(result.ComparableIDs) != 5 {
		t.Errorf("comparable_ids count = %d, want 5", len(result.ComparableIDs))
	}
	if result.AlgorithmVersion != "v2.0" {
		t.Errorf("algorithm version = %v, want v2.0", result.AlgorithmVersion)
	}
}

func TestValuate_BearBaseBullRange(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	params := ValuationParams{
		TargetTransactionID:  "target-1",
		SnapshotID:           "snapshot-1",
		AlgorithmVersion:     "v2.0",
		ConfigurationVersion: "v2.0",
		OutlierMethod:        "IQR",
	}

	comparables := make([]domain.ComparableResult, 5)
	for i := range comparables {
		comparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.9,
			AreaSimilarity:        0.95,
			DistanceM:             100.0,
			ZoningMatch:           true,
			LandUseMatch:          true,
			RoadAccessMatch:       true,
		}
	}

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{
				Count:  5,
				Min:    50000,
				P25:    60000,
				Median: 65000,
				Mean:   65000,
				P75:    70000,
				MaxVal: 80000,
			},
		},
	}

	result, err := eng.Valuate(ctx, params, comparables, stats)
	if err != nil {
		t.Fatalf("Valuate error: %v", err)
	}

	// Bear (P25) should be <= Base (P50) <= Bull (P75)
	if result.BearValue > result.BaseValue {
		t.Errorf("bear_value %d > base_value %d, bear should be <= base", result.BearValue, result.BaseValue)
	}
	if result.BaseValue > result.BullValue {
		t.Errorf("base_value %d > bull_value %d, base should be <= bull", result.BaseValue, result.BullValue)
	}
}

func TestValuate_Deterministic(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	params := ValuationParams{
		TargetTransactionID:  "target-1",
		SnapshotID:           "snapshot-1",
		AlgorithmVersion:     "v2.0",
		ConfigurationVersion: "v2.0",
		OutlierMethod:        "IQR",
	}

	comparables := make([]domain.ComparableResult, 5)
	for i := range comparables {
		comparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.9,
			AreaSimilarity:        0.95,
			DistanceM:             100.0,
			ZoningMatch:           true,
			LandUseMatch:          true,
			RoadAccessMatch:       true,
		}
	}

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{
				Count:  5,
				Min:    50000,
				P25:    60000,
				Median: 65000,
				Mean:   65000,
				P75:    70000,
				MaxVal: 80000,
			},
		},
	}

	// Run twice — must produce identical results (except auto-generated ID/CreatedAt)
	result1, err := eng.Valuate(ctx, params, comparables, stats)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	result2, err := eng.Valuate(ctx, params, comparables, stats)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Deterministic: same valuation values
	if result1.BearValue != result2.BearValue {
		t.Errorf("bear values differ: %d != %d", result1.BearValue, result2.BearValue)
	}
	if result1.BaseValue != result2.BaseValue {
		t.Errorf("base values differ: %d != %d", result1.BaseValue, result2.BaseValue)
	}
	if result1.BullValue != result2.BullValue {
		t.Errorf("bull values differ: %d != %d", result1.BullValue, result2.BullValue)
	}
	if result1.Confidence != result2.Confidence {
		t.Errorf("confidence differs: %v != %v", result1.Confidence, result2.Confidence)
	}
	if result1.QueryHash != result2.QueryHash {
		t.Errorf("query hash differs: %v != %v", result1.QueryHash, result2.QueryHash)
	}
}

func TestValuate_DifferentInput_DifferentHash(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	params1 := ValuationParams{
		TargetTransactionID: "target-1",
		SnapshotID:          "snapshot-1",
		AlgorithmVersion:    "v2.0",
	}

	params2 := ValuationParams{
		TargetTransactionID: "target-2",
		SnapshotID:          "snapshot-1",
		AlgorithmVersion:    "v2.0",
	}

	comparables := make([]domain.ComparableResult, 5)
	for i := range comparables {
		comparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.9,
			AreaSimilarity:        0.95,
			DistanceM:             100.0,
			ZoningMatch:           true,
			LandUseMatch:          true,
			RoadAccessMatch:       true,
		}
	}

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{
				Count: 5, Median: 65000,
			},
		},
	}

	result1, err := eng.Valuate(ctx, params1, comparables, stats)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	result2, err := eng.Valuate(ctx, params2, comparables, stats)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if result1.QueryHash == result2.QueryHash {
		t.Errorf("query hashes should differ for different target IDs")
	}
}

func TestValuate_ConfidenceLevels(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{
				Count:  5,
				Median: 65000,
			},
		},
	}

	// HIGH: 5+ comparables, all matches, good scores
	highComparables := make([]domain.ComparableResult, 5)
	for i := range highComparables {
		highComparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.9,
			AreaSimilarity:        0.95,
			DistanceM:             100.0,
			ZoningMatch:           true,
			LandUseMatch:          true,
			RoadAccessMatch:       true,
		}
	}

	params := ValuationParams{
		TargetTransactionID: "target",
		SnapshotID:          "snap",
		AlgorithmVersion:    "v2.0",
	}

	result, err := eng.Valuate(ctx, params, highComparables, stats)
	if err != nil {
		t.Fatalf("Valuate error: %v", err)
	}
	if result.Confidence != domain.ConfidenceHigh {
		t.Errorf("confidence = %v, want %v", result.Confidence, domain.ConfidenceHigh)
	}

	// LOW: enough count but poor matches
	lowComparables := make([]domain.ComparableResult, 5)
	for i := range lowComparables {
		lowComparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.1,
			AreaSimilarity:        0.1,
			DistanceM:            1000.0,
			ZoningMatch:           false,
			LandUseMatch:          false,
			RoadAccessMatch:       false,
		}
	}

	result, err = eng.Valuate(ctx, params, lowComparables, stats)
	if err != nil {
		t.Fatalf("Valuate error: %v", err)
	}
	if result.Confidence != domain.ConfidenceLow {
		t.Errorf("confidence = %v, want %v", result.Confidence, domain.ConfidenceLow)
	}
}

func TestValuate_MediumConfidence(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	params := ValuationParams{
		TargetTransactionID: "target",
		SnapshotID:          "snap",
		AlgorithmVersion:    "v2.0",
	}

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{
				Count:  5,
				Median: 65000,
			},
		},
	}

	// MEDIUM: 5 comparables, 3 zoning match (60%), area sim 0.7
	comparables := make([]domain.ComparableResult, 5)
	for i := range comparables {
		comparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.5,
			AreaSimilarity:        0.7,
			DistanceM:             200.0,
			ZoningMatch:           i < 3, // 3 out of 5 match
			LandUseMatch:          false,
			RoadAccessMatch:       false,
		}
	}

	result, err := eng.Valuate(ctx, params, comparables, stats)
	if err != nil {
		t.Fatalf("Valuate error: %v", err)
	}
	if result.Confidence != domain.ConfidenceMedium {
		t.Errorf("confidence = %v, want %v", result.Confidence, domain.ConfidenceMedium)
	}
}

func TestPercentile_Valuation(t *testing.T) {
	tests := []struct {
		name   string
		sorted []float64
		p      float64
		want   float64
	}{
		{"empty", []float64{}, 50, 0},
		{"single", []float64{42}, 50, 42},
		{"p50_odd", []float64{10, 20, 30, 40, 50}, 50, 30},
		{"p50_even", []float64{10, 20, 30, 40}, 50, 25},
		{"p25", []float64{10, 20, 30, 40, 50}, 25, 20},
		{"p75", []float64{10, 20, 30, 40, 50}, 75, 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.sorted, tt.p)
			if got != tt.want {
				t.Errorf("percentile(%v, %v) = %v, want %v", tt.sorted, tt.p, tt, tt.want)
			}
		})
	}
}

func TestWeightedMedian(t *testing.T) {
	prices := []float64{100, 200, 300, 400, 500}
	weights := []float64{1, 1, 1, 1, 1}
	// Equal weights → weighted median = regular median = 300
	result := WeightedMedian(prices, weights)
	if result != 300 {
		t.Errorf("weighted median with equal weights = %v, want 300", result)
	}
}

func TestWeightedMedian_Weighted(t *testing.T) {
	prices := []float64{100, 200, 300}
	weights := []float64{1, 10, 1}
	// Heavy weight on 200 → median should be 200
	result := WeightedMedian(prices, weights)
	if result != 200 {
		t.Errorf("weighted median = %v, want 200", result)
	}
}

func TestWeightedMedian_Empty(t *testing.T) {
	result := WeightedMedian([]float64{}, []float64{})
	if result != 0 {
		t.Errorf("weighted median of empty = %v, want 0", result)
	}
}

func TestValuate_ResultIncludesStatistics(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	params := ValuationParams{
		TargetTransactionID: "target-1",
		SnapshotID:          "snapshot-1",
		AlgorithmVersion:    "v2.0",
	}

	comparables := make([]domain.ComparableResult, 5)
	for i := range comparables {
		comparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.9,
			AreaSimilarity:        0.95,
			DistanceM:             100.0,
			ZoningMatch:           true,
			LandUseMatch:          true,
			RoadAccessMatch:       true,
		}
	}

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{
				Count:  5,
				Median: 65000,
			},
		},
		AlgorithmVersion: "v2.0",
		OutlierInfo: domain.OutlierInfo{
			Method: "IQR",
		},
	}

	result, err := eng.Valuate(ctx, params, comparables, stats)
	if err != nil {
		t.Fatalf("Valuate error: %v", err)
	}

	if result.RawStatistics == nil {
		t.Error("RawStatistics should be populated in result")
	}
	if result.RawStatistics.AlgorithmVersion != "v2.0" {
		t.Errorf("stats algorithm version = %v, want v2.0", result.RawStatistics.AlgorithmVersion)
	}
}

func TestValuate_QueryHashDeterministic(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.DefaultValuationConfig(),
	})
	ctx := context.Background()

	params := ValuationParams{
		TargetTransactionID:  "target-1",
		SnapshotID:           "snapshot-1",
		AlgorithmVersion:     "v2.0",
		ConfigurationVersion: "config-v1",
		OutlierMethod:        "IQR",
	}

	comparables := make([]domain.ComparableResult, 5)
	for i := range comparables {
		comparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.8,
			AreaSimilarity:        0.9,
			DistanceM:             100.0,
			ZoningMatch:           true,
			LandUseMatch:          true,
			RoadAccessMatch:       true,
		}
	}

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{Count: 5, Median: 65000},
		},
	}

	r1, _ := eng.Valuate(ctx, params, comparables, stats)
	r2, _ := eng.Valuate(ctx, params, comparables, stats)

	if r1.QueryHash != r2.QueryHash {
		t.Errorf("query hash should be deterministic: %v != %v", r1.QueryHash, r2.QueryHash)
	}
	if r1.QueryHash == "" {
		t.Error("query hash should not be empty")
	}
}

func TestValuate_InsufficientBelowMinimum(t *testing.T) {
	eng := NewValuationEngine(ValuationEngineConfig{
		Config: domain.ValuationConfig{
			MinimumRequiredComparables: 3,
			OutlierMethod:              "IQR",
			IQRK:                       1.5,
		},
	})
	ctx := context.Background()

	params := ValuationParams{
		TargetTransactionID: "target-1",
		SnapshotID:          "snapshot-1",
		AlgorithmVersion:    "v2.0",
	}

	// Only 2 comparables, minimum is 3
	comparables := make([]domain.ComparableResult, 2)
	for i := range comparables {
		comparables[i] = domain.ComparableResult{
			CandidateTransactionID: "cand-" + string(rune('A'+i)),
			TotalScore:            0.9,
		}
	}

	stats := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing: domain.PriceStatistics{Count: 2, Median: 65000},
		},
	}

	result, err := eng.Valuate(ctx, params, comparables, stats)
	if err != nil {
		t.Fatalf("Valuate error: %v", err)
	}

	if result.Status != "INSUFFICIENT_DATA" {
		t.Errorf("status = %v, want INSUFFICIENT_DATA", result.Status)
	}
	if result.Confidence != domain.ConfidenceInsufficient {
		t.Errorf("confidence = %v, want %v", result.Confidence, domain.ConfidenceInsufficient)
	}
	if result.BaseValue != 0 {
		t.Errorf("base value = %d, want 0 for insufficient data", result.BaseValue)
	}
}
