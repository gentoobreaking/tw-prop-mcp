package normalizer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/parser"
)

// PingToSqmFactor is 1 坪 = 3.305785 平方公尺.
const PingToSqmFactor = 3.305785

// Normalizer converts raw CSV rows (map[string]string) to domain objects.
// P2 Raw Immutable: never mutates input map, always creates new artifact.
type Normalizer struct{}

// New creates a Normalizer.
func New() *Normalizer { return &Normalizer{} }

// NormalizeTransaction converts a raw row to Transaction.
// snapshotID is assigned to SnapshotID. Four-key (county+district+section+land_number) is required.
func (n *Normalizer) NormalizeTransaction(row map[string]string, snapshotID string) (*domain.Transaction, error) {
	if row == nil {
		return nil, fmt.Errorf("nil row")
	}
	// Deep copy check: ensure we don't mutate input (read-only)
	// Validate four keys
	county := strings.TrimSpace(row["county"])
	district := strings.TrimSpace(row["district"])
	section := strings.TrimSpace(row["section"])
	landNumber := strings.TrimSpace(row["land_number"])
	if county == "" {
		return nil, fmt.Errorf("missing required field: county")
	}
	if district == "" {
		return nil, fmt.Errorf("missing required field: district")
	}
	if section == "" {
		return nil, fmt.Errorf("missing required field: section")
	}
	if landNumber == "" {
		return nil, fmt.Errorf("missing required field: land_number")
	}

	// Transaction date
	dateRaw := strings.TrimSpace(row["transaction_date"])
	if dateRaw == "" {
		// fallback keys
		if v := strings.TrimSpace(row["date"]); v != "" {
			dateRaw = v
		}
	}
	if dateRaw == "" {
		return nil, fmt.Errorf("missing required field: transaction_date")
	}
	tDate, err := parser.ParseROCDate(dateRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction_date %q: %w", dateRaw, err)
	}

	// Prices - required
	totalPriceRaw := strings.TrimSpace(row["total_price"])
	unitPriceRaw := strings.TrimSpace(row["unit_price"])
	if totalPriceRaw == "" {
		return nil, fmt.Errorf("missing required field: total_price")
	}
	if unitPriceRaw == "" {
		return nil, fmt.Errorf("missing required field: unit_price")
	}
	totalPrice, err := parser.ParsePrice(totalPriceRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid total_price %q: %w", totalPriceRaw, err)
	}
	unitPrice, err := parser.ParsePrice(unitPriceRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid unit_price %q: %w", unitPriceRaw, err)
	}

	// Areas - handle ping conversion
	landArea, err := resolveArea(row, []string{"land_area_sqm", "land_area"}, []string{"land_area_ping", "land_area_ping_sqm"})
	if err != nil {
		return nil, fmt.Errorf("invalid land_area: %w", err)
	}
	buildingArea, err := resolveArea(row, []string{"building_area_sqm", "building_area"}, []string{"building_area_ping"})
	if err != nil {
		return nil, fmt.Errorf("invalid building_area: %w", err)
	}
	parkingArea, err := resolveArea(row, []string{"parking_area_sqm", "parking_area"}, []string{"parking_area_ping"})
	if err != nil {
		return nil, fmt.Errorf("invalid parking_area: %w", err)
	}

	// Zoning normalization
	urbanZoning := normalizeUrbanZoning(strings.TrimSpace(row["urban_zoning"]))
	nonUrbanZoning := strings.TrimSpace(row["non_urban_zoning"])
	landUseCategory := normalizeLandUseCategory(strings.TrimSpace(row["land_use_category"]))

	// Building info
	buildingType := normalizeBuildingType(strings.TrimSpace(row["building_type"]))
	floor := strings.TrimSpace(row["floor"])
	age := 0
	if s := strings.TrimSpace(row["age"]); s != "" {
		parsed, err := parser.ParsePrice(s) // reuse integer parsing
		if err != nil {
			// try generic int parse after cleaning
			cleaned := strings.ReplaceAll(s, ",", "")
			cleaned = strings.TrimSpace(cleaned)
			// remove 年 suffix
			cleaned = strings.TrimSuffix(cleaned, "年")
			cleaned = strings.TrimSpace(cleaned)
			var pErr error
			parsed, pErr = parser.ParsePrice(cleaned)
			if pErr != nil {
				return nil, fmt.Errorf("invalid age %q: %w", s, pErr)
			}
		}
		age = int(parsed)
		if age < 0 {
			return nil, fmt.Errorf("invalid age %q: negative", s)
		}
	}

	parkingPrice := int64(0)
	if s := strings.TrimSpace(row["parking_price"]); s != "" {
		v, err := parser.ParsePrice(s)
		if err != nil {
			return nil, fmt.Errorf("invalid parking_price %q: %w", s, err)
		}
		parkingPrice = v
	}

	// Transaction ID
	transactionID := strings.TrimSpace(row["transaction_id"])
	if transactionID == "" {
		transactionID = strings.TrimSpace(row["serial"])
	}
	if transactionID == "" {
		transactionID = uuid.NewString()
	}

	// Transaction type / target
	transactionType := strings.TrimSpace(row["transaction_type"])
	// fallback: if transaction_target indicates
	transactionTarget := strings.TrimSpace(row["transaction_target"])
	if transactionType == "" && transactionTarget != "" {
		// keep as is, no auto-derive
	}

	// Source record hash
	sourceHash := strings.TrimSpace(row["source_record_hash"])
	if sourceHash == "" {
		sourceHash = hashRow(row)
	}

	// Snapshot ID from param or row
	sid := snapshotID
	if sid == "" {
		sid = strings.TrimSpace(row["snapshot_id"])
	}

	now := time.Now().UTC()
	tx := &domain.Transaction{
		ID:                uuid.NewString(),
		SnapshotID:        sid,
		TransactionID:     transactionID,
		TransactionDate:   tDate,
		TransactionType:   transactionType,
		County:            county,
		District:          district,
		Section:           section,
		LandNumber:        landNumber,
		TransactionTarget: transactionTarget,
		TotalPrice:        totalPrice,
		UnitPrice:         unitPrice,
		LandAreaSqm:       landArea,
		BuildingAreaSqm:   buildingArea,
		UrbanZoning:       urbanZoning,
		NonUrbanZoning:    nonUrbanZoning,
		LandUseCategory:   landUseCategory,
		BuildingType:      buildingType,
		Floor:             floor,
		Age:               age,
		ParkingAreaSqm:    parkingArea,
		ParkingPrice:      parkingPrice,
		SourceRecordHash:  sourceHash,
		CreatedAt:         now,
	}
	// ImportBatchID optional
	if v := strings.TrimSpace(row["import_batch_id"]); v != "" {
		tx.ImportBatchID = v
	}
	return tx, nil
}

