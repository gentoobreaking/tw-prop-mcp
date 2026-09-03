package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository/db"
)

// Transaction repository errors.
var ErrTransactionNotFound = errors.New("transaction not found")

// SearchFilter defines search criteria for transactions.
// County and District are required; other fields are optional (nil = no filter).
type SearchFilter struct {
	County     string
	District   string
	Section    *string
	LandNumber *string
	StartDate  *time.Time
	EndDate    *time.Time
	Limit      int32
	Offset     int32
}

// StatisticsResult holds aggregated transaction statistics for a region.
type StatisticsResult struct {
	Count              int64
	MinPrice           int64
	MaxPrice           int64
	AvgPrice           int64
	MedianPrice        float64
	P25Price           float64
	P75Price           float64
	MedianLandArea     float64
	MedianBuildingArea float64
}

// TransactionRepository defines persistence operations for transactions.
type TransactionRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error)
	Search(ctx context.Context, filter SearchFilter) ([]domain.Transaction, error)
	BatchInsert(ctx context.Context, txns []domain.Transaction) (int64, error)
	GetStatistics(ctx context.Context, county, district, section string) (StatisticsResult, error)
}

type transactionRepository struct {
	queries *db.Queries
	db      DBTX
}

// NewTransactionRepository creates a repository backed by pgx + sqlc.
func NewTransactionRepository(dbt DBTX) TransactionRepository {
	return &transactionRepository{
		queries: db.New(dbt),
		db:      dbt,
	}
}

// batchInsertSize controls how many rows are flushed per CopyFrom call.
const batchInsertSize = 256

// GetByID fetches a transaction by UUID.
func (r *transactionRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error) {
	var uid pgtype.UUID
	if err := uid.Scan(id.String()); err != nil {
		return domain.Transaction{}, fmt.Errorf("invalid transaction id: %w", err)
	}
	row, err := r.queries.GetTransactionByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Transaction{}, ErrTransactionNotFound
		}
		return domain.Transaction{}, err
	}
	return toDomainTransaction(row), nil
}

