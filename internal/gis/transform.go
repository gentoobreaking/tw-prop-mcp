package gis

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// EPSG constants for coordinate reference systems.
const (
	EPSG4326 = 4326 // External API: WGS84
	EPSG3826 = 3826 // Internal storage: TWD97 / TM2 zone 121
)

// DBQuerier is the minimal database interface required by GIS operations.
// Both *pgx.Conn and pgx.Tx satisfy this interface, so the same code works
// inside a transaction or against a pooled connection.
type DBQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// stripSRID removes the "SRID=xxx;" prefix from an EWKT string, returning
// plain WKT suitable for ST_GeomFromText.  If the input has no SRID prefix
// it is returned trimmed of surrounding whitespace.
func stripSRID(wkt string) string {
	s := strings.TrimSpace(wkt)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, ";"); idx != -1 {
		prefix := s[:idx]
		if strings.HasPrefix(strings.ToUpper(prefix), "SRID=") {
			return strings.TrimSpace(s[idx+1:])
		}
	}
	return s
}

// TransformWKTToInternal transforms a WKT geometry in EPSG:4326 (WGS84) to
// EPSG:3826 (TWD97 / TM2 zone 121) EWKT using PostGIS.  The output includes
// the SRID prefix, e.g. "SRID=3826;MULTIPOLYGON(((…)))".
//
// All spatial computation is delegated to PostGIS (ST_Transform); no
// transformation is performed in Go memory.
func TransformWKTToInternal(ctx context.Context, db DBQuerier, wkt4326 string) (ewkt3826 string, err error) {
	if stripSRID(wkt4326) == "" {
		return "", fmt.Errorf("transform 4326→3826: empty WKT input")
	}
	const q = `SELECT ST_AsEWKT(ST_Transform(ST_SetSRID(ST_GeomFromText($1), 4326), 3826))`
	err = db.QueryRow(ctx, q, stripSRID(wkt4326)).Scan(&ewkt3826)
	if err != nil {
		return "", fmt.Errorf("transform 4326→3826: %w", err)
	}
	return ewkt3826, nil
}

// TransformWKTToExternal transforms an EWKT/WKT geometry in EPSG:3826
// (internal storage) to plain WKT in EPSG:4326 (WGS84) using PostGIS.
// Input may include an "SRID=3826;" prefix; it is stripped before processing.
func TransformWKTToExternal(ctx context.Context, db DBQuerier, wkt3826 string) (wkt4326 string, err error) {
	if stripSRID(wkt3826) == "" {
		return "", fmt.Errorf("transform 3826→4326: empty WKT input")
	}
	const q = `SELECT ST_AsText(ST_Transform(ST_SetSRID(ST_GeomFromText($1), 3826), 4326))`
	err = db.QueryRow(ctx, q, stripSRID(wkt3826)).Scan(&wkt4326)
	if err != nil {
		return "", fmt.Errorf("transform 3826→4326: %w", err)
	}
	return wkt4326, nil
}
