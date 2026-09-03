package normalizer

import (
	"math"
	"testing"
)

func TestNormalizer_Transaction_Success(t *testing.T) {
	n := New()
	snapshotID := "snap-001"

	row1 := map[string]string{
		"county":             "台北市",
		"district":           "中正區",
		"section":            "重慶段一小段",
		"land_number":        "123",
		"transaction_date":   "110/05/20",
		"total_price":        "1,000,000",
		"unit_price":         "9980",
		"land_area_sqm":      "100.5",
		"building_area_sqm":  "0",
		"urban_zoning":       "住",
		"land_use_category":  "甲種建築用地",
		"building_type":      "住宅大樓",
		"transaction_id":     "TX001",
		"transaction_target": "土地",
	}
	row2 := map[string]string{
		"county":             "台北市",
		"district":           "大安區",
		"section":            "大安段二小段",
		"land_number":        "456",
		"transaction_date":   "111年01月01日",
		"total_price":        "2,500,000",
		"unit_price":         "12500",
		"building_area_sqm":  "120.5",
		"land_area_sqm":      "200.3",
		"urban_zoning":       "商",
		"land_use_category":  "乙種建築用地",
		"building_type":      "華廈",
		"floor":              "3",
		"age":                "10",
		"parking_area_sqm":   "10",
		"parking_price":      "100000",
		"transaction_id":     "TX002",
	}

	tx1, err := n.NormalizeTransaction(row1, snapshotID)
	if err != nil {
		t.Fatalf("row1 failed: %v", err)
	}
	if tx1.County != "台北市" || tx1.District != "中正區" {
		t.Fatalf("location mismatch: %+v", tx1)
	}
	if tx1.UrbanZoning != "住宅區" {
		t.Fatalf("urban zoning normalize failed: got %q want 住宅區", tx1.UrbanZoning)
	}
	if tx1.TotalPrice != 1000000 || tx1.UnitPrice != 9980 {
		t.Fatalf("price mismatch: %d %d", tx1.TotalPrice, tx1.UnitPrice)
	}
	if math.Abs(tx1.LandAreaSqm-100.5) > 0.001 {
		t.Fatalf("land area mismatch: %f", tx1.LandAreaSqm)
	}
	if tx1.SnapshotID != snapshotID {
		t.Fatalf("snapshot id mismatch")
	}
	// PricePerPing
	expectedPing := float64(9980) * PingToSqmFactor
	if math.Abs(tx1.PricePerPing()-expectedPing) > 0.01 {
		t.Fatalf("PricePerPing mismatch: got %f want %f", tx1.PricePerPing(), expectedPing)
	}

	tx2, err := n.NormalizeTransaction(row2, snapshotID)
	if err != nil {
		t.Fatalf("row2 failed: %v", err)
	}
	if tx2.BuildingType != "華廈" || tx2.Floor != "3" || tx2.Age != 10 {
		t.Fatalf("building info mismatch: %+v", tx2)
	}
	if tx2.UrbanZoning != "商業區" {
		t.Fatalf("urban zoning row2: got %q", tx2.UrbanZoning)
	}
	if math.Abs(tx2.ParkingAreaSqm-10) > 0.001 || tx2.ParkingPrice != 100000 {
		t.Fatalf("parking mismatch")
	}
}

func TestNormalizer_Parcel(t *testing.T) {
	n := New()
	row := map[string]string{
		"county":            "台北市",
		"district":          "中山區",
		"section":           "中山段二小段",
		"land_number":       "00120000",
		"area_sqm":          "123.45",
		"urban_zoning":      "住",
		"land_use_category": "住宅",
		"source":            "NLSC",
		"source_version":    "2024Q1",
	}
	p, err := n.NormalizeParcel(row)
	if err != nil {
		t.Fatalf("parcel failed: %v", err)
	}
	if p.County != "台北市" || p.AreaSqm != 123.45 {
		t.Fatalf("parcel mismatch: %+v", p)
	}
	if p.UrbanZoning != "住宅區" {
		t.Fatalf("parcel zoning: got %q", p.UrbanZoning)
	}
	if p.Geometry != "" {
		t.Fatalf("geometry should be empty, got %q", p.Geometry)
	}
	if p.Source != "NLSC" || p.SourceVersion != "2024Q1" {
		t.Fatalf("source mismatch: %+v", p)
	}
}

func TestNormalizer_PingToSqm(t *testing.T) {
	n := New()
	row := map[string]string{
		"county":           "台北市",
		"district":         "中正區",
		"section":          "重慶段一小段",
		"land_number":      "999",
		"transaction_date": "110/05/20",
		"total_price":      "1000000",
		"unit_price":       "10000",
		"land_area_ping":   "10",
		"transaction_id":   "TX-PING",
	}
	tx, err := n.NormalizeTransaction(row, "snap-001")
	if err != nil {
		t.Fatalf("ping tx failed: %v", err)
	}
	expected := 10 * PingToSqmFactor // 33.05785
	if math.Abs(tx.LandAreaSqm-expected) > 0.0001 {
		t.Fatalf("ping conversion failed: got %f want %f", tx.LandAreaSqm, expected)
	}

	// Parcel ping
	parcelRow := map[string]string{
		"county":      "台北市",
		"district":    "中山區",
		"section":     "中山段二小段",
		"land_number": "00120001",
		"area_ping":   "1",
		"source":      "NLSC",
	}
	p, err := n.NormalizeParcel(parcelRow)
	if err != nil {
		t.Fatalf("parcel ping failed: %v", err)
	}
	if math.Abs(p.AreaSqm-PingToSqmFactor) > 0.0001 {
		t.Fatalf("parcel ping: got %f want %f", p.AreaSqm, PingToSqmFactor)
	}
	// Direct check 1坪 = 3.305785
	if math.Abs(PingToSqmFactor-3.305785) > 1e-9 {
		t.Fatalf("PingToSqmFactor constant wrong")
	}
}

