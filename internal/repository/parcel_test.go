package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
)

// mockRow implements pgx.Row
type mockRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	return m.scanFunc(dest...)
}

// mockRows implements pgx.Rows for Query expectations
type mockRows struct {
	rows [][]any
	idx  int
	err  error
	// scan template: called per Next
}

func (m *mockRows) Close() {}
func (m *mockRows) Err() error { return m.err }
func (m *mockRows) CommandTag() pgconn.CommandTag { return pgconn.NewCommandTag("SELECT") }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) Next() bool {
	if m.idx < len(m.rows) {
		return true
	}
	return false
}
func (m *mockRows) Scan(dest ...any) error {
	if m.idx >= len(m.rows) {
		return errors.New("no more rows")
	}
	row := m.rows[m.idx]
	m.idx++
	if len(row) != len(dest) {
		// allow partial: if dest expects more, fail
		return errors.New("column count mismatch")
	}
	for i, v := range row {
		switch d := dest[i].(type) {
		case *pgtype.UUID:
			if vv, ok := v.(pgtype.UUID); ok {
				*d = vv
			} else if v == nil {
				*d = pgtype.UUID{Valid: false}
			}
		case *string:
			if vv, ok := v.(string); ok {
				*d = vv
			} else if vv, ok := v.(*string); ok && vv != nil {
				*d = *vv
			}
		case **string:
			// dest is **string (nullable *string)
			if v == nil {
				*d = nil
			} else if vv, ok := v.(string); ok {
				s := vv
				*d = &s
			} else if vv, ok := v.(*string); ok {
				*d = vv
			}
		case *pgtype.Numeric:
			if vv, ok := v.(pgtype.Numeric); ok {
				*d = vv
			}
		case *pgtype.Text:
			if vv, ok := v.(pgtype.Text); ok {
				*d = vv
			}
		case *pgtype.Timestamptz:
			if vv, ok := v.(pgtype.Timestamptz); ok {
				*d = vv
			}
		default:
			// generic fallback via string
			if s, ok := v.(string); ok {
				switch d2 := dest[i].(type) {
				case *string:
					*d2 = s
				}
			}
		}
	}
	return nil
}
func (m *mockRows) Values() ([]any, error) { return nil, nil }
func (m *mockRows) RawValues() [][]byte { return nil }
func (m *mockRows) Conn() *pgx.Conn { return nil }

// mockDBTX implements DBTX with function hooks
type mockDBTX struct {
	queryRowFn  func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn     func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFn      func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	copyFromFn  func(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
}
func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}
func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}
func (m *mockDBTX) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	if m.copyFromFn != nil {
		return m.copyFromFn(ctx, tableName, columnNames, rowSrc)
	}
	return 0, nil
}

func TestParcelRepositoryInterface(t *testing.T) {
	var _ ParcelRepository = (*parcelRepository)(nil)
}

func TestParcelModelValidationViaRepo(t *testing.T) {
	// Validate domain model required fields via repo Create validation
	repo := NewParcelRepository(&mockDBTX{})
	_, err := repo.GetByID(context.Background(), "not-a-uuid")
	if err == nil {
		t.Fatalf("expected error for invalid uuid")
	}
	// Search requires county/district
	_, err = repo.Search(context.Background(), ParcelFilter{County: "", District: "中山區"})
	if err == nil {
		t.Fatalf("expected error for missing county")
	}
	_, err = repo.Search(context.Background(), ParcelFilter{County: "台北市", District: ""})
	if err == nil {
		t.Fatalf("expected error for missing district")
	}
}

