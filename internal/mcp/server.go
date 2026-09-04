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

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerConfig holds the configuration for the MCP server.
type ServerConfig struct {
	Name                 string
	Version              string
	SnapshotID           string
	AlgorithmVersion     string
	ConfigurationVersion string
	DatabaseDSN          string
	EnableHTTP           bool
	HTTPAddr             string
	RequestIDHeader      string
}

// Server is the MCP server for Taiwan property valuation.
// Wraps the official MCP SDK server with provenance injection,
// observability, and AI Isolation enforcement (P4).
type Server struct {
	server  *mcpapi.Server
	config  ServerConfig
	metrics *Metrics
}

// NewServer creates a new MCP server with the given configuration.
func NewServer(config ServerConfig) *Server {
	impl := &mcpapi.Implementation{
		Name:    config.Name,
		Version: config.Version,
	}
	opts := &mcpapi.ServerOptions{}
	srv := mcpapi.NewServer(impl, opts)

	s := &Server{
		server:  srv,
		config:  config,
		metrics: newMetrics(),
	}

	s.registerTools()
	s.registerResources()

	return s
}

// registerTools registers all MCP tools with provenance injection.
func (s *Server) registerTools() {
	registerTransactionTools(s.server, s)
	registerParcelTools(s.server, s)
	registerGISTools(s.server, s)
	registerComparableTools(s.server, s)
	registerValuationTools(s.server, s)
	registerProvenanceTools(s.server, s)
}

// registerResources registers MCP resources.
func (s *Server) registerResources() {
	registerResources(s.server, s)
}

// RunStdio starts the server with stdio transport.
func (s *Server) RunStdio(ctx context.Context) error {
	t := &mcpapi.StdioTransport{}
	return s.server.Run(ctx, t)
}

// RunHTTP starts the server with Streamable HTTP transport.
// Exposes /mcp endpoint with request_id middleware.
func (s *Server) RunHTTP(ctx context.Context) error {
	mux := http.NewServeMux()

	handler := mcpapi.NewStreamableHTTPHandler(func(r *http.Request) *mcpapi.Server {
		return s.server
	}, &mcpapi.StreamableHTTPOptions{})

	mux.Handle("/mcp", s.requestIDMiddleware(handler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	// Readiness probe — reflects whether the server can handle requests.
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","tools_registered":true}`))
	})
	// Prometheus metrics endpoint (Spec T024: observability)
	mux.Handle("/metrics", promhttp.Handler())

	addr := s.config.HTTPAddr
	if addr == "" {
		addr = ":8080"
	}

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return httpSrv.ListenAndServe()
}

// ExposedServer returns the underlying MCP SDK server for testing.
func (s *Server) ExposedServer() *mcpapi.Server {
	return s.server
}

// requestIDMiddleware adds request_id to context for logging.
func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(s.config.RequestIDHeader)
		if requestID == "" {
			requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
		}
		ctx := WithRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// logRequest writes a structured log entry for observability.
func (s *Server) logRequest(toolName, requestID, queryHash string, duration time.Duration, err error) {
	entry := RequestLogEntry{
		RequestID:        requestID,
		ToolName:         toolName,
		SnapshotID:       s.config.SnapshotID,
		AlgorithmVersion: s.config.AlgorithmVersion,
		QueryHash:        queryHash,
		DurationMs:       duration.Milliseconds(),
	}
	if err != nil {
		entry.Error = err.Error()
	}
	logJSON, _ := json.Marshal(entry)
	fmt.Fprintf(os.Stderr, "mcp_request: %s\n", string(logJSON))
}

func ErrorResult(err *McpError) (*mcpapi.CallToolResult, error) {
	content := fmt.Sprintf(`{"error":{"code":"%s","message":"%s","retryable":%t}}`,
		err.Code, err.Message, err.Retryable)
	return &mcpapi.CallToolResult{
		Content: []mcpapi.Content{
			&mcpapi.TextContent{Text: content},
		},
		IsError: true,
	}, nil
}
