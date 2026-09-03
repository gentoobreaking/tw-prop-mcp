package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/repository"
)

// TransactionService provides business logic for transaction queries
type TransactionService struct {
	txRepo repository.TransactionRepository
}

// NewTransactionService creates a new TransactionService
func NewTransactionService(txRepo repository.TransactionRepository) *TransactionService {
	return &TransactionService{txRepo: txRepo}
}

// SearchParams represents the search parameters for transactions
type SearchParams struct {
	County         string     `json:"county"`           // required
	District       string     `json:"district"`         // required
	Section        string     `json:"section,omitempty"`
	LandNumber     string     `json:"land_number,omitempty"`
	TransactionType string    `json:"transaction_type,omitempty"`
	DateFrom       *time.Time `json:"date_from,omitempty"`
	DateTo         *time.Time `json:"date_to,omitempty"`
	Limit          int        `json:"limit,omitempty"`
	Offset         int        `json:"offset,omitempty"`
}

// TransactionData represents a transaction in the service response
type TransactionData struct {
	ID                  string    `json:"id"`
	TransactionID       string    `json:"transaction_id"`
	TransactionDate     time.Time `json:"transaction_date"`
	TransactionType     string    `json:"transaction_type"`
	County              string    `json:"county"`
	District            string    `json:"district"`
	Section             string    `json:"section"`
	LandNumber          string    `json:"land_number"`
	TransactionTarget   string    `json:"transaction_target"`
	TotalPrice          int64     `json:"total_price"`
	UnitPrice           int64     `json:"unit_price"`
	PricePerPing        float64   `json:"price_per_ping"`
	LandAreaSqm         float64   `json:"land_area_sqm"`
	BuildingAreaSqm     float64   `json:"building_area_sqm"`
	UrbanZoning         string    `json:"urban_zoning"`
	NonUrbanZoning      string    `json:"non_urban_zoning"`
	LandUseCategory     string    `json:"land_use_category"`
	BuildingType        string    `json:"building_type"`
	Floor               string    `json:"floor"`
	Age                 int       `json:"age"`
	ParkingAreaSqm      float64   `json:"parking_area_sqm"`
	ParkingPrice        int64     `json:"parking_price"`
	SourceRecordHash    string    `json:"source_record_hash"`
	CreatedAt           time.Time `json:"created_at"`
}

// SearchResult represents the result of a search query
type SearchResult struct {
	Data           []TransactionData `json:"data"`
	TotalCount     int               `json:"total_count"`
	Limit          int               `json:"limit"`
	Offset         int               `json:"offset"`
	Metadata       SearchMetadata    `json:"metadata"`
	DataProvenance []ProvenanceInfo  `json:"data_provenance"`
}

// SearchMetadata contains metadata about the search
type SearchMetadata struct {
	AlgorithmVersion string    `json:"algorithm_version"`
	SnapshotID       string    `json:"snapshot_id"`
	GeneratedAt      time.Time `json:"generated_at"`
	QueryHash        string    `json:"query_hash"`
}

// ProvenanceInfo contains provenance information for the data
type ProvenanceInfo struct {
	Source          string    `json:"source"`
	SourceVersion   string    `json:"source_version"`
	SnapshotID      string    `json:"snapshot_id"`
	DownloadedAt    time.Time `json:"downloaded_at"`
	FileSHA256      string    `json:"file_sha256"`
	RecordCount     int64     `json:"record_count"`
}

// StatisticsParams represents parameters for statistics query
type StatisticsParams struct {
	County   string `json:"county"`   // required
	District string `json:"district"` // required
	Section  string `json:"section,omitempty"`
}

// StatisticsResult represents statistics result
type StatisticsResult struct {
	Count            int64     `json:"count"`
	PricePerPing     PriceStats `json:"price_per_ping"`
	TotalPrice       PriceStats `json:"total_price"`
	UnitPrice        PriceStats `json:"unit_price"`
	LandAreaSqm      AreaStats  `json:"land_area_sqm"`
	BuildingAreaSqm  AreaStats  `json:"building_area_sqm"`
	Metadata         StatsMetadata `json:"metadata"`
}

// PriceStats contains price statistics
type PriceStats struct {
	Min   int64     `json:"min"`
	P10   int64     `json:"p10"`
	P25   int64     `json:"p25"`
	Median int64    `json:"median"`
	Mean  float64   `json:"mean"`
	P75   int64     `json:"p75"`
	P90   int64     `json:"p90"`
	Max   int64     `json:"max"`
}

