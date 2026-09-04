package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	mcp "tw-prop-mcp/internal/mcp"
)

func main() {
	// CLI flags (override env vars for local dev)
	var (
		transport  = flag.String("transport", "", "Transport: 'stdio' or 'http' (env: MCP_TRANSPORT)")
		addr       = flag.String("addr", "", "HTTP listen address (env: MCP_HTTP_ADDR)")
		snapshotID = flag.String("snapshot-id", "", "Dataset snapshot ID (env: DEFAULT_SNAPSHOT_VERSION)")
		algorithm  = flag.String("algorithm", "", "Algorithm version (env: ALGORITHM_VERSION)")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "tw-prop-mcp — Taiwan Real-Estate MCP Server v2.0.0")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: realestate-mcp [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Environment variables (applied when flag not set):")
		fmt.Fprintln(os.Stderr, "  MCP_TRANSPORT            stdio|http")
		fmt.Fprintln(os.Stderr, "  MCP_HTTP_ADDR            :8080")
		fmt.Fprintln(os.Stderr, "  DATABASE_URL             postgresql://user:pass@host:5432/db")
		fmt.Fprintln(os.Stderr, "  DEFAULT_SNAPSHOT_VERSION latest")
		fmt.Fprintln(os.Stderr, "  ALGORITHM_VERSION        comparable-v2.0")
		fmt.Fprintln(os.Stderr, "  CONFIGURATION_VERSION    v2.0")
		fmt.Fprintln(os.Stderr, "  OTEL_EXPORTER_OTLP_ENDPOINT http://localhost:4318")
		fmt.Fprintln(os.Stderr, "  LOG_LEVEL                info")
	}

	// Handle --version
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println("tw-prop-mcp v2.0.0")
			return
		}
	}
	flag.Parse()

	// Resolve transport from flag or env
	transportVal := *transport
	if transportVal == "" {
		transportVal = os.Getenv("MCP_TRANSPORT")
	}
	if transportVal == "" {
		transportVal = "http" // default
	}

	// Resolve HTTP address
	httpAddr := *addr
	if httpAddr == "" {
		httpAddr = os.Getenv("MCP_HTTP_ADDR")
	}
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	// Resolve snapshot and algorithm version
	snapID := *snapshotID
	if snapID == "" {
		snapID = os.Getenv("DEFAULT_SNAPSHOT_VERSION")
	}
	if snapID == "" {
		snapID = "latest"
	}

	algVersion := *algorithm
	if algVersion == "" {
		algVersion = os.Getenv("ALGORITHM_VERSION")
	}
	if algVersion == "" {
		algVersion = "comparable-v2.0"
	}

	configVersion := os.Getenv("CONFIGURATION_VERSION")
	if configVersion == "" {
		configVersion = "v2.0"
	}

	// Initialize OpenTelemetry tracer
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdown := mcp.InitTracer(ctx)
	defer shutdown(ctx)

	// Create MCP server
	srv := mcp.NewServer(mcp.ServerConfig{
		Name:                 "tw-prop-mcp",
		Version:              "v2.0.0",
		SnapshotID:           snapID,
		AlgorithmVersion:     algVersion,
		ConfigurationVersion: configVersion,
		DatabaseDSN:          os.Getenv("DATABASE_URL"),
		EnableHTTP:           true,
		HTTPAddr:             httpAddr,
		RequestIDHeader:      "X-Request-ID",
	})

	fmt.Fprintf(os.Stderr, "tw-prop-mcp: MCP server starting (transport=%s, addr=%s)\n", transportVal, httpAddr)

	switch transportVal {
	case "stdio":
		if err := srv.RunStdio(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "mcp: stdio server error: %v\n", err)
			os.Exit(1)
		}
	case "http":
		if err := srv.RunHTTP(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "mcp: http server error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "mcp: unknown transport %q (use 'stdio' or 'http')\n", transportVal)
		os.Exit(2)
	}
}