// Search returns transactions matching the filter. County and District are required.
// Nil optional filters (Section, LandNumber, Date range) are passed as NULL/empty.
func (r *transactionRepository) Search(ctx context.Context, filter SearchFilter) ([]domain.Transaction, error) {
	if filter.County == "" || filter.District == "" {
		return nil, fmt.Errorf("county and district are required for search")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Empty string means "no filter" – the SQL uses ($3::text = '' OR section = $3).
	section := ""
	if filter.Section != nil {
		section = *filter.Section
	}
	landNumber := ""
	if filter.LandNumber != nil {
		landNumber = *filter.LandNumber
	}

	var startDate, endDate pgtype.Date
	if filter.StartDate != nil {
		startDate = pgtype.Date{Time: *filter.StartDate, Valid: true}
	}
	if filter.EndDate != nil {
		endDate = pgtype.Date{Time: *filter.EndDate, Valid: true}
	}

	rows, err := r.queries.SearchTransactions(ctx, db.SearchTransactionsParams{
		County:   filter.County,
		District: filter.District,
		Column3:  section,
		Column4:  landNumber,
		Column5:  startDate,
		Column6:  endDate,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainTransaction(row))
	}
	return out, nil
}

// BatchInsert inserts transactions in batches of batchInsertSize rows using CopyFrom.
func (r *transactionRepository) BatchInsert(ctx context.Context, txns []domain.Transaction) (int64, error) {
	var total int64
	for i := 0; i < len(txns); i += batchInsertSize {
		end := i + batchInsertSize
		if end > len(txns) {
			end = len(txns)
		}
		chunk := txns[i:end]
		params := make([]db.BatchInsertTransactionsParams, 0, len(chunk))
		for j := range chunk {
			p, err := toBatchInsertParam(chunk[j])
			if err != nil {
				return total, fmt.Errorf("batch insert at row %d: %w", i+j, err)
			}
			params = append(params, p)
		}
		n, err := r.queries.BatchInsertTransactions(ctx, params)
		if err != nil {
			return total, fmt.Errorf("batch insert rows %d-%d: %w", i, end, err)
		}
		total += n
	}
	return total, nil
}

// GetStatistics returns aggregated statistics for a county/district/section combination.
func (r *transactionRepository) GetStatistics(ctx context.Context, county, district, section string) (StatisticsResult, error) {
	sectionParam := pgtype.Text{String: section, Valid: section != ""}

	stats, err := r.queries.GetTransactionStats(ctx, db.GetTransactionStatsParams{
		County:   county,
		District: district,
		Section:  sectionParam,
	})
	if err != nil {
		return StatisticsResult{}, fmt.Errorf("transaction stats: %w", err)
	}

	percentiles, err := r.queries.GetTransactionPercentiles(ctx, db.GetTransactionPercentilesParams{
		County:   county,
		District: district,
		Section:  sectionParam,
	})
	if err != nil {
		return StatisticsResult{}, fmt.Errorf("transaction percentiles: %w", err)
	}

	return StatisticsResult{
		Count:              stats.Cnt,
		MinPrice:           stats.MinPrice,
		MaxPrice:           stats.MaxPrice,
		AvgPrice:           stats.AvgPrice,
		MedianPrice:        percentiles.MedianPrice,
		P25Price:           percentiles.P25Price,
		P75Price:           percentiles.P75Price,
		MedianLandArea:     percentiles.MedianLandArea,
		MedianBuildingArea: percentiles.MedianBuildingArea,
	}, nil
}

// toDomainTransaction converts a db.Transaction row to domain.Transaction.
// Only does type conversion — does not mutate domain invariants.
func toDomainTransaction(row db.Transaction) domain.Transaction {
	t := domain.Transaction{
		ID:               uuidToString(row.ID),
		TransactionID:    row.TransactionID,
		TransactionType:  row.TransactionType,
		County:           row.County,
		District:         row.District,
		TotalPrice:       row.TotalPrice,
		UnitPrice:        row.UnitPrice,
		SourceRecordHash: row.SourceRecordHash,
	}
	if row.SnapshotID.Valid {
		t.SnapshotID = uuidToString(row.SnapshotID)
	}
	if row.ImportBatchID.Valid {
		t.ImportBatchID = uuidToString(row.ImportBatchID)
	}
	if row.TransactionDate.Valid {
		t.TransactionDate = row.TransactionDate.Time
	}
	t.Section = textToString(row.Section)
	t.LandNumber = textToString(row.LandNumber)
	t.TransactionTarget = textToString(row.TransactionTarget)
	if v, err := numericToFloat64(row.LandAreaSqm); err == nil {
		t.LandAreaSqm = v
	}
	if v, err := numericToFloat64(row.BuildingAreaSqm); err == nil {
		t.BuildingAreaSqm = v
	}
	t.UrbanZoning = textToString(row.UrbanZoning)
	t.NonUrbanZoning = textToString(row.NonUrbanZoning)
	t.LandUseCategory = textToString(row.LandUseCategory)
	t.BuildingType = textToString(row.BuildingType)
	t.Floor = textToString(row.Floor)
	t.Age = int(row.Age.Int32)
	if v, err := numericToFloat64(row.ParkingAreaSqm); err == nil {
		t.ParkingAreaSqm = v
	}
	if row.ParkingPrice.Valid {
		t.ParkingPrice = row.ParkingPrice.Int64
	}
	if row.CreatedAt.Valid {
		t.CreatedAt = row.CreatedAt.Time
	}
	return t
}

// toBatchInsertParam converts domain.Transaction to db.BatchInsertTransactionsParams.
func toBatchInsertParam(t domain.Transaction) (db.BatchInsertTransactionsParams, error) {
	snapshotID, err := parseUUID(t.SnapshotID)
	if err != nil {
		return db.BatchInsertTransactionsParams{}, fmt.Errorf("snapshot_id: %w", err)
	}
	importBatchID, err := parseUUID(t.ImportBatchID)
	if err != nil {
		return db.BatchInsertTransactionsParams{}, fmt.Errorf("import_batch_id: %w", err)
	}
	return db.BatchInsertTransactionsParams{
		SnapshotID:        snapshotID,
		ImportBatchID:     importBatchID,
		TransactionID:     t.TransactionID,
		TransactionDate:   pgtype.Date{Time: t.TransactionDate, Valid: true},
		TransactionType:   t.TransactionType,
		County:            t.County,
		District:          t.District,
		Section:           textFromString(t.Section),
		LandNumber:        textFromString(t.LandNumber),
		TransactionTarget: textFromString(t.TransactionTarget),
		TotalPrice:        t.TotalPrice,
		UnitPrice:         t.UnitPrice,
		LandAreaSqm:       numericFromFloat64(t.LandAreaSqm),
		BuildingAreaSqm:   numericFromFloat64(t.BuildingAreaSqm),
		UrbanZoning:       textFromString(t.UrbanZoning),
		NonUrbanZoning:    textFromString(t.NonUrbanZoning),
		LandUseCategory:   textFromString(t.LandUseCategory),
		BuildingType:      textFromString(t.BuildingType),
		Floor:             textFromString(t.Floor),
		Age:               pgtype.Int4{Int32: int32(t.Age), Valid: t.Age > 0},
		ParkingAreaSqm:    numericFromFloat64(t.ParkingAreaSqm),
		ParkingPrice:      pgtype.Int8{Int64: t.ParkingPrice, Valid: t.ParkingPrice > 0},
		SourceRecordHash:  t.SourceRecordHash,
	}, nil
}

// textToString converts pgtype.Text to string, returning "" for NULL.
func textToString(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

// textFromString converts string to pgtype.Text, with Valid=false for empty strings.
func textFromString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// numericFromFloat64 converts float64 to pgtype.Numeric, returning NULL (invalid) for zero.
// The error from Scan is suppressed because fmt.Sprintf("%f", f) always produces a valid numeric string.
func numericFromFloat64(f float64) pgtype.Numeric {
	if f == 0 {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", f))
	return n
}
