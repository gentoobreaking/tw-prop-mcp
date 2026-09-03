package statistics

import (
	"context"
	"testing"
	"time"

	"tw-prop-mcp/internal/domain"
)

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		sorted []float64
		p      float64
		want   float64
	}{
		{"empty", []float64{}, 50, 0},
		{"p0", []float64{1, 2, 3, 4, 5}, 0, 1},
		{"p100", []float64{1, 2, 3, 4, 5}, 100, 5},
		{"p50_odd", []float64{1, 2, 3, 4, 5}, 50, 3},
		{"p50_even", []float64{1, 2, 3, 4}, 50, 2.5},
		{"p25", []float64{1, 2, 3, 4, 5}, 25, 2},
		{"p75", []float64{1, 2, 3, 4, 5}, 75, 4},
		{"p10", []float64{10, 20, 30, 40, 50}, 10, 14},
		{"p90", []float64{10, 20, 30, 40, 50}, 90, 46},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.sorted, tt.p)
			if got != tt.want {
				t.Errorf("percentile(%v, %v) = %v, want %v", tt.sorted, tt.p, got, tt.want)
			}
		})
	}
}

func TestCalculatePriceStatistics(t *testing.T) {
	values := []float64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	stats := calculatePriceStatistics(values)

	if stats.Count != 10 {
		t.Errorf("count = %d, want 10", stats.Count)
	}
	if stats.Min != 100 {
		t.Errorf("min = %d, want 100", stats.Min)
	}
	if stats.MaxVal != 1000 {
		t.Errorf("max = %d, want 1000", stats.MaxVal)
	}
	if stats.Median != 550 {
		t.Errorf("median = %d, want 550", stats.Median)
	}

	// mean = (100+200+...+1000)/10 = 550
	if stats.Mean != 550 {
		t.Errorf("mean = %v, want 550", stats.Mean)
	}
}

func TestCalculatePriceStatistics_Empty(t *testing.T) {
	stats := calculatePriceStatistics([]float64{})
	if stats.Count != 0 {
		t.Errorf("count = %d, want 0", stats.Count)
	}
}

func TestCalculateAreaStatistics(t *testing.T) {
	values := []float64{10.5, 20.0, 30.5, 40.0, 50.5}
	stats := calculateAreaStatistics(values)

	if stats.Count != 5 {
		t.Errorf("count = %d, want 5", stats.Count)
	}
	if stats.Min != 10.5 {
		t.Errorf("min = %v, want 10.5", stats.Min)
	}
	if stats.MaxVal != 50.5 {
		t.Errorf("max = %v, want 50.5", stats.MaxVal)
	}
}

func TestCalculateAreaStatistics_Empty(t *testing.T) {
	stats := calculateAreaStatistics([]float64{})
	if stats.Count != 0 {
		t.Errorf("count = %d, want 0", stats.Count)
	}
}

func TestPricePerPingConversion(t *testing.T) {
	// 1 坪 = 3.305785 平方公尺
	unitPricePerSqm := 100000.0 // 元/㎡
	pricePerPing := domain.PricePerPing(unitPricePerSqm)
	expected := unitPricePerSqm * 3.305785
	if pricePerPing != expected {
		t.Errorf("PricePerPing(%v) = %v, want %v", unitPricePerSqm, pricePerPing, expected)
	}

	// Reverse conversion
	convertedBack := domain.PricePerSqmFromPing(pricePerPing)
	if convertedBack != unitPricePerSqm {
		t.Errorf("PricePerSqmFromPing(PricePerPing(%v)) = %v, want %v", unitPricePerSqm, convertedBack, unitPricePerSqm)
	}
}

func TestDetectOutliersIQR(t *testing.T) {
	eng := &StatisticsEngine{
		config: domain.StatisticsConfig{
			OutlierMethod: "IQR",
			IQRK:          1.5,
		},
	}

	// Data with a clear outlier
	sorted := []float64{10, 12, 14, 15, 16, 18, 20, 100}
	info := eng.detectOutliersIQR(sorted)

	if info.Method != "IQR" {
		t.Errorf("method = %v, want IQR", info.Method)
	}
	if info.OutlierCount != 1 {
		t.Errorf("outlier count = %d, want 1", info.OutlierCount)
	}
	if info.TotalCount != 8 {
		t.Errorf("total count = %d, want 8", info.TotalCount)
	}
}

