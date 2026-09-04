# Taiwan Real-Estate Actual Transaction MCP

> Taiwan Ministry of the Interior real price registration data served as an MCP server. Deterministic, reproducible, and AI-isolated.

## Overview

tw-prop-mcp exposes Taiwan's official real-estate transaction data (MOI 實價登錄) through the [Model Context Protocol (MCP)](https://spec.modelcontextprotocol.io/). AI agents query **17 typed tools** covering transactions, parcels, GIS geometry, comparable analysis, valuation, and provenance — all backed by PostgreSQL + PostGIS with deterministic query hashing and AI isolation enforcement.

The server ensures:
- **Deterministic**: Same snapshot + query params + algorithm + config → same result (query hash verified)
- **AI Isolation**: Tool parameters are structured. SQL, PostGIS expressions, and valuation formulas are rejected
- **Artifact Locking**: Once locked, snapshots, algorithms, and configs cannot be modified (DB-level constraints)
- **Provenance**: Every result traces Transaction → Snapshot → Official Source

## Architecture

```
                    ┌──────────────────────────────────┐
                    │            MCP Client             │
                    │  (Claude, Cursor, custom agent)   │
                    └──────────┬───────────────────────┘
                               │ MCP over stdio or HTTP
                               ▼
                    ┌──────────────────────────────────┐
                    │     cmd/realestate-mcp            │
                    │  CLI flags + env var resolution   │
                    └──────────┬───────────────────────┘
                               │ initializes
                               ▼
                    ┌──────────────────────────────────┐
                    │    internal/mcp/*_tools.go       │
                    │  ┌────────┬────────────┐         │
                    │  │17 tools│5 resources │         │
                    │  │        │3 prompts   │         │
                    │  └────┬───┴─────┬──────┘         │
                    └──────┼─────────┼────────────────┘
                           │   calls   │
                           ▼           ▼
              ┌─────────────────┐ ┌─────────────────┐
              │  Service Layer  │ │  Repository     │
              │ valuation/      │ │ repository/     │
              │ statistics/     │ │ (sqlc gen)      │
              │ service/        │ │                 │
              └────────┬────────┘ └────────┬────────┘
                       │ uses           uses
                       ▼                 ▼
              ┌─────────────────┐ ┌─────────────────┐
              │  PostgreSQL     │ │  PostGIS 3.5    │
              │ 16 + PostGIS    │ │ (EPSG:3826→4326)│
              │ migrations/     │ │                 │
              └─────────────────┘ └─────────────────┘
```

### Components

| Layer | Package | Responsibility |
|-------|---------|----------------|
| Entry point | `cmd/realestate-mcp/main.go` | CLI flags, env resolution, OTel init, server bootstrap |
| MCP Interface | `internal/mcp/` | 17 tools, 5 resources, 3 prompts, AI isolation, observability, error model |
| Service | `internal/service/`, `internal/valuation/`, `internal/statistics/` | Business logic: comparable scoring, statistics, road access |
| Repository | `internal/repository/` | pgx/v5 + sqlc generated queries |
| Domain | `internal/domain/` | Core types: Transaction, Parcel, Valuation, Provenance, RoadAccess |
| Ingestion | `internal/downloader/`, `internal/importpipeline/` | MOI data download, parse, normalize, validate, import pipeline |
| GIS | `internal/gis/` | Coordinate transform (EPSG:3826 ↔ 4326), geometry adapter |
| Frontend | `frontend/` | React + TypeScript + Google Maps (separate Dockerfile) |

## Features

### 17 MCP Tools

**Transaction Tools** (`internal/mcp/transaction_tools.go`)
| Tool | Description |
|------|-------------|
| `search_transactions` | Filter by county/district/section, price range, date range |
| `get_transaction` | Get single transaction by UUID |
| `get_transaction_statistics` | Min/P25/median/mean/P75/P90/max for a geographic area |

**Parcel Tools** (`internal/mcp/parcel_tools.go`)
| Tool | Description |
|------|-------------|
| `get_parcel` | Get parcel by UUID |
| `search_parcels` | Search parcels by section + land number |

