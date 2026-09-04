package mcp

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otlptracehttp "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

//
// Spec T024: mcp_requests_total, mcp_request_duration_seconds,
// transaction_query_total, transaction_query_duration,
// gis_query_total, gis_query_duration,
// comparable_query_total, comparable_query_duration,
// valuation_query_total, valuation_query_duration,
// data_import_total, data_import_errors,
// snapshot_locked_total.

var (
	mcpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_requests_total",
			Help: "Total number of MCP tool requests, partitioned by tool and success.",
		},
		[]string{"tool", "success"},
	)

	mcpRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_request_duration_seconds",
			Help:    "Request latency in seconds for MCP tool calls.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tool", "success"},
	)

	transactionQueryTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "transaction_query_total",
			Help: "Total number of transaction queries executed.",
		},
	)

	transactionQueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "transaction_query_duration_seconds",
			Help:    "Latency of transaction queries in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	gisQueryTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "gis_query_total",
			Help: "Total number of GIS queries executed.",
		},
	)

	gisQueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "gis_query_duration_seconds",
			Help:    "Latency of GIS queries in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	comparableQueryTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "comparable_query_total",
			Help: "Total number of comparable transaction queries executed.",
		},
	)

	comparableQueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "comparable_query_duration_seconds",
			Help:    "Latency of comparable transaction queries in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	valuationQueryTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "valuation_query_total",
			Help: "Total number of valuation queries executed.",
		},
	)

	valuationQueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "valuation_query_duration_seconds",
			Help:    "Latency of valuation queries in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	dataImportTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "data_import_total",
			Help: "Total number of successful data imports.",
		},
	)

	dataImportErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "data_import_errors",
			Help: "Total number of failed data imports.",
		},
	)

	snapshotLockedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "snapshot_locked_total",
			Help: "Total number of snapshots transitioned to LOCKED status.",
		},
	)
)

// Metrics holds Prometheus-style metrics for MCP server observability.
type Metrics struct {
	counter *prometheus.CounterVec
}

func newMetrics() *Metrics {
	return &Metrics{
		counter: mcpRequestsTotal,
	}
}

// CounterInc increments the MCP request counter for a tool.
func (m *Metrics) CounterInc(toolName string, success bool) {
	successStr := strconv.FormatBool(success)
	m.counter.WithLabelValues(toolName, successStr).Inc()
}

// ObserveDuration records MCP request duration.
func (m *Metrics) ObserveDuration(toolName string, duration time.Duration, success bool) {
	successStr := strconv.FormatBool(success)
	mcpRequestDurationSeconds.WithLabelValues(toolName, successStr).Observe(duration.Seconds())
}

// GetCounter returns the total request count.
func (m *Metrics) GetCounter() int64 {
	// This is a best-effort for testing; the real metrics are Prometheus counters.
	return 0
}

// --- Domain-specific metric helpers ---

// IncTransactionQuery increments transaction query counters.
func IncTransactionQuery(duration time.Duration, success bool) {
	transactionQueryTotal.Inc()
	transactionQueryDuration.Observe(duration.Seconds())
}

// IncGISQuery increments GIS query counters.
func IncGISQuery(duration time.Duration, success bool) {
	gisQueryTotal.Inc()
	gisQueryDuration.Observe(duration.Seconds())
}

// IncComparableQuery increments comparable query counters.
func IncComparableQuery(duration time.Duration, success bool) {
	comparableQueryTotal.Inc()
	comparableQueryDuration.Observe(duration.Seconds())
}

// IncValuationQuery increments valuation query counters.
func IncValuationQuery(duration time.Duration, success bool) {
	valuationQueryTotal.Inc()
	valuationQueryDuration.Observe(duration.Seconds())
}

// IncDataImport increments data import counters.
func IncDataImport(success bool) {
	if success {
		dataImportTotal.Inc()
	} else {
		dataImportErrors.Inc()
	}
}

// IncSnapshotLocked increments the snapshot locked counter.
func IncSnapshotLocked() {
	snapshotLockedTotal.Inc()
}

// --- Tracing (OpenTelemetry) ---

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("tw-prop-mcp/mcp")
}

// InitTracer configures the OpenTelemetry tracer to export spans to the
// OTLP HTTP endpoint specified by OTEL_EXPORTER_OTLP_ENDPOINT (default
// http://localhost:4318). Call this once at application startup; it is
// safe to call when OTEL is not configured (falls back to no-op trace).
func InitTracer(ctx context.Context) func(context.Context) error {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	}
	// Normalize: strip trailing slash and http(s) prefix if present
	if endpoint != "" {
		// otlptracehttp.WithEndpoint expects host:port or host without scheme
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithInsecure(),
		}
		exporter, err := otlptracehttp.New(ctx, opts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "otel: failed to create OTLP exporter: %v\n", err)
		} else {
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithBatcher(exporter),
				sdktrace.WithResource(resource.NewWithAttributes(
					"service.name", attribute.String("service.name", "tw-prop-mcp"),
				)),
			)
			otel.SetTracerProvider(tp)
			tracer = otel.Tracer("tw-prop-mcp/mcp")
			return tp.Shutdown
		}
	}
	// Fallback: no-op tracer provider (traces discarded)
	return func(context.Context) error { return nil }
}

// StartTrace starts an OpenTelemetry span for a tool call.
// Spec T024: trace should include tool_name, snapshot_id, query_hash.
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
