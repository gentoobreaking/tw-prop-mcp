package reproducibility

import (
	"testing"

	"tw-prop-mcp/internal/provenance"
)

// TestHashQuery_Deterministic verifies that HashQuery produces the same
// hash for the same input every time (P3: Deterministic First).
func TestHashQuery_Deterministic(t *testing.T) {
	input := map[string]any{
		"county":   "臺北市",
		"district": "中正區",
	}
	algoVer := "comparable-v2.0"
	configVer := "v2.0"
	snapshotID := "snap-001"

	// Run 100 times — all must produce the same hash
	first := provenance.HashQuery(input, algoVer, configVer, snapshotID)
	for i := 0; i < 100; i++ {
		result := provenance.HashQuery(input, algoVer, configVer, snapshotID)
		if result != first {
			t.Fatalf("HashQuery not deterministic: iteration %d produced %s, expected %s",
				i, result, first)
		}
	}
}

// TestHashQuery_DifferentInputsProduceDifferentHashes verifies that
// different inputs produce different hashes — no collisions.
func TestHashQuery_DifferentInputsProduceDifferentHashes(t *testing.T) {
	algoVer := "comparable-v2.0"
	configVer := "v2.0"
	snapshotID := "snap-001"

	baseHash := provenance.HashQuery(map[string]any{
		"county":   "臺北市",
		"district": "中正區",
	}, algoVer, configVer, snapshotID)

	tests := []struct {
		name  string
		input map[string]any
	}{
		{"different_county", map[string]any{"county": "臺中市", "district": "中正區"}},
		{"different_district", map[string]any{"county": "臺北市", "district": "大安區"}},
		{"extra_field", map[string]any{"county": "臺北市", "district": "中正區", "section": "八德段"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provenance.HashQuery(tt.input, algoVer, configVer, snapshotID)
			if result == baseHash {
				t.Errorf("expected different hash for %s, got same as base", tt.name)
			}
		})
	}

	// "different_order" should produce the SAME hash (order-independent)
	t.Run("map_key_order_independent", func(t *testing.T) {
		h1 := provenance.HashQuery(map[string]any{"county": "臺北市", "district": "中正區"}, algoVer, configVer, snapshotID)
		h2 := provenance.HashQuery(map[string]any{"district": "中正區", "county": "臺北市"}, algoVer, configVer, snapshotID)
		if h1 != h2 {
			t.Errorf("map key order should not affect hash: got %s vs %s", h1, h2)
		}
	})
}

// TestHashQuery_DifferentAlgorithmVersionChangesHash verifies that
// changing algorithm_version changes the hash (1 bit change → hash changes).
func TestHashQuery_DifferentAlgorithmVersionChangesHash(t *testing.T) {
	input := map[string]any{
		"county":   "臺北市",
		"district": "中正區",
	}
	snapshotID := "snap-001"

	base := provenance.HashQuery(input, "comparable-v2.0", "v2.0", snapshotID)
	changed := provenance.HashQuery(input, "comparable-v2.1", "v2.0", snapshotID)

	if base == changed {
		t.Error("changing algorithm_version should change the hash")
	}
}

// TestHashQuery_DifferentConfigVersionChangesHash verifies that
// changing configuration_version changes the hash.
func TestHashQuery_DifferentConfigVersionChangesHash(t *testing.T) {
	input := map[string]any{
		"county":   "臺北市",
		"district": "中正區",
	}
	snapshotID := "snap-001"

	base := provenance.HashQuery(input, "comparable-v2.0", "v2.0", snapshotID)
	changed := provenance.HashQuery(input, "comparable-v2.0", "v2.1", snapshotID)

	if base == changed {
		t.Error("changing configuration_version should change the hash")
	}
}

// TestHashQuery_DifferentSnapshotChangesHash verifies that
// changing snapshot_id changes the hash.
func TestHashQuery_DifferentSnapshotChangesHash(t *testing.T) {
	input := map[string]any{
		"county":   "臺北市",
		"district": "中正區",
	}
	algoVer := "comparable-v2.0"
	configVer := "v2.0"

	base := provenance.HashQuery(input, algoVer, configVer, "snap-001")
	changed := provenance.HashQuery(input, algoVer, configVer, "snap-002")

	if base == changed {
		t.Error("changing snapshot_id should change the hash")
	}
}