func TestParcelRepository_Create_Success(t *testing.T) {
	called := false
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if !strings.Contains(sql, "INSERT INTO parcel") {
				t.Fatalf("unexpected sql: %s", sql)
			}
			called = true
			// Verify args binding: county, district, section, land_number at positions
			// args[1]=county, args[2]=district, args[3]=section, args[4]=landNumber
			if len(args) < 14 {
				t.Fatalf("expected 14 args, got %d", len(args))
			}
			if args[1] != "台北市" {
				t.Fatalf("county arg mismatch: %v", args[1])
			}
			if args[2] != "中山區" {
				t.Fatalf("district arg mismatch: %v", args[2])
			}
			if args[8] == "" {
				t.Fatalf("geometry WKT should not be empty")
			}
			geom, ok := args[8].(string)
			if !ok || geom == "" {
				t.Fatalf("geometry arg not string or empty")
			}
			if !strings.Contains(geom, "MULTIPOLYGON") {
				t.Fatalf("geometry should contain MULTIPOLYGON, got %q", geom)
			}
			return &mockRow{
				scanFunc: func(dest ...any) error {
					if len(dest) != 19 {
						t.Fatalf("Create RETURNING expected 19 cols, got %d", len(dest))
					}
					// Fill dest with plausible values
					for i, d := range dest {
						switch v := d.(type) {
						case *pgtype.UUID:
							var u pgtype.UUID
							_ = u.Scan("11111111-1111-1111-1111-111111111111")
							*v = u
						case *string:
							switch i {
							case 1:
								*v = "台北市"
							case 2:
								*v = "中山區"
							case 3:
								*v = "中山段二小段"
							case 4:
								*v = "00120000"
							case 8:
								*v = "MULTIPOLYGON(((121.5 25.0,121.6 25.0,121.6 25.1,121.5 25.1,121.5 25.0)))"
							case 11:
								*v = "NLSC"
							case 12:
								*v = "2024Q1"
							default:
								*v = "mock"
							}
						case **string:
							s := "POINT(121.55 25.05)"
							if i == 9 || i == 10 {
								*v = &s
							} else if i == 16 || i == 17 || i == 18 {
								// 4326 cols
								ss := "POINT(121.55 25.05)"
								if i == 16 {
									ss = "MULTIPOLYGON(((121.5 25.0,121.6 25.0,121.6 25.1,121.5 25.1,121.5 25.0)))"
								}
								*v = &ss
							} else {
								*v = &s
							}
						case *pgtype.Numeric:
							_ = v.Scan("123.45")
						case *pgtype.Text:
							*v = pgtype.Text{String: "住", Valid: true}
							if i == 7 {
								v.String = "住宅"
							}
						case *pgtype.Timestamptz:
							*v = pgtype.Timestamptz{Time: pgtype.Timestamptz{}.Time, Valid: true}
						}
					}
					return nil
				},
			}
		},
	}
	repo := NewParcelRepository(mock)
	p := &domain.Parcel{
		County:          "台北市",
		District:        "中山區",
		Section:         "中山段二小段",
		LandNumber:      "00120000",
		AreaSqm:         123.45,
		UrbanZoning:     "住",
		LandUseCategory: "住宅",
		Geometry:        "MULTIPOLYGON(((121.5 25.0,121.6 25.0,121.6 25.1,121.5 25.1,121.5 25.0)))",
		Source:          "NLSC",
		SourceVersion:   "2024Q1",
	}
	if err := repo.Create(context.Background(), p); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !called {
		t.Fatalf("QueryRow not called")
	}
	if p.ID == "" {
		t.Fatalf("expected ID populated after Create")
	}
	if p.Geometry4326 == "" {
		t.Fatalf("expected Geometry4326 populated")
	}
	if p.Centroid != "" {
		// centroid computed via PostGIS ST_Centroid fallback should be set
		if !strings.Contains(p.Centroid, "POINT") {
			t.Fatalf("centroid should be POINT WKT, got %q", p.Centroid)
		}
	}
}

func TestParcelRepository_Create_UniqueViolation(t *testing.T) {
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					return &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint \"parcel_county_district_section_land_number_source_source_version_key\""}
				},
			}
		},
	}
	repo := NewParcelRepository(mock)
	p := &domain.Parcel{
		County:        "台北市",
		District:      "中山區",
		Section:       "中山段二小段",
		LandNumber:    "00120000",
		AreaSqm:       100,
		Geometry:      "MULTIPOLYGON(((0 0,1 0,1 1,0 1,0 0)))",
		Source:        "NLSC",
		SourceVersion: "2024Q1",
	}
	err := repo.Create(context.Background(), p)
	if !errors.Is(err, ErrParcelExists) {
		t.Fatalf("expected ErrParcelExists, got %v", err)
	}
}

func TestParcelRepository_GetByID_NotFound(t *testing.T) {
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "ST_Transform") {
				// 4326 fetch after not found should not be called; but if called, return not found
				return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			}
			if strings.Contains(sql, "FROM parcel WHERE id") {
				return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			}
			return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	repo := NewParcelRepository(mock)
	_, err := repo.GetByID(context.Background(), "11111111-1111-1111-1111-111111111111")
	if !errors.Is(err, ErrParcelNotFound) {
		t.Fatalf("expected ErrParcelNotFound, got %v", err)
	}
}