**Comparable Tools** (`internal/mcp/comparable_tools.go`)
| Tool | Description |
|------|-------------|
| `find_comparable_transactions` | Find and score comparable transactions |
| `score_comparable_transactions` | Score specific transactions as comparables |

**GIS Tools** (`internal/mcp/gis_tools.go`)
| Tool | Description |
|------|-------------|
| `get_parcel_geometry` | WKT geometry (EPSG:4326) |
| `get_parcel_location` | Centroid, bbox, coordinates |
| `check_road_access` | Road adjacency classification (4 types) |
| `find_nearby_roads` | Roads within search radius |
| `get_parcel_map_context` | Combined parcel + roads + comparables for map display |

**Valuation Tools** (`internal/mcp/valuation_tools.go`)
| Tool | Description |
|------|-------------|
| `estimate_land_value` | Bear/base/bull estimation with confidence |
| `estimate_property_value` | Land + building valuation |
| `explain_valuation` | Human-readable valuation explanation |

**Provenance Tools** (`internal/mcp/provenance_tools.go`)
| Tool | Description |
|------|-------------|
| `get_data_snapshot` | Snapshot metadata (source, version, record count, status) |
| `get_data_provenance` | Full provenance chain for any result |

### 5 MCP Resources (`internal/mcp/resources.go`)
- `realestate://snapshot/{snapshot_id}` — dataset snapshot metadata
- `realestate://transaction/{transaction_id}` — transaction provenance
- `realestate://parcel/{parcel_id}` — parcel geometry + ownership
- `realestate://valuation/{valuation_id}` — full valuation result
- `realestate://algorithm/{version}` — algorithm config + weights

### 3 MCP Prompts (`internal/mcp/prompts.go`)
| Prompt | Purpose |
|--------|---------|
| `prompt_explain_valuation` | Explain valuation methodology after `estimate_land_value` |
| `prompt_analyze_comparables` | Structured comparable transaction analysis |
| `prompt_debug_transaction` | Diagnose unexpected query results |

### Data Pipeline (`internal/importpipeline/pipeline.go`)
1. Download — fetch CSV from MOI (`--auto` discovers latest URL)
2. Verify checksum — SHA256 validation of source file
3. Parse — CSV → intermediate rows (`internal/parser/`)
4. Enrich — derive county/district from filename
5. Normalize — clean + standardize fields (`internal/normalizer/`)
6. Validate — quality checks (`internal/validator/`)
7. Deduplicate — remove duplicate records
8. Import — `BEGIN` / `COMMIT` / `ROLLBACK` transactional batch insert
9. Lock — snapshot transition to LOCKED (immutable)

### Key Principles
- **Deterministic**: Query hash = `canonicalize(snapshot_id, query_params, algorithm_version, config_version)`
- **AI Isolation**: `ProhibitedFields` validates all tool inputs — rejects `sql`, `where`, `postgis`, `valuation_formula`, `weights` (P4)
- **Artifact Lock**: Migrations 002-004 create DB-level triggers enforcing immutability of snapshots, configs, and raw data (P5)
- **Provenance**: All results include `source`, `snapshot_id`, `source_record_hash`, `import_batch_id` (P6)

## Project Structure

