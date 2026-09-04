package importpipeline

import (
	"testing"
)

func TestParseSectionLandNumber(t *testing.T) {
	tests := []struct {
		name        string
		addr        string
		wantSection string
		wantLandNo  string
	}{
		{
			name:        "full address with section and land number",
			addr:        "臺北市中山區中山段一小段0001-0002地號",
			wantSection: "臺北市中山區中山段一小段",
			wantLandNo:  "0001-0002",
		},
		{
			name:        "land reference only",
			addr:        "奇岩段四小段38地號",
			wantSection: "奇岩段四小段",
			wantLandNo:  "38",
		},
		{
			name:        "land reference with dash number",
			addr:        "光華段二小段720-1地號",
			wantSection: "光華段二小段",
			wantLandNo:  "720-1",
		},
		{
			name:        "street address without land number",
			addr:        "臺北市中山區中山北路二段６５巷９號",
			wantSection: "",
			wantLandNo:  "",
		},
		{
			name:        "empty address",
			addr:        "",
			wantSection: "",
			wantLandNo:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section, landNo := parseSectionLandNumber(tt.addr)
			if section != tt.wantSection {
				t.Errorf("section = %q, want %q", section, tt.wantSection)
			}
			if landNo != tt.wantLandNo {
				t.Errorf("land_number = %q, want %q", landNo, tt.wantLandNo)
			}
		})
	}
}

func TestCountyFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"a_lvr_land_a.csv", "臺北市"},
		{"b_lvr_land_a.csv", "臺中市"},
		{"c_lvr_land_a.csv", "基隆市"},
		{"d_lvr_land_a.csv", "臺南市"},
		{"e_lvr_land_a.csv", "高雄市"},
		{"f_lvr_land_a.csv", "新北市"},
		{"g_lvr_land_a.csv", "宜蘭縣"},
		{"h_lvr_land_a.csv", "桃園市"},
		{"v_lvr_land_a.csv", "臺東縣"},
		{"w_lvr_land_a.csv", "金門縣"},
		{"x_lvr_land_a.csv", "澎湖縣"},
		{"zz_lvr_land_a.csv", ""},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := countyFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("countyFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestEnrichRows(t *testing.T) {
	pp := NewImportPipeline(PipelineConfig{
		DataProvider: "臺北市",
		SnapshotID:   "test-snapshot",
	}, nil)

	rows := []map[string]string{
		{
			"district":       "中山區",
			"parcel_address": "臺北市中山區中山段一小段0001-0002地號",
		},
		{
			"district":       "大安區",
			"parcel_address": "臺北市大安區大安段二小段0003-0004地號",
		},
	}

	// Verify pre-enrichment state
	if rows[0]["county"] != "" {
		t.Fatalf("expected empty county before enrichment, got %q", rows[0]["county"])
	}
	if rows[0]["section"] != "" {
		t.Fatalf("expected empty section before enrichment, got %q", rows[0]["section"])
	}

	enriched := pp.enrichRows(rows)

	for i, row := range enriched {
		if row["county"] != "臺北市" {
			t.Errorf("row %d: county = %q, want %q", i, row["county"], "臺北市")
		}
		if row["section"] == "" {
			t.Errorf("row %d: section should not be empty after enrichment", i)
		}
		if row["land_number"] == "" {
			t.Errorf("row %d: land_number should not be empty after enrichment", i)
		}
	}
}

func TestEnrichRowsFromFilename(t *testing.T) {
	pp := NewImportPipeline(PipelineConfig{
		DownloadURL: "https://example.com/a_lvr_land_a.csv",
		SnapshotID:  "test-snapshot",
	}, nil)
	// DataProvider is empty — should derive from filename

	rows := []map[string]string{
		{
			"parcel_address": "奇岩段四小段38地號",
		},
	}

	enriched := pp.enrichRows(rows)
	if enriched[0]["county"] != "臺北市" {
		t.Errorf("county = %q, want %q (derived from filename prefix)", enriched[0]["county"], "臺北市")
	}
}

func TestMOIAddressRe(t *testing.T) {
	// Verify the regex handles all MOI address variants
	cases := []string{
		"臺北市中山區中山段一小段0001-0002地號",
		"奇岩段四小段38地號",
		"光華段二小段720-1地號",
		"河堤段六小段560地號",
		"康寧段三小段377-4地號",
	}
	for _, addr := range cases {
		m := moiAddressRe.FindStringSubmatch(addr)
		if m == nil {
			t.Errorf("address %q did not match regex", addr)
		}
	}
}
