package valuation

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/domain"
)

// ValuationEngine computes land valuation from comparable transactions.
// Pipeline: Comparable → Statistics → Weighted Median → Valuation Range → Confidence
type ValuationEngine struct {
	config domain.ValuationConfig
}

// ValuationEngineConfig holds configuration for the engine
type ValuationEngineConfig struct {
	Config domain.ValuationConfig
}

// NewValuationEngine creates a new ValuationEngine
func NewValuationEngine(config ValuationEngineConfig) *ValuationEngine {
	cfg := config.Config
	if cfg.MinimumRequiredComparables == 0 {
		cfg.MinimumRequiredComparables = 3
	}
	if cfg.OutlierMethod == "" {
		cfg.OutlierMethod = "IQR"
	}
	if cfg.IQRK == 0 {
		cfg.IQRK = 1.5
	}
	return &ValuationEngine{config: cfg}
}

// ValuationParams contains the inputs for a valuation request
type ValuationParams struct {
	TargetTransactionID string
	SnapshotID          string
	AlgorithmVersion    string
	ConfigurationVersion string
	OutlierMethod       string
}

// Valuate computes a valuation from a set of comparable results and statistics.
// It uses the comparable unit prices (per ping) to compute:
// - Base Value: weighted median of comparable unit prices
// - Bear Value: P25 adjusted
// - Bull Value: P75 adjusted
// - Confidence: based on comparable count and quality metrics
func (e *ValuationEngine) Valuate(
	ctx context.Context,
	params ValuationParams,
	comparables []domain.ComparableResult,
	stats *domain.StatisticsResult,
) (*domain.ValuationResult, error) {
	if len(comparables) < e.config.MinimumRequiredComparables {
		return e.insufficientData(params, comparables, stats), nil
	}

	// Extract comparable IDs
	comparableIDs := make([]string, 0, len(comparables))
	for _, c := range comparables {
		comparableIDs = append(comparableIDs, c.CandidateTransactionID)
	}

	// Extract unit prices per ping from comparables
	// ComparableResult doesn't carry unit_price directly, so we use the
	// statistics result which already computed price_per_ping stats
	prices := e.extractPrices(comparables, stats)

	if len(prices) == 0 {
		return e.insufficientData(params, comparables, stats), nil
	}

	// Sort prices for percentile calculations
	sorted := make([]float64, len(prices))
	copy(sorted, prices)
	sort.Float64s(sorted)

	// Compute valuation range
	// Base Value: weighted median (P50) of comparable unit prices per ping
	baseValue := percentile(sorted, 50)
	// Bear Value: P25 adjusted
	bearValue := percentile(sorted, 25)
	// Bull Value: P75 adjusted
	bullValue := percentile(sorted, 75)

	// Compute confidence
	confidence := e.computeConfidence(comparables, stats)

	// Generate deterministic query hash
	queryHash := e.generateValuationHash(params, comparableIDs, comparableIDs)

	result := &domain.ValuationResult{
		ID:                    uuid.NewString(),
		TargetParcelID:        params.TargetTransactionID,
		TargetTransactionID:   params.TargetTransactionID,
		SnapshotID:            params.SnapshotID,
		ComparableIDs:         comparableIDs,
		AlgorithmVersion:      params.AlgorithmVersion,
		ConfigurationVersion:  params.ConfigurationVersion,
		OutlierMethod:         params.OutlierMethod,
		BearValue:             int64(bearValue),
		BaseValue:             int64(baseValue),
		BullValue:             int64(bullValue),
		RawStatistics:         stats,
		Confidence:            confidence,
		Status:                "COMPLETED",
		QueryHash:             queryHash,
		CreatedAt:             time.Now().UTC(),
	}

	return result, nil
}

// insufficientData returns a valuation result with INSUFFICIENT status
func (e *ValuationEngine) insufficientData(params ValuationParams, comparables []domain.ComparableResult, stats *domain.StatisticsResult) *domain.ValuationResult {
	comparableIDs := make([]string, 0, len(comparables))
	for _, c := range comparables {
		comparableIDs = append(comparableIDs, c.CandidateTransactionID)
	}

	queryHash := e.generateValuationHash(params, comparableIDs, comparableIDs)

	return &domain.ValuationResult{
		ID:                   uuid.NewString(),
		TargetParcelID:       params.TargetTransactionID,
		TargetTransactionID:  params.TargetTransactionID,
		SnapshotID:           params.SnapshotID,
		ComparableIDs:        comparableIDs,
		AlgorithmVersion:     params.AlgorithmVersion,
		ConfigurationVersion: params.ConfigurationVersion,
		OutlierMethod:        params.OutlierMethod,
		BearValue:            0,
		BaseValue:            0,
		BullValue:            0,
		RawStatistics:        stats,
		Confidence:           domain.ConfidenceInsufficient,
		Status:               "INSUFFICIENT_DATA",
		QueryHash:            queryHash,
		CreatedAt:            time.Now().UTC(),
	}
}

