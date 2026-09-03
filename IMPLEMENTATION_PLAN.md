# Chapter 6 — IMPLEMENTATION_PLAN.md
# 實作順序

---

# Phase 0 — Repository Bootstrap

建立：

```text
SPEC.md
DATA_MODEL.md
MCP_API.md
GIS_SPEC.md
VALUATION_SPEC.md
IMPLEMENTATION_PLAN.md
```

建立：

```text
go.mod
Makefile
Dockerfile
README.md
```

完成：

```bash
go test ./...
go vet ./...
go build ./...
```

Acceptance:

```text
BUILD PASS
TEST PASS
```

---

# Phase 1 — PostgreSQL/PostGIS

建立：

```text
PostgreSQL
PostGIS
migration framework
sqlc
```

建立：

```text
dataset_snapshot
import_batch
transaction
parcel
```

Acceptance:

```text
migration up/down
schema test
constraint test
```

---

# Phase 2 — Official Data Downloader

實作：

```text
Downloader
Checksum
Snapshot
Archive
```

流程：

```text
download
 ↓
sha256
 ↓
store raw
 ↓
create snapshot
```

Acceptance：

```text
same source
same checksum
same snapshot
```

---

# Phase 3 — Parser / Normalizer

建立：

```text
parser
normalizer
validator
```

處理：

```text
CSV
encoding
欄位名稱
日期
價格
面積
地段
地號
使用分區
使用地類別
```

Acceptance：

```text
known sample dataset
→ expected normalized records
```

---

# Phase 4 — Transaction Engine

實作：

```text
search transaction
get transaction
statistics
```

SQL 必須經：

```text
sqlc
```

禁止：

```text
dynamic SQL from AI
```

Acceptance：

```text
query result deterministic
```

---

# Phase 5 — Parcel / GIS

建立：

```text
parcel
geometry
coordinate transformation
```

導入：

```text
official GIS source
```

Acceptance：

```text
known parcel
→ correct geometry
→ correct centroid
```

---

# Phase 6 — Road Access

實作：

```text
nearest road
road distance
road adjacency
road width
```

Acceptance：

測試：

```text
ROAD_ADJACENT
ROAD_NEARBY
NO_ROAD_DETECTED
UNKNOWN
```

四種 case 都必須存在。

---

# Phase 7 — Comparable Engine

實作：

```text
hard filter
area score
time score
distance score
zoning score
land-use score
road score
total score
```

Acceptance：

給定固定 snapshot：

```text
query
→ fixed comparable list
→ fixed scores
```

---

# Phase 8 — Statistics

實作：

```text
min
P10
P25
median
mean
P75
P90
max
```

建立 regression tests。

---

# Phase 9 — Valuation Engine

實作：

```text
bear
base
bull
confidence
```

建立：

```text
valuation_config
algorithm_version
```

Acceptance：

```text
same snapshot
same config
same algorithm
→ same valuation
```

---

# Phase 10 — MCP Server

使用官方 Go MCP SDK：

```text
github.com/modelcontextprotocol/go-sdk/mcp
```

SDK 官方 quick start 即採 `mcp.NewServer()`、`mcp.AddTool()` 等方式建立 server。

實作：

```text
search_transactions
get_transaction

get_parcel
search_parcels

find_comparable_transactions

get_parcel_geometry
check_road_access

estimate_land_value

get_data_provenance
```

---

# Phase 11 — MCP Contract Tests

建立：

```text
tests/contract/
```

測試：

```text
tool name
input schema
output schema
error schema
provenance
```

例如：

```text
search_transactions
    input schema stable
    output schema stable
```

---

# Phase 12 — Reproducibility Test

這是 v2.0 的核心測試。

執行：

```text
Query A
```

取得：

```text
result hash = X
```

重新執行：

```text
Query A
```

必須：

```text
result hash = X
```

---

# Phase 13 — Artifact Lock Test

驗證：

```text
raw data
snapshot
algorithm
valuation config
```

在 locked 狀態：

```text
UPDATE → FAIL
DELETE → FAIL
```

---

# Phase 14 — AI Isolation Test

測試 AI 是否可以：

```text
inject SQL
inject PostGIS
change valuation weights
modify snapshot
```

預期：

```text
ALL DENIED
```

---

# Phase 15 — Frontend

Frontend 不參與核心計算。

```text
React
+
TypeScript
+
Google Maps
```

功能：

```text
parcel polygon
transaction marker
road
satellite
Street View
comparable transactions
valuation result
```

Google Maps API 需要 API key / billing，因此前端整合必須另外管理 credential 與 usage。

---

# Phase 16 — Kubernetes / OpenShift

