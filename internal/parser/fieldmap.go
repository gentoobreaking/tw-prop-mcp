package parser

import (
	"strings"
)

// FieldMapCNToEN maps Chinese header names to English codes.
// It covers official real-price CSV headers, MANIFEST and schema variants.
var FieldMapCNToEN = map[string]string{
	"鄉鎮市區":                   "district",
	"交易標的":                   "transaction_target",
	"土地區段位置建物區段門牌":                 "section",
	"土地區段位置建物區段":                  "section",
	"土地區段位置":                   "section",
	"建物區段門牌":                   "section",
	"土地移轉總面積平方公尺":                 "land_area_sqm",
	"交易年月日":                   "transaction_date",
	"總價元":                    "total_price",
	"單價元平方公尺":                 "unit_price",
	"單價(元/平方公尺)":               "unit_price",
	"建物移轉總面積平方公尺":                 "building_area_sqm",
	"都市土地使用分區":                 "urban_zoning",
	"非都市土地使用分區":                 "non_urban_zoning",
	"非都市土地使用編定":                 "land_use_category",
	"使用分區":                   "urban_zoning",
	"使用地類別":                  "land_use_category",
	"交易筆棟數":                  "transaction_count",
	"移轉層次":                   "floor",
	"總樓層數":                   "total_floors",
	"建物型態":                   "building_type",
	"主要用途":                   "main_use",
	"主要建材":                   "main_material",
	"建築完成年月":                 "building_complete_date",
	"建物現況格局-房":                "room_count",
	"建物現況格局-廳":                "hall_count",
	"建物現況格局-衛":                "bathroom_count",
	"建物現況格局-隔間":               "compartment",
	"有無管理組織":                 "has_management",
	"車位類別":                   "parking_type",
	"車位移轉總面積(平方公尺)":            "parking_area_sqm",
	"車位移轉總面積平方公尺":              "parking_area_sqm",
	"車位總價元":                  "parking_price",
	"備註":                     "note",
	"編號":                     "serial",
	"縣市":                     "county",
	"地段":                     "section",
	"地號":                     "land_number",
	"段":                      "section",
	"號":                      "land_number",
	"車位移轉總面積（平方公尺）":           "parking_area_sqm",
	"土地面積平方公尺":                 "land_area_sqm",
	"建物面積平方公尺":                 "building_area_sqm",
	"總價":                     "total_price",
	"單價":                     "unit_price",
	"交易日期":                   "transaction_date",
	"成交年月日":                  "transaction_date",
	"檔案名稱":                   "file_name",
	"檔案":                     "file_name",
	"資料筆數":                   "record_count",
	"筆數":                     "record_count",
	"下載時間":                   "downloaded_at",
	"來源":                     "source",
	"版本":                     "source_version",
	"來源版本":                   "source_version",
	"發布日期":                   "published_at",
	"checksum":               "file_sha256",
	"檔案SHA256":               "file_sha256",
	"district":               "district",
	"county":                 "county",
}

// NormalizeHeader cleans a raw CSV header and maps it to an English code.
// It handles: UTF-8 BOM prefix, whitespace (including full-width ideographic space \u3000),
// full-width punctuation normalization.
func NormalizeHeader(header string) string {
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
	if v, ok := FieldMapCNToEN[s]; ok {
		return v
	}
	lower := strings.ToLower(s)
	if v, ok := FieldMapCNToEN[lower]; ok {
		return v
	}
	return s
}