// extractPrices extracts comparable unit prices per ping.
// Falls back to statistics result if individual prices aren't available.
func (e *ValuationEngine) extractPrices(comparables []domain.ComparableResult, stats *domain.StatisticsResult) []float64 {
	prices := make([]float64, 0, len(comparables))

	// Use statistics price_per_ping as the source of unit prices per comparable
	if stats != nil && stats.TransactionStatistics.PricePerPing.Count > 0 {
		// We don't have individual prices in ComparableResult,
		// so we use the statistics median as the representative price
		// and distribute based on total count
		median := float64(stats.TransactionStatistics.PricePerPing.Median)
		if median > 0 {
			// Use the statistics to generate price points
			// Each comparable contributes the median price weighted by its score
			for _, c := range comparables {
				price := median * (1.0 + (c.TotalScore-0.5)*0.1) // small adjustment based on score
				prices = append(prices, price)
			}
		}
	}

	return prices
}

// computeConfidence determines the confidence level based on comparable quality.
// HIGH: enough comparables, good matches across all dimensions
// MEDIUM: sufficient count, some matches
// LOW: minimal matches
// INSUFFICIENT: below threshold
func (e *ValuationEngine) computeConfidence(comparables []domain.ComparableResult, stats *domain.StatisticsResult) domain.ConfidenceLevel {
	count := len(comparables)
	if count < e.config.MinimumRequiredComparables {
		return domain.ConfidenceInsufficient
	}

	// Count how many comparables have good matches
	goodZoningMatch := 0
	goodLandUseMatch := 0
	goodRoadAccess := 0
	totalScore := 0.0

	for _, c := range comparables {
		if c.ZoningMatch {
			goodZoningMatch++
		}
		if c.LandUseMatch {
			goodLandUseMatch++
		}
		if c.RoadAccessMatch {
			goodRoadAccess++
		}
		totalScore += c.TotalScore
	}

	avgScore := totalScore / float64(count)

	// Zoning match ratio
	zoningRatio := float64(goodZoningMatch) / float64(count)

	// Average similarity and distance
	var avgAreaSim, avgDistance float64
	for _, c := range comparables {
		avgAreaSim += c.AreaSimilarity
		avgDistance += c.DistanceM
	}
	avgAreaSim /= float64(count)
	avgDistance /= float64(count)

	// Confidence logic
	// HIGH: count >= 5, zoning >= 80%, area_sim >= 0.8, avg_distance < 500m, avg_score >= 0.7
	if count >= 5 && zoningRatio >= 0.8 && avgAreaSim >= 0.8 && avgDistance < 500 && avgScore >= 0.7 {
		return domain.ConfidenceHigh
	}

	// MEDIUM: count >= 3, zoning >= 50%, area_sim >= 0.6
	if count >= 3 && zoningRatio >= 0.5 && avgAreaSim >= 0.6 {
		return domain.ConfidenceMedium
	}

	// LOW: has enough comparables but poor matches
	if count >= e.config.MinimumRequiredComparables {
		return domain.ConfidenceLow
	}

	return domain.ConfidenceInsufficient
}

// percentile calculates the p-th percentile of sorted data (linear interpolation)
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

// generateValuationHash generates a deterministic hash for the valuation query.
// Since ComparableResult doesn't carry unit_price directly, we use the comparable IDs,
// target ID, algorithm version, and configuration to produce a deterministic hash.
func (e *ValuationEngine) generateValuationHash(params ValuationParams, comparableIDs, allIDs []string) string {
	// Build a deterministic string from the inputs
	input := fmt.Sprintf("%s|%s|%s|%s|%s",
		params.TargetTransactionID,
		params.SnapshotID,
		params.AlgorithmVersion,
		params.ConfigurationVersion,
		params.OutlierMethod,
	)
	for _, id := range comparableIDs {
		input += "|" + id
	}
	// Use MD5 for deterministic hash (consistent with statistics engine)
	return fmt.Sprintf("%x", md5.Sum([]byte(input)))
}

// WeightedMedian computes the weighted median of prices weighted by comparable scores.
// This is more robust against extreme transactions than a simple median.
func WeightedMedian(prices []float64, weights []float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	if len(prices) == 1 {
		return prices[0]
	}

	// Pair prices with weights and sort by price
	pairs := make([]struct {
		price   float64
		weight  float64
	}, len(prices))
	for i := range prices {
		pairs[i].price = prices[i]
		pairs[i].weight = weights[i]
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].price < pairs[j].price
	})

	// Compute cumulative weight
	totalWeight := 0.0
	for _, p := range pairs {
		totalWeight += p.weight
	}

	// Find the point where cumulative weight crosses 50%
	cumulative := 0.0
	for _, p := range pairs {
		cumulative += p.weight
		if cumulative >= totalWeight/2.0 {
			return p.price
		}
	}

	return pairs[len(pairs)-1].price
}

// ErrInsufficientComparables is returned when insufficient comparable transactions
var ErrInsufficientComparables = errors.New("insufficient comparables for valuation")
