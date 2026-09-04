# tw-prop-mcp — Kubernetes / OpenShift Deployment

> Taiwan Real-Estate Actual Transaction MCP Server — containerised deployment
> on Kubernetes (including OpenShift).

## Architecture

```
                    ┌─────────────┐
                    │ MCP Client  │
                    └──────┬──────┘
                           │
                     OpenShift Route
                    (or Ingress)
                           │
                    ┌──────▼──────┐
                    │ MCP Server  │  ← ServiceMonitor → Prometheus
                    │             │       /healthz   readiness/liveness
                    └──────┬──────┘
                           │
                 ┌─────────▼─────────┐
                 │ PostgreSQL 16     │  StatefulSet
                 │ + PostGIS 3.5     │  PVC-backed
                 └─────────┬─────────┘
                           │
                ┌──────────▼──────────┐
                │ Dataset migrations  │  ConfigMap (schema-migrations)
                └─────────────────────┘

   ┌────────────┐
   │  CronJob    │  weekly (Sun 02:00 UTC)
   └──────┬──────┘
          │
          ▼
   Official Data (MOI real-price registration)
          │
          ▼
   Import Pipeline
   (download → checksum → archive → parse → normalize
    → validate → deduplicate → import → lock snapshot)
```

## Components

| File | Kind | Purpose |
|------|------|---------|
| `namespace.yaml` | Namespace | Isolated namespace `tw-prop-mcp` |
| `configmap.yaml` | ConfigMap | Non-secret application configuration |
| `secret.yaml` | Secret | DB credentials, Google Maps API key (base64 placeholders) |
| `mcp-server-deployment.yaml` | Deployment | MCP server (2 replicas, non-root, probes, resource limits) |
| `mcp-server-service.yaml` | Service | ClusterIP exposing MCP server on port 8080 |
| `mcp-server-route.yaml` | Route | OpenShift Route — external HTTPS endpoint |
| `postgres-statefulset.yaml` | StatefulSet | PostgreSQL 16 + PostGIS 3.5 with init-migration container |
| `postgres-service.yaml` | Service | ClusterIP for PostgreSQL (port 5432) |
| `postgres-pvc.yaml` | PVC | 20 Gi persistent volume for PostgreSQL data |
| `migrations-configmap.yaml` | ConfigMap | SQL migration files injected into the init container |
| `cronjob-data-import.yaml` | CronJob | Weekly official data download + import pipeline |
| `servicemonitor.yaml` | ServiceMonitor | Prometheus metrics scraping for the MCP server |

## Image References

| Component | Image | Notes |
|-----------|-------|-------|
| MCP Server | `ghcr.io/tw-prop-mcp/server:latest` | Built from `Dockerfile` (Go 1.26 + alpine base) |
| PostgreSQL + PostGIS | `docker.io/postgis/postgis:16-3.5` | amd64 only — see arm64 note below |
| Frontend | `ghcr.io/tw-prop-mcp/frontend:latest` | Separate deployment in `frontend/` |

### arm64 Consideration

The official `postgis/postgis:16-3.5` image is currently **amd64-only**.
On arm64 clusters (Apple Silicon, AWS Graviton, etc.) use the Crunchy Data
image instead:

```yaml
image: docker.io/crunchydata/postgres:16.6
```

Install PostGIS 3.5 as a separate step in an init-container that runs
`CREATE EXTENSION IF NOT EXISTS postgis;` after the database starts.

## MCP Server Environment Variables

The MCP server reads all configuration from environment variables (see
`internal/config/config.go` and `internal/mcp/server.go`):

### Database
| Variable | Source | Description |
|----------|--------|-------------|
| `DB_HOST` | ConfigMap | PostgreSQL host (Service DNS) |
| `DB_PORT` | ConfigMap | PostgreSQL port (5432) |
| `DB_NAME` | ConfigMap | Database name (`prop`) |
| `DB_USER` | Secret | Database username |
| `DB_PASSWORD` | Secret | Database password |
| `DB_SSLMODE` | ConfigMap | SSL mode (`require` / `disable`) |
| `DATABASE_URL` | computed | Full pgx connection string |

