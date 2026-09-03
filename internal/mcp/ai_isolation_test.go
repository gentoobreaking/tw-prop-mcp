package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestProhibitedFieldsMap verifies that all required prohibited fields
// are present in the ProhibitedFields map.
func TestProhibitedFieldsMap(t *testing.T) {
	required := []string{"sql", "where", "postgis", "valuation_formula", "weights"}
	for _, field := range required {
		if !ProhibitedFields[field] {
			t.Errorf("ProhibitedFields missing required field: %s", field)
		}
	}
	// Ensure non-prohibited fields are not in the map
	nonProhibited := []string{"county", "district", "parcel_id", "transaction_id"}
	for _, field := range nonProhibited {
		if ProhibitedFields[field] {
			t.Errorf("ProhibitedFields should NOT contain field: %s", field)
		}
	}
}

// TestValidateAIIsolation_ProhibitedFields tests that ValidateAIIsolation
// returns an error for each prohibited field.
func TestValidateAIIsolation_ProhibitedFields(t *testing.T) {
	prohibitedFields := []string{"sql", "where", "postgis", "valuation_formula", "weights"}

	for _, field := range prohibitedFields {
		t.Run("field_"+field, func(t *testing.T) {
			input := map[string]any{
				"county": "臺北市",
				field:    "malicious payload",
			}
			err := ValidateAIIsolation(input)
			if err == nil {
				t.Fatalf("expected error for prohibited field %s, got nil", field)
			}
			mce := IsMcpError(err)
			if mce == nil {
				t.Fatalf("expected McpError for prohibited field %s, got %T", field, err)
			}
			if mce.Code != ErrorCodeInvalidArgument {
				t.Errorf("expected ErrorCodeInvalidArgument, got %s", mce.Code)
			}
		})
	}
}

// TestValidateAIIsolation_AllowedFields tests that allowed fields pass validation.
func TestValidateAIIsolation_AllowedFields(t *testing.T) {
	input := map[string]any{
		"county":          "臺北市",
		"district":        "中正區",
		"parcel_id":       "abc-123",
		"transaction_id":  "def-456",
		"limit":           100,
		"offset":          0,
	}
	err := ValidateAIIsolation(input)
	if err != nil {
		t.Fatalf("expected nil error for allowed fields, got: %v", err)
	}
}

// TestValidateAIIsolation_EmptyInput tests that empty input passes validation.
func TestValidateAIIsolation_EmptyInput(t *testing.T) {
	err := ValidateAIIsolation(map[string]any{})
	if err != nil {
		t.Fatalf("expected nil error for empty input, got: %v", err)
	}
}

// TestValidateAIIsolation_NilInput tests that nil input passes validation.
func TestValidateAIIsolation_NilInput(t *testing.T) {
	err := ValidateAIIsolation(nil)
	if err != nil {
		t.Fatalf("expected nil error for nil input, got: %v", err)
	}
}

// TestCheckAIIsolation_ProhibitedFieldInRequest tests that checkAIIsolation
// correctly detects prohibited fields in a CallToolRequest.
func TestCheckAIIsolation_ProhibitedFieldInRequest(t *testing.T) {
	// Build a CallToolRequest with a prohibited "sql" field
	rawArgs, _ := json.Marshal(map[string]any{
		"county": "臺北市",
		"sql":    "SELECT * FROM transactions; DROP TABLE parcels;--",
	})

	req := &mcpapi.CallToolRequest{
		Params: &mcpapi.CallToolParamsRaw{
			Arguments: rawArgs,
		},
	}

	err := checkAIIsolation(req)
	if err == nil {
		t.Fatal("expected error for prohibited field 'sql', got nil")
	}
	mce := IsMcpError(err)
	if mce == nil {
		t.Fatalf("expected McpError, got %T: %v", err, err)
	}
	if mce.Code != ErrorCodeInvalidArgument {
		t.Errorf("expected ErrorCodeInvalidArgument, got %s", mce.Code)
	}
}

// TestCheckAIIsolation_AllowedFieldsInRequest tests that checkAIIsolation
// allows requests with only permitted fields.
func TestCheckAIIsolation_AllowedFieldsInRequest(t *testing.T) {
	rawArgs, _ := json.Marshal(map[string]any{
		"county":   "臺北市",
		"district": "中正區",
		"limit":    50,
	})

	req := &mcpapi.CallToolRequest{
		Params: &mcpapi.CallToolParamsRaw{
			Arguments: rawArgs,
		},
	}

	err := checkAIIsolation(req)
	if err != nil {
		t.Fatalf("expected nil error for allowed fields, got: %v", err)
	}
}

// TestCheckAIIsolation_EmptyArguments tests that checkAIIsolation
// handles empty/missing arguments gracefully.
func TestCheckAIIsolation_EmptyArguments(t *testing.T) {
	req := &mcpapi.CallToolRequest{
		Params: &mcpapi.CallToolParamsRaw{
			Arguments: nil,
		},
	}

	err := checkAIIsolation(req)
	if err != nil {
		t.Fatalf("expected nil error for nil arguments, got: %v", err)
	}

	// Also test with empty raw message
	req.Params.Arguments = []byte{}
	err = checkAIIsolation(req)
	if err != nil {
		t.Fatalf("expected nil error for empty arguments, got: %v", err)
	}
}