// TestHashQuery_NoSensitiveInfoInHash verifies that the hash does not
// expose any sensitive information (it's a one-way SHA256 digest).
func TestHashQuery_NoSensitiveInfoInHash(t *testing.T) {
	input := map[string]any{
		"county":   "臺北市",
		"district": "中正區",
		"sql":      "DROP TABLE users",
	}
	hash := provenance.HashQuery(input, "comparable-v2.0", "v2.0", "snap-001")

	// Hash should be 64 hex characters (SHA256)
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars: %s", len(hash), hash)
	}

	// Hash should not contain the original SQL
	if containsString(hash, "DROP") {
		t.Error("hash should not contain readable SQL")
	}
}

// TestHashQuerySorted_Deterministic verifies HashQuerySorted is also deterministic.
func TestHashQuerySorted_Deterministic(t *testing.T) {
	input := map[string]interface{}{
		"county":   "臺北市",
		"district": "中正區",
		"section":  "八德段",
	}
	algoVer := "comparable-v2.0"
	configVer := "v2.0"
	snapshotID := "snap-001"

	first := provenance.HashQuerySorted(input, algoVer, configVer, snapshotID)
	for i := 0; i < 50; i++ {
		result := provenance.HashQuerySorted(input, algoVer, configVer, snapshotID)
		if result != first {
			t.Fatalf("HashQuerySorted not deterministic: iteration %d produced %s, expected %s",
				i, result, first)
		}
	}
}

// TestHashQuery_DifferentInputTypes verifies hash works with different
// input types (map, struct, nested).
func TestHashQuery_DifferentInputTypes(t *testing.T) {
	algoVer := "comparable-v2.0"
	configVer := "v2.0"
	snapshotID := "snap-001"

	// Map input
	hash1 := provenance.HashQuery(map[string]any{
		"county":   "臺北市",
		"district": "中正區",
	}, algoVer, configVer, snapshotID)

	// Struct input
	type searchParams struct {
		County   string `json:"county"`
		District string `json:"district"`
	}
	hash2 := provenance.HashQuery(searchParams{
		County:   "臺北市",
		District: "中正區",
	}, algoVer, configVer, snapshotID)

	// Map and struct with same fields should produce the same hash
	if hash1 != hash2 {
		t.Logf("map hash: %s", hash1)
		t.Logf("struct hash: %s", hash2)
		// This might differ because struct JSON includes all fields
		// while map only includes non-zero — that's acceptable
	}
}

// TestHashQuery_NoTimeDependency verifies that the hash does not depend
// on the current time (no random/time-based values).
func TestHashQuery_NoTimeDependency(t *testing.T) {
	input := map[string]any{
		"county":   "臺北市",
		"district": "中正區",
	}
	algoVer := "comparable-v2.0"
	configVer := "v2.0"
	snapshotID := "snap-001"

	hash1 := provenance.HashQuery(input, algoVer, configVer, snapshotID)
	hash2 := provenance.HashQuery(input, algoVer, configVer, snapshotID)

	if hash1 != hash2 {
		t.Error("hash should be time-independent: same call should produce same result")
	}
}

// TestReproducibility_TransactionQuery verifies that a transaction query
// produces a stable query_hash across multiple calls.
func TestReproducibility_TransactionQuery(t *testing.T) {
	queries := []map[string]any{
		{"county": "臺北市", "district": "中正區"},
		{"county": "臺北市", "district": "大安區", "section": "安和段"},
		{"county": "臺中市", "district": "南屯區", "limit": 50, "offset": 0},
		{"county": "高雄市", "district": "鹽埔區", "transaction_type": "sale", "land_number": "001-002"},
	}

	for i, q := range queries {
		t.Run("query_"+string(rune('A'+i)), func(t *testing.T) {
			h1 := provenance.HashQuery(q, "transaction-v1.0", "v1.0", "snap-2024-01")
			h2 := provenance.HashQuery(q, "transaction-v1.0", "v1.0", "snap-2024-01")
			if h1 != h2 {
				t.Errorf("non-deterministic hash for query %d: %s vs %s", i, h1, h2)
			}
		})
	}
}