### MCP Runtime
| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_HTTP_ADDR` | `:8080` | HTTP listen address |
| `MCP_TRANSPORT` | `http` | `http` (Streamable HTTP) or `stdio` |

### Observability
| Variable | Description |
|----------|-------------|
| `METRICS_ENABLED` | Enable Prometheus metrics |
| `METRICS_PATH` | `/metrics` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint |
| `OTEL_SERVICE_NAME` | `tw-prop-mcp-mcp-server` |

### Valuation / Algorithm
| Variable | Default | Description |
|----------|---------|-------------|
| `VALUATION_CONFIG_VERSION` | `v2.0` | Active valuation config version |
| `ALGORITHM_VERSION` | `comparable-v2.0` | Active algorithm version |
| `CONFIGURATION_VERSION` | `v2.0` | Configuration snapshot version |

### Data Sources
| Variable | Description |
|----------|-------------|
| `DATA_SOURCE_NAME` | `MOI` (Ministry of the Interior) |
| `MOI_DATA_BASE_URL` | Base URL for MOI real-price registration |
| `MOI_DATA_DOWNLOAD_URL` | Secret — full download URL |
| `NLSC_WFS_BASE_URL` | National Land Surveying GIS WFS endpoint |

## Data Import Pipeline

The **CronJob** (`cronjob-data-import.yaml`) runs weekly (Sunday 02:00 UTC)
and triggers the import pipeline:

1. **Download** — fetches the latest MOI real-price registration CSV archive
2. **Checksum** — verifies SHA256 against the official checksum
3. **Archive** — stores raw file immutably (`raw/source_snapshot/{id}/`)
4. **Snapshot** — creates a `dataset_snapshot` row with status `PENDING`
5. **Parse** — extracts CSV from the archive
6. **Normalize** — converts raw rows to domain objects
7. **Validate** — applies business-rule checks
8. **Deduplicate** — removes duplicates by `source_record_hash`
9. **Import** — bulk-copies into PostgreSQL via `pgx`
10. **Lock** — transitions snapshot status to `LOCKED` (immutable, per P2/P5)

## Deployment

### Prerequisites

- Kubernetes 1.26+ or OpenShift 4.12+
- PostgreSQL operator **not** required (StatefulSet is self-contained)
- Prometheus Operator (for ServiceMonitor)
- The `postgis/postgis:16-3.5` image (or arm64 equivalent)

### Apply

```bash
# 1. Create the namespace
kubectl apply -f deploy/base/namespace.yaml

# 2. Create secrets (update with real credentials first!)
kubectl apply -f deploy/base/secret.yaml

# 3. Create config and migrations
kubectl apply -f deploy/base/configmap.yaml
kubectl apply -f deploy/base/migrations-configmap.yaml

# 4. Deploy PostgreSQL
kubectl apply -f deploy/base/postgres-pvc.yaml
kubectl apply -f deploy/base/postgres-service.yaml
kubectl apply -f deploy/base/postgres-statefulset.yaml

# 5. Deploy MCP Server
kubectl apply -f deploy/base/mcp-server-service.yaml
kubectl apply -f deploy/base/mcp-server-deployment.yaml
kubectl apply -f deploy/base/mcp-server-route.yaml   # OpenShift only

# 6. Deploy data import CronJob and monitoring
kubectl apply -f deploy/base/cronjob-data-import.yaml
kubectl apply -f deploy/base/servicemonitor.yaml

# 7. (Optional) Deploy frontend
kubectl apply -f deploy/base/frontend-deployment.yaml
kubectl apply -f deploy/base/frontend-service.yaml
```

### Verify

```bash
# Check pods
kubectl -n tw-prop-mcp get pods

# Check health
kubectl -n tw-prop-mcp port-forward svc/mcp-server 8080:8080
curl http://localhost:8080/healthz

# Check MCP endpoint
curl http://localhost:8080/mcp
```

## Security Notes

- **Secrets use placeholder credentials** — replace `POSTGRES_PASSWORD`,
  `POSTGRES_SUPERUSER_PASSWORD`, and `GOOGLE_MAPS_API_KEY` before production.
- MCP server runs as **non-root** (UID 10001).
- PostgreSQL uses **non-root** (UID 26 / `postgres` user).
- TLS is enforced:
  - MCP server: edge-terminated at the OpenShift Route (HTTPS only).
  - PostgreSQL: `sslmode=require` (configure TLS in production).
- No real credentials are committed to the repository.