// NormalizeParcel converts a raw row to Parcel.
// Geometry fields are left empty (to be filled by GIS import).
// Four-key is required. Area is unified to sqm (ping * 3.305785).
func (n *Normalizer) NormalizeParcel(row map[string]string) (*domain.Parcel, error) {
	if row == nil {
		return nil, fmt.Errorf("nil row")
	}
	county := strings.TrimSpace(row["county"])
	district := strings.TrimSpace(row["district"])
	section := strings.TrimSpace(row["section"])
	landNumber := strings.TrimSpace(row["land_number"])
	if county == "" {
		return nil, fmt.Errorf("missing required field: county")
	}
	if district == "" {
		return nil, fmt.Errorf("missing required field: district")
	}
	if section == "" {
		return nil, fmt.Errorf("missing required field: section")
	}
	if landNumber == "" {
		return nil, fmt.Errorf("missing required field: land_number")
	}

	area, err := resolveArea(row, []string{"area_sqm", "area", "land_area_sqm"}, []string{"area_ping", "land_area_ping", "area_ping_sqm"})
	if err != nil {
		return nil, fmt.Errorf("invalid area: %w", err)
	}
	// Fallback: try generic area key with possible unit suffix
	if area == 0 {
		// try any area-related key with 坪 value
		for k, v := range row {
			if strings.Contains(k, "area") && strings.TrimSpace(v) != "" {
				// if value contains 坪, parse as ping
				if strings.Contains(v, "坪") {
					cleaned := strings.ReplaceAll(v, "坪", "")
					cleaned = strings.TrimSpace(cleaned)
					f, e := parser.ParseArea(cleaned)
					if e == nil {
						area = f * PingToSqmFactor
						break
					}
				}
			}
		}
	}
	if area <= 0 {
		return nil, fmt.Errorf("missing or invalid area_sqm")
	}

	urbanZoning := normalizeUrbanZoning(strings.TrimSpace(row["urban_zoning"]))
	landUseCategory := normalizeLandUseCategory(strings.TrimSpace(row["land_use_category"]))
	source := strings.TrimSpace(row["source"])
	if source == "" {
		source = "MOI"
	}
	sourceVersion := strings.TrimSpace(row["source_version"])
	if sourceVersion == "" {
		sourceVersion = strings.TrimSpace(row["version"])
	}
	if sourceVersion == "" {
		sourceVersion = "unknown"
	}

	now := time.Now().UTC()
	p := &domain.Parcel{
		ID:              uuid.NewString(),
		County:          county,
		District:        district,
		Section:         section,
		LandNumber:      landNumber,
		AreaSqm:         area,
		UrbanZoning:     urbanZoning,
		LandUseCategory: landUseCategory,
		Geometry:        "", // to be filled by GIS import
		Centroid:        "",
		Source:          source,
		SourceVersion:   sourceVersion,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if v := strings.TrimSpace(row["import_batch_id"]); v != "" {
		p.ImportBatchID = v
	}
	if v := strings.TrimSpace(row["geometry"]); v != "" {
		p.Geometry = v
	}
	if v := strings.TrimSpace(row["centroid"]); v != "" {
		p.Centroid = v
	}
	return p, nil
}

// helpers

func resolveArea(row map[string]string, sqmKeys, pingKeys []string) (float64, error) {
	// Check sqm keys first
	for _, k := range sqmKeys {
		if v, ok := row[k]; ok {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			// If value contains 坪 suffix, treat as ping
			if strings.Contains(v, "坪") {
				cleaned := strings.ReplaceAll(v, "坪", "")
				cleaned = strings.TrimSpace(cleaned)
				f, err := parser.ParseArea(cleaned)
				if err != nil {
					return 0, err
				}
				return f * PingToSqmFactor, nil
			}
			// Check unit hint field
			if isPingUnit(row) {
				f, err := parser.ParseArea(v)
				if err != nil {
					return 0, err
				}
				return f * PingToSqmFactor, nil
			}
			f, err := parser.ParseArea(v)
			if err != nil {
				return 0, err
			}
			return f, nil
		}
	}
	// Check ping keys
	for _, k := range pingKeys {
		if v, ok := row[k]; ok {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			v = strings.ReplaceAll(v, "坪", "")
			v = strings.TrimSpace(v)
			f, err := parser.ParseArea(v)
			if err != nil {
				return 0, err
			}
			return f * PingToSqmFactor, nil
		}
	}
	// No area field present -> 0, no error (optional for transaction)
	return 0, nil
}

func isPingUnit(row map[string]string) bool {
	for _, k := range []string{"area_unit", "unit", "area_unit_sqm"} {
		if v, ok := row[k]; ok && strings.Contains(strings.TrimSpace(v), "坪") {
			return true
		}
	}
	return false
}

func normalizeUrbanZoning(s string) string {
	if s == "" {
		return ""
	}
	// Trim and handle short codes
	m := map[string]string{
		"住":  "住宅區",
		"商":  "商業區",
		"工":  "工業區",
		"農":  "農業區",
		"住商": "住商區",
		"住宅": "住宅區",
		"商業": "商業區",
		"工業": "工業區",
	}
	if v, ok := m[s]; ok {
		return v
	}
	// Already full form or unknown -> return as is
	return s
}

func normalizeLandUseCategory(s string) string {
	if s == "" {
		return ""
	}
	m := map[string]string{
		"住": "住宅區",
		"甲建": "甲種建築用地",
		"乙建": "乙種建築用地",
		"丙建": "丙種建築用地",
		"丁建": "丁種建築用地",
	}
	if v, ok := m[s]; ok {
		return v
	}
	return s
}

func normalizeBuildingType(s string) string {
	return strings.TrimSpace(s)
}

func hashRow(row map[string]string) string {
	keys := make([]string, 0, len(row))
	for k := range row {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(row[k]))
		h.Write([]byte(";"))
	}
	return hex.EncodeToString(h.Sum(nil))
}