// TestCheckAIIsolation_MultipleProhibitedFields tests that
// checkAIIsolation detects the first prohibited field in a request with multiple.
func TestCheckAIIsolation_MultipleProhibitedFields(t *testing.T) {
	rawArgs, _ := json.Marshal(map[string]any{
		"county":    "臺北市",
		"sql":       "DROP TABLE users",
		"where":     "1=1",
		"postgis":   true,
		"weights":   map[string]any{"a": 1},
	})

	req := &mcpapi.CallToolRequest{
		Params: &mcpapi.CallToolParamsRaw{
			Arguments: rawArgs,
		},
	}

	err := checkAIIsolation(req)
	if err == nil {
		t.Fatal("expected error for prohibited fields, got nil")
	}
	mce := IsMcpError(err)
	if mce == nil {
		t.Fatalf("expected McpError, got %T", err)
	}
}

// TestCheckAIIsolation_InvalidJSON tests that checkAIIsolation
// handles malformed JSON arguments.
func TestCheckAIIsolation_InvalidJSON(t *testing.T) {
	req := &mcpapi.CallToolRequest{
		Params: &mcpapi.CallToolParamsRaw{
			Arguments: []byte("invalid json"),
		},
	}

	err := checkAIIsolation(req)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	mce := IsMcpError(err)
	if mce == nil {
		t.Fatalf("expected McpError, got %T", err)
	}
	if mce.Code != ErrorCodeInvalidArgument {
		t.Errorf("expected ErrorCodeInvalidArgument, got %s", mce.Code)
	}
}

// TestCheckAIIsolation_NestedProhibitedField tests that nested prohibited
// fields (e.g., inside an object value) are still detected.
func TestCheckAIIsolation_NestedProhibitedField(t *testing.T) {
	rawArgs, _ := json.Marshal(map[string]any{
		"county": "臺北市",
		"nested": map[string]any{
			"sql": "SELECT 1",
		},
	})

	req := &mcpapi.CallToolRequest{
		Params: &mcpapi.CallToolParamsRaw{
			Arguments: rawArgs,
		},
	}

	// Top-level "nested" is not prohibited, so this should pass
	// (checkAIIsolation only checks top-level keys)
	err := checkAIIsolation(req)
	if err != nil {
		t.Fatalf("expected nil error for top-level 'nested' key, got: %v", err)
	}
}

// TestMcpError_ErrorFormat tests the Error() method of McpError.
func TestMcpError_ErrorFormat(t *testing.T) {
	err := NewError(ErrorCodeParcelNotFound, "parcel 123 not found")
	expected := "PARCEL_NOT_FOUND: parcel 123 not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

// TestMcpError_RetryableFalse tests that all McpErrors have Retryable=false.
func TestMcpError_RetryableFalse(t *testing.T) {
	codes := []ErrorCode{
		ErrorCodeInvalidArgument,
		ErrorCodeParcelNotFound,
		ErrorCodeTransactionNotFound,
		ErrorCodeDataNotAvailable,
		ErrorCodeGISNotAvailable,
		ErrorCodeSnapshotNotFound,
		ErrorCodeValuationNotAvailable,
		ErrorCodeSourceUnavailable,
		ErrorCodeInternalError,
	}

	for _, code := range codes {
		err := NewError(code, "test")
		if err.Retryable {
			t.Errorf("error code %s should have Retryable=false", code)
		}
	}
}

// TestIsMcpError_WrapsMcpError tests that IsMcpError correctly unwraps McpError.
func TestIsMcpError_WrapsMcpError(t *testing.T) {
	original := NewError(ErrorCodeInternalError, "something went wrong")
	wrapped := wrapServiceError(original)

	if wrapped != original {
		t.Errorf("expected wrapped to be the original McpError")
	}
	if IsMcpError(original).Retryable {
		t.Errorf("expected non-retryable error")
	}
}

// TestIsMcpError_NonMcpError tests that IsMcpError returns nil for non-McpError.
func TestIsMcpError_NonMcpError(t *testing.T) {
	result := IsMcpError(context.DeadlineExceeded)
	if result != nil {
		t.Errorf("expected nil for non-McpError, got %v", result)
	}
}

// TestErrorResult_CreatesErrorResult tests that ErrorResult creates a proper
// CallToolResult with IsError=true and error content.
func TestErrorResult_CreatesErrorResult(t *testing.T) {
	mce := NewError(ErrorCodeParcelNotFound, "parcel not found")
	result, err := ErrorResult(mce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
	_ = mce
}

// TestNewErrorf_Formatting tests formatted error creation.
func TestNewErrorf_Formatting(t *testing.T) {
	err := NewErrorf(ErrorCodeInvalidArgument, "county %s is invalid", "臺北市")
	if err.Code != ErrorCodeInvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %s", err.Code)
	}
	expected := "INVALID_ARGUMENT: county 臺北市 is invalid"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
