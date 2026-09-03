package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository"
)

// mockTxRepo implements TransactionRepository for testing
type mockTxRepo struct {
	transactions []domain.Transaction
}

func (m *mockTxRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error) {
	for _, txn := range m.transactions {
		if txn.ID == id.String() {
			return txn, nil
		}
	}
	return domain.Transaction{}, errors.New("not found")
}

func (m *mockTxRepo) Search(ctx context.Context, filter repository.SearchFilter) ([]domain.Transaction, error) {
	return m.transactions, nil
}

func (m *mockTxRepo) BatchInsert(ctx context.Context, txns []domain.Transaction) (int64, error) {
	return int64(len(txns)), nil
}

func (m *mockTxRepo) GetStatistics(ctx context.Context, county, district, section string) (repository.StatisticsResult, error) {
	return repository.StatisticsResult{
		Count:              10,
		MinPrice:           1000000,
		MaxPrice:           5000000,
		AvgPrice:           3000000,
		MedianPrice:        3000000,
		P25Price:           2000000,
		P75Price:           4000000,
		MedianLandArea:     100.0,
		MedianBuildingArea: 80.0,
	}, nil
}

// mockSnapshotRepo for testing
type mockSnapshotRepo struct {
	snapshots []domain.DatasetSnapshot
}

func (m *mockSnapshotRepo) Create(ctx context.Context, arg repository.CreateSnapshotParams) (domain.DatasetSnapshot, error) {
	return domain.DatasetSnapshot{}, nil
}

func (m *mockSnapshotRepo) GetByID(ctx context.Context, id string) (domain.DatasetSnapshot, error) {
	return domain.DatasetSnapshot{}, nil
}

func (m *mockSnapshotRepo) List(ctx context.Context) ([]domain.DatasetSnapshot, error) {
	return m.snapshots, nil
}

func (m *mockSnapshotRepo) UpdateStatus(ctx context.Context, id string, to domain.SnapshotStatus) error {
	return nil
}

func (m *mockSnapshotRepo) Lock(ctx context.Context, id string) error {
	return nil
}

func (m *mockSnapshotRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func TestTransactionService_SearchTransactions(t *testing.T) {
	mockRepo := &mockTxRepo{
		transactions: []domain.Transaction{
			{
				ID:                uuid.New().String(),
				TransactionID:     "TXN001",
				TransactionDate:   time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
				TransactionType:   "A",
				County:            "台北市",
				District:          "大安區",
				Section:           "忠孝段",
				LandNumber:        "0001",
				TransactionTarget: "土地",
				TotalPrice:        10000000,
				UnitPrice:         300000,
				LandAreaSqm:       33.33,
				BuildingAreaSqm:   0,
				UrbanZoning:       "住宅區",
				LandUseCategory:   "住宅",
				BuildingType:      "",
				Floor:             "",
				Age:               0,
				ParkingAreaSqm:    0,
				ParkingPrice:      0,
				SourceRecordHash:  "abc123",
				CreatedAt:         time.Now(),
			},
		},
	}

	svc := NewTransactionService(mockRepo)

	ctx := context.Background()
	params := SearchParams{
		County:   "台北市",
		District: "大安區",
		Limit:    10,
		Offset:   0,
	}

	result, err := svc.SearchTransactions(ctx, params)
	if err != nil {
		t.Fatalf("SearchTransactions error: %v", err)
	}

	if len(result.Data) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(result.Data))
	}

	if result.Data[0].County != "台北市" {
		t.Errorf("expected county 台北市, got %s", result.Data[0].County)
	}

	if result.Data[0].PricePerPing <= 0 {
		t.Errorf("expected PricePerPing > 0, got %f", result.Data[0].PricePerPing)
	}

	if result.Metadata.AlgorithmVersion != "v2.0" {
		t.Errorf("expected algorithm version v2.0, got %s", result.Metadata.AlgorithmVersion)
	}

	if result.Metadata.QueryHash == "" {
		t.Error("expected query hash to be generated")
	}
}

func TestTransactionService_SearchTransactions_MissingCounty(t *testing.T) {
	mockRepo := &mockTxRepo{}
	svc := NewTransactionService(mockRepo)

	ctx := context.Background()
	params := SearchParams{
		District: "大安區",
	}

	_, err := svc.SearchTransactions(ctx, params)
	if err == nil {
		t.Error("expected error for missing county")
	}
}

func TestTransactionService_SearchTransactions_MissingDistrict(t *testing.T) {
	mockRepo := &mockTxRepo{}
	svc := NewTransactionService(mockRepo)

	ctx := context.Background()
	params := SearchParams{
		County: "台北市",
	}

	_, err := svc.SearchTransactions(ctx, params)
	if err == nil {
		t.Error("expected error for missing district")
	}
}

func TestTransactionService_SearchTransactions_InvalidDateRange(t *testing.T) {
	mockRepo := &mockTxRepo{}
	svc := NewTransactionService(mockRepo)

	ctx := context.Background()
	dateFrom := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	dateTo := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	params := SearchParams{
		County:    "台北市",
		District:  "大安區",
		DateFrom:  &dateFrom,
		DateTo:    &dateTo,
	}

	_, err := svc.SearchTransactions(ctx, params)
	if err == nil {
		t.Error("expected error for invalid date range")
	}
}

