package validator

import (
	"testing"
	"time"

	"tw-prop-mcp/internal/clock"
	"tw-prop-mcp/internal/domain"
)

// fixedClock returns a clock.NowFunc pinned to t, for deterministic date tests.
func fixedClock(t time.Time) clock.NowFunc {
	return func() time.Time { return t }
}

// testToday is the pinned "today" used across date-validation tests.
var testToday = time.Date(2026, time.September, 3, 15, 0, 0, 0, time.UTC)

// validTransaction returns a fully-valid Transaction for mutation in tests.
func validTransaction() *domain.Transaction {
	return &domain.Transaction{
		TransactionID:    "TX-0001",
		TransactionDate:  time.Date(2026, time.September, 3, 0, 0, 0, 0, time.UTC),
		County:           "台北市",
		District:         "中正區",
		Section:          "重慶段一小段",
		LandNumber:       "00123",
		TotalPrice:       1000000,
		UnitPrice:        9980,
		LandAreaSqm:      100.5,
		BuildingAreaSqm:  120.0,
		ParkingAreaSqm:   10.0,
		ParkingPrice:     100000,
		Age:              10,
		SourceRecordHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
}

// validParcel returns a fully-valid Parcel for mutation in tests.
func validParcel() *domain.Parcel {
	return &domain.Parcel{
		County:          "台北市",
		District:        "中正區",
		Section:         "重慶段一小段",
		LandNumber:      "00123",
		AreaSqm:         123.45,
		UrbanZoning:     "住",
		LandUseCategory: "住宅",
		Geometry:        "MULTIPOLYGON(((0 0, 1 0, 1 1, 0 1, 0 0)))",
		Source:          "NLSC",
		SourceVersion:   "2024Q1",
	}
}

// hasErrorOnField reports whether issues contains an error-level issue on field.
func hasErrorOnField(issues []ValidationIssue, field string) bool {
	for _, iss := range issues {
		if iss.Level == LevelError && iss.Field == field {
			return true
		}
	}
	return false
}

// hasErrorCode reports whether issues contains an issue with the given code.
func hasErrorCode(issues []ValidationIssue, code string) bool {
	for _, iss := range issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Transaction tests
// ---------------------------------------------------------------------------

func TestValidator_Transaction_Valid(t *testing.T) {
	v := New(fixedClock(testToday))
	tx := validTransaction()
	issues := v.ValidateTransaction(tx)
	if v.HasBlockingErrors(issues) {
		t.Fatalf("expected no blocking errors for valid transaction, got %d issues: %+v", len(issues), issues)
	}
}

func TestValidator_Transaction_MissingFields(t *testing.T) {
	v := New(fixedClock(testToday))
	requiredFields := []struct {
		field string
		clear func(tx *domain.Transaction)
	}{
		{"county", func(tx *domain.Transaction) { tx.County = "" }},
		{"district", func(tx *domain.Transaction) { tx.District = "" }},
		{"section", func(tx *domain.Transaction) { tx.Section = "" }},
		{"land_number", func(tx *domain.Transaction) { tx.LandNumber = "" }},
		{"transaction_id", func(tx *domain.Transaction) { tx.TransactionID = "" }},
		{"source_record_hash", func(tx *domain.Transaction) { tx.SourceRecordHash = "" }},
	}
	for _, rf := range requiredFields {
		tx := validTransaction()
		rf.clear(tx)
		issues := v.ValidateTransaction(tx)
		if !hasErrorOnField(issues, rf.field) {
			t.Errorf("missing %s: expected error-level issue on field %q, got: %+v", rf.field, rf.field, issues)
		}
	}

	// transaction_date zero value must also be flagged.
	tx := validTransaction()
	tx.TransactionDate = time.Time{}
	issues := v.ValidateTransaction(tx)
	if !hasErrorOnField(issues, "transaction_date") {
		t.Errorf("missing transaction_date: expected error-level issue, got: %+v", issues)
	}
}

func TestValidator_Transaction_InvalidPriceArea(t *testing.T) {
	v := New(fixedClock(testToday))

	// total_price <= 0 must error.
	tx := validTransaction()
	tx.TotalPrice = 0
	issues := v.ValidateTransaction(tx)
	if !hasErrorOnField(issues, "total_price") {
		t.Errorf("total_price=0: expected error on total_price, got: %+v", issues)
	}

	tx = validTransaction()
	tx.TotalPrice = -1
	issues = v.ValidateTransaction(tx)
	if !hasErrorOnField(issues, "total_price") {
		t.Errorf("total_price=-1: expected error on total_price, got: %+v", issues)
	}

	// land_area_sqm <= 0 must error.
	tx = validTransaction()
	tx.LandAreaSqm = 0
	issues = v.ValidateTransaction(tx)
	if !hasErrorOnField(issues, "land_area_sqm") {
		t.Errorf("land_area_sqm=0: expected error on land_area_sqm, got: %+v", issues)
	}

	tx = validTransaction()
	tx.LandAreaSqm = -5
	issues = v.ValidateTransaction(tx)
	if !hasErrorOnField(issues, "land_area_sqm") {
		t.Errorf("land_area_sqm=-5: expected error on land_area_sqm, got: %+v", issues)
	}

	// Non-negative areas that are allowed to be zero must NOT error.
	tx = validTransaction()
	tx.BuildingAreaSqm = 0
	tx.ParkingAreaSqm = 0
	tx.ParkingPrice = 0
	tx.Age = 0
	issues = v.ValidateTransaction(tx)
	if v.HasBlockingErrors(issues) {
		t.Errorf("zero allowed values should not block, got: %+v", issues)
	}
}

func TestValidator_Transaction_FutureDate(t *testing.T) {
	v := New(fixedClock(testToday))
	tx := validTransaction()
	tx.TransactionDate = testToday.AddDate(0, 0, 1) // tomorrow
	issues := v.ValidateTransaction(tx)
	if !hasErrorCode(issues, CodeDateFuture) {
		t.Fatalf("expected %s issue for future date, got: %+v", CodeDateFuture, issues)
	}
	if !v.HasBlockingErrors(issues) {
		t.Fatalf("future date must be blocking, got: %+v", issues)
	}
}

func TestValidator_Transaction_ReasonableMinDate(t *testing.T) {
	v := New(fixedClock(testToday))
	tx := validTransaction()
	tx.TransactionDate = time.Date(1899, time.January, 1, 0, 0, 0, 0, time.UTC)
	issues := v.ValidateTransaction(tx)
	if !hasErrorCode(issues, CodeDateTooEarly) {
		t.Fatalf("expected %s issue for date before 1990, got: %+v", CodeDateTooEarly, issues)
	}
	if !v.HasBlockingErrors(issues) {
		t.Fatalf("date before reasonable minimum must be blocking, got: %+v", issues)
	}
}

func TestValidator_Transaction_UnitPriceConsistency(t *testing.T) {
	v := New(fixedClock(testToday))
	tx := validTransaction()
	tx.UnitPrice = 9980 // positive
	tx.TotalPrice = 0  // non-positive -> inconsistent
	issues := v.ValidateTransaction(tx)
	if !hasErrorCode(issues, CodeUnitPriceInconsistency) {
		t.Fatalf("expected %s issue when unit_price>0 and total_price<=0, got: %+v",
			CodeUnitPriceInconsistency, issues)
	}
	if !v.HasBlockingErrors(issues) {
		t.Fatalf("unit price inconsistency must be blocking, got: %+v", issues)
	}
}

func TestValidator_Transaction_AgeNegative(t *testing.T) {
	v := New(fixedClock(testToday))
	tx := validTransaction()
	tx.Age = -1
	issues := v.ValidateTransaction(tx)
	if !hasErrorOnField(issues, "age") {
		t.Fatalf("expected error on age when age<0, got: %+v", issues)
	}
	if !v.HasBlockingErrors(issues) {
		t.Fatalf("negative age must be blocking, got: %+v", issues)
	}
}

// ---------------------------------------------------------------------------
// Parcel tests
// ---------------------------------------------------------------------------

func TestValidator_Parcel_Valid(t *testing.T) {
	v := New(fixedClock(testToday))
	p := validParcel()
	issues := v.ValidateParcel(p)
	if v.HasBlockingErrors(issues) {
		t.Fatalf("expected no blocking errors for valid parcel, got: %+v", issues)
	}
}

func TestValidator_Parcel_MissingFields(t *testing.T) {
	v := New(fixedClock(testToday))
	requiredFields := []struct {
		field string
		clear func(p *domain.Parcel)
	}{
		{"county", func(p *domain.Parcel) { p.County = "" }},
		{"district", func(p *domain.Parcel) { p.District = "" }},
		{"section", func(p *domain.Parcel) { p.Section = "" }},
		{"land_number", func(p *domain.Parcel) { p.LandNumber = "" }},
	}
	for _, rf := range requiredFields {
		p := validParcel()
		rf.clear(p)
		issues := v.ValidateParcel(p)
		if !hasErrorOnField(issues, rf.field) {
			t.Errorf("missing %s: expected error-level issue on field %q, got: %+v",
				rf.field, rf.field, issues)
		}
	}
}

func TestValidator_Parcel_InvalidArea(t *testing.T) {
	v := New(fixedClock(testToday))
	p := validParcel()
	p.AreaSqm = 0
	issues := v.ValidateParcel(p)
	if !hasErrorOnField(issues, "area_sqm") {
		t.Fatalf("expected error on area_sqm when area<=0, got: %+v", issues)
	}
	if !v.HasBlockingErrors(issues) {
		t.Fatalf("invalid parcel area must be blocking, got: %+v", issues)
	}

	p = validParcel()
	p.AreaSqm = -12.5
	issues = v.ValidateParcel(p)
	if !hasErrorOnField(issues, "area_sqm") {
		t.Fatalf("expected error on area_sqm when area<0, got: %+v", issues)
	}
}

// ---------------------------------------------------------------------------
// Immutability test
// ---------------------------------------------------------------------------

func TestValidator_Immutable(t *testing.T) {
	v := New(fixedClock(testToday))

	// Validate a valid transaction (happy path must not mutate).
	tx := validTransaction()
	before := *tx
	if issues := v.ValidateTransaction(tx); v.HasBlockingErrors(issues) {
		t.Fatalf("valid transaction should produce no blocking errors: %+v", issues)
	}
	if *tx != before {
		t.Errorf("validator mutated valid input transaction\nbefore: %+v\nafter:  %+v", before, *tx)
	}

	// Validate an invalid transaction (error paths must also not mutate).
	tx2 := validTransaction()
	tx2.TotalPrice = 0
	tx2.Age = -5
	before2 := *tx2
	_ = v.ValidateTransaction(tx2)
	if *tx2 != before2 {
		t.Errorf("validator mutated invalid input transaction\nbefore: %+v\nafter:  %+v", before2, *tx2)
	}

	// Validate a parcel too (happy path).
	p := validParcel()
	beforeP := *p
	if issues := v.ValidateParcel(p); v.HasBlockingErrors(issues) {
		t.Fatalf("valid parcel should produce no blocking errors: %+v", issues)
	}
	if *p != beforeP {
		t.Errorf("validator mutated input parcel\nbefore: %+v\nafter:  %+v", beforeP, *p)
	}
}

// ---------------------------------------------------------------------------
// Helpers / level & WKT coverage
// ---------------------------------------------------------------------------

func TestHasBlockingErrors_ByLevel(t *testing.T) {
	v := New(fixedClock(testToday))

	info := []ValidationIssue{{Level: LevelInfo, Code: CodeGeometryPending, Field: "geometry", Message: "pending"}}
	warn := []ValidationIssue{{Level: LevelWarning, Code: CodeGeometryWKTInvalid, Field: "geometry", Message: "bad wkt"}}
	errs := []ValidationIssue{{Level: LevelError, Code: CodeRequiredField, Field: "county", Message: "missing"}}

	if v.HasBlockingErrors(info) {
		t.Errorf("info-only issues must not be blocking")
	}
	if v.HasBlockingErrors(warn) {
		t.Errorf("warning-only issues must not be blocking")
	}
	if !v.HasBlockingErrors(errs) {
		t.Errorf("error-level issues must be blocking")
	}
	if !v.HasBlockingErrors(append(warn, errs...)) {
		t.Errorf("mixed warning+error issues must be blocking")
	}
}

func TestValidator_Parcel_GeoWKTPrecheck(t *testing.T) {
	v := New(fixedClock(testToday))

	// Malformed geometry -> warning (not a hard error) and still non-blocking.
	p := validParcel()
	p.Geometry = "not-a-wkt"
	issues := v.ValidateParcel(p)
	if !hasErrorCode(issues, CodeGeometryWKTInvalid) {
		t.Errorf("expected %s for malformed geometry, got: %+v", CodeGeometryWKTInvalid, issues)
	}
	if v.HasBlockingErrors(issues) {
		t.Errorf("malformed geometry must be a warning, not a blocking error, got: %+v", issues)
	}

	// Valid EWKT with SRID prefix must pass the pre-check.
	p = validParcel()
	p.Geometry = "SRID=3826;MULTIPOLYGON(((0 0, 1 0, 1 1, 0 1, 0 0)))"
	issues = v.ValidateParcel(p)
	if hasErrorCode(issues, CodeGeometryWKTInvalid) {
		t.Errorf("valid EWKT must not trigger %s, got: %+v", CodeGeometryWKTInvalid, issues)
	}

	// In Basic mode the WKT pre-check is skipped entirely.
	basic := &Validator{Clock: fixedClock(testToday), Level: LevelBasic}
	p = validParcel()
	p.Geometry = "not-a-wkt"
	issues = basic.ValidateParcel(p)
	if hasErrorCode(issues, CodeGeometryWKTInvalid) {
		t.Errorf("basic mode must skip WKT pre-check, got: %+v", issues)
	}
}

func TestValidator_Parcel_EmptyGeometryInfo(t *testing.T) {
	v := New(fixedClock(testToday))
	p := validParcel()
	p.Geometry = ""
	issues := v.ValidateParcel(p)
	if !hasErrorCode(issues, CodeGeometryPending) {
		t.Errorf("expected %s for empty geometry in full mode, got: %+v", CodeGeometryPending, issues)
	}
	if v.HasBlockingErrors(issues) {
		t.Errorf("empty geometry is expected (GIS fills it) and must not block, got: %+v", issues)
	}
}

func TestValidationError_ErrorFormat(t *testing.T) {
	ve := &ValidationError{Issues: []ValidationIssue{
		{Level: LevelError, Code: CodeRequiredField, Field: "county", Message: "county is required"},
		{Level: LevelWarning, Code: CodeGeometryWKTInvalid, Field: "geometry", Message: "bad wkt"},
	}}
	s := ve.Error()
	if s == "" {
		t.Fatal("Error() should be non-empty")
	}
}