func TestNormalizer_Immutable(t *testing.T) {
	n := New()
	row := map[string]string{
		"county":           "台北市",
		"district":         "中正區",
		"section":          "重慶段一小段",
		"land_number":      "123",
		"transaction_date": "110/05/20",
		"total_price":      "1000000",
		"unit_price":       "9980",
		"urban_zoning":     "住",
	}
	// copy for comparison
	orig := make(map[string]string, len(row))
	for k, v := range row {
		orig[k] = v
	}
	_, err := n.NormalizeTransaction(row, "snap-001")
	if err != nil {
		t.Fatalf("immutable tx failed: %v", err)
	}
	if len(row) != len(orig) {
		t.Fatalf("input map size mutated")
	}
	for k, v := range orig {
		if row[k] != v {
			t.Fatalf("input map mutated: key %s got %q want %q", k, row[k], v)
		}
	}
	// Parcel immutable
	parcelRow := map[string]string{
		"county":      "台北市",
		"district":    "中山區",
		"section":     "中山段二小段",
		"land_number": "00120000",
		"area_sqm":    "100",
	}
	orig2 := make(map[string]string, len(parcelRow))
	for k, v := range parcelRow {
		orig2[k] = v
	}
	_, err = n.NormalizeParcel(parcelRow)
	if err != nil {
		t.Fatalf("immutable parcel failed: %v", err)
	}
	for k, v := range orig2 {
		if parcelRow[k] != v {
			t.Fatalf("parcel input mutated")
		}
	}
}

func TestNormalizer_MissingRequiredField(t *testing.T) {
	n := New()
	base := map[string]string{
		"county":           "台北市",
		"district":         "中正區",
		"section":          "重慶段一小段",
		"land_number":      "123",
		"transaction_date": "110/05/20",
		"total_price":      "1000000",
		"unit_price":       "9980",
	}
	// Missing county
	row := copyMap(base)
	delete(row, "county")
	if _, err := n.NormalizeTransaction(row, "snap-001"); err == nil {
		t.Fatalf("expected error for missing county")
	}
	// Missing district
	row = copyMap(base)
	delete(row, "district")
	if _, err := n.NormalizeTransaction(row, "snap-001"); err == nil {
		t.Fatalf("expected error for missing district")
	}
	// Missing section
	row = copyMap(base)
	delete(row, "section")
	if _, err := n.NormalizeTransaction(row, "snap-001"); err == nil {
		t.Fatalf("expected error for missing section")
	}
	// Missing land_number
	row = copyMap(base)
	delete(row, "land_number")
	if _, err := n.NormalizeTransaction(row, "snap-001"); err == nil {
		t.Fatalf("expected error for missing land_number")
	}
	// Parcel missing
	parcelRow := map[string]string{
		"county":      "台北市",
		"district":    "中山區",
		"section":     "中山段二小段",
		"land_number": "00120000",
		"area_sqm":    "100",
	}
	delete(parcelRow, "county")
	if _, err := n.NormalizeParcel(parcelRow); err == nil {
		t.Fatalf("expected parcel error for missing county")
	}
}

func TestNormalizer_InvalidDatePrice(t *testing.T) {
	n := New()
	base := map[string]string{
		"county":           "台北市",
		"district":         "中正區",
		"section":          "重慶段一小段",
		"land_number":      "123",
		"transaction_date": "110/05/20",
		"total_price":      "1000000",
		"unit_price":       "9980",
	}
	// Invalid date
	row := copyMap(base)
	row["transaction_date"] = "invalid-date"
	if _, err := n.NormalizeTransaction(row, "snap-001"); err == nil {
		t.Fatalf("expected error for invalid date")
	}
	// Invalid price
	row = copyMap(base)
	row["total_price"] = "not-a-price"
	if _, err := n.NormalizeTransaction(row, "snap-001"); err == nil {
		t.Fatalf("expected error for invalid price")
	}
	row = copyMap(base)
	row["unit_price"] = "abc"
	if _, err := n.NormalizeTransaction(row, "snap-001"); err == nil {
		t.Fatalf("expected error for invalid unit_price")
	}
	// Invalid area
	row = copyMap(base)
	row["land_area_sqm"] = "not-area"
	if _, err := n.NormalizeTransaction(row, "snap-001"); err == nil {
		t.Fatalf("expected error for invalid area")
	}
}

func copyMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