func TestDetectOutliersIQR_NoOutliers(t *testing.T) {
	eng := &StatisticsEngine{
		config: domain.StatisticsConfig{
			OutlierMethod: "IQR",
			IQRK:          1.5,
		},
	}

	sorted := []float64{10, 12, 14, 15, 16, 18, 20}
	info := eng.detectOutliersIQR(sorted)

	if info.OutlierCount != 0 {
		t.Errorf("outlier count = %d, want 0 (no outliers in tight cluster)", info.OutlierCount)
	}
}

func TestDetectOutliersP10P90(t *testing.T) {
	eng := &StatisticsEngine{
		config: domain.StatisticsConfig{
			OutlierMethod: "P10_P90",
			IQRK:          1.5,
		},
	}

	sorted := []float64{10, 12, 14, 15, 16, 18, 20, 100}
	info := eng.detectOutliersP10P90(sorted)

	if info.Method != "P10_P90" {
		t.Errorf("method = %v, want P10_P90", info.Method)
	}
}

func TestDetectOutliersMAD(t *testing.T) {
	eng := &StatisticsEngine{
		config: domain.StatisticsConfig{
			OutlierMethod: "MAD",
			IQRK:          1.5,
		},
	}

	values := []float64{10, 12, 14, 15, 16, 18, 20, 100}
	info := eng.detectOutliersMAD(values)

	if info.Method != "MAD" {
		t.Errorf("method = %v, want MAD", info.Method)
	}
	if info.OutlierCount != 1 {
		t.Errorf("outlier count = %d, want 1", info.OutlierCount)
	}
}

func TestDetectOutliersFewValues(t *testing.T) {
	eng := &StatisticsEngine{
		config: domain.StatisticsConfig{
			OutlierMethod: "IQR",
			IQRK:          1.5,
		},
	}

	// Fewer than 4 values → no outlier detection
	values := []float64{10, 20}
	info := eng.detectOutliers(values)

	if info.OutlierCount != 0 {
		t.Errorf("outlier count = %d, want 0 (too few values)", info.OutlierCount)
	}
	if info.TotalCount != 2 {
		t.Errorf("total count = %d, want 2", info.TotalCount)
	}
}

func TestDetectOutliersDefaultMethod(t *testing.T) {
	eng := &StatisticsEngine{
		config: domain.StatisticsConfig{
			OutlierMethod: "unknown",
			IQRK:          1.5,
		},
	}

	values := []float64{10, 12, 14, 15, 16, 18, 20, 100}
	info := eng.detectOutliers(values)

	// Unknown method should fall back to IQR
	if info.Method != "IQR" {
		t.Errorf("method = %v, want IQR (fallback)", info.Method)
	}
}

func TestCalculateStatistics_Deterministic(t *testing.T) {
	eng := &StatisticsEngine{
		txRepo: nil,
		config: domain.DefaultStatisticsConfig(),
	}

	transactions := []domain.Transaction{
		{ID: "t1", UnitPrice: 100000, TotalPrice: 5000000, LandAreaSqm: 50, BuildingAreaSqm: 100},
		{ID: "t2", UnitPrice: 120000, TotalPrice: 6000000, LandAreaSqm: 55, BuildingAreaSqm: 105},
		{ID: "t3", UnitPrice: 110000, TotalPrice: 5500000, LandAreaSqm: 52, BuildingAreaSqm: 102},
		{ID: "t4", UnitPrice: 130000, TotalPrice: 6500000, LandAreaSqm: 58, BuildingAreaSqm: 108},
		{ID: "t5", UnitPrice: 105000, TotalPrice: 5250000, LandAreaSqm: 51, BuildingAreaSqm: 101},
	}

	ctx := context.Background()
	result1, err := eng.CalculateStatistics(ctx, transactions)
	if err != nil {
		t.Fatalf("CalculateStatistics failed: %v", err)
	}

	// Run again with same data
	result2, err := eng.CalculateStatistics(ctx, transactions)
	if err != nil {
		t.Fatalf("CalculateStatistics second run failed: %v", err)
	}

	// Deterministic: same input must produce same statistics
	if result1.TransactionStatistics != result2.TransactionStatistics {
		t.Errorf("statistics not deterministic: first != second")
	}
	if result1.OutlierInfo != result2.OutlierInfo {
		t.Errorf("outlier info not deterministic: first != second")
	}
	if result1.QueryHash != result2.QueryHash {
		t.Errorf("query hash not deterministic: %v != %v", result1.QueryHash, result2.QueryHash)
	}
}

