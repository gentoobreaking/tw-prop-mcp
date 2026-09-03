package parser

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// helper to encode string to Big5 bytes
func toBig5(s string) []byte {
	enc := traditionalchinese.Big5.NewEncoder()
	b, _, err := transform.Bytes(enc, []byte(s))
	if err != nil {
		panic(err)
	}
	return b
}

func TestEncoding_Detect(t *testing.T) {
	// UTF-8 without BOM
	utf8Data := []byte("鄉鎮市區,交易標的")
	if got := DetectEncoding(utf8Data); got != "utf-8" {
		t.Fatalf("expected utf-8 got %q", got)
	}
	// UTF-8 BOM
	bomData := append([]byte{0xEF, 0xBB, 0xBF}, utf8Data...)
	if got := DetectEncoding(bomData); got != "utf-8-bom" {
		t.Fatalf("expected utf-8-bom got %q", got)
	}
	// Big5
	big5Data := toBig5("鄉鎮市區,交易標的")
	if got := DetectEncoding(big5Data); got != "big5" {
		t.Fatalf("expected big5 got %q", got)
	}
	// Empty should be utf-8 (empty is valid utf8)
	if got := DetectEncoding([]byte{}); got != "utf-8" {
		t.Fatalf("expected utf-8 for empty got %q", got)
	}
}

func TestEncoding_DecodeReader(t *testing.T) {
	// UTF-8-BOM should strip BOM
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...)
	r, enc, err := DecodeReader(bytes.NewReader(bom))
	if err != nil {
		t.Fatal(err)
	}
	if enc != "utf-8-bom" {
		t.Fatalf("expected utf-8-bom got %q", enc)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	if buf.String() != "hello" {
		t.Fatalf("expected hello got %q", buf.String())
	}

	// Big5 decode
	orig := "鄉鎮市區"
	big5 := toBig5(orig)
	r, enc, err = DecodeReader(bytes.NewReader(big5))
	if err != nil {
		t.Fatal(err)
	}
	if enc != "big5" {
		t.Fatalf("expected big5 got %q", enc)
	}
	buf.Reset()
	buf.ReadFrom(r)
	if buf.String() != orig {
		t.Fatalf("big5 decode: expected %q got %q", orig, buf.String())
	}

	// UTF-8 plain
	r, enc, err = DecodeReader(strings.NewReader("交易標的"))
	if err != nil {
		t.Fatal(err)
	}
	if enc != "utf-8" {
		t.Fatalf("expected utf-8 got %q", enc)
	}
	buf.Reset()
	buf.ReadFrom(r)
	if buf.String() != "交易標的" {
		t.Fatalf("expected 交易標的 got %q", buf.String())
	}
}

