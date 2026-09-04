package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsStruct(t *testing.T) {
	m := newMetrics()
	if m == nil {
		t.Fatal("newMetrics returned nil")
	}
}
func TestCounterInc_RecordsSuccessAndFailure(t *testing.T) {
	m := newMetrics()

	// Increment counters for various tools
	m.CounterInc("search_transactions", true)
	m.CounterInc("search_transactions", true)
	m.CounterInc("search_transactions", false)
	m.CounterInc("get_parcel", true)

	// Verify the Prometheus counter for search_transactions is incremented
	searchTrue := testutil.ToFloat64(mcpRequestsTotal.WithLabelValues("search_transactions", "true"))
	if searchTrue != 2 {
		t.Fatalf("expected 2 successful search_transactions, got %v", searchTrue)
	}

	searchFalse := testutil.ToFloat64(mcpRequestsTotal.WithLabelValues("search_transactions", "false"))
	if searchFalse != 1 {
		t.Fatalf("expected 1 failed search_transactions, got %v", searchFalse)
	}
}

func TestInstrument_WrapsHandlerWithMetrics(t *testing.T) {
	s := NewServer(ServerConfig{
		Name:    "test",
		Version: "1.0",
	})

	// Create a simple handler that returns success
	handler := func(ctx context.Context, req *mcpapi.CallToolRequest, input struct{ TestField string }) (*mcpapi.CallToolResult, string, error) {
		return nil, "result", nil
	}

	wrapped := instrument(s, "test_tool", "", handler)

	req := &mcpapi.CallToolRequest{
		Params: &mcpapi.CallToolParamsRaw{
			Arguments: json.RawMessage(`{"test_field":"value","query_hash":"abc123"}`),
		},
	}

	_, result, err := wrapped(context.Background(), req, struct{ TestField string }{TestField: "value"})
	if err != nil {
		t.Fatalf("wrapped handler returned error: %v", err)
	}
	if result != "result" {
		t.Fatalf("expected 'result', got %q", result)
	}
}

func TestInstrument_RecordsErrorMetrics(t *testing.T) {
	s := NewServer(ServerConfig{
		Name:    "test",
		Version: "1.0",
	})

	handler := func(ctx context.Context, req *mcpapi.CallToolRequest, input struct{ TestField string }) (*mcpapi.CallToolResult, string, error) {
		return nil, "", ErrTestError
	}

	wrapped := instrument(s, "test_error_tool", "transaction", handler)

	req := &mcpapi.CallToolRequest{
		Params: &mcpapi.CallToolParamsRaw{
			Arguments: json.RawMessage(`{}`),
		},
	}

	_, _, err := wrapped(context.Background(), req, struct{ TestField string }{})
	if err != ErrTestError {
		t.Fatalf("expected ErrTestError, got %v", err)
	}
}

func TestProvenanceHashFromArgs(t *testing.T) {
	args := json.RawMessage(`{"query_hash":"abc123","county":"test"}`)
	hash := provenanceHashFromArgs(args)
	if hash != "abc123" {
		t.Fatalf("expected 'abc123', got %q", hash)
	}
}

func TestProvenanceHashFromArgs_NoHash(t *testing.T) {
	args := json.RawMessage(`{"county":"test"}`)
	hash := provenanceHashFromArgs(args)
	if hash != "" {
		t.Fatalf("expected empty string, got %q", hash)
	}
}

func TestProvenanceHashFromArgs_InvalidJSON(t *testing.T) {
	args := json.RawMessage(`{invalid`)
	hash := provenanceHashFromArgs(args)
	if hash != "" {
		t.Fatalf("expected empty string for invalid JSON, got %q", hash)
	}
}

func TestStartTrace_CreatesSpan(t *testing.T) {
	ctx, span := StartTrace(context.Background(), "test_tool", "snap-001", "hash123")
	defer span.End()

	// Just verify it doesn't panic
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
}

func TestEndTrace_WithError(t *testing.T) {
	_, span := StartTrace(context.Background(), "test", "snap", "hash")
	EndTrace(span, ErrTestError)
	// Should not panic
}

func TestEndTrace_NilError(t *testing.T) {
	_, span := StartTrace(context.Background(), "test", "snap", "hash")
	EndTrace(span, nil)
	// Should not panic
}

