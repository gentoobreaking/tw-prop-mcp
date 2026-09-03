// Package mcp implements the Model Context Protocol (MCP) server for
// Taiwan property valuation data.
//
// This package provides MCP tools and resources for querying real estate
// transactions, parcels, comparable analysis, land valuation, and data
// provenance. It wraps the official MCP Go SDK (v1.7.0) with:
//
//   - Typed tool I/O via [mcpapi.AddTool] generics
//   - AI Isolation enforcement (prohibited fields: sql, where, postgis, etc.)
//   - Provenance injection (P6) on all responses
//   - Observability via structured request logging
//   - Error model with retryable flags and error codes
//
// # Architecture
//
// The server struct wraps [mcpapi.Server] and registers tools in per-domain
// files:
//
//   - transaction_tools.go: search_transactions, get_transaction,
//     get_transaction_statistics
//   - parcel_tools.go: get_parcel, search_parcels
//   - gis_tools.go: get_parcel_geometry, get_parcel_location,
//     find_nearby_roads, get_parcel_map_context, check_road_access
//   - comparable_tools.go: find_comparable_transactions,
//     score_comparable_transactions
//   - valuation_tools.go: estimate_land_value, estimate_property_value,
//     explain_valuation
//   - provenance_tools.go: get_data_snapshot, get_data_provenance
//   - resources.go: realestate:// URIs
//
// # Transport
//
// The server supports two transports:
//
//   - Stdio: for MCP client integration via stdin/stdout
//   - HTTP: Streamable HTTP on /mcp endpoint with /healthz
//
// # AI Isolation (P4)
//
// All tool handlers validate raw arguments against [ProhibitedFields] before
// any business logic executes, preventing LLM prompt injection from escalating
// to SQL or custom query execution.
package mcp
