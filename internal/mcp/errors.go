// Package mcp implements the Model Context Protocol server for Taiwan property
// valuation data queries. It provides tools for searching transactions, parcels,
// GIS data, comparables, and valuations, all with provenance tracking and
// AI Isolation enforcement (P4).
package mcp

import (
	"errors"
	"fmt"
)

// ErrorCode enumerates all error codes used in MCP tool responses.
// Spec T017: 9 codes, all with retryable=false.
type ErrorCode string

const (
	// INVALID_ARGUMENT: parameter validation failure (missing required fields,
	// or prohibited fields like SQL/WHERE/PostGIS/weights).
	ErrorCodeInvalidArgument ErrorCode = "INVALID_ARGUMENT"

	// PARCEL_NOT_FOUND: parcel lookup returned no results.
	ErrorCodeParcelNotFound ErrorCode = "PARCEL_NOT_FOUND"

	// TRANSACTION_NOT_FOUND: transaction lookup returned no results.
	ErrorCodeTransactionNotFound ErrorCode = "TRANSACTION_NOT_FOUND"

	// DATA_NOT_AVAILABLE: requested data (e.g., comparable, provenance) is missing.
	ErrorCodeDataNotAvailable ErrorCode = "DATA_NOT_AVAILABLE"

	// GIS_NOT_AVAILABLE: GIS/geometry operation unavailable (e.g., postgis not loaded).
	ErrorCodeGISNotAvailable ErrorCode = "GIS_NOT_AVAILABLE"

	// SNAPSHOT_NOT_FOUND: dataset snapshot lookup returned no results.
	ErrorCodeSnapshotNotFound ErrorCode = "SNAPSHOT_NOT_FOUND"

	// VALUATION_NOT_AVAILABLE: valuation could not be computed/retrieved.
	ErrorCodeValuationNotAvailable ErrorCode = "VALUATION_NOT_AVAILABLE"

	// SOURCE_UNAVAILABLE: upstream data source unavailable.
	ErrorCodeSourceUnavailable ErrorCode = "SOURCE_UNAVAILABLE"

	// INTERNAL_ERROR: unexpected server error.
	ErrorCodeInternalError ErrorCode = "INTERNAL_ERROR"
)

// McpError is the unified error type returned by all MCP tools.
// Spec T017: {error{code, message, retryable}}.
type McpError struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Retryable bool      `json:"retryable"` // always false for all 9 codes per spec
}

func (e *McpError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError creates an McpError with retryable=false.
func NewError(code ErrorCode, message string) *McpError {
	return &McpError{Code: code, Message: message, Retryable: false}
}

// NewErrorf creates an McpError with formatted message.
func NewErrorf(code ErrorCode, format string, args ...interface{}) *McpError {
	return &McpError{Code: code, Message: fmt.Sprintf(format, args...), Retryable: false}
}

// IsMcpError returns the McpError if err wraps one, or nil.
func IsMcpError(err error) *McpError {
	if err == nil {
		return nil
	}
	var mce *McpError
	if errors.As(err, &mce) {
		return mce
	}
	return nil
}

// ProhibitedFields lists parameters that are rejected by AI Isolation (P4).
// Any tool input containing these keys → INVALID_ARGUMENT.
var ProhibitedFields = map[string]bool{
	"sql":             true,
	"where":           true,
	"postgis":         true,
	"valuation_formula": true,
	"weights":         true,
}

// ValidateAIIsolation checks that the given input map does not contain
// any prohibited fields. This is called by every tool handler.
func ValidateAIIsolation(input map[string]any) error {
	for key := range input {
		if ProhibitedFields[key] {
			return NewError(ErrorCodeInvalidArgument,
				fmt.Sprintf("prohibited field '%s': AI isolation requires structured parameters only", key))
		}
	}
	return nil
}
