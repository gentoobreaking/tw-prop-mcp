package mcp

import (
	"context"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// --- Prometheus-style metrics (in-memory, no external dependency) ---

var metricsRegistry = struct {
	sync.RWMutex
	counters map[string]*counter
	gauges  map[string]*gauge
	histograms map[string]*histogram
}{
	counters:  make(map[string]*counter),
	gauges:    make(map[string]*gauge),
	histograms: make(map[string]*histogram),
}

type counter struct {
	value int64
}

type gauge struct {
	value float64
}

type histogram struct {
	buckets map[float64]int64
	count    int64
	sum      float64
}

// Metrics holds Prometheus-style metrics for MCP server observability.
// Spec T017: mcp_requests_total, mcp_request_duration_seconds.
type Metrics struct {
	requestsTotal counter
	requestDuration histogram
}

func newMetrics() *Metrics {
	return &Metrics{
		requestsTotal: counter{},
		requestDuration: histogram{buckets: map[float64]int64{}},
	}
}

// CounterInc increments a counter.
func (m *Metrics) CounterInc(toolName string, success bool) {
	m.requestsTotal.value++
}

// ObserveDuration records a request duration.
func (m *Metrics) ObserveDuration(toolName string, duration time.Duration, success bool) {
	m.requestDuration.count++
	m.requestDuration.sum += duration.Seconds()
}

// GetCounter returns the requests total counter value.
func (m *Metrics) GetCounter() int64 {
	return m.requestsTotal.value
}

// --- Tracing (OpenTelemetry) ---

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("tw-prop-mcp/mcp")
}

// StartTrace starts an OpenTelemetry span for a tool call.
// Spec T017: trace should include tool_name, snapshot_id, query_hash.
func StartTrace(ctx context.Context, toolName, snapshotID, queryHash string) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, "mcp.tool_call")
	span.SetAttributes(
		attribute.String("tool_name", toolName),
		attribute.String("snapshot_id", snapshotID),
		attribute.String("query_hash", queryHash),
	)
	return ctx, span
}

// EndTrace ends the span with success/failure status.
func EndTrace(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
	}
	span.End()
}

// requestIDKey is the context key for request_id.
type requestIDKey struct{}

// WithRequestID adds a request_id to the context for logging.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// GetRequestID retrieves the request_id from context.
func GetRequestID(ctx context.Context) string {
	v := ctx.Value(requestIDKey{})
	if v == nil {
		return ""
	}
	return v.(string)
}

// LogRequest logs a tool request with structured fields.
// Spec T017: log includes request_id, tool_name, snapshot_id, algorithm_version, query_hash.
type RequestLogEntry struct {
	RequestID        string `json:"request_id"`
	ToolName         string `json:"tool_name"`
	SnapshotID       string `json:"snapshot_id,omitempty"`
	AlgorithmVersion string `json:"algorithm_version,omitempty"`
	QueryHash        string `json:"query_hash,omitempty"`
	DurationMs       int64  `json:"duration_ms,omitempty"`
	Error            string `json:"error,omitempty"`
}

// FormatDuration returns duration in milliseconds as string.
func FormatDuration(d time.Duration) string {
	return strconv.FormatInt(d.Milliseconds(), 10)
}