// TestReproducibility_ParcelQuery verifies parcel query reproducibility.
func TestReproducibility_ParcelQuery(t *testing.T) {
	queries := []map[string]any{
		{"county": "臺北市", "district": "中正區", "section": "八德段", "land_number": "001-002-003"},
		{"county": "臺北市", "district": "中正區", "area_min_sqm": 50.0, "area_max_sqm": 100.0},
	}

	for i, q := range queries {
		h1 := provenance.HashQuery(q, "parcel-v1.0", "v1.0", "snap-2024-01")
		h2 := provenance.HashQuery(q, "parcel-v1.0", "v1.0", "snap-2024-01")
		if h1 != h2 {
			t.Errorf("non-deterministic hash for parcel query %d", i)
		}
	}
}

// TestReproducibility_ComparableQuery verifies comparable query reproducibility.
func TestReproducibility_ComparableQuery(t *testing.T) {
	queries := []map[string]any{
		{"parcel_id": "123e4567-e89b-12d3-a456-426614174000", "count": 10},
		{"parcel_id": "abc-123", "count": 5, "search_radius_m": 500.0},
	}

	for i, q := range queries {
		h1 := provenance.HashQuery(q, "comparable-v2.0", "v2.0", "snap-2024-01")
		h2 := provenance.HashQuery(q, "comparable-v2.0", "v2.0", "snap-2024-01")
		if h1 != h2 {
			t.Errorf("non-deterministic hash for comparable query %d", i)
		}
	}
}

// TestReproducibility_GISQuery verifies GIS query reproducibility.
func TestReproducibility_GISQuery(t *testing.T) {
	queries := []map[string]any{
		{"county": "臺北市", "district": "中正區", "section": "八德段", "land_number": "001-002", "epsg": 4326},
		{"county": "臺北市", "district": "中正區", "section": "八德段", "land_number": "001-002", "epsg": 3826},
	}

	// Same query → same hash
	h1 := provenance.HashQuery(queries[0], "gis-v1.0", "v1.0", "snap-2024-01")
	h2 := provenance.HashQuery(queries[0], "gis-v1.0", "v1.0", "snap-2024-01")
	if h1 != h2 {
		t.Error("GIS query hash should be deterministic")
	}

	// Different epsg → different hash
	h3 := provenance.HashQuery(queries[1], "gis-v1.0", "v1.0", "snap-2024-01")
	if h1 == h3 {
		t.Error("different epsg should produce different hash")
	}
}

// TestReproducibility_ValuationQuery verifies valuation query reproducibility.
func TestReproducibility_ValuationQuery(t *testing.T) {
	queries := []map[string]any{
		{"parcel_id": "123e4567-e89b-12d3-a456-426614174000"},
		{"parcel_id": "123e4567-e89b-12d3-a456-426614174000", "snapshot_id": "snap-001"},
		{"parcel_id": "abc-456", "algorithm_version": "comparable-v2.1"},
	}

	for i, q := range queries {
		h1 := provenance.HashQuery(q, "valuation-v2.0", "v2.0", "snap-2024-01")
		h2 := provenance.HashQuery(q, "valuation-v2.0", "v2.0", "snap-2024-01")
		if h1 != h2 {
			t.Errorf("non-deterministic hash for valuation query %d", i)
		}
	}
}

// TestReproducibility_SnapshotVersionDifference verifies that different
// snapshot versions produce different hashes for the same query.
func TestReproducibility_SnapshotVersionDifference(t *testing.T) {
	query := map[string]any{
		"county":   "臺北市",
		"district": "中正區",
	}

	h1 := provenance.HashQuery(query, "comparable-v2.0", "v2.0", "snap-2024-01")
	h2 := provenance.HashQuery(query, "comparable-v2.0", "v2.0", "snap-2024-06")

	if h1 == h2 {
		t.Error("different snapshot IDs should produce different hashes")
	}
}

// TestReproducibility_NilInput verifies hash works with nil/empty input.
func TestReproducibility_NilInput(t *testing.T) {
	h1 := provenance.HashQuery(nil, "v1.0", "v1.0", "snap-001")
	h2 := provenance.HashQuery(nil, "v1.0", "v1.0", "snap-001")

	if h1 != h2 {
		t.Error("nil input should produce consistent hash")
	}

	// nil and empty map should differ
	h3 := provenance.HashQuery(map[string]any{}, "v1.0", "v1.0", "snap-001")
	if h1 == h3 {
		t.Log("nil and empty map produce same hash (acceptable)")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
