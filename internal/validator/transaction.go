package validator

import (
	"fmt"
	"time"

	"tw-prop-mcp/internal/domain"
)

// ValidateTransaction validates a *domain.Transaction against business rules.
// It checks required fields, numeric ranges, date logic, and unit/total price
// consistency. Issues are returned in field order; blocking (error-level)
// issues indicate the record must not be imported.
//
// Validation rules (see DATA_MODEL.md "transaction" table, point 3 of T007):
//
//   - (a) Required fields: county, district, section, land_number,
//     transaction_id, transaction_date (non-zero), and source_record_hash
//     (non-empty). Uniqueness of source_record_hash is enforced by the
//     repository UNIQUE(snapshot_id, source_record_hash) constraint, not here.
//   - (b) Numeric ranges: total_price > 0, land_area_sqm > 0,
//     building_area_sqm >= 0, parking_area_sqm >= 0, parking_price >= 0,
//     age >= 0.
//   - (c) Date logic: transaction_date must be <= today (see Clock) and
//     >= ReasonableMinDate (1990-01-01).
//   - (d) Consistency: unit_price > 0 while total_price <= 0 is inconsistent.
//
// The input Transaction is treated as read-only and is never mutated.
func (v *Validator) ValidateTransaction(tx *domain.Transaction) []ValidationIssue {
	var issues []ValidationIssue
	if tx == nil {
		appendIssue(&issues, LevelError, CodeRequiredField, "transaction",
			"transaction must not be nil", nil)
		return issues
	}

	// (a) Required fields.
	appendRequired(&issues, "county", tx.County)
	appendRequired(&issues, "district", tx.District)
	appendRequired(&issues, "section", tx.Section)
	appendRequired(&issues, "land_number", tx.LandNumber)
	appendRequired(&issues, "transaction_id", tx.TransactionID)
	appendRequired(&issues, "source_record_hash", tx.SourceRecordHash)
	if tx.TransactionDate.IsZero() {
		appendIssue(&issues, LevelError, CodeRequiredField, "transaction_date",
			"transaction_date is required", nil)
	}

	// (b) Numeric ranges.
	appendPositiveInt64(&issues, "total_price", tx.TotalPrice)
	appendPositiveFloat64(&issues, "land_area_sqm", tx.LandAreaSqm)
	appendNonNegativeFloat64(&issues, "building_area_sqm", tx.BuildingAreaSqm)
	appendNonNegativeFloat64(&issues, "parking_area_sqm", tx.ParkingAreaSqm)
	appendNonNegativeInt64(&issues, "parking_price", tx.ParkingPrice)
	appendNonNegativeInt(&issues, "age", tx.Age)

	// (c) Date logic (only when a date is present).
	if !tx.TransactionDate.IsZero() {
		v.appendDateIssues(&issues, "transaction_date", tx.TransactionDate)
	}

	// (d) Unit/total price consistency.
	if tx.UnitPrice > 0 && tx.TotalPrice <= 0 {
		appendIssue(&issues, LevelError, CodeUnitPriceInconsistency, "unit_price",
			"unit_price is positive but total_price is not positive, indicating an inconsistency",
			tx.UnitPrice)
	}

	return issues
}

// appendDateIssues checks that t is not in the future and not before
// ReasonableMinDate. today is resolved from the validator's Clock for test
// determinism.
func (v *Validator) appendDateIssues(issues *[]ValidationIssue, field string, t time.Time) {
	today := v.today()
	const layout = "2006-01-02"
	if compareDates(t, today) > 0 {
		appendIssue(issues, LevelError, CodeDateFuture, field,
			fmt.Sprintf("transaction_date %s is after today (%s)",
				t.Format(layout), today.Format(layout)), t)
	}
	if compareDates(t, ReasonableMinDate) < 0 {
		appendIssue(issues, LevelError, CodeDateTooEarly, field,
			fmt.Sprintf("transaction_date %s is before the minimum allowed date (%s)",
				t.Format(layout), ReasonableMinDate.Format(layout)), t)
	}
}

