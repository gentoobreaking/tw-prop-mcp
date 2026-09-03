package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
)

func TestTransactionRepositoryInterface(t *testing.T) {
	var _ TransactionRepository = (*transactionRepository)(nil)
}

func TestTransactionRepository_Search_EmptyResult(t *testing.T) {
	// Search with valid county/district should delegate to sqlc SearchTransactions (Query).
	// Mock returns empty rows → expect empty result, no error.
	section := "中山段"
	mock := &mockDBTX{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if len(args) < 7 {
				t.Fatalf("expected at least 7 args, got %d", len(args))
			}
			if args[0] != "台北市" {
				t.Fatalf("county mismatch: %v", args[0])
			}
			if args[1] != "大安區" {
				t.Fatalf("district mismatch: %v", args[1])
			}
			return &mockRows{rows: [][]any{}}, nil
		},
	}
	repo := NewTransactionRepository(mock)

	filter := SearchFilter{
		County:    "台北市",
		District:  "大安區",
		Section:   &section,
		Limit:     10,
		Offset:    0,
	}
	results, err := repo.Search(context.Background(), filter)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty result, got %d rows", len(results))
	}
}

func TestTransactionRepository_BatchInsertCount(t *testing.T) {
	// Verify that BatchInsert correctly batches into chunks of batchInsertSize (256)
	// and returns the total count across all batches.
	var batchCount int
	var totalRows int64
	mock := &mockDBTX{
		copyFromFn: func(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
			batchCount++
			var count int64
			for rowSrc.Next() {
				_, err := rowSrc.Values()
				if err != nil {
					return 0, err
				}
				count++
			}
			if err := rowSrc.Err(); err != nil {
				return 0, err
			}
			totalRows += count
			return count, nil
		},
	}
	repo := NewTransactionRepository(mock)

	const numTxns = 300 // exceeds batchInsertSize (256) → expects 2 batches
	txns := make([]domain.Transaction, numTxns)
	for i := range txns {
		txns[i] = domain.Transaction{
			SnapshotID:        "11111111-1111-1111-1111-111111111111",
			ImportBatchID:     "22222222-2222-2222-2222-222222222222",
			TransactionID:     fmt.Sprintf("TX%04d", i),
			TransactionDate:   time.Date(2024, 1, 1+i%28, 0, 0, 0, 0, time.UTC),
			TransactionType:   "土地",
			County:            "台北市",
			District:          "中山區",
			Section:           "中山段",
			LandNumber:        fmt.Sprintf("0001-%04d", i),
			TotalPrice:        int64(1000000 + i*100000),
			UnitPrice:         int64(10000 + i*1000),
			LandAreaSqm:       50.0 + float64(i),
			BuildingAreaSqm:   30.0 + float64(i),
			UrbanZoning:       "住",
			LandUseCategory:   "住宅",
			BuildingType:      "鋼筋混凸",
			Floor:             "5",
			Age:               i,
			ParkingAreaSqm:    20.0 + float64(i),
			ParkingPrice:      int64(50000 + i*1000),
			SourceRecordHash:  fmt.Sprintf("hash%064d", i),
		}
	}

	inserted, err := repo.BatchInsert(context.Background(), txns)
	if err != nil {
		t.Fatalf("BatchInsert failed: %v", err)
	}
	if inserted != 300 {
		t.Fatalf("expected 300 inserted, got %d", inserted)
	}
	if totalRows != 300 {
		t.Fatalf("expected 300 total rows consumed by mocks, got %d", totalRows)
	}
	// 300 / 256 = 2 batches (256 + 44)
	if batchCount != 2 {
		t.Fatalf("expected 2 CopyFrom calls (batching at %d), got %d", batchInsertSize, batchCount)
	}
}

func TestTransactionRepository_Search_RequiresCountyDistrict(t *testing.T) {
	repo := NewTransactionRepository(&mockDBTX{})
	_, err := repo.Search(context.Background(), SearchFilter{County: "", District: "中山區"})
	if err == nil {
		t.Fatalf("expected error for empty county")
	}
	_, err = repo.Search(context.Background(), SearchFilter{County: "台北市", District: ""})
	if err == nil {
		t.Fatalf("expected error for empty district")
	}
}