部署：

```text
Deployment
Service
ConfigMap
Secret
CronJob
ServiceMonitor
Route
```

架構：

```text
                   ┌─────────────┐
                   │ MCP Client  │
                   └──────┬──────┘
                          │
                    OpenShift Route
                          │
                   ┌──────▼──────┐
                   │ MCP Server  │
                   └──────┬──────┘
                          │
                 ┌────────▼────────┐
                 │ PostgreSQL      │
                 │ + PostGIS       │
                 └─────────────────┘

CronJob
   │
   ▼
Official Data
   │
   ▼
Importer
```

---

# Phase 17 — Observability

Metrics：

```text
mcp_requests_total
mcp_request_duration_seconds

transaction_query_total
transaction_query_duration

gis_query_total
gis_query_duration

comparable_query_total
valuation_query_total

data_import_total
data_import_errors

snapshot_locked_total
```

Logs 必須包含：

```text
request_id
tool_name
snapshot_id
algorithm_version
query_hash
```

---

# Phase 18 — Final Acceptance Test

完整測試：

```text
指定一筆土地
       ↓
取得 parcel
       ↓
取得 geometry
       ↓
判斷 road access
       ↓
查詢近 5 年交易
       ↓
same section
       ↓
similar area
       ↓
same zoning
       ↓
same land-use
       ↓
comparable ranking
       ↓
statistics
       ↓
valuation
       ↓
provenance
```

最終結果必須可以回答：

```text
這筆土地在哪？
面積多少？
是否臨路？
附近道路如何？
過去交易有哪些？
哪些交易被選為 Comparable？
為什麼選它們？
每筆交易多少錢？
每坪多少？
市場中位數多少？
估值區間多少？
使用哪個 snapshot？
使用哪個 algorithm？
使用哪組 configuration？
```

---

# 7. Definition of Done

v2.0 不以：

```text
MCP server 能啟動
```

作為完成。

必須同時滿足：

```text
[✓] Official data ingestion
[✓] Immutable snapshot
[✓] Provenance
[✓] PostgreSQL/PostGIS
[✓] Transaction query
[✓] Parcel query
[✓] GIS geometry
[✓] Road access
[✓] Comparable engine
[✓] Statistics
[✓] Valuation engine
[✓] MCP interface
[✓] Contract tests
[✓] Reproducibility
[✓] Artifact locking
[✓] AI isolation
[✓] Kubernetes deployment
```

---

# 8. v2.0 Architecture Boundary

最終系統必須維持以下邊界：

```text
┌──────────────────────────────────────────────┐
│                  AI / LLM                    │
│                                              │
│ interpretation / explanation / tool choice  │
└──────────────────────┬───────────────────────┘
                       │ MCP
                       ▼
┌──────────────────────────────────────────────┐
│                  MCP Layer                   │
│                                              │
│ schemas / validation / authorization         │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│              Deterministic Services          │
│                                              │
│ transaction / parcel / GIS / comparable     │
│ statistics / valuation                       │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                 Repository                   │
│                                              │
│ SQL / PostGIS / snapshot                     │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│             Immutable Data Layer             │
│                                              │
│ official raw data / snapshots / provenance   │
└──────────────────────────────────────────────┘
```

**LLM 不得跨越 MCP / Service boundary。**

---

# 9. v2.0 Implementation Order Summary

真正寫 code 時，嚴格按照：

```text
01  Repository / Bootstrap
02  PostgreSQL + PostGIS
03  Snapshot Model
04  Official Data Downloader
05  Parser
06  Normalizer
07  Validator
08  Transaction Repository
09  Transaction Service
10  Parcel Model
11  GIS Adapter
12  Geometry Engine
13  Road Access Engine
14  Comparable Engine
15  Statistics Engine
16  Valuation Engine
17  Provenance
18  MCP Server
19  MCP Contract Tests
20  Reproducibility Tests
21  Artifact Lock Tests
22  AI Isolation Tests
23  Frontend
24  Kubernetes/OpenShift
25  Observability
26  End-to-End Acceptance
```

**不得反過來先做 MCP UI，再補資料層。**

---

# 10. v2.0 Core Philosophy

本專案最重要的不是：

```text
「AI 能不能回答房價？」
```

而是：

```text
「AI 所得到的答案，
是否可以被另一個人、另一台機器、
在相同資料與相同版本下重新得到？」 
```

因此：

```text
Official Data
      ↓
Immutable Snapshot
      ↓
Deterministic Computation
      ↓
Versioned Result
      ↓
MCP
      ↓
AI Explanation
```

才是本專案 v2.0 的核心架構。

---
# End of Specification v2.0
