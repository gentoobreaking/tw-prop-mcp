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
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tw-prop-mcp/internal/repository"
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
	server        *mcpapi.Server
	config        ServerConfig
	metrics       *Metrics
	configVersion string
	snapshotID    string
	TxRepo        repository.TransactionRepository
	ParcelRepo    repository.ParcelRepository
	Pool          *pgxpool.Pool
	SnapshotRepo  repository.SnapshotRepository
	Scheduler     interface {
		IsStale(ctx context.Context) (bool, string)
		CheckAndRefresh(ctx context.Context) (bool, error)
		GetProgress() any
		ImportLocalZip(ctx context.Context, zipPath string) (bool, error)
		ForceRefresh(ctx context.Context) (bool, error)
	}
}
func NewServer(config ServerConfig) *Server {
	impl := &mcpapi.Implementation{
		Name:    config.Name,
		Version: config.Version,
	}
	opts := &mcpapi.ServerOptions{}
	srv := mcpapi.NewServer(impl, opts)

	s := &Server{
		server:        srv,
		config:        config,
		metrics:       newMetrics(),
		configVersion: config.ConfigurationVersion,
		snapshotID:    config.SnapshotID,
	}
	// Initialize database repositories if DSN configured
	if config.DatabaseDSN != "" {
		ctx := context.Background()
		poolCfg, err := pgxpool.ParseConfig(config.DatabaseDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tw-prop-mcp: failed to parse database DSN: %v\n", err)
		} else {
			// Default pgxpool MaxConns is 4 — too low for concurrent MCP tool calls
			// (search + parallel parcel loads can easily exceed 4). Bump to 20.
			if poolCfg.MaxConns < 20 {
				poolCfg.MaxConns = 20
			}
			pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "tw-prop-mcp: failed to open database: %v\n", err)
			} else if err := pool.Ping(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "tw-prop-mcp: database ping failed: %v\n", err)
			} else {
				s.Pool = pool
				s.ParcelRepo = repository.NewParcelRepository(pool)
				s.TxRepo = repository.NewTransactionRepository(pool)
				s.SnapshotRepo = repository.NewSnapshotRepository(pool)
			}
		}
	}
	s.registerTools()
	s.registerResources()
	s.registerPrompts()
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
	registerFreshnessTools(s.server, s)
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
	}, &mcpapi.StreamableHTTPOptions{
		// SessionTimeout: auto-close idle SSE sessions after 10 minutes of inactivity.
		// Without this, disconnected browser sessions accumulate (the browser
		// loses the session ID on hard refresh) and exhaust server connections,
		// causing subsequent MCP requests to hang (search stuck on "搜尋中…").
		SessionTimeout: 10 * time.Minute,
	})

	mux.Handle("/mcp", s.requestIDMiddleware(handler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","tools_registered":true}`))
	})
	mux.Handle("/metrics", promhttp.Handler())
	// Admin: manual zip upload and progress polling for frontend
	mux.HandleFunc("/admin/import", s.handleAdminImport)
	mux.HandleFunc("/admin/progress", s.handleAdminProgress)

	addr := s.config.HTTPAddr
	if addr == "" {
		addr = ":8080"
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return httpSrv.ListenAndServe()
}

func (s *Server) handleAdminProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "content-type")
		w.WriteHeader(204)
		return
	}
	if s.Scheduler == nil {
		w.Write([]byte(`{"running":false,"stage":"idle","percent":0,"message":"scheduler not configured"}`))
		return
	}
	progress := s.Scheduler.GetProgress()
	b, _ := json.Marshal(progress)
	w.Write(b)
}

func (s *Server) handleAdminImport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "content-type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Scheduler == nil {
		http.Error(w, `{"error":"scheduler not configured"}`, http.StatusServiceUnavailable)
		return
	}
	// 32MB max
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"parse form: %v"}`, err), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"file field required (name=file)"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()
	if !isZipFile(header.Filename) {
		http.Error(w, `{"error":"only .zip allowed (lvr_landcsv.zip)"}`, http.StatusBadRequest)
		return
	}
	tmpPath := fmt.Sprintf("%s/%s", os.TempDir(), header.Filename)
	dst, err := os.Create(tmpPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"create temp: %v"}`, err), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		http.Error(w, fmt.Sprintf(`{"error":"save file: %v"}`, err), http.StatusInternalServerError)
		return
	}
	dst.Close()
	// Trigger import in background
	go func(path string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		defer os.Remove(path)
		_, _ = s.Scheduler.ImportLocalZip(ctx, path)
	}(tmpPath)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"started":true,"message":"zip upload received, import started in background; poll /admin/progress"}`))
}

func isZipFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".zip")
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