func TestFieldMap_Normalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"鄉鎮市區", "district"},
		{" 交易標的 ", "transaction_target"},
		{"\uFEFF鄉鎮市區", "district"},                 // BOM
		{"\u3000鄉鎮市區\u3000", "district"},          // ideographic space
		{"土地區段位置建物區段門牌", "section"},
		{"土地移轉總面積平方公尺", "land_area_sqm"},
		{"交易年月日", "transaction_date"},
		{"總價元", "total_price"},
		{"車位移轉總面積（平方公尺）", "parking_area_sqm"}, // full-width paren
		{"車位移轉總面積(平方公尺)", "parking_area_sqm"},
		{" 非都市土地使用編定 ", "land_use_category"},
		{"檔案名稱", "file_name"},
		{"未知欄位XYZ", "未知欄位XYZ"}, // fallback unchanged
	}
	for _, c := range cases {
		got := NormalizeHeader(c.in)
		if got != c.want {
			t.Fatalf("NormalizeHeader(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestParseROCDate(t *testing.T) {
	cases := []struct {
		in   string
		want string // ISO date
	}{
		{"110/05/20", "2021-05-20"},
		{"110.05.20", "2021-05-20"},
		{"110年05月20日", "2021-05-20"},
		{"2021-05-20", "2021-05-20"},
		{"2021/05/20", "2021-05-20"},
		{"2021.05.20", "2021-05-20"},
		{"111年01月01日", "2022-01-01"},
		{"2021-01-01", "2021-01-01"},
		{" 110/5/2 ", "2021-05-02"},
	}
	for _, c := range cases {
		got, err := ParseROCDate(c.in)
		if err != nil {
			t.Fatalf("ParseROCDate(%q) error: %v", c.in, err)
		}
		gotStr := got.Format("2006-01-02")
		if gotStr != c.want {
			t.Fatalf("ParseROCDate(%q) = %q want %q", c.in, gotStr, c.want)
		}
		// Also ensure UTC
		if got.Location() != time.UTC {
			t.Fatalf("expected UTC location for %q", c.in)
		}
	}
	// Invalid cases
	if _, err := ParseROCDate(""); err == nil {
		t.Fatal("expected error for empty date")
	}
	if _, err := ParseROCDate("abc"); err == nil {
		t.Fatal("expected error for abc")
	}
	if _, err := ParseROCDate("110/13/01"); err == nil {
		t.Fatal("expected error for month 13")
	}
}

func TestParsePrice(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1,234,567", 1234567},
		{" 1,234 ", 1234},
		{"1000000", 1000000},
		{" 2,500,000 ", 2500000},
		{"1,234.00", 1234},
	}
	for _, c := range cases {
		got, err := ParsePrice(c.in)
		if err != nil {
			t.Fatalf("ParsePrice(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("ParsePrice(%q)=%d want %d", c.in, got, c.want)
		}
	}
	if _, err := ParsePrice(""); err == nil {
		t.Fatal("expected error for empty price")
	}
	if _, err := ParsePrice("abc"); err == nil {
		t.Fatal("expected error for abc price")
	}
}

func TestParseArea(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"100.5", 100.5},
		{"1,234.56", 1234.56},
		{"  99.9 ", 99.9},
		{"0", 0},
	}
	for _, c := range cases {
		got, err := ParseArea(c.in)
		if err != nil {
			t.Fatalf("ParseArea(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("ParseArea(%q)=%v want %v", c.in, got, c.want)
		}
	}
	if _, err := ParseArea(""); err == nil {
		t.Fatal("expected error for empty area")
	}
}

func TestParseCSV_Sample(t *testing.T) {
	p := NewParser()
	csvData := "鄉鎮市區,交易標的,土地區段位置建物區段門牌,土地移轉總面積平方公尺,交易年月日,總價元\n" +
		"中正區,土地,台北市中正區重慶段,100.5,110/05/20,\"1,000,000\"\n" +
		"大安區,建物,台北市大安區大安段,200.3,111年01月01日,\"2,500,000\"\n"

	rows, err := p.ParseCSV(context.Background(), strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("ParseCSV error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows got %d", len(rows))
	}
	// Check first row mapping
	r0 := rows[0]
	if r0["district"] != "中正區" {
		t.Fatalf("r0 district expected 中正區 got %q", r0["district"])
	}
	if r0["transaction_target"] != "土地" {
		t.Fatalf("r0 transaction_target got %q", r0["transaction_target"])
	}
	if r0["section"] != "台北市中正區重慶段" {
		t.Fatalf("r0 section got %q", r0["section"])
	}
	if r0["land_area_sqm"] != "100.5" {
		t.Fatalf("r0 land_area_sqm got %q", r0["land_area_sqm"])
	}
	if r0["transaction_date"] != "110/05/20" {
		t.Fatalf("r0 transaction_date got %q", r0["transaction_date"])
	}
	if r0["total_price"] != "1,000,000" {
		t.Fatalf("r0 total_price got %q", r0["total_price"])
	}

	// Verify date and price conversion works on parsed values
	d, err := ParseROCDate(r0["transaction_date"])
	if err != nil {
		t.Fatal(err)
	}
	if d.Format("2006-01-02") != "2021-05-20" {
		t.Fatalf("r0 date parse got %q", d.Format("2006-01-02"))
	}
	price, err := ParsePrice(r0["total_price"])
	if err != nil {
		t.Fatal(err)
	}
	if price != 1000000 {
		t.Fatalf("price got %d want 1000000", price)
	}
	area, err := ParseArea(r0["land_area_sqm"])
	if err != nil {
		t.Fatal(err)
	}
	if area != 100.5 {
		t.Fatalf("area got %v", area)
	}

	r1 := rows[1]
	if r1["district"] != "大安區" {
		t.Fatalf("r1 district got %q", r1["district"])
	}
	if r1["transaction_date"] != "111年01月01日" {
		t.Fatalf("r1 transaction_date got %q", r1["transaction_date"])
	}
	d2, _ := ParseROCDate(r1["transaction_date"])
	if d2.Format("2006-01-02") != "2022-01-01" {
		t.Fatalf("r1 date got %q", d2.Format("2006-01-02"))
	}
}

func TestParseCSV_EmptyRowsAndBOM(t *testing.T) {
	p := NewParser()
	// BOM + empty line
	csvData := "\uFEFF鄉鎮市區,總價元\n" +
		"中正區,\"1,000\"\n" +
		"\n" + // empty line should be skipped
		"大安區,\"2,000\"\n"
	rows, err := p.ParseCSV(context.Background(), strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("ParseCSV error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows got %d", len(rows))
	}
	if rows[0]["district"] != "中正區" || rows[0]["total_price"] != "1,000" {
		t.Fatalf("row0 got %+v", rows[0])
	}
}

func TestParseCSV_Big5(t *testing.T) {
	p := NewParser()
	origCSV := "鄉鎮市區,總價元\n中正區,\"1,000,000\"\n大安區,\"2,000,000\"\n"
	big5 := toBig5(origCSV)
	rows, err := p.ParseCSV(context.Background(), bytes.NewReader(big5))
	if err != nil {
		t.Fatalf("ParseCSV big5 error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows got %d", len(rows))
	}
	if rows[0]["district"] != "中正區" {
		t.Fatalf("big5 district got %q", rows[0]["district"])
	}
	if rows[0]["total_price"] != "1,000,000" {
		t.Fatalf("big5 price got %q", rows[0]["total_price"])
	}
}

func TestParse_ManifestCSV(t *testing.T) {
	p := NewParser()
	manifest := "檔案名稱,資料筆數,下載時間\n" +
		"file1.csv,100,2021-05-20\n" +
		"file2.csv,200,2021-05-21\n"
	rows, err := p.ParseManifestCSV(context.Background(), strings.NewReader(manifest))
	if err != nil {
		t.Fatalf("ParseManifestCSV error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows got %d", len(rows))
	}
	if rows[0]["file_name"] != "file1.csv" {
		t.Fatalf("manifest file_name got %q", rows[0]["file_name"])
	}
	if rows[0]["record_count"] != "100" {
		t.Fatalf("manifest record_count got %q", rows[0]["record_count"])
	}
	if rows[0]["downloaded_at"] != "2021-05-20" {
		t.Fatalf("manifest downloaded_at got %q", rows[0]["downloaded_at"])
	}
}

func TestParse_ManifestCSV_EnglishHeader(t *testing.T) {
	p := NewParser()
	manifest := "file_name,record_count\nfile1.csv,10\n"
	rows, err := p.ParseManifestCSV(context.Background(), strings.NewReader(manifest))
	if err != nil {
		t.Fatalf("ParseManifestCSV error: %v", err)
	}
	if rows[0]["file_name"] != "file1.csv" {
		t.Fatalf("got %q", rows[0]["file_name"])
	}
}

func TestParser_CustomFieldMap(t *testing.T) {
	p := &Parser{FieldMap: map[string]string{"自定義欄位": "custom_field"}}
	rows, err := p.ParseCSV(context.Background(), strings.NewReader("自定義欄位\nvalue1\n"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if rows[0]["custom_field"] != "value1" {
		t.Fatalf("custom map failed got %+v", rows[0])
	}
}