```
tw-prop-mcp/
├── cmd/realestate-mcp/main.go       # Entry point: CLI flags, env, OTel, server
├── internal/
│   ├── clock/                        # Clock abstraction for testing
│   ├── comparable/                   # Comparable engine core
│   │   └── engine.go                 # Filtering + scoring (hardcoded weights)
│   ├── config/                       # Server config + DB connection
│   ├── domain/                       # Core domain models
│   │   ├── transaction.go
│   │   ├── parcel.go
│   │   ├── valuation.go
│   │   ├── comparable.go
│   │   ├── statistics.go
│   │   ├── parcel_road_access.go
│   │   ├── road_segment.go
│   │   ├── provenance.go
│   │   └── snapshot.go
│   ├── downloader/                   # MOI data download + archiving
│   │   ├── downloader.go             # HTTP downloader with retry
│   │   ├── discover.go               # Auto-discover latest URL from landing page
│   │   ├── checksum.go               # SHA256 checksum verification
│   │   ├── archive.go                # Archive management
│   │   └── snapshot.go               # Idempotent snapshot logic
│   ├── gis/                          # GIS adapters and transforms
│   │   ├── transform.go              # EPSG:3826 ↔ 4326
│   │   ├── adapter.go                # PostGIS geometry adapter
│   │   └── importer.go               # Geometry import utilities
│   ├── importpipeline/               # Full import pipeline
│   │   └── pipeline.go               # 9-stage pipeline (download→lock)
│   ├── mcp/                          # MCP server, tools, resources, prompts
│   │   ├── server.go                 # Server bootstrap, HTTP/stdio transport
│   │   ├── transaction_tools.go
│   │   ├── parcel_tools.go
│   │   ├── comparable_tools.go
│   │   ├── gis_tools.go
│   │   ├── valuation_tools.go
│   │   ├── provenance_tools.go
│   │   ├── resources.go              # 5 realestate:// resources
│   │   ├── prompts.go                # 3 prompt templates
│   │   ├── errors.go                 # ProhibitedFields, McpError
│   │   ├── instrument.go             # Provenance injection, query hash
│   │   └── observability.go          # Metrics, tracing, request logging
│   ├── normalizer/                   # CSV normalization
│   │   ├── normalizer.go
│   │   ├── transaction.go
│   │   └── parcel.go
│   ├── parser/                       # CSV parsing
│   │   ├── parser.go
│   │   ├── fieldmap.go
│   │   └── encoding.go
│   ├── provenance/                   # Hash generation + service
│   │   ├── hash.go
│   │   └── service.go
│   ├── repository/                   # Data access (sqlc generated)
│   │   ├── db/                       # SQLC output + Queries
│   │   ├── transaction.go
│   │   ├── parcel.go
│   │   ├── snapshot.go
│   │   ├── road_segment.go
│   │   ├── parcel_road_access.go
│   │   └── valuation_result.go
│   ├── service/                      # Business logic
│   │   ├── transaction.go
│   │   ├── road_access.go
│   └── statistics/                  # Statistics engine
│       └── engine.go                # Percentile calculations
├── migrations/                        # golang-migrate SQL migrations
├── sqlc.yaml                         # SQLC configuration
├── Dockerfile                        # Multi-stage (golang:1.26-alpine → alpine)
├── Dockerfile.frontend               # Frontend build (node:20-alpine → nginx)
├── Makefile                          # Build, test, lint, migrate targets
├── frontend/                         # React + TypeScript + Google Maps
├── tests/                            # Test suites
│   ├── contract/                     # MCP contract tests
│   ├── e2e/                          # End-to-end acceptance tests
│   ├── integration/                  # Integration tests (PostgreSQL)
│   ├── isolation/                    # AI injection prevention tests
│   ├── reproducibility/              # Determinism + query hash tests
│   └── golden/                       # Golden dataset for regression
├── scripts/verify.sh                 # 16-step automated verification
├── SPEC.md                           # System spec (P1-P6 principles)
├── DATA_MODEL.md                     # Database schema + domain models
├── MCP_API.md                        # MCP tools, resources, error model
├── GIS_SPEC.md                       # Coordinate system + road access
├── VALUATION_SPEC.md                 # Comparable + statistics + valuation
├── IMPLEMENTATION_PLAN.md            # 18-stage implementation plan
└── go.mod
```

## Requirements

### Runtime
- **Go**: 1.26+
- **PostgreSQL**: 16+ with PostGIS 3.5+ extension
- **Environment**: Any (Docker recommended for PostgreSQL)

### External Dependencies
- **MOI Real Price Registration**: `https://plvr.land.moi.gov.tw/` — data source
- **OpenTelemetry Collector** (optional): for span/metrics collection via `OTEL_EXPORTER_OTLP_ENDPOINT`
- **Google Maps API** (frontend only): required for map rendering in `frontend/`

## Installation

### From Source

```bash
# 1. Clone
git clone <repo-url>
cd tw-prop-mcp

# 2. Ensure Go 1.26+
go version  # should show 1.26.x

# 3. Build
make build  # outputs to bin/realestate-mcp
# Or: go build -o bin/realestate-mcp ./cmd/realestate-mcp
```

### Docker

```bash
# Main server (Go + Alpine, non-root)
docker build -t tw-prop-mcp .

# Frontend (React + nginx)
docker build -f Dockerfile.frontend -t tw-prop-mcp-frontend frontend/
```

### Run with Docker Compose

```bash
# Start PostgreSQL + PostGIS
docker compose up -d postgres

# Run server against local database
docker run -it --network host \
  -e DATABASE_URL=postgresql://prop:prop_dev_only@localhost:5432/prop \
  -e MCP_HTTP_ADDR=:8080 \
  -e MCP_TRANSPORT=http \
  tw-prop-mcp
```

## Configuration

### Server Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_TRANSPORT` | `http` | Transport: `stdio` or `http` |
| `MCP_HTTP_ADDR` | `:8080` | HTTP listen address (HTTP mode only) |
| `DATABASE_URL` | — | PostgreSQL connection string (`postgresql://user:pass@host:5432/db`) |
| `DEFAULT_SNAPSHOT_VERSION` | `latest` | Default snapshot ID for queries |
| `ALGORITHM_VERSION` | `comparable-v2.0` | Algorithm version for query hashing |
| `CONFIGURATION_VERSION` | `v2.0` | Configuration version for query hashing |
| `DATA_IMPORT_URL` | — | Direct download URL for data import |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OpenTelemetry OTLRP HTTP endpoint (e.g. `http://localhost:4318`) |
| `LOG_LEVEL` | `info` | Log level (info, debug, warn, error) |

### CLI Flags

| Flag | Env Var | Default | Description |
|-----|---------|---------|-------------|
| `--transport` | `MCP_TRANSPORT` | `http` | `stdio` or `http` |
| `--addr` | `MCP_HTTP_ADDR` | `:8080` | HTTP listen address |
| `--snapshot-id` | `DEFAULT_SNAPSHOT_VERSION` | `latest` | Default dataset snapshot |
| `--algorithm` | `ALGORITHM_VERSION` | `comparable-v2.0` | Algorithm version |
| `--data-url` | `DATA_IMPORT_URL` | — | Direct download URL |
| `--auto` | — | `false` | Auto-discover latest data URL from MOI landing page |

### Data Import

```bash
# Auto-discover latest MOI data URL and import
./bin/realestate-mcp --auto --data-url "" --snapshot-id 2025Q1

# Or specify a direct URL
./bin/realestate-mcp --data-url https://plvr.land.moi.gov.tw/Download/GetFile?type=csv&id=XXXX
```

## Quick Start

### 1. Start PostgreSQL with PostGIS

```bash
docker run -d --name postgres \
  -e POSTGRES_DB=prop \
  -e POSTGRES_USER=prop \
  -e POSTGRES_PASSWORD=prop_dev_only \
  -p 5432:5432 \
  postgis/postgis:16-3.5

# Run migrations
go run ./cmd/migrate up  # or: migrate -path migrations -database "postgresql://prop:prop_dev_only@localhost:5432/prop?sslmode=disable" up
```

### 2. Import Data

```bash
# Auto-discover latest URL from MOI landing page and import
export DATABASE_URL=postgresql://prop:prop_dev_only@localhost:5432/prop
./bin/realestate-mcp --auto --snapshot-id 2025Q1
```

### 3. Start the MCP Server

```bash
# HTTP mode (for remote clients)
export MCP_TRANSPORT=http
export MCP_HTTP_ADDR=:8080
export DATABASE_URL=postgresql://prop:prop_dev_only@localhost:5432/prop
./bin/realestate-mcp

# Stdio mode (for Claude Desktop)
export MCP_TRANSPORT=stdio
export DATABASE_URL=postgresql://prop:prop_dev_only@localhost:5432/prop
./bin/realestate-mcp
```

### 4. Query Transactions

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "search_transactions",
      "arguments": {
        "county": "澎湖縣",
        "district": "西嶼鄉",
        "limit": 10
      }
    }
  }'
```

## MCP API

### Tool Schema

All tools use typed input/output via `mcpapi.AddTool` generics. Each response includes `metadata` with:
- `query_hash` — deterministic hash of (snapshot + params + algorithm + config)
- `snapshot_id` — dataset version the result comes from
- `algorithm_version` — algorithm used
- `configuration_version` — config version used

### Error Model (`internal/mcp/errors.go`)

| Code | Meaning |
|------|---------|
| `INVALID_ARGUMENT` | Parameter validation failed |
| `PARCEL_NOT_FOUND` | Parcel UUID not in database |
| `TRANSACTION_NOT_FOUND` | Transaction UUID not found |
| `DATA_NOT_AVAILABLE` | No data for the query parameters |
| `GIS_NOT_AVAILABLE` | PostGIS geometry operation failed |
| `SNAPSHOT_NOT_FOUND` | Snapshot ID not found |
| `VALUATION_NOT_AVAILABLE` | Insufficient comparables for valuation |
| `SOURCE_UNAVAILABLE` | Official data source unreachable |
| `INTERNAL_ERROR` | Server-side exception |

### AI Isolation (P4)

All tool handlers validate raw arguments against `ProhibitedFields` before any business logic:

```go
blockedFields := []string{"sql", "where", "postgis", "valuation_formula", "weights", "expression", "raw_sql"}
```

Rejected input returns `INVALID_ARGUMENT` error. This prevents LLM prompt injection from escalating to SQL or custom query execution.

## Data Model

Core entities (see `internal/domain/`):

- **Transaction**: `total_price`, `unit_price` as `int64` (no float for currency), `section`/`land_number` as parcel identity key
- **Parcel**: geometry in EPSG:3826 (PostGIS), output converted to EPSG:4326
- **ValuationResult**: Bear (P25), Base (P50), Bull (P75) values + Confidence level (HIGH/MEDIUM/LOW/INSUFFICIENT)
- **ComparableCandidate**: scored transaction with `area_similarity`, `distance_meters`, `time_weight`, `zoning_match`, `land_use_match`, `road_access_match`

### Parcel Identity (4-key)
A parcel is uniquely identified by: `county + district + section + land_number`

### Statistics
`statistics/engine.go` computes:
- Percentiles: P0/P10/P25/median(mean)/P75/P90/P100
- Outliers: IQR method with configurable k-factor
- Area conversion: 1 坪 = 3.305785 m²

## Valuation Engine

The valuation engine (`internal/valuation/engine.go`) implements:

1. **Comparable filtering**: Same county → district → section → building type
2. **Distance scoring**: Linear distance to road + parcel centroid proximity
3. **Time weight**: Exponential decay `exp(-lambda * days_since_sale)`
4. **Area similarity**: `min(area_a, area_b) / max(area_a, area_b)`
5. **Outlier removal**: IQR method (k=1.5) on comparable unit prices
6. **Bear/Base/Bull**: P25, P50 (weighted median), P75 of outlier-adjusted comparables
7. **Confidence**: HIGH (≥5 comparables), MEDIUM (≥2), LOW (<2) or INSUFFICIENT

## Error Handling

- All MCP tool errors return structured `McpError` with code + retryable flag
- Database errors wrapped with stage context via `ImportPipelineError`
- Network errors (HTTP 5xx, timeouts) are marked retryable in the import pipeline

## Logging and Observability

### Prometheus Metrics (`internal/mcp/observability.go`)
- `mcp_requests_total` — per-tool request counter
- `mcp_request_duration_seconds` — histogram per tool
- `transaction_query_total` — transaction query counter
- `gis_query_total` — GIS query counter
- `comparable_query_total` — comparable query counter
- `valuation_query_total` — valuation query counter
- `data_import_total` — import success/fail counter
- `snapshot_locked_total` — snapshot lock counter

### OpenTelemetry
- OTLP HTTP exporter via `OTEL_EXPORTER_OTLP_ENDPOINT` (default: `http://localhost:4318`)
- `BatchSpanProcessor` for buffered export
- Service name: `tw-prop-mcp`
- Falls back to no-op tracer when endpoint not configured

### Structured Logging
- Request-level logging with `request_id`, `tool_name`, `snapshot_id`, `query_hash`
- Import pipeline logs each stage with structured fields

### HTTP Endpoints (HTTP transport only)
- `/mcp` — MCP Streamable HTTP endpoint
- `/healthz` — Liveness probe
- `/readyz` — Readiness probe
- `/metrics` — Prometheus metrics

## Testing

```bash
# Unit tests (no PostgreSQL required)
go test ./internal/... -count=1

# Exclude config tests (require PostgreSQL container)
make test

# Integration tests (require PostgreSQL + PostGIS)
make test-integration
go test -tags=integration ./tests/integration/... -count=1

# E2E acceptance tests
go test -tags=e2e ./tests/e2e/... -count=1

# Contract tests
go test ./tests/contract/... -count=1

# Isolation tests (AI injection prevention)
go test ./tests/isolation/... -count=1

# Reproducibility tests (determinism verification)
go test ./tests/reproducibility/... -count=1

# Artifact lock tests
go test -tags=integration ./tests/artifact_lock/... -count=1

# Benchmarks (real MOI data)
go test -bench=. -benchmem ./internal/importpipeline/...

# Full verification suite
bash scripts/verify.sh  # 20 steps, runs all above
```

### Test Counts
- Unit tests: 244 pass (2 config tests fail without PostgreSQL container)
- Integration: 10 pass
- E2E: 7 pass
- Contract: 12 pass
- Isolation: 11 pass
- Reproducibility: 16 pass
- Artifact lock: 10 pass
- Benchmarks: 7 pass

## Build

```bash
make build          # go build -o bin/realestate-mcp ./cmd/realestate-mcp
make vet            # go vet ./...
make lint           # golangci-lint run ./... || go vet ./...
make sqlc           # Regenerate sqlc code
make tidy           # go mod tidy
```

## Deployment

### Docker

```bash
# Build multi-stage image (golang:1.26-alpine → alpine:latest, non-root)
docker build -t tw-prop-mcp .

# Run
docker run -p 8080:8080 \
  -e DATABASE_URL=postgresql://prop:prop_dev_only@db:5432/prop \
  -e MCP_TRANSPORT=http \
  tw-prop-mcp
```

### Frontend (React + Google Maps)

```bash
# Build and serve
docker build -f Dockerfile.frontend -t tw-prop-mcp-frontend .
docker run -p 80:80 tw-prop-mcp-frontend
```

### Health Checks
- `GET /healthz` — liveness (always 200)
- `GET /readyz` — readiness (returns `{"status":"ready"}`)
- `GET /metrics` — Prometheus metrics

> **Note**: `/readyz` returns a static OK without DB connectivity check. This is a known limitation — [NEEDS VERIFICATION] for production readiness.

## Development

### Adding a New MCP Tool

1. Define input/output structs with typed fields
2. Implement handler function matching `mcpapi.ToolHandler` signature
3. Register via `mcpapi.AddTool(srv, &mcpapi.Tool{...}, handler)` in the appropriate `*_tools.go` file
4. Add provenance injection via `instrument.go`
5. Add to `tests/contract/contract_test.go`

### Code Style
- `gofmt` / `goimports` formatting
- `golangci-lint` for static analysis
- Test coverage target: ≥ 80%

### Git Conventions
- Conventional Commits (`feat:`, `fix:`, `docs:`, etc.)
- One task per commit
- `main` branch protected

## Limitations

- **No historical data import**: `--auto` fetches only the latest MOI snapshot. Historical data backfill requires manual URL enumeration
- **Single snapshot**: Current import pipeline overwrites the latest snapshot. Multi-snapshot retention needs additional import batch IDs
- **`/readyz` health check**: Does not verify database connectivity — returns static OK
- **`DATABASE_URL` required**: Server cannot start without a database connection
- **Frontend integration unverified**: React frontend builds correctly (`tsc --noEmit` passes) but browser-level integration with MCP server is not tested in CI
- **PostGIS dependency**: All GIS operations require PostGIS extension. Without it, `check_road_access` and geometry tools may fail
- **No rate limiting**: MCP server does not implement request rate limiting. AI agents making many concurrent calls may overwhelm the database

## License

Licensed under the **Apache License 2.0**. See [`LICENSE`](LICENSE) for details.

> This project is for research and reference purposes only. Data sourced from the Ministry of the Interior's real price registration provides. Use of this data must comply with the relevant laws and regulations of Taiwan.
