package parser

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Parser holds optional custom field mapping.
// If FieldMap is nil, the default FieldMapCNToEN is used.
type Parser struct {
	FieldMap map[string]string
}

// NewParser creates a Parser with default field map.
func NewParser() *Parser {
	return &Parser{FieldMap: FieldMapCNToEN}
}

// normalizeHeader uses the parser's FieldMap if provided, otherwise the global one.
// It delegates to NormalizeHeader but respects custom map.
func (p *Parser) normalizeHeader(h string) string {
	// First get cleaned key via same cleaning logic
	// We replicate cleaning then lookup in custom map to allow custom overrides
	cleaned := cleanHeader(h)
	if p != nil && p.FieldMap != nil {
		if v, ok := p.FieldMap[cleaned]; ok {
			return v
		}
		lower := strings.ToLower(cleaned)
		if v, ok := p.FieldMap[lower]; ok {
			return v
		}
		return cleaned
	}
	// fallback to global NormalizeHeader logic (which already maps via FieldMapCNToEN)
	return NormalizeHeader(h)
}

// cleanHeader is the shared cleaning step without map lookup.
func cleanHeader(header string) string {
	s := strings.TrimSpace(header)
	s = strings.TrimPrefix(s, "\uFEFF")
	s = strings.TrimPrefix(s, string([]byte{0xEF, 0xBB, 0xBF}))
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\u3000", "")
	s = strings.ReplaceAll(s, "（", "(")
	s = strings.ReplaceAll(s, "）", ")")
	s = strings.ReplaceAll(s, "，", ",")
	s = strings.ReplaceAll(s, "：", ":")
	s = strings.ReplaceAll(s, "　", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// ParseCSV reads CSV from r, auto-detects encoding, normalizes headers to English codes,
// and returns each row as map[englishCode]value. Empty rows are skipped.
// Line numbers in errors are 1-indexed (header is line 1).
func (p *Parser) ParseCSV(ctx context.Context, r io.Reader) ([]map[string]string, error) {
	decoded, _, err := DecodeReader(r)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	cr := csv.NewReader(decoded)
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = false

	// Read header
	headers, err := cr.Read()
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, fmt.Errorf("read header: %w", err)
	}

	// Normalize headers
	normHeaders := make([]string, len(headers))
	for i, h := range headers {
		normHeaders[i] = p.normalizeHeader(h)
	}

	var out []map[string]string
	lineNum := 1 // header is line 1
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		record, err := cr.Read()
		lineNum++
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv parse error at line %d: %w", lineNum, err)
		}
		// Skip empty rows (all fields empty after trimming)
		isEmpty := true
		for _, f := range record {
			if strings.TrimSpace(f) != "" {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			continue
		}
		m := make(map[string]string, len(normHeaders))
		for i, h := range normHeaders {
			var v string
			if i < len(record) {
				v = strings.TrimSpace(record[i])
			} else {
				v = ""
			}
			m[h] = v
		}
		// If record has more fields than headers, store extras as extra_<n>
		if len(record) > len(normHeaders) {
			for i := len(normHeaders); i < len(record); i++ {
				k := fmt.Sprintf("extra_%d", i-len(normHeaders)+1)
				m[k] = strings.TrimSpace(record[i])
			}
		}
		out = append(out, m)
	}
	return out, nil
}

// ParseOfficialCSV is a convenience wrapper that opens a file path and parses it.
func (p *Parser) ParseOfficialCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return p.ParseCSV(context.Background(), f)
}

// ParseManifestCSV parses MANIFEST.CSV with same logic but explicitly handles manifest headers.
// Currently delegates to ParseCSV (manifest headers are also in field map).
func (p *Parser) ParseManifestCSV(ctx context.Context, r io.Reader) ([]map[string]string, error) {
	return p.ParseCSV(ctx, r)
}

// ParseOfficialManifestCSV opens a manifest file path and parses it.
func (p *Parser) ParseOfficialManifestCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return p.ParseManifestCSV(context.Background(), f)
}

// ParseROCDate parses a date string that may be in ROC era or Gregorian.
// Supported forms:
//   - 110/05/20  -> 2021-05-20 (ROC year = year + 1911)
//   - 110.05.20
//   - 110年05月20日
//   - 2021-05-20 (Gregorian)
//   - 2021/05/20, 2021.05.20
// Returns time.Time at 00:00:00 UTC.
func ParseROCDate(s string) (time.Time, error) {
	orig := s
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\uFEFF")
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date string: %q", orig)
	}

	// ROC with 年月日
	if strings.Contains(s, "年") {
		// Use regex to extract numbers
		re := regexp.MustCompile(`(\d+)\s*年\s*(\d+)\s*月\s*(\d+)\s*日?`)
		m := re.FindStringSubmatch(s)
		if len(m) != 4 {
			return time.Time{}, fmt.Errorf("invalid ROC date %q", orig)
		}
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		y = rocToAD(y)
		return validateDate(y, mo, d, orig)
	}

	// Normalize separators to "/"
	// Handle -, ., /
	var sep string
	switch {
	case strings.Contains(s, "/"):
		sep = "/"
	case strings.Contains(s, "."):
		sep = "."
	case strings.Contains(s, "-"):
		sep = "-"
	default:
		return time.Time{}, fmt.Errorf("unknown date format %q", orig)
	}
	parts := strings.Split(s, sep)
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid date %q", orig)
	}
	y, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	mo, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	d, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err1 != nil || err2 != nil || err3 != nil {
		return time.Time{}, fmt.Errorf("invalid date %q", orig)
	}
	y = rocToAD(y)
	return validateDate(y, mo, d, orig)
}

func rocToAD(y int) int {
	// Heuristic: if year < 1000, treat as ROC year (1911 + y). Otherwise Gregorian.
	// 1911 is year 0 in ROC (1912 = year 1). So AD = 1911 + ROC.
	if y < 1000 {
		return y + 1911
	}
	return y
}

func validateDate(y, m, d int, orig string) (time.Time, error) {
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, fmt.Errorf("invalid date %q", orig)
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	// Check that normalization didn't overflow (e.g., Feb 30 -> Mar 2)
	if int(t.Month()) != m || t.Day() != d {
		return time.Time{}, fmt.Errorf("invalid date %q", orig)
	}
	return t, nil
}

// ParsePrice parses a price string removing commas and whitespace.
func ParsePrice(s string) (int64, error) {
	cleaned := strings.ReplaceAll(s, ",", "")
	cleaned = strings.ReplaceAll(cleaned, "\u3000", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "\t", "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return 0, fmt.Errorf("empty price")
	}
	// Remove full-width comma
	cleaned = strings.ReplaceAll(cleaned, "，", "")
	// Handle possible decimal? Official price should be integer; truncate.
	if strings.Contains(cleaned, ".") {
		f, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid price %q: %w", s, err)
		}
		return int64(f), nil
	}
	v, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price %q: %w", s, err)
	}
	return v, nil
}

// ParseArea parses an area string removing commas and whitespace, returning float64.
func ParseArea(s string) (float64, error) {
	cleaned := strings.ReplaceAll(s, ",", "")
	cleaned = strings.ReplaceAll(cleaned, "\u3000", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "\t", "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return 0, fmt.Errorf("empty area")
	}
	cleaned = strings.ReplaceAll(cleaned, "，", "")
	v, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid area %q: %w", s, err)
	}
	return v, nil
}
