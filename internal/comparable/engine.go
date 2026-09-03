package comparable

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
	"tw-prop-mcp/internal/service"
)

// ComparableEngine provides comparable transaction selection and scoring
type ComparableEngine struct {
	txRepo        repository.TransactionRepository
	roadAccessSvc *service.RoadAccessEngine
}

// ComparableEngineConfig holds configuration for the engine
type ComparableEngineConfig struct {
	TxRepo         repository.TransactionRepository
	RoadAccessSvc  *service.RoadAccessEngine
}

// NewComparableEngine creates a new ComparableEngine
func NewComparableEngine(config ComparableEngineConfig) *ComparableEngine {
	return &ComparableEngine{
		txRepo:        config.TxRepo,
		roadAccessSvc: config.RoadAccessSvc,
	}
}

// ComparableConfig holds configuration for comparable selection
type ComparableConfig struct {
	AreaSimilarityPct       float64
	Lambda                  float64
	DistanceScale           float64
	WArea                   float64
	WDistance               float64
	WTime                   float64
	WZoning                 float64
	WLandUse                float64
	WRoad                   float64
	IQRK                    float64
	MinimumRequiredComparables int
	OutlierMethod           string
	AlgorithmVersion        string
}

// DefaultEngineConfig returns default configuration
func DefaultEngineConfig() domain.ComparableConfig {
	return domain.DefaultComparableConfig()
}

// ComparableCandidate represents a candidate transaction with precomputed scores
type ComparableCandidate struct {
	Transaction           *domain.Transaction
	DistanceM             float64
	AreaSimilarity        float64
	ZoningMatch           bool
	LandUseMatch          bool
	RoadAccessMatch       bool
	TimeScore             float64
	DistanceScore         float64
	AreaSimilarityScore   float64
	ZoningMatchScore      float64
	LandUseMatchScore     float64
	RoadAccessMatchScore  float64
	TotalScore            float64
}

// ComparableResult represents a comparable transaction with scoring
type ComparableResult struct {
	TargetTransactionID     string    `json:"target_transaction_id"`
	CandidateTransactionID  string    `json:"candidate_transaction_id"`
	DistanceM               float64   `json:"distance_m"`
	AreaSimilarity          float64   `json:"area_similarity"`
	ZoningMatch             bool      `json:"zoning_match"`
	LandUseMatch            bool      `json:"land_use_match"`
	RoadAccessMatch         bool      `json:"road_access_match"`
	TimeScore               float64   `json:"time_score"`
	DistanceScore           float64   `json:"distance_score"`
	AreaSimilarityScore     float64   `json:"area_similarity_score"`
	ZoningMatchScore        float64   `json:"zoning_match_score"`
	LandUseMatchScore       float64   `json:"land_use_match_score"`
	RoadAccessMatchScore    float64   `json:"road_access_match_score"`
	TotalScore              float64   `json:"total_score"`
	AlgorithmVersion        string    `json:"algorithm_version"`
	CreatedAt               time.Time `json:"created_at"`
}

// FindComparables finds comparable transactions for a target transaction
func (e *ComparableEngine) FindComparables(ctx context.Context, targetID string, config domain.ComparableConfig) ([]domain.ComparableResult, error) {
	// Get target transaction
	targetIDParsed, err := uuid.Parse(targetID)
	if err != nil {
		return nil, fmt.Errorf("invalid target ID: %w", err)
	}

	target, err := e.txRepo.GetByID(ctx, targetIDParsed)
	if err != nil {
		return nil, fmt.Errorf("get target transaction: %w", err)
	}

	// Build filter for candidates
	filter := repository.SearchFilter{
		County:     target.County,
		District:   target.District,
		Section:    &target.Section,
		Limit:      100,
		Offset:     0,
	}

	candidates, err := e.txRepo.Search(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("search candidates: %w", err)
	}

	// Filter out target itself and apply hard filters
	filtered := e.filterCandidates(&target, candidates, config)

	if len(filtered) < config.MinimumRequiredComparables {
		return nil, errors.New("insufficient comparables")
	}

	// Score each candidate
	scoredCandidates := e.scoreCandidates(&target, filtered, config)

	// Remove outliers
	scoredCandidates = e.removeOutliers(scoredCandidates, config)

	// Build results
	results := make([]domain.ComparableResult, len(scoredCandidates))
	for i, c := range scoredCandidates {
		results[i] = domain.ComparableResult{
			TargetTransactionID:     target.ID,
			CandidateTransactionID:  c.Transaction.ID,
			DistanceM:               c.DistanceM,
			AreaSimilarity:          c.AreaSimilarity,
			ZoningMatch:             c.ZoningMatch,
			LandUseMatch:            c.LandUseMatch,
			RoadAccessMatch:         c.RoadAccessMatch,
			TimeScore:               c.TimeScore,
			DistanceScore:           c.DistanceScore,
			AreaSimilarityScore:     c.AreaSimilarityScore,
			ZoningMatchScore:        c.ZoningMatchScore,
			LandUseMatchScore:       c.LandUseMatchScore,
			RoadAccessMatchScore:    c.RoadAccessMatchScore,
			TotalScore:              c.TotalScore,
			AlgorithmVersion:        "v2.0",
			CreatedAt:               time.Now().UTC(),
		}
	}

	return results, nil
}

