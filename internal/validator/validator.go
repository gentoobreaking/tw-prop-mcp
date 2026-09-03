// Package validator validates normalized domain objects (Transaction, Parcel)
// against the business rules defined in DATA_MODEL.md and SPEC.md.
//
// The Validator runs after the T006 Normalizer (which converts raw CSV rows
// into domain objects) and before repository insertion. Uniqueness and
// foreign-key constraints that require external state are enforced by the
// repository layer (UNIQUE indexes); the Validator focuses on self-contained
// structural, range, and temporal rules.
package validator

import (
	"fmt"
	"strings"
	"time"

	"tw-prop-mcp/internal/clock"
	"tw-prop-mcp/internal/domain"
)

// ValidationLevel controls how thorough validation is.
type ValidationLevel int

const (
	// LevelBasic checks self-contained structural, range, and temporal rules.
	LevelBasic ValidationLevel = iota
	// LevelFull additionally checks supplementary rules that depend on
	// externally-supplied data (e.g. GIS geometry completeness).
	LevelFull
)

// Severity levels for validation issues. The ordering matters:
// info < warning < error. See HasBlockingErrors.
const (
	LevelInfo    = "info"
	LevelWarning = "warning"
	LevelError   = "error"
)

// Severity rank used by HasBlockingErrors. Maps level strings to an ordering
// so that comparisons are not done lexicographically (where "warning" > "error").
var severityRank = map[string]int{
	LevelInfo:    0,
	LevelWarning: 1,
	LevelError:   2,
}

// Issue code constants. Codes are grouped by prefix:
//   - E_ prefix: error-level issues (blocking)
//   - W_ prefix: warning-level issues
//   - I_ prefix: info-level issues (non-blocking)
const (
	CodeRequiredField          = "E_REQUIRED_FIELD"
	CodeRangeViolation         = "E_RANGE_VIOLATION"
	CodeDateFuture             = "E_DATE_FUTURE"
	CodeDateTooEarly           = "E_DATE_TOO_EARLY"
	CodeUnitPriceInconsistency = "E_UNIT_PRICE_INCONSISTENCY"
	CodeGeometryWKTInvalid     = "W_GEOMETRY_WKT_INVALID"
	CodeGeometryPending        = "I_GEOMETRY_PENDING"
)

// ReasonableMinDate is the earliest acceptable transaction date.
// Transactions dated before 1990-01-01 are rejected as implausible.
var ReasonableMinDate = time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC)

// ValidationIssue represents a single validation finding.
type ValidationIssue struct {
	Level   string `json:"level"` // error, warning, or info
	Code    string `json:"code"`  // stable machine-readable code
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// ValidationError wraps a set of issues and implements the error interface.
// It is intended for callers that need an error value when one or more
// blocking (error-level) issues are present.
type ValidationError struct {
	Issues []ValidationIssue
}

func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "validation error: no issues"
	}
	parts := make([]string, 0, len(e.Issues))
	for _, iss := range e.Issues {
		parts = append(parts, fmt.Sprintf("[%s] %s: %s", iss.Level, iss.Field, iss.Message))
	}
	return "validation error: " + strings.Join(parts, "; ")
}

// Validator validates domain objects against business rules.
type Validator struct {
	// Clock provides the current time, enabling deterministic date validation
	// in tests. Defaults to clock.Default (time.Now) when nil.
	Clock clock.NowFunc
	// Level controls how thorough validation is. A zero value defaults to
	// LevelFull via New; LevelBasic performs only self-contained checks.
	Level ValidationLevel
}

// New creates a Validator with the given NowFunc. If c is nil, clock.Default
// (time.Now) is used. The level defaults to LevelFull.
func New(c clock.NowFunc) *Validator {
	if c == nil {
		c = clock.Default
	}
	return &Validator{Clock: c, Level: LevelFull}
}

// today returns the current time as seen by the validator.
func (v *Validator) today() time.Time {
	if v.Clock != nil {
		return v.Clock()
	}
	return time.Now()
}

// isFull reports whether the validator runs with full-level (supplementary)
// checks enabled.
func (v *Validator) isFull() bool {
	return v.Level >= LevelFull
}

// HasBlockingErrors reports whether any issue has severity >= error, i.e.
// whether validation produced errors that should block import.
func (v *Validator) HasBlockingErrors(issues []ValidationIssue) bool {
	for _, iss := range issues {
		if severityRank[iss.Level] >= severityRank[LevelError] {
			return true
		}
	}
	return false
}

// compareDates compares two times by calendar date only, returning -1/0/1.
// This avoids timezone and sub-day time-of-day mismatches between a parsed
// transaction date (midnight UTC) and the clock's current time.
func compareDates(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	switch {
	case ay < by:
		return -1
	case ay > by:
		return 1
	}
	switch {
	case am < bm:
		return -1
	case am > bm:
		return 1
	}
	switch {
	case ad < bd:
		return -1
	case ad > bd:
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Shared append helpers
// ---------------------------------------------------------------------------

// appendIssue appends an issue to the provided slice.
func appendIssue(issues *[]ValidationIssue, level, code, field, message string, value any) {
	*issues = append(*issues, ValidationIssue{
		Level: level, Code: code, Field: field, Message: message, Value: value,
	})
}

func appendRequired(issues *[]ValidationIssue, field, value string) {
	if strings.TrimSpace(value) == "" {
		appendIssue(issues, LevelError, CodeRequiredField, field,
			fmt.Sprintf("%s is required", field), value)
	}
}

func appendPositiveInt64(issues *[]ValidationIssue, field string, value int64) {
	if value <= 0 {
		appendIssue(issues, LevelError, CodeRangeViolation, field,
			fmt.Sprintf("%s must be greater than 0", field), value)
	}
}

func appendPositiveFloat64(issues *[]ValidationIssue, field string, value float64) {
	if value <= 0 {
		appendIssue(issues, LevelError, CodeRangeViolation, field,
			fmt.Sprintf("%s must be greater than 0", field), value)
	}
}

func appendNonNegativeFloat64(issues *[]ValidationIssue, field string, value float64) {
	if value < 0 {
		appendIssue(issues, LevelError, CodeRangeViolation, field,
			fmt.Sprintf("%s must be greater than or equal to 0", field), value)
	}
}

func appendNonNegativeInt64(issues *[]ValidationIssue, field string, value int64) {
	if value < 0 {
		appendIssue(issues, LevelError, CodeRangeViolation, field,
			fmt.Sprintf("%s must be greater than or equal to 0", field), value)
	}
}

func appendNonNegativeInt(issues *[]ValidationIssue, field string, value int) {
	if value < 0 {
		appendIssue(issues, LevelError, CodeRangeViolation, field,
			fmt.Sprintf("%s must be greater than or equal to 0", field), value)
	}
}

// Compile-time assertions that the domain types referenced by the public
// methods are wired to the expected domain package symbols.
var (
	_ *domain.Transaction
	_ *domain.Parcel
)
