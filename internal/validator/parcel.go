package validator

import (
	"strings"

	"tw-prop-mcp/internal/domain"
)

// ValidateParcel validates a *domain.Parcel against business rules.
// It checks required fields, area range, and (in full mode) performs a
// lightweight WKT pre-check on the geometry. Issues are returned in field
// order; blocking (error-level) issues indicate the record must not be
// imported.
//
// Validation rules (see DATA_MODEL.md "parcel" table, point 4 of T007):
//
//   - (a) Required fields: county, district, section, land_number.
//   - (b) Numeric range: area_sqm > 0.
//   - (c) urban_zoning / land_use_category are optional (supplied by GIS
//     import); they are not flagged when empty.
//   - (d) Geometry WKT pre-check (full mode only): when Geometry is present it
//     must look like valid WKT/EWKT. Uniqueness of the four-key
//     (county, district, section, land_number) is enforced by the repository
//     UNIQUE index, not here.
//
// The input Parcel is treated as read-only and is never mutated.
func (v *Validator) ValidateParcel(p *domain.Parcel) []ValidationIssue {
	var issues []ValidationIssue
	if p == nil {
		appendIssue(&issues, LevelError, CodeRequiredField, "parcel",
			"parcel must not be nil", nil)
		return issues
	}

	// (a) Required fields. Per DATA_MODEL.md these four form the parcel identity;
	// land_number must be validated alongside the others (not in isolation).
	appendRequired(&issues, "county", p.County)
	appendRequired(&issues, "district", p.District)
	appendRequired(&issues, "section", p.Section)
	appendRequired(&issues, "land_number", p.LandNumber)

	// (b) Area.
	appendPositiveFloat64(&issues, "area_sqm", p.AreaSqm)

	// (c) urban_zoning / land_use_category: optional, no check.

	// (d) Geometry WKT pre-check (full mode only). Empty geometry is expected
	// for parcels not yet processed by GIS import; flag it as non-blocking info.
	if v.isFull() {
		switch strings.TrimSpace(p.Geometry) {
		case "":
			appendIssue(&issues, LevelInfo, CodeGeometryPending, "geometry",
				"geometry is empty; expected to be populated by GIS import", nil)
		default:
			if !looksLikeWKT(p.Geometry) {
				appendIssue(&issues, LevelWarning, CodeGeometryWKTInvalid, "geometry",
					"geometry is present but does not look like valid WKT/EWKT", p.Geometry)
			}
		}
	}

	return issues
}

// wktTypes are the WKT geometry type keywords recognised by the pre-check.
var wktTypes = []string{
	"POINT", "LINESTRING", "POLYGON",
	"MULTIPOINT", "MULTILINESTRING", "MULTIPOLYGON",
	"GEOMETRYCOLLECTION",
}

// looksLikeWKT performs a lightweight structural pre-check of a WKT/EWKT
// string. It is intentionally permissive: it only rejects values that are
// clearly not WKT, so that a malformed GIS import is flagged as a warning
// rather than a hard failure. Valid examples:
//
//	"MULTIPOLYGON(((0 0, 1 0, 1 1, 0 1, 0 0)))"
//	"SRID=3826;MULTIPOLYGON(((...)))"
//	"POINT(121.5 25.0)"
func looksLikeWKT(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	// Strip an optional "SRID=xxxx;" EWKT prefix.
	if strings.HasPrefix(strings.ToUpper(s), "SRID=") {
		idx := strings.Index(s, ";")
		if idx < 0 {
			return false
		}
		s = strings.TrimSpace(s[idx+1:])
	}
	upper := strings.ToUpper(s)
	for _, wt := range wktTypes {
		prefix := wt + "("
		if strings.HasPrefix(upper, prefix) {
			return strings.HasSuffix(upper, ")")
		}
	}
	return false
}