// AreaStats contains area statistics
type AreaStats struct {
	Min    float64 `json:"min"`
	P10    float64 `json:"p10"`
	P25    float64 `json:"p25"`
	Median float64 `json:"median"`
	Mean   float64 `json:"mean"`
	P75    float64 `json:"p75"`
	P90    float64 `json:"p90"`
	Max    float64 `json:"max"`
}

// StatsMetadata contains statistics metadata
type StatsMetadata struct {
	AlgorithmVersion string    `json:"algorithm_version"`
	SnapshotID       string    `json:"snapshot_id"`
	GeneratedAt      time.Time `json:"generated_at"`
	QueryHash        string    `json:"query_hash"`
}

// SearchTransactions searches transactions with the given parameters
func (s *TransactionService) SearchTransactions(ctx context.Context, params SearchParams) (*SearchResult, error) {
	if params.County == "" || params.District == "" {
		return nil, errors.New("county and district are required")
	}

	// Validate date range
	if params.DateFrom != nil && params.DateTo != nil && params.DateFrom.After(*params.DateTo) {
		return nil, errors.New("date_from must be before date_to")
	}

	// Build filter
	limit := int32(params.Limit)
	if params.Limit <= 0 {
		limit = 50
	}
	offset := int32(params.Offset)

	filter := repository.SearchFilter{
		County:     params.County,
		District:   params.District,
		Section:    &params.Section,
		LandNumber: &params.LandNumber,
		StartDate:  params.DateFrom,
		EndDate:    params.DateTo,
		Limit:      limit,
		Offset:     offset,
	}

	// Query transactions
	txns, err := s.txRepo.Search(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("search transactions: %w", err)
	}

	// Convert to service response
	data := make([]TransactionData, len(txns))
	for i, txn := range txns {
		data[i] = TransactionData{
			ID:                txn.ID,
			TransactionID:     txn.TransactionID,
			TransactionDate:   txn.TransactionDate,
			TransactionType:   txn.TransactionType,
			County:            txn.County,
			District:          txn.District,
			Section:           txn.Section,
			LandNumber:        txn.LandNumber,
			TransactionTarget: txn.TransactionTarget,
			TotalPrice:        txn.TotalPrice,
			UnitPrice:         txn.UnitPrice,
			PricePerPing:      txn.PricePerPing(),
			LandAreaSqm:       txn.LandAreaSqm,
			BuildingAreaSqm:   txn.BuildingAreaSqm,
			UrbanZoning:       txn.UrbanZoning,
			NonUrbanZoning:    txn.NonUrbanZoning,
			LandUseCategory:   txn.LandUseCategory,
			BuildingType:      txn.BuildingType,
			Floor:             txn.Floor,
			Age:               txn.Age,
			ParkingAreaSqm:    txn.ParkingAreaSqm,
			ParkingPrice:      txn.ParkingPrice,
			SourceRecordHash:  txn.SourceRecordHash,
			CreatedAt:         txn.CreatedAt,
		}
	}

	// Generate query hash
	queryHash := generateQueryHash(params)

	// Build result
	result := &SearchResult{
		Data:       make([]TransactionData, len(txns)),
		TotalCount: len(txns),
		Limit:      params.Limit,
		Offset:     params.Offset,
		Metadata: SearchMetadata{
			AlgorithmVersion: "v2.0",
			SnapshotID:       "",
			GeneratedAt:      time.Now().UTC(),
			QueryHash:        queryHash,
		},
		DataProvenance: []ProvenanceInfo{},
	}

	for i, txn := range txns {
		result.Data[i] = TransactionData{
			ID:                txn.ID,
			TransactionID:     txn.TransactionID,
			TransactionDate:   txn.TransactionDate,
			TransactionType:   txn.TransactionType,
			County:            txn.County,
			District:          txn.District,
			Section:           txn.Section,
			LandNumber:        txn.LandNumber,
			TransactionTarget: txn.TransactionTarget,
			TotalPrice:        txn.TotalPrice,
			UnitPrice:         txn.UnitPrice,
			PricePerPing:      txn.PricePerPing(),
			LandAreaSqm:       txn.LandAreaSqm,
			BuildingAreaSqm:   txn.BuildingAreaSqm,
			UrbanZoning:       txn.UrbanZoning,
			NonUrbanZoning:    txn.NonUrbanZoning,
			LandUseCategory:   txn.LandUseCategory,
			BuildingType:      txn.BuildingType,
			Floor:             txn.Floor,
			Age:               txn.Age,
			ParkingAreaSqm:    txn.ParkingAreaSqm,
			ParkingPrice:      txn.ParkingPrice,
			SourceRecordHash:  txn.SourceRecordHash,
			CreatedAt:         txn.CreatedAt,
		}
	}

	return result, nil
}

