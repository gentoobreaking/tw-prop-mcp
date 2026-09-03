package domain

import (
	"time"
)

// ComparableResult represents a comparable transaction with scoring
type ComparableResult struct {
	ID                  string    `json:"id"`
	TargetTransactionID string    `json:"target_transaction_id"`
	CandidateTransactionID string `json:"candidate_transaction_id"`
	DistanceM           float64   `json:"distance_m"`
	AreaSimilarity      float64   `json:"area_similarity"`
	ZoningMatch         bool      `json:"zoning_match"`
	LandUseMatch        bool      `json:"land_use_match"`
	RoadAccessMatch     bool      `json:"road_access_match"`
	TimeScore           float64   `json:"time_score"`
	DistanceScore       float64   `json:"distance_score"`
	AreaSimilarityScore float64   `json:"area_similarity_score"`
	ZoningMatchScore    float64   `json:"zoning_match_score"`
	LandUseMatchScore   float64   `json:"land_use_match_score"`
	RoadAccessMatchScore float64  `json:"road_access_match_score"`
	TotalScore          float64   `json:"total_score"`
	AlgorithmVersion    string    `json:"algorithm_version"`
	CreatedAt           time.Time `json:"created_at"`
}

// ComparableConfig holds configuration for comparable engine
type ComparableConfig struct {
	// AreaSimilarityPct is the maximum area difference percentage (default 30%)
	AreaSimilarityPct float64
	// Lambda for time decay (months^-1)
	Lambda float64
	// DistanceScale for spatial decay (meters)
	DistanceScale float64
	// Weights for each dimension
	WArea        float64
	WDistance    float64
	WTime        float64
	WZoning      float64
	WLandUse     float64
	WRoad        float64
	// IQR multiplier for outlier detection
	IQRK float64
	// Minimum required comparables
	MinimumRequiredComparables int
	// Outlier method: IQR, P10_P90, MAD
	OutlierMethod string
}

// DefaultComparableConfig returns default configuration
func DefaultComparableConfig() ComparableConfig {
	return ComparableConfig{
		AreaSimilarityPct:          30.0,
		Lambda:                     0.05,
		DistanceScale:              500.0,
		WArea:        0.30,
		WDistance:    0.20,
		WTime:        0.15,
		WZoning:      0.15,
		WLandUse:     0.10,
		WRoad:        0.10,
		IQRK:         1.5,
		MinimumRequiredComparables: 3,
		OutlierMethod: "IQR",
	}
}

// ComparableCandidate represents a candidate transaction with precomputed scores
type ComparableCandidate struct {
	Transaction           *Transaction
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

// ComparableFilter defines search criteria for comparables
type ComparableFilter struct {
	County          string
	District        string
	Section         string
	TransactionType *string
	DateFrom        *time.Time
	DateTo          *time.Time
	MinAreaSqm      *float64
	MaxAreaSqm      *float64
	MinPrice        *int64
	MaxPrice        *int64
	Limit           int
	Offset          int
}