func TestParcelRepository_GetByLandNumber_Binding(t *testing.T) {
	called := false
	mock := &mockDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "FROM parcel WHERE county") && strings.Contains(sql, "land_number") && !strings.Contains(sql, "ST_Transform") {
				called = true
				if len(args) != 4 {
					t.Fatalf("GetParcelByLandNumber expected 4 args, got %d", len(args))
				}
				if args[0] != "台北市" || args[1] != "中山區" || args[2] != "中山段二小段" || args[3] != "00120000" {
					t.Fatalf("land number args mismatch: %v", args)
				}
				// Return a row for sqlc's Scan
				return &mockRow{
					scanFunc: func(dest ...any) error {
						// db.Parcel has 15 fields: ID, County, District, Section, LandNumber, AreaSqm, UrbanZoning, LandUseCategory, Geometry, Centroid, Bbox, Source, SourceVersion, ImportBatchID, CreatedAt, UpdatedAt
						// But parcel.sql.go GetParcelByLandNumber expects 15 columns? Actually 15? Check mapping: ID, County, District, Section, LandNumber, AreaSqm, UrbanZoning, LandUseCategory, Geometry, Centroid, Bbox, Source, SourceVersion, ImportBatchID, CreatedAt, UpdatedAt = 16
						// We fill each dest
						for i, d := range dest {
							switch v := d.(type) {
							case *pgtype.UUID:
								var u pgtype.UUID
								_ = u.Scan("11111111-1111-1111-1111-111111111111")
								*v = u
							case *string:
								switch i {
								case 1:
									*v = "台北市"
								case 2:
									*v = "中山區"
								case 3:
									*v = "中山段二小段"
								case 4:
									*v = "00120000"
								case 8:
									*v = "MULTIPOLYGON(((0 0,1 0,1 1,0 1,0 0)))"
								case 11:
									*v = "NLSC"
								case 12:
									*v = "2024Q1"
								default:
									*v = "x"
								}
							case *pgtype.Numeric:
								_ = v.Scan("100")
							case *pgtype.Text:
								*v = pgtype.Text{String: "住", Valid: true}
							case *interface{}:
								*v = "POINT(0 0)"
							case *pgtype.Timestamptz:
								*v = pgtype.Timestamptz{Valid: true}
							default:
								// handle generic interface{}
								if s, ok := d.(*interface{}); ok {
									*s = "POINT(0 0)"
								}
							}
						}
						return nil
					},
				}
			}
			if strings.Contains(sql, "ST_Transform") {
				return &mockRow{
					scanFunc: func(dest ...any) error {
						for i, d := range dest {
							if pp, ok := d.(**string); ok {
								s := "POINT(121.5 25.0)"
								if i == 0 {
									s = "MULTIPOLYGON(((121.5 25.0,121.6 25.0,121.6 25.1,121.5 25.1,121.5 25.0)))"
								}
								*pp = &s
							}
						}
						return nil
					},
				}
			}
			return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	repo := NewParcelRepository(mock)
	p, err := repo.GetByLandNumber(context.Background(), "台北市", "中山區", "中山段二小段", "00120000")
	if err != nil {
		t.Fatalf("GetByLandNumber failed: %v", err)
	}
	if !called {
		t.Fatalf("expected QueryRow for GetParcelByLandNumber to be called")
	}
	if p.County != "台北市" || p.LandNumber != "00120000" {
		t.Fatalf("returned parcel fields mismatch")
	}
	if p.Geometry4326 == "" {
		t.Fatalf("expected Geometry4326 populated")
	}
}

func TestParcelRepository_Search_Binding(t *testing.T) {
	// Search with section and area filters should delegate to sqlc SearchParcels path
	// We mock Query to handle custom search when needed, but section non-empty goes via sqlc path which uses Query.
	// sqlc SearchParcels uses Query with specific SQL containing "ORDER BY section, land_number"
	section := "中山段二小段"
	min := 100.0
	max := 200.0
	mock := &mockDBTX{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			// This path is used by sqlc SearchParcels (Query) when section non-empty
			if !strings.Contains(sql, "FROM parcel") {
				t.Fatalf("unexpected search sql: %s", sql)
			}
			if !strings.Contains(sql, "ORDER BY section") {
				t.Fatalf("search sql should order by section")
			}
			// args: County, District, Column3(section), Column4(min), Column5(max), Limit, Offset
			if args[0] != "台北市" || args[1] != "中山區" {
				t.Fatalf("search county/district mismatch: %v", args[:2])
			}
			return &mockRows{rows: [][]any{}}, nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "ST_Transform") {
				return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			}
			return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	repo := NewParcelRepository(mock)
	filter := ParcelFilter{
		County:   "台北市",
		District: "中山區",
		Section:  &section,
		MinArea:  &min,
		MaxArea:  &max,
		Limit:    10,
		Offset:   0,
	}
	_, err := repo.Search(context.Background(), filter)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Also test custom search path (no section, no area filters) uses Query with custom sql
	mock2 := &mockDBTX{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if !strings.Contains(sql, "WHERE county=$1 AND district=$2") {
				t.Fatalf("custom search sql missing county/district filter: %s", sql)
			}
			return &mockRows{rows: [][]any{}}, nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	repo2 := NewParcelRepository(mock2)
	_, err = repo2.Search(context.Background(), ParcelFilter{County: "台北市", District: "中山區", Limit: 5})
	if err != nil {
		t.Fatalf("custom Search failed: %v", err)
	}
}