func TestTransactionService_GetTransaction(t *testing.T) {
	txn := domain.Transaction{
		ID:                uuid.New().String(),
		TransactionID:     "TXN001",
		TransactionDate:   time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
		TransactionType:   "A",
		County:            "台北市",
		District:          "大安區",
		Section:           "忠孝段",
		LandNumber:        "0001",
		TransactionTarget: "土地",
		TotalPrice:        10000000,
		UnitPrice:         300000,
		LandAreaSqm:       33.33,
		BuildingAreaSqm:   0,
		UrbanZoning:       "住宅區",
		LandUseCategory:   "住宅",
		BuildingType:      "",
		Floor:             "",
		Age:               0,
		ParkingAreaSqm:    0,
		ParkingPrice:      0,
		SourceRecordHash:  "abc123",
		CreatedAt:         time.Now(),
	}

	mockRepo := &mockTxRepo{
		transactions: []domain.Transaction{txn},
	}

	svc := NewTransactionService(mockRepo)

	ctx := context.Background()
	result, err := svc.GetTransaction(ctx, txn.ID)
	if err != nil {
		t.Fatalf("GetTransaction error: %v", err)
	}

	if result.ID != txn.ID {
		t.Errorf("expected ID %s, got %s", txn.ID, result.ID)
	}

	if result.PricePerPing <= 0 {
		t.Errorf("expected PricePerPing > 0, got %f", result.PricePerPing)
	}
}

func TestTransactionService_GetTransaction_InvalidID(t *testing.T) {
	mockRepo := &mockTxRepo{}
	svc := NewTransactionService(mockRepo)

	ctx := context.Background()
	_, err := svc.GetTransaction(ctx, "invalid-id")
	if err == nil {
		t.Error("expected error for invalid ID")
	}
}

func TestTransactionService_GetTransactionStatistics(t *testing.T) {
	mockRepo := &mockTxRepo{}
	svc := NewTransactionService(mockRepo)

	ctx := context.Background()

	result, err := svc.GetTransactionStatistics(ctx, StatisticsParams{
		County:   "台北市",
		District: "大安區",
		Section:  "忠孝段",
	})
	if err != nil {
		t.Fatalf("GetTransactionStatistics error: %v", err)
	}

	if result.Count != 10 {
		t.Errorf("expected count 10, got %d", result.Count)
	}

	if result.PricePerPing.Min != 1000000 {
		t.Errorf("expected min price 1000000, got %d", result.PricePerPing.Min)
	}

	if result.Metadata.AlgorithmVersion != "v2.0" {
		t.Errorf("expected algorithm version v2.0, got %s", result.Metadata.AlgorithmVersion)
	}
}

func TestTransactionService_GetTransactionStatistics_MissingCounty(t *testing.T) {
	mockRepo := &mockTxRepo{}
	svc := NewTransactionService(mockRepo)

	ctx := context.Background()
	_, err := svc.GetTransactionStatistics(ctx, StatisticsParams{
		District: "大安區",
	})
	if err == nil {
		t.Error("expected error for missing county")
	}
}

func TestValidateSearchParams(t *testing.T) {
	valid := SearchParams{
		County:   "台北市",
		District: "大安區",
	}
	if err := ValidateSearchParams(valid); err != nil {
		t.Errorf("valid params should pass: %v", err)
	}

	missingCounty := SearchParams{District: "大安區"}
	if err := ValidateSearchParams(missingCounty); err == nil {
		t.Error("expected error for missing county")
	}

	missingDistrict := SearchParams{County: "台北市"}
	if err := ValidateSearchParams(missingDistrict); err == nil {
		t.Error("expected error for missing district")
	}

	invalidDateRange := SearchParams{
		County:   "台北市",
		District: "大安區",
		DateFrom: func() *time.Time { t := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC); return &t }(),
		DateTo:   func() *time.Time { t := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC); return &t }(),
	}
	if err := ValidateSearchParams(invalidDateRange); err == nil {
		t.Error("expected error for invalid date range")
	}

	negativeLimit := SearchParams{
		County:   "台北市",
		District: "大安區",
		Limit:    -1,
	}
	if err := ValidateSearchParams(negativeLimit); err == nil {
		t.Error("expected error for negative limit")
	}
}

func TestValidateStatisticsParams(t *testing.T) {
	valid := StatisticsParams{County: "台北市", District: "大安區"}
	if err := ValidateStatisticsParams(valid); err != nil {
		t.Errorf("valid params should pass: %v", err)
	}

	missingCounty := StatisticsParams{District: "大安區"}
	if err := ValidateStatisticsParams(missingCounty); err == nil {
		t.Error("expected error for missing county")
	}

	missingDistrict := StatisticsParams{County: "台北市"}
	if err := ValidateStatisticsParams(missingDistrict); err == nil {
		t.Error("expected error for missing district")
	}
}