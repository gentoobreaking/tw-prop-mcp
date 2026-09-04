package mcp

import (
	"context"
	"encoding/json"
	"time"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
)

// instrument wraps a tool handler with observability.
//
// Spec T024: every tool call must increment mcp_requests_total and
// mcp_request_duration_seconds. Domain-specific tools also increment
// transaction_query_total, gis_query_total, comparable_query_total, etc.
//
// Categories:
//   - "transaction": increments transaction_query_total + duration
//   - "gis": increments gis_query_total + duration
//   - "comparable": increments comparable_query_total + duration
//   - "valuation": increments valuation_query_total + duration
//   - "": increments only mcp_requests_total + mcp_request_duration_seconds
func instrument[In, Out any](
	s *Server,
	toolName string,
	category string,
	handler func(ctx context.Context, req *mcpapi.CallToolRequest, input In) (*mcpapi.CallToolResult, Out, error),
) func(ctx context.Context, req *mcpapi.CallToolRequest, input In) (*mcpapi.CallToolResult, Out, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input In) (*mcpapi.CallToolResult, Out, error) {
		// Extract query hash and snapshot for tracing/logging
		var queryHash, snapshotID string
		if req.Params != nil && req.Params.Arguments != nil {
			queryHash = provenanceHashFromArgs(req.Params.Arguments)
		}
		snapshotID = s.config.SnapshotID

		// Start trace span
		ctx, span := StartTrace(ctx, toolName, snapshotID, queryHash)
		defer span.End()

		// Record request ID for logging
		requestID := GetRequestID(ctx)

		start := time.Now()
		res, out, err := handler(ctx, req, input)
		duration := time.Since(start)

		success := err == nil && (res == nil || !res.IsError)

		// MCP-level metrics (all tools)
		s.metrics.CounterInc(toolName, success)
		s.metrics.ObserveDuration(toolName, duration, success)

		// Domain-specific metrics
		switch category {
		case "transaction":
			IncTransactionQuery(duration, success)
		case "gis":
			IncGISQuery(duration, success)
		case "comparable":
			IncComparableQuery(duration, success)
		case "valuation":
			IncValuationQuery(duration, success)
		}

		// Structured logging (JSON to stderr for log aggregation)
		s.logRequest(toolName, requestID, queryHash, duration, err)

		return res, out, err
	}
}

// provenanceHashFromArgs attempts to extract a query_hash from raw JSON arguments.
// If the arguments don't contain query_hash, returns empty string.
func provenanceHashFromArgs(args json.RawMessage) string {
	var argMap map[string]json.RawMessage
	if err := json.Unmarshal(args, &argMap); err != nil {
		return ""
	}
	if hash, ok := argMap["query_hash"]; ok {
		var s string
		if err := json.Unmarshal(hash, &s); err == nil {
			return s
		}
	}
	return ""
}