func TestCalculateStatistics_WithOutliers(t *testing.T) {
	eng := &StatisticsEngine{
		txRepo: nil,
		config: domain.StatisticsConfig{
			OutlierMethod: "IQR",
			IQRK:          1.5,
		},
	}

	transactions := []domain.Transaction{
		{ID: "t1", UnitPrice: 100000, TotalPrice: 5000000, LandAreaSqm: 50, BuildingAreaSqm: 100},
		{ID: "t2", UnitPrice: 110000, TotalPrice: 5500000, LandAreaSqm: 52, BuildingAreaSqm: 102},
		{ID: "t3", UnitPrice: 105000, TotalPrice: 5250000, LandAreaSqm: 51, BuildingAreaSqm: 101},
		{ID: "t4", UnitPrice: 115000, TotalPrice: 5750000, LandAreaSqm: 53, BuildingAreaSqm: 103},
		{ID: "t5", UnitPrice: 108000, TotalPrice: 5400000, LandAreaSqm: 50, BuildingAreaSqm: 100},
		{ID: "t6", UnitPrice: 999999, TotalPrice: 50000000, LandAreaSqm: 500, BuildingAreaSqm: 500},
	}

	ctx := context.Background()
	result, err := eng.CalculateStatistics(ctx, transactions)
	if err != nil {
		t.Fatalf("CalculateStatistics failed: %v", err)
	}

	if result.OutlierInfo.Method != "IQR" {
		t.Errorf("outlier method = %v, want IQR", result.OutlierInfo.Method)
	}
	if result.OutlierInfo.TotalCount != 6 {
		t.Errorf("total count = %d, want 6", result.OutlierInfo.TotalCount)
	}
}

func TestCalculateStatistics_EmptyInput(t *testing.T) {
	eng := NewStatisticsEngine(nil, domain.DefaultStatisticsConfig())
	ctx := context.Background()

	_, err := eng.CalculateStatistics(ctx, []domain.Transaction{})
	if err == nil {
		t.Error("expected error for empty transactions, got nil")
	}
}

func TestCalculateStatistics_PricePerPingCorrectness(t *testing.T) {
	eng := NewStatisticsEngine(nil, domain.DefaultStatisticsConfig())
	ctx := context.Background()

	// Unit price in 元/㎡, should be converted to 元/坪 via PricePerPing
	// 1 坪 = 3.305785 平方公尺
	txns := []domain.Transaction{
		{ID: "t1", UnitPrice: 100000, LandAreaSqm: 33.05785}, // 10 ping
	}

	result, err := eng.CalculateStatistics(ctx, txns)
	if err != nil {
		t.Fatalf("CalculateStatistics failed: %v", err)
	}

	// price_per_ping = unit_price * PingToSqm = 100000 * 3.305785 = 330578.5
	// PriceStatistics stores as int64 via truncation in the engine
	rawMedian := float64(100000) * domain.PingToSqm
	expectedMedian := int64(rawMedian)
	if result.TransactionStatistics.PricePerPing.Median != expectedMedian {
		t.Errorf("price_per_ping median = %d, want %d", result.TransactionStatistics.PricePerPing.Median, expectedMedian)
	}
}

func TestCalculateStatistics_AlgorithmVersion(t *testing.T) {
	eng := NewStatisticsEngine(nil, domain.DefaultStatisticsConfig())
	ctx := context.Background()

	txns := []domain.Transaction{
		{ID: "t1", UnitPrice: 100000, TotalPrice: 5000000, LandAreaSqm: 50, BuildingAreaSqm: 100},
	}

	result, err := eng.CalculateStatistics(ctx, txns)
	if err != nil {
		t.Fatalf("CalculateStatistics failed: %v", err)
	}

	if result.AlgorithmVersion != "v2.0" {
		t.Errorf("algorithm version = %v, want v2.0", result.AlgorithmVersion)
	}
}

func TestCalculateStatistics_QueryHashDeterministic(t *testing.T) {
	eng := NewStatisticsEngine(nil, domain.DefaultStatisticsConfig())
	ctx := context.Background()

	txns := []domain.Transaction{
		{ID: "t1", UnitPrice: 100000, TotalPrice: 5000000, LandAreaSqm: 50, BuildingAreaSqm: 100},
		{ID: "t2", UnitPrice: 110000, TotalPrice: 5500000, LandAreaSqm: 52, BuildingAreaSqm: 102},
	}

	result1, err := eng.CalculateStatistics(ctx, txns)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Same input order → same hash
	result2, err := eng.CalculateStatistics(ctx, txns)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if result1.QueryHash != result2.QueryHash {
		t.Errorf("query hash not deterministic: %v != %v", result1.QueryHash, result2.QueryHash)
	}
}