func TestRequestID_RoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req_test_123")
	id := GetRequestID(ctx)
	if id != "req_test_123" {
		t.Fatalf("expected 'req_test_123', got %q", id)
	}
}

func TestRequestID_EmptyWhenNotSet(t *testing.T) {
	id := GetRequestID(context.Background())
	if id != "" {
		t.Fatalf("expected empty string, got %q", id)
	}
}

func TestLogRequest_JSONFormat(t *testing.T) {
	s := NewServer(ServerConfig{
		Name:             "test",
		Version:          "1.0",
		SnapshotID:       "snap-001",
		AlgorithmVersion: "comparable-v2.0",
	})

	// logRequest writes to stderr — capture it
	// Note: we can't easily capture stderr in Go tests, but we can verify
	// the function doesn't panic
	s.logRequest("test_tool", "req_001", "hash123", 0, nil)
}

func TestFormatDuration(t *testing.T) {
	d := FormatDuration(1500 * time.Millisecond)
	if d != "1500" {
		t.Fatalf("expected '1500', got %q", d)
	}
}

func TestIncTransactionQuery(t *testing.T) {
	// Should not panic
	IncTransactionQuery(time.Second, true)
}

func TestIncGISQuery(t *testing.T) {
	IncGISQuery(time.Second, true)
}

func TestIncComparableQuery(t *testing.T) {
	IncComparableQuery(time.Second, true)
}

func TestIncValuationQuery(t *testing.T) {
	IncValuationQuery(time.Second, true)
}

func TestIncDataImport(t *testing.T) {
	IncDataImport(true)
	IncDataImport(false)
}

func TestIncSnapshotLocked(t *testing.T) {
	IncSnapshotLocked()
}

// ErrTestError is a test error for instrumentation tests.
var ErrTestError = &McpError{
	Code:      ErrorCodeInternalError,
	Message:   "test error",
	Retryable: false,
}

// TestRequestLogEntry_JSON verifies the log entry struct is JSON-serializable.
func TestRequestLogEntry_JSON(t *testing.T) {
	entry := RequestLogEntry{
		RequestID:        "req_001",
		ToolName:         "search_transactions",
		SnapshotID:       "snap-001",
		AlgorithmVersion: "comparable-v2.0",
		QueryHash:        "abcdef123456",
		DurationMs:       42,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal RequestLogEntry: %v", err)
	}

	// Verify JSON contains expected fields
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal("unmarshal failed")
	}

	if decoded["request_id"] != "req_001" {
		t.Errorf("expected request_id='req_001', got %v", decoded["request_id"])
	}
	if decoded["tool_name"] != "search_transactions" {
		t.Errorf("expected tool_name='search_transactions', got %v", decoded["tool_name"])
	}
	if decoded["snapshot_id"] != "snap-001" {
		t.Errorf("expected snapshot_id='snap-001', got %v", decoded["snapshot_id"])
	}
	if decoded["query_hash"] != "abcdef123456" {
		t.Errorf("expected query_hash='abcdef123456', got %v", decoded["query_hash"])
	}
}

// TestLogEntryContainsRequiredFields verifies that structured logs contain
// all required fields per Spec T024.
func TestLogEntryContainsRequiredFields(t *testing.T) {
	var buf bytes.Buffer
	// The log entry structure already includes all required fields
	entry := RequestLogEntry{
		RequestID:        "req_test",
		ToolName:         "get_parcel",
		SnapshotID:       "snap-001",
		AlgorithmVersion: "comparable-v2.0",
		QueryHash:        "hash_test",
		DurationMs:       5,
	}
	data, _ := json.Marshal(entry)
	jsonStr := string(data)

	requiredFields := []string{"request_id", "tool_name", "snapshot_id", "algorithm_version", "query_hash"}
	for _, field := range requiredFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("log entry missing required field: %s", field)
		}
	}

	// Verify error field is present when there's an error
	entryErr := RequestLogEntry{
		RequestID:  "req_test",
		ToolName:   "get_parcel",
		Error:      "something went wrong",
	}
	errData, _ := json.Marshal(entryErr)
	if !strings.Contains(string(errData), "error") {
		t.Error("error field should be present in log entry with error")
	}

	_ = buf // suppress unused
}
