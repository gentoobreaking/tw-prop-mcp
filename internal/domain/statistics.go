package domain

import "time"

// StatisticsConfig holds configuration for statistics engine
type StatisticsConfig struct {
	// OutlierMethod: IQR, P10_P90, MAD
	OutlierMethod string
	// IQRK multiplier for IQR outlier detection
	IQRK float64
}

// DefaultStatisticsConfig returns default configuration
func DefaultStatisticsConfig() StatisticsConfig {
	return StatisticsConfig{
		OutlierMethod: "IQR",
		IQRK:          1.5,
	}
}

// PriceStatistics holds price statistics (per ping)
type PriceStatistics struct {
	Count   int64   `json:"count"`
	Min     int64   `json:"min"`
	P10     int64   `json:"p10"`
	P25     int64   `json:"p25"`
	Median  int64   `json:"median"`
	Mean    float64 `json:"mean"`
	P75     int64   `json:"p75"`
	P90     int64   `json:"p90"`
	MaxVal  int64   `json:"max"`
}

// AreaStatistics holds area statistics (sqm)
type AreaStatistics struct {
	Count   int64   `json:"count"`
	Min     float64 `json:"min"`
	P10     float64 `json:"p10"`
	P25     float64 `json:"p25"`
	Median  float64 `json:"median"`
	Mean    float64 `json:"mean"`
	P75     float64 `json:"p75"`
	P90     float64 `json:"p90"`
	MaxVal  float64 `json:"max"`
}

// TransactionStatistics holds complete transaction statistics
type TransactionStatistics struct {
	PricePerPing    PriceStatistics `json:"price_per_ping"`
	TotalPrice      PriceStatistics `json:"total_price"`
	UnitPrice       PriceStatistics `json:"unit_price"`
	LandAreaSqm     AreaStatistics  `json:"land_area_sqm"`
	BuildingAreaSqm AreaStatistics  `json:"building_area_sqm"`
}

// StatisticsResult holds complete statistics result
type StatisticsResult struct {
	TransactionStatistics TransactionStatistics `json:"transaction_statistics"`
	OutlierInfo           OutlierInfo         `json:"outlier_info"`
	AlgorithmVersion      string              `json:"algorithm_version"`
	GeneratedAt           time.Time           `json:"generated_at"`
	QueryHash             string              `json:"query_hash"`
}

// OutlierInfo contains outlier detection information
type OutlierInfo struct {
	Method       string  `json:"method"`
	IQR          float64 `json:"iqr,omitempty"`
	Q1           float64 `json:"q1,omitempty"`
	Q3           float64 `json:"q3,omitempty"`
	LowerBound   float64 `json:"lower_bound,omitempty"`
	UpperBound   float64 `json:"upper_bound,omitempty"`
	OutlierCount int     `json:"outlier_count,omitempty"`
	TotalCount   int     `json:"total_count,omitempty"`
}

// PricePerPing converts unit price (元/㎡) to price per ping (元/坪)
func PricePerPing(unitPricePerSqm float64) float64 {
	return unitPricePerSqm * PingToSqm
}

// PricePerSqmFromPing converts price per ping to price per sqm
func PricePerSqmFromPing(pricePerPing float64) float64 {
	return pricePerPing / PingToSqm
}