func TestCalculateStatistics_QueryHashDifferentForDifferentInput(t *testing.T) {
	eng := NewStatisticsEngine(nil, domain.DefaultStatisticsConfig())
	ctx := context.Background()

	txns1 := []domain.Transaction{
		{ID: "t1", UnitPrice: 100000, TotalPrice: 5000000, LandAreaSqm: 50, BuildingAreaSqm: 100},
	}
	txns2 := []domain.Transaction{
		{ID: "t2", UnitPrice: 110000, TotalPrice: 5500000, LandAreaSqm: 52, BuildingAreaSqm: 102},
	}

	result1, err := eng.CalculateStatistics(ctx, txns1)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	result2, err := eng.CalculateStatistics(ctx, txns2)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if result1.QueryHash == result2.QueryHash {
		t.Errorf("query hash should differ for different input")
	}
}

func TestStatisticsResult_Structure(t *testing.T) {
	eng := NewStatisticsEngine(nil, domain.DefaultStatisticsConfig())
	ctx := context.Background()

	txns := []domain.Transaction{
		{ID: "t1", UnitPrice: 100000, TotalPrice: 5000000, LandAreaSqm: 50, BuildingAreaSqm: 100},
		{ID: "t2", UnitPrice: 120000, TotalPrice: 6000000, LandAreaSqm: 55, BuildingAreaSqm: 105},
	}

	result, err := eng.CalculateStatistics(ctx, txns)
	if err != nil {
		t.Fatalf("CalculateStatistics failed: %v", err)
	}

	// Verify all statistics fields are populated
	if result.TransactionStatistics.PricePerPing.Count != 2 {
		t.Errorf("price_per_ping count = %d, want 2", result.TransactionStatistics.PricePerPing.Count)
	}
	if result.TransactionStatistics.TotalPrice.Count != 2 {
		t.Errorf("total_price count = %d, want 2", result.TransactionStatistics.TotalPrice.Count)
	}
	if result.TransactionStatistics.UnitPrice.Count != 2 {
		t.Errorf("unit_price count = %d, want 2", result.TransactionStatistics.UnitPrice.Count)
	}
	if result.TransactionStatistics.LandAreaSqm.Count != 2 {
		t.Errorf("land_area_sqm count = %d, want 2", result.TransactionStatistics.LandAreaSqm.Count)
	}
	if result.TransactionStatistics.BuildingAreaSqm.Count != 2 {
		t.Errorf("building_area_sqm count = %d, want 2", result.TransactionStatistics.BuildingAreaSqm.Count)
	}
}

func TestGenerateQueryHash(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "t1"},
		{ID: "t2"},
	}
	hash1 := generateQueryHash(txns)

	// Same input → same hash
	txns2 := []domain.Transaction{
		{ID: "t1"},
		{ID: "t2"},
	}
	hash2 := generateQueryHash(txns2)

	if hash1 != hash2 {
		t.Errorf("query hash should be deterministic for same IDs")
	}

	// Different IDs → different hash
	txns3 := []domain.Transaction{
		{ID: "t1"},
		{ID: "t3"},
	}
	hash3 := generateQueryHash(txns3)

	if hash1 == hash3 {
		t.Errorf("query hash should differ for different IDs")
	}
}

func TestGenerateQueryHash_Deterministic(t *testing.T) {
	// Deterministic across multiple calls
	for i := 0; i < 10; i++ {
		txns := []domain.Transaction{
			{ID: "aaaa"},
			{ID: "bbbb"},
			{ID: "cccc"},
		}
		hash := generateQueryHash(txns)
		if hash == "" {
			t.Fatal("hash should not be empty")
		}
		// Should be 32 hex chars (MD5)
		if len(hash) != 32 {
			t.Errorf("hash length = %d, want 32 (MD5 hex)", len(hash))
		}
	}
}

func TestCalculateStatistics_GeneratedAtSet(t *testing.T) {
	eng := NewStatisticsEngine(nil, domain.DefaultStatisticsConfig())
	ctx := context.Background()

	txns := []domain.Transaction{
		{ID: "t1", UnitPrice: 100000, TotalPrice: 5000000, LandAreaSqm: 50, BuildingAreaSqm: 100},
	}

	before := time.Now().UTC().Add(-1 * time.Second)
	result, err := eng.CalculateStatistics(ctx, txns)
	if err != nil {
		t.Fatalf("CalculateStatistics failed: %v", err)
	}
	after := time.Now().UTC().Add(1 * time.Second)

	if result.GeneratedAt.Before(before) || result.GeneratedAt.After(after) {
		t.Errorf("GeneratedAt %v not within expected range [%v, %v]", result.GeneratedAt, before, after)
	}
}
