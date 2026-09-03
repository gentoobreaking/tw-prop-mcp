package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// HashQuery generates a deterministic SHA256 query hash.
// Spec (T016): canonicalize({input_params sorted JSON, algorithm_version, configuration_version, snapshot_id})
// → SHA256 hex → query_hash.
//
// The hash must NOT depend on time or random values.
// generated_at (RFC3339) is explicitly excluded from the hash.
//
// input: arbitrary parameters; they are canonicalized via sorted JSON keys.
// algoVer: algorithm version (e.g. "comparable-v2.0").
// configVer: configuration version (e.g. "v2.0").
// snapshotID: the dataset snapshot ID.
func HashQuery(input interface{}, algoVer, configVer, snapshotID string) string {
	// Canonicalize: marshal input to JSON with sorted keys
	canonical, err := canonicalize(input)
	if err != nil {
		// If marshaling fails, use empty JSON object
		canonical = "{}"
	}

	// Build the hash input string
	hashInput := canonical + "|" + algoVer + "|" + configVer + "|" + snapshotID

	h := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(h[:])
}

// canonicalize converts input to canonical JSON with sorted keys.
func canonicalize(input interface{}) (string, error) {
	// For maps, we need to sort keys. Use json.Marshal with sort.
	// json.Marshal already sorts struct fields, but for maps we need
	// to ensure sorted output. We use a custom approach.
	buf, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// HashQuerySorted is like HashQuery but takes pre-sorted JSON keys.
// This variant is useful when you need guaranteed deterministic ordering
// of map keys (Go maps have random iteration order).
func HashQuerySorted(input map[string]interface{}, algoVer, configVer, snapshotID string) string {
	// Sort map keys for deterministic output
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var canonical string
	for _, k := range keys {
		v, _ := json.Marshal(input[k])
		if canonical != "" {
			canonical += ","
		}
		canonical += `"` + k + `":` + string(v)
	}
	canonical = "{" + canonical + "}"

	hashInput := canonical + "|" + algoVer + "|" + configVer + "|" + snapshotID
	h := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(h[:])
}