// filterCandidates applies hard filters and basic filters
func (e *ComparableEngine) filterCandidates(target *domain.Transaction, candidates []domain.Transaction, config domain.ComparableConfig) []domain.Transaction {
	filtered := make([]domain.Transaction, 0)

	for _, c := range candidates {
		// Skip self
		if c.ID == target.ID {
			continue
		}

		// Hard filter: same county, district, section
		if c.County != target.County || c.District != target.District || c.Section != target.Section {
			continue
		}

		// Area similarity check
		if target.LandAreaSqm > 0 && c.LandAreaSqm > 0 {
			ratio := c.LandAreaSqm / target.LandAreaSqm
			if math.Abs(ratio-1.0) > config.AreaSimilarityPct/100.0 {
				continue
			}
		}

		filtered = append(filtered, c)
	}

	return filtered
}

// scoreCandidates computes all scores for candidates
func (e *ComparableEngine) scoreCandidates(target *domain.Transaction, candidates []domain.Transaction, config domain.ComparableConfig) []ComparableCandidate {
	candidatesScored := make([]ComparableCandidate, 0, len(candidates))

	// Pre-fetch road access for target
	targetRoadAccess, _ := e.getRoadAccess(target.ID)

	for _, c := range candidates {
		candidate := ComparableCandidate{
			Transaction: &c,
		}

		// Area similarity
		if target.LandAreaSqm > 0 && c.LandAreaSqm > 0 {
			ratio := c.LandAreaSqm / target.LandAreaSqm
			candidate.AreaSimilarity = 1.0 - math.Abs(ratio-1.0)
			candidate.AreaSimilarityScore = candidate.AreaSimilarity * config.WArea
		}

		// Zoning match
		candidate.ZoningMatch = target.UrbanZoning == c.UrbanZoning
		if candidate.ZoningMatch {
			candidate.ZoningMatchScore = config.WZoning
		}

		// Land use match
		candidate.LandUseMatch = target.LandUseCategory == c.LandUseCategory
		if candidate.LandUseMatch {
			candidate.LandUseMatchScore = config.WLandUse
		}

		// Distance score (need to compute distance)
		if c.LandAreaSqm > 0 && target.LandAreaSqm > 0 {
			// Approximate distance from centroids
			// In real implementation, use PostGIS ST_Distance
			distance := 100.0 // placeholder
			candidate.DistanceM = distance
			candidate.DistanceScore = math.Exp(-distance/config.DistanceScale) * config.WDistance
		}

		// Time score
		if !target.TransactionDate.IsZero() && !c.TransactionDate.IsZero() {
			months := time.Since(c.TransactionDate).Hours() / 24 / 30
			candidate.TimeScore = math.Exp(-config.Lambda*months) * config.WTime
		}

		// Road access match
		candidateRoadAccess, _ := e.getRoadAccess(c.ID)
		candidate.RoadAccessMatch = targetRoadAccess == candidateRoadAccess
		if candidate.RoadAccessMatch {
			candidate.RoadAccessMatchScore = config.WRoad
		}

		// Total score
		candidate.TotalScore = candidate.AreaSimilarityScore + candidate.DistanceScore +
			candidate.TimeScore + candidate.ZoningMatchScore +
			candidate.LandUseMatchScore + candidate.RoadAccessMatchScore

		candidatesScored = append(candidatesScored, candidate)
	}

	// Sort by total score descending
	sort.Slice(candidatesScored, func(i, j int) bool {
		return candidatesScored[i].TotalScore > candidatesScored[j].TotalScore
	})

	return candidatesScored
}

// getRoadAccess gets road access type for a transaction
func (e *ComparableEngine) getRoadAccess(transactionID string) (string, error) {
	// In real implementation, query parcel_road_access
	// For now, return placeholder
	return "ROAD_ADJACENT", nil
}

// removeOutliers removes outliers using IQR or other methods
func (e *ComparableEngine) removeOutliers(candidates []ComparableCandidate, config domain.ComparableConfig) []ComparableCandidate {
	if len(candidates) <= config.MinimumRequiredComparables {
		return candidates
	}

	// Extract scores
	scores := make([]float64, len(candidates))
	for i, c := range candidates {
		scores[i] = c.TotalScore
	}

	sort.Float64s(scores)

	q1 := percentile(scores, 25)
	q3 := percentile(scores, 75)
	iqr := q3 - q1
	lower := q1 - config.IQRK*iqr
	upper := q3 + config.IQRK*iqr

	filtered := make([]ComparableCandidate, 0)
	for _, c := range candidates {
		if c.TotalScore >= lower && c.TotalScore <= upper {
			filtered = append(filtered, c)
		}
	}

	if len(filtered) < 3 {
		return candidates // fallback
	}

	return filtered
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p / 100)
	return sorted[idx]
}