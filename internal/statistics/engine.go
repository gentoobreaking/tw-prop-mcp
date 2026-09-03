package statistics

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// StatisticsEngine provides statistical calculations for transactions
type StatisticsEngine struct {
	txRepo repository.TransactionRepository
	config domain.StatisticsConfig
}

// NewStatisticsEngine creates a new StatisticsEngine
func NewStatisticsEngine(txRepo repository.TransactionRepository, config domain.StatisticsConfig) *StatisticsEngine {
	return &StatisticsEngine{
		txRepo: txRepo,
		config: config,
	}
}

// CalculateStatistics calculates statistics for a set of transactions
func (e *StatisticsEngine) CalculateStatistics(ctx context.Context, transactions []domain.Transaction) (*domain.StatisticsResult, error) {
	if len(transactions) == 0 {
		return nil, errors.New("no transactions provided for statistics")
	}

	// Extract price per ping values
	prices := make([]float64, 0, len(transactions))
	landAreas := make([]float64, 0, len(transactions))
	buildingAreas := make([]float64, 0, len(transactions))
	totalPrices := make([]float64, 0, len(transactions))
	unitPrices := make([]float64, 0, len(transactions))

	for _, txn := range transactions {
		pricePerPing := domain.PricePerPing(float64(txn.UnitPrice))
		prices = append(prices, pricePerPing)
		landAreas = append(landAreas, txn.LandAreaSqm)
		buildingAreas = append(buildingAreas, txn.BuildingAreaSqm)
		totalPrices = append(totalPrices, float64(txn.TotalPrice))
		unitPrices = append(unitPrices, float64(txn.UnitPrice))
	}

	// Calculate statistics
	priceStats := calculatePriceStatistics(prices)
	totalPriceStats := calculatePriceStatistics(totalPrices)
	unitPriceStats := calculatePriceStatistics(unitPrices)
	landAreaStats := calculateAreaStatistics(landAreas)
	buildingAreaStats := calculateAreaStatistics(buildingAreas)

	// Outlier detection
	outlierInfo := e.detectOutliers(prices)

	// Build result
	result := &domain.StatisticsResult{
		TransactionStatistics: domain.TransactionStatistics{
			PricePerPing:    priceStats,
			TotalPrice:      totalPriceStats,
			UnitPrice:       unitPriceStats,
			LandAreaSqm:     landAreaStats,
			BuildingAreaSqm: buildingAreaStats,
		},
		OutlierInfo:       outlierInfo,
		AlgorithmVersion:  "v2.0",
		GeneratedAt:       time.Now().UTC(),
		QueryHash:         generateQueryHash(transactions),
	}

	return result, nil
}

// calculatePriceStatistics calculates statistics for price values
func calculatePriceStatistics(values []float64) domain.PriceStatistics {
	if len(values) == 0 {
		return domain.PriceStatistics{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	count := len(sorted)
	minVal := int64(sorted[0])
	maxVal := int64(sorted[count-1])

	// Mean
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(count)

	// Percentiles
	p10 := percentile(sorted, 10)
	p25 := percentile(sorted, 25)
	median := percentile(sorted, 50)
	p75 := percentile(sorted, 75)
	p90 := percentile(sorted, 90)

	return domain.PriceStatistics{
		Count:  int64(count),
		Min:    minVal,
		P10:    int64(p10),
		P25:    int64(p25),
		Median: int64(median),
		Mean:   mean,
		P75:    int64(p75),
		P90:    int64(p90),
		MaxVal:    maxVal,
	}
}

// calculateAreaStatistics calculates statistics for area values
func calculateAreaStatistics(values []float64) domain.AreaStatistics {
	if len(values) == 0 {
		return domain.AreaStatistics{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	count := len(sorted)
	minVal := sorted[0]
	maxVal := sorted[count-1]

	// Mean
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(count)

	// Percentiles
	p10 := percentile(sorted, 10)
	p25 := percentile(sorted, 25)
	median := percentile(sorted, 50)
	p75 := percentile(sorted, 75)
	p90 := percentile(sorted, 90)

	return domain.AreaStatistics{
		Count:  int64(count),
		Min:    minVal,
		P10:    p10,
		P25:    p25,
		Median: median,
		Mean:   mean,
		P75:    p75,
		P90:    p90,
		MaxVal:    maxVal,
	}
}

// detectOutliers detects outliers using specified method
func (e *StatisticsEngine) detectOutliers(values []float64) domain.OutlierInfo {
	if len(values) < 4 {
		return domain.OutlierInfo{
			Method:      e.config.OutlierMethod,
			TotalCount:  len(values),
			OutlierCount: 0,
		}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	switch e.config.OutlierMethod {
	case "IQR":
		return e.detectOutliersIQR(sorted)
	case "P10_P90":
		return e.detectOutliersP10P90(sorted)
	case "MAD":
		return e.detectOutliersMAD(values)
	default:
		return e.detectOutliersIQR(sorted)
	}
}

// detectOutliersIQR detects outliers using IQR method
func (e *StatisticsEngine) detectOutliersIQR(sorted []float64) domain.OutlierInfo {
	count := len(sorted)
	q1 := percentile(sorted, 25)
	q3 := percentile(sorted, 75)
	iqr := q3 - q1
	lower := q1 - 1.5*iqr
	upper := q3 + 1.5*iqr

	outlierCount := 0
	for _, v := range sorted {
		if v < lower || v > upper {
			outlierCount++
		}
	}

	return domain.OutlierInfo{
		Method:       "IQR",
		IQR:          iqr,
		Q1:           q1,
		Q3:           q3,
		LowerBound:   lower,
		UpperBound:   upper,
		OutlierCount: outlierCount,
		TotalCount:   count,
	}
}

// detectOutliersP10P90 detects outliers using P10/P90 method
func (e *StatisticsEngine) detectOutliersP10P90(sorted []float64) domain.OutlierInfo {
	p10 := percentile(sorted, 10)
	p90 := percentile(sorted, 90)

	lower := p10
	upper := p90

	outlierCount := 0
	for _, v := range sorted {
		if v < lower || v > upper {
			outlierCount++
		}
	}

	return domain.OutlierInfo{
		Method:       "P10_P90",
		LowerBound:   p10,
		UpperBound:   p90,
		OutlierCount: outlierCount,
		TotalCount:   len(sorted),
	}
}

// detectOutliersMAD detects outliers using Median Absolute Deviation
func (e *StatisticsEngine) detectOutliersMAD(values []float64) domain.OutlierInfo {
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	median := percentile(sorted, 50)

	// Calculate absolute deviations from median
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - median)
	}

	sortedDev := make([]float64, len(deviations))
	copy(sortedDev, deviations)
	sort.Float64s(sortedDev)

	mad := percentile(sortedDev, 50)
	threshold := 3.0 * mad // 3 * MAD threshold

	lower := median - threshold
	upper := median + threshold

	outlierCount := 0
	for _, v := range values {
		if v < lower || v > upper {
			outlierCount++
		}
	}

	return domain.OutlierInfo{
		Method:       "MAD",
		LowerBound:   lower,
		UpperBound:   upper,
		OutlierCount: outlierCount,
		TotalCount:   len(values),
	}
}

// percentile calculates the p-th percentile of sorted data
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	idx := p / 100.0 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	weight := idx - float64(lower)
	return sorted[lower]*(1.0-weight) + sorted[upper]*weight
}

// generateQueryHash generates a hash for the query
func generateQueryHash(transactions []domain.Transaction) string {
	// Simple hash based on transaction IDs
	var ids string
	for _, t := range transactions {
		ids += t.ID
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(ids)))
}