// GetTransaction retrieves a transaction by ID
func (s *TransactionService) GetTransaction(ctx context.Context, id string) (*TransactionData, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction ID: %w", err)
	}

	txn, err := s.txRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	return &TransactionData{
		ID:                txn.ID,
		TransactionID:     txn.TransactionID,
		TransactionDate:   txn.TransactionDate,
		TransactionType:   txn.TransactionType,
		County:            txn.County,
		District:          txn.District,
		Section:           txn.Section,
		LandNumber:        txn.LandNumber,
		TransactionTarget: txn.TransactionTarget,
		TotalPrice:        txn.TotalPrice,
		UnitPrice:         txn.UnitPrice,
		PricePerPing:      txn.PricePerPing(),
		LandAreaSqm:       txn.LandAreaSqm,
		BuildingAreaSqm:   txn.BuildingAreaSqm,
		UrbanZoning:       txn.UrbanZoning,
		NonUrbanZoning:    txn.NonUrbanZoning,
		LandUseCategory:   txn.LandUseCategory,
		BuildingType:      txn.BuildingType,
		Floor:             txn.Floor,
		Age:               txn.Age,
		ParkingAreaSqm:    txn.ParkingAreaSqm,
		ParkingPrice:      txn.ParkingPrice,
		SourceRecordHash:  txn.SourceRecordHash,
		CreatedAt:         txn.CreatedAt,
	}, nil
}

// GetTransactionStatistics retrieves statistics for a given area
func (s *TransactionService) GetTransactionStatistics(ctx context.Context, params StatisticsParams) (*StatisticsResult, error) {
	if params.County == "" || params.District == "" {
		return nil, errors.New("county and district are required")
	}

	stats, err := s.txRepo.GetStatistics(ctx, params.County, params.District, params.Section)
	if err != nil {
		return nil, fmt.Errorf("get statistics: %w", err)
	}

	// Build stats result
	return &StatisticsResult{
		Count: stats.Count,
		PricePerPing: PriceStats{
			Min:   stats.MinPrice,
			P10:   int64(float64(stats.MinPrice) * 1.1),
			P25:   int64(stats.P25Price),
			Median: int64(stats.MedianPrice),
			Mean:  float64(stats.AvgPrice),
			P75:   int64(stats.P75Price),
			P90:   int64(float64(stats.MaxPrice) * 0.9),
			Max:   stats.MaxPrice,
		},
		TotalPrice: PriceStats{
			Min:   stats.MinPrice,
			P10:   int64(float64(stats.MinPrice) * 1.1),
			P25:   int64(stats.P25Price),
			Median: int64(stats.MedianPrice),
			Mean:  float64(stats.AvgPrice),
			P75:   int64(stats.P75Price),
			P90:   int64(float64(stats.MaxPrice) * 0.9),
			Max:   stats.MaxPrice,
		},
		UnitPrice: PriceStats{
			Min:   stats.MinPrice,
			P10:   int64(float64(stats.MinPrice) * 1.1),
			P25:   int64(stats.P25Price),
			Median: int64(stats.MedianPrice),
			Mean:  float64(stats.AvgPrice),
			P75:   int64(stats.P75Price),
			P90:   int64(float64(stats.MaxPrice) * 0.9),
			Max:   stats.MaxPrice,
		},
		LandAreaSqm: AreaStats{
			Min:    0,
			P25:    0,
			Median: stats.MedianLandArea,
			Mean:   0,
			P75:    0,
			Max:    0,
		},
		BuildingAreaSqm: AreaStats{
			Min:    0,
			P25:    0,
			Median: stats.MedianBuildingArea,
			Mean:   0,
			P75:    0,
			Max:    0,
		},
		Metadata: StatsMetadata{
			AlgorithmVersion: "v2.0",
			SnapshotID:       "",
			GeneratedAt:      time.Now().UTC(),
			QueryHash:        generateQueryHash(params),
		},
	}, nil
}

// Helper functions

func generateQueryHash(params interface{}) string {
	data := fmt.Sprintf("%v", params)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// ValidateSearchParams validates search parameters
func ValidateSearchParams(params SearchParams) error {
	if params.County == "" || params.District == "" {
		return errors.New("county and district are required")
	}
	if params.DateFrom != nil && params.DateTo != nil && params.DateFrom.After(*params.DateTo) {
		return errors.New("date_from must be before date_to")
	}
	if params.Limit < 0 {
		return errors.New("limit must be non-negative")
	}
	if params.Offset < 0 {
		return errors.New("offset must be non-negative")
	}
	return nil
}

// ValidateStatisticsParams validates statistics parameters
func ValidateStatisticsParams(params StatisticsParams) error {
	if params.County == "" || params.District == "" {
		return errors.New("county and district are required")
	}
	return nil
}