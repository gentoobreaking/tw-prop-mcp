# Taiwan Real-Estate MCP Server

> Taiwan Ministry of the Interior real price registration data (實價登錄) served through the [Model Context Protocol (MCP)](https://spec.modelcontextprotocol.io/). Deterministic, reproducible, and AI-isolated.

## Overview

`tw-prop-mcp` exposes Taiwan's official real-estate transaction data through a typed MCP server. Clients (Claude, Cursor, custom agents) query 17 tools covering transactions, parcels, GIS geometry, comparable analysis, valuation, and provenance — all backed by PostgreSQL + PostGIS with deterministic query hashing and AI isolation enforcement.

Design principles ([SPEC.md](SPEC.md)):

- **P1 Deterministic**: Same snapshot + params + algorithm + config → same output (query hash verified)
- **P2 Immutable Raw Data**: Official source data is read-only archive; snapshots become lockable
- **P4 AI Isolation**: Tool parameters are structured — SQL, PostGIS expressions, and valuation formulas are rejected
- **P5 Artifact Locking**: DB-level constraints prevent mutation of locked snapshots, algorithms, configs
- **P6 Provenance**: Every result traces Transaction → Snapshot → Official Source

## Architecture

```
                 ┌─────────────────────┐
                 │    MCP Client       │  (Claude, Cursor, custom agent)
                 └─────────┬───────────┘
                           │  MCP-over-HTTP (SSE) or stdio
                           ▼
                 ┌─────────────────────┐
                 │  cmd/realestate-mcp │  CLI flags, env, OTel init
                 └─────────┬───────────┘
                           │ initializes
                           ▼
                 ┌─────────────────────┐
                 │  internal/mcp/      │  17 tools · 5 resources · 3 prompts
                 │  *_tools.go         │  AI isolation · provenance injection
                 └────┬──────────┬─────┘
                      │         │
                 uses  │         │ uses
                      ▼         ▼
         ┌─────────────────┐ ┌──────────────────┐
         │  Service Layer  │ │  Repository      │
         │  valuation/     │ │  (sqlc gen)      │
         │  statistics/    │ │  pgx/v5          │
         └────────┬────────┘ └────────┬────────┘
                  │ uses            uses
                  ▼                 ▼
         ┌─────────────────┐ ┌──────────────────┐
         │  PostgreSQL 16  │ │  PostGIS 3.6     │
         │  + PostGIS      │ │  EPSG:3826↔4326  │
         └─────────────────┘ └──────────────────┘
```

### Components

| Layer | Package | Responsibility |
|-------|---------|----------------|
| Entry point | `cmd/realestate-mcp/main.go` | CLI flags, env resolution, OTel init, server bootstrap |
| MCP Interface | `internal/mcp/` | 17 tools, 5 resources, 3 prompts, AI isolation, observability, error model |
| Service | `internal/service/`, `internal/valuation/`, `internal/statistics/` | Business logic: comparable scoring, statistics, road access |
| Repository | `internal/repository/` | pgx/v5 + sqlc generated queries |
| Domain | `internal/domain/` | Core types: Transaction, Parcel, Valuation, Provenance |
| GIS | `internal/gis/` | Coordinate transform (EPSG:3826 ↔ 4326) |
| Ingestion | `internal/downloader/`, `internal/importpipeline/` | MOI data download → parse → normalize → validate → import |
| Frontend | `frontend/` | React + TypeScript — Leaflet (default) or Google Maps, MCP client |

## Quick Start

```bash
# 1. Create .env (secrets never committed)
cp env.example .env
# Edit .env — optionally set GOOGLE_MAPS_API_KEY for Google Maps provider

# 2. Build and start all services
docker compose up -d --build

# 3. Verify
curl http://localhost/healthz     # {"status":"ok"}
open http://localhost/            # Frontend map UI

# 4. Check PostGIS extension
docker exec tw-prop-postgres psql -U prop -d prop -c "SELECT extname,extversion FROM pg_extension WHERE extname='postgis';"
```

## MCP Tools

### Transaction Tools
| Tool | Description |
|------|-------------|
| `search_transactions` | Filter by county/district/section, price range, date range, land/area type |
| `get_transaction` | Single transaction by UUID |
| `get_transaction_statistics` | Min/P25/median/mean/P75/P90/max for a geographic area |

### Parcel Tools
| Tool | Description |
|------|-------------|
| `get_parcel` | Parcel by county/district/section/land_number |
| `search_parcels` | Search by section + land number |

### Comparable Tools
| Tool | Description |
|------|-------------|
| `find_comparable_transactions` | Find and score comparable transactions |
| `score_comparable_transactions` | Score specific transactions as comparables |

### GIS Tools
| Tool | Description |
|------|-------------|
| `get_parcel_geometry` | WKT geometry in EPSG:4326 |
| `get_parcel_location` | Centroid lat/lng, bbox, map context |
| `check_road_access` | Road adjacency classification (ROAD_ADJACENT/ROAD_NEARBY/NO_ROAD_DETECTED/UNKNOWN) |
| `find_nearby_roads` | Roads within search radius |
| `get_parcel_map_context` | Combined parcel + roads + comparables for map display |

### Valuation Tools
| Tool | Description |
|------|-------------|
| `estimate_land_value` | Bear/base/bull estimation with confidence |
| `estimate_property_value` | Land + building valuation |
| `explain_valuation` | Human-readable valuation explanation |

### Provenance Tools
| Tool | Description |
|------|-------------|
| `get_data_snapshot` | Snapshot metadata (source, version, record count) |
| `get_data_provenance` | Full provenance chain for any result |

See [MCP_API.md](MCP_API.md) for full input/output schemas.

## MCP Resources
- `realestate://snapshot/{snapshot_id}` — dataset snapshot metadata
- `realestate://transaction/{transaction_id}` — transaction provenance
- `realestate://parcel/{parcel_id}` — parcel geometry + ownership
- `realestate://valuation/{valuation_id}` — full valuation result
- `realestate://algorithm/{version}` — algorithm config + weights

## MCP Prompts
- `prompt_explain_valuation` — Explain methodology after `estimate_land_value`
- `prompt_analyze_comparables` — Structured comparable analysis
- `prompt_debug_transaction` — Diagnose unexpected query results

## Configuration

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | (required) | PostgreSQL DSN |
| `MCP_TRANSPORT` | `http` | `http` or `stdio` |
| `MCP_HTTP_ADDR` | `:8080` | HTTP listen address |
| `LOG_LEVEL` | `info` | `debug\|info\|warn\|error` |
| `DEFAULT_SNAPSHOT_VERSION` | `latest` | Default snapshot for queries |
| `ALGORITHM_VERSION` | `comparable-v2.0` | Default algorithm version |
| `CONFIGURATION_VERSION` | `v2.0` | Default valuation config version |
| `GOOGLE_MAPS_API_KEY` | (build-time) | Google Maps JS API key (optional — Leaflet is default) |
| `MAP_PROVIDER` | `leaflet` | `leaflet` or `google` (runtime, frontend only) |
| `MCP_SERVER_URL` | `/mcp` | MCP server URL for frontend (runtime, injected via nginx) |

### Docker Compose

```bash
docker compose up -d --build
```

Services:

| Service | Port | Description |
|---------|------|-------------|
| `tw-prop-postgres` | 5432 | PostgreSQL 16 + PostGIS 3.6 (arm64 Alpine) |
| `tw-prop-mcp` | 8080 | Go MCP server (HTTP transport) |
| `tw-prop-frontend` | 80 | React + nginx frontend |

Frontend `runtime-config.js` is generated at container start by nginx entrypoint hook — config changes do not require rebuild.

### Frontend Runtime Config

`runtime-config.js` is injected at container start:
```json
{
  "MCP_SERVER_URL": "/mcp",
  "MAP_PROVIDER": "leaflet"
}
```

Switching to Google Maps:
```bash
# Edit .env
MAP_PROVIDER=google
GOOGLE_MAPS_API_KEY=your_real_api_key_here
# Restart
docker compose up -d --build tw-prop-frontend
```

## Data Pipeline

The import pipeline ([SPEC.md](SPEC.md) §P2):

1. **Download** — fetch CSV from MOI `plvr.land.moi.gov.tw`
2. **Verify checksum** — SHA256 validation
3. **Parse** — CSV → intermediate rows
4. **Enrich** — derive county/district from filename
5. **Normalize** — clean + standardize fields
6. **Validate** — quality checks
7. **Deduplicate** — remove duplicate records
8. **Import** — transactional batch insert (`BEGIN`/`COMMIT`)
9. **Lock** — snapshot transition to LOCKED (immutable)

```
make migrate       # Run database migrations
make seed          # Load sample data (for development)
make verify        # Run 16-step automated verification suite
```

## Coordinate Systems

- **Stored**: EPSG:3826 (Taiwan Zone 2, TWD97) — PostGIS native
- **API Output**: EPSG:4326 (WGS84) — via `ST_Transform(geometry, 4326)`
- Transform layer in `internal/gis/transform.go`

## Error Model

MCP tool errors follow structured format:
```json
{
  "error": {
    "code": "PARCEL_NOT_FOUND",
    "message": "...",
    "retryable": false
  }
}
```

Error codes: `INVALID_ARGUMENT`, `PARCEL_NOT_FOUND`, `TRANSACTION_NOT_FOUND`, `DATA_NOT_AVAILABLE`, `GIS_NOT_AVAILABLE`, `SNAPSHOT_NOT_FOUND`, `VALUATION_NOT_AVAILABLE`, `SOURCE_UNAVAILABLE`, `INTERNAL_ERROR`.

## Testing

```bash
# All tests
make test

# Test categories
go test ./tests/isolation/...    # AI injection prevention (P4)
go test ./tests/reproducibility/ # Determinism + query hash
go test ./tests/contract/...     # MCP contract tests
go test ./tests/integration/...  # PostgreSQL integration
go test ./tests/e2e/...          # End-to-end acceptance
```

Test coverage is enforced at 80% minimum. See `scripts/verify.sh` for full 16-step verification.

## Build

```bash
make build    # Go binary: bin/realestate-mcp
make docker   # Build Docker images
```

Frontend:
```bash
cd frontend
npm ci
npm run dev    # http://localhost:5173
npm run build  # Production build to dist/
```

## Deployment

### Docker Compose (Development)
```bash
docker compose up -d --build
```

### Kubernetes/OpenShift (Production)
Reference architecture documented in [SPEC.md](SPEC.md) — requires PostgreSQL 16+ with PostGIS 3.5+, persistent volumes, and OpenTelemetry collector.

## Troubleshooting

**Parcels not rendering on map**:
- Ensure sample data loaded: `docker compose exec tw-prop-postgres psql -U prop -d prop -c "SELECT COUNT(*) FROM parcel;"`
- Check PostGIS: `docker exec tw-prop-postgres psql -U prop -d prop -c "SELECT extname FROM pg_extension;"`
- Verify MCP connectivity: `curl http://localhost/healthz`

**MCP connection errors**:
- Frontend connects via `/mcp` (nginx proxy — same origin)
- Direct server: `http://localhost:8080/mcp`
- Use `MCP_TRANSPORT=stdio` for local development without HTTP

**WKT geometry not rendering**:
- LeafletParcelLayer parses WKT MULTIPOLYGON from MCP `/mcp` endpoint
- Verify geometry coordinates are EPSG:4326 (WGS84 lat/lng)

## Limitations

- Sample data must be loaded separately (`make seed` or SQL import) — production requires full MOI dataset import via pipeline
- Google Maps provider requires valid API key; Leaflet + OSM is default
- K8s/OpenShift deployment manifests not included in repo — documented in SPEC.md only
- Frontend parcel search UI not yet implemented (hardcoded sample parcel in `useMCP.ts`)

## Development

```bash
# Prerequisites: Go 1.26+, PostgreSQL 16+, Docker
git clone <repo-url>
cd tw-prop-mcp
make deps      # Install Go tooling
make migrate   # Database migrations
make seed      # Sample data
make test      # All tests
```

## License

This project is licensed under the Apache License 2.0 — see [LICENSE](LICENSE).

Data sourced from Taiwan Ministry of the Interior Real Price Registration (實價登錄). Use of this data must comply with relevant laws and regulations of Taiwan.