func TestTransactionRepository_GetByID_NotFound(t *testing.T) {
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	repo := NewTransactionRepository(mock)
	_, err := repo.GetByID(context.Background(), mustParseUUID(t, "11111111-1111-1111-1111-111111111111"))
	if !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestTransactionRepository_GetByID_Success(t *testing.T) {
	testID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	uid := mustParseUUID(t, testID)
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					// db.Transaction has 25 fields; fill them in order
					// See db.Transaction struct in models.go
					for i, d := range dest {
						switch v := d.(type) {
						case *pgtype.UUID:
							if i == 0 {
								_ = v.Scan(testID) // id
							} else if i == 1 {
								_ = v.Scan("11111111-1111-1111-1111-111111111111") // snapshot_id
							} else if i == 2 {
								_ = v.Scan("22222222-2222-2222-2222-222222222222") // import_batch_id
							}
						case *string:
							switch i {
							case 3:
								*v = "TX001"
							case 5:
								*v = "土地"
							case 6:
								*v = "台北市"
							case 7:
								*v = "中山區"
							case 23:
								*v = "hash0001" + "0000000000000000000000000000000000000000000000000000000000000001"
							default:
								*v = "mock"
							}
						case *pgtype.Text:
							*v = pgtype.Text{String: "mock", Valid: true}
						case *int64:
							*v = 1000000
						case *pgtype.Numeric:
							_ = v.Scan("50.0")
						case *pgtype.Int4:
							v.Int32 = 10
							v.Valid = true
						case *pgtype.Int8:
							v.Int64 = 50000
							v.Valid = true
						case *pgtype.Date:
							_ = v.Scan("2024-01-15")
						case *pgtype.Timestamptz:
							v.Time = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
							v.Valid = true
						}
					}
					return nil
				},
			}
		},
	}
	repo := NewTransactionRepository(mock)
	result, err := repo.GetByID(context.Background(), uid)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if result.ID != testID {
		t.Fatalf("expected ID %s, got %s", testID, result.ID)
	}
	if result.County != "台北市" {
		t.Fatalf("expected county 台北市, got %s", result.County)
	}
	if result.District != "中山區" {
		t.Fatalf("expected district 中山區, got %s", result.District)
	}
	if result.TotalPrice != 1000000 {
		t.Fatalf("expected total_price 1000000, got %d", result.TotalPrice)
	}
}

func TestTransactionRepository_GetStatistics_Merges(t *testing.T) {
	// Verify that GetStatistics calls both GetTransactionStats and GetTransactionPercentiles
	// and merges results into StatisticsResult.
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					// GetTransactionStats scans [Cnt, MinPrice, MaxPrice, AvgPrice] (4 args)
					// GetTransactionPercentiles scans [MedianPrice, P25Price, P75Price, MedianLandArea, MedianBuildingArea] (5 args)
					switch len(dest) {
					case 4:
						// GetTransactionStats
						*dest[0].(*int64) = 5
						*dest[1].(*int64) = 1000000
						*dest[2].(*int64) = 5000000
						*dest[3].(*int64) = 3000000
					case 5:
						// GetTransactionPercentiles
						*dest[0].(*float64) = 3000000
						*dest[1].(*float64) = 2000000
						*dest[2].(*float64) = 4000000
						*dest[3].(*float64) = 70.0
						*dest[4].(*float64) = 50.0
					}
					return nil
				},
			}
		},
	}
	repo := NewTransactionRepository(mock)
	result, err := repo.GetStatistics(context.Background(), "台北市", "中山區", "中山段")
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}
	if result.Count != 5 {
		t.Fatalf("expected count 5, got %d", result.Count)
	}
	if result.MinPrice != 1000000 {
		t.Fatalf("expected min price 1000000, got %d", result.MinPrice)
	}
	if result.MaxPrice != 5000000 {
		t.Fatalf("expected max price 5000000, got %d", result.MaxPrice)
	}
	if result.AvgPrice != 3000000 {
		t.Fatalf("expected avg price 3000000, got %d", result.AvgPrice)
	}
	if result.MedianPrice != 3000000 {
		t.Fatalf("expected median price 3000000, got %f", result.MedianPrice)
	}
	if result.P25Price != 2000000 {
		t.Fatalf("expected p25 price 2000000, got %f", result.P25Price)
	}
	if result.P75Price != 4000000 {
		t.Fatalf("expected p75 price 4000000, got %f", result.P75Price)
	}
	if result.MedianLandArea != 70.0 {
		t.Fatalf("expected median land area 70, got %f", result.MedianLandArea)
	}
	if result.MedianBuildingArea != 50.0 {
		t.Fatalf("expected median building area 50, got %f", result.MedianBuildingArea)
	}
}

// mustParseUUID parses a UUID string or fails the test.
func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return id
}
