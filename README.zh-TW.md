# 臺灣實價登錄 MCP 伺服器

> 基於內政部實價登錄資料的 MCP Server，具備確定性、可重現性與 AI 隔離特性。

## 概述

tw-prop-mcp 透過 [Model Context Protocol (MCP)](https://spec.modelcontextprotocol.io/) 對外提供臺灣官方不動產交易資料（MOI 實價登錄）。AI Agent 可以透過 **17 個具型別的工具** 查詢交易資料、地號資訊、GIS 幾何、可比交易分析、土地估價與資料溯源。所有查詢都基於 PostgreSQL + PostGIS，並具備查詢雜湊驗證與 AI 隔離機制。

### 核心保證

- **確定性 (Deterministic)**: 相同輸入（快照 + 查詢參數 + 演算法版本 + 設定版本） → 必產生相同結果
- **AI 隔離 (AI Isolation)**: 工具參數為結構化格式，拒絕 SQL、PostGIS 表達式與估價公式
- **Artifact 鎖定 (Artifact Locking)**: 快照、演算法與設定一旦鎖定即不可修改（資料庫層級 constraint）
- **資料溯源 (Provenance)**: 每筆結果皆可追溯 → Transaction → Snapshot → 官方資料源

## 架構

```
                    ┌──────────────────────────────────┐
                    │            MCP Client             │
                    │ (Claude, Cursor, 自訂 Agent)      │
                    └──────────┬───────────────────────┘
                               │ MCP over stdio or HTTP
                               ▼
                    ┌──────────────────────────────────┐
                    │     cmd/realestate-mcp            │
                    │ CLI flags + env var resolution   │
                    └──────────┬───────────────────────┘
                               │ 初始化
                               ▼
                    ┌──────────────────────────────────┐
                    │    internal/mcp/*_tools.go       │
                    │  ┌────────┬────────────┐         │
                    │  │17 tools│5 resources │         │
                    │  │        │3 prompts   │         │
                    │  └────┬───┴─────┬──────┘         │
                    └──────┼─────────┼────────────────┘
                           │   呼叫    │
                           ▼           ▼
              ┌─────────────────┐ ┌─────────────────┐
              │  服務層         │ │  資料存取層     │
              │ valuation/      │ │ repository/     │
              │ statistics/     │ │ (sqlc 生成)     │
              │ service/        │ │                 │
              └────────┬────────┘ └────────┬────────┘
                       │ 使用             使用
                       ▼                 ▼
              ┌─────────────────┐ ┌─────────────────┐
              │  PostgreSQL     │ │  PostGIS 3.5    │
              │ 16 + PostGIS    │ │ (EPSG:3826→4326)│
              │ migrations/     │ │                 │
              └─────────────────┘ └─────────────────┘
```

### 組件架構

| 層級 | 套件 | 職責 |
|------|------|------|
| 入口點 | `cmd/realestate-mcp/main.go` | CLI flags、環境變數解析、OTel 初始化、伺服器啟動 |
| MCP 接口 | `internal/mcp/` | 17 個 tools、5 個 resources、3 個 prompts、AI 隔離、可觀測性、錯誤模型 |
| 服務層 | `internal/service/`, `internal/valuation/`, `internal/statistics/` | 商業邏輯：可比評分、統計、道路臨接判定 |
| 資料存取 | `internal/repository/` | pgx/v5 + sqlc 生成的查詢 |
| 領域模型 | `internal/domain/` | 核心類別：Transaction、Parcel、Valuation、Provenance、RoadAccess |
| 資料匯入 | `internal/downloader/`, `internal/importpipeline/` | MOI 資料下載、解析、正規化、驗證、匯入管線 |
| GIS | `internal/gis/` | 座標系轉換 (EPSG:3826 ↔ 4326)、幾何適配 |
| 前端 | `frontend/` | React + TypeScript + Google Maps (獨立 Dockerfile) |

## 功能列表

### 17 個 MCP Tools

#### 交易工具 (`internal/mcp/transaction_tools.go`)
| Tool | 說明 |
|------|------|
| `search_transactions` | 依照縣市/鄉鎮/段、價格範圍、日期範圍搜尋 |
| `get_transaction` | 取得單筆交易明細（依 UUID） |
| `get_transaction_statistics` | 取得區域統計指標（Min/P25/中位數/平均/P75/P90/Max） |

#### 地號工具 (`internal/mcp/parcel_tools.go`)
| Tool | 說明 |
|------|------|
| `get_parcel` | 取得地號詳情（依 UUID） |
| `search_parcels` | 依段名+地號搜尋 |

#### 可比交易工具 (`internal/mcp/comparable_tools.go`)
| Tool | 說明 |
|------|------|
| `find_comparable_transactions` | 尋找並評分可比交易 |
| `score_comparable_transactions` | 對指定交易進行可比評分 |

#### GIS 工具 (`internal/mcp/gis_tools.go`)
| Tool | 說明 |
|------|------|
| `get_parcel_geometry` | WKT 幾何資料 (EPSG:4326) |
| `get_parcel_location` | 中心點、包圍盒、座標 |
| `check_road_access` | 道路臨接分類（4 種類型） |
| `find_nearby_roads` | 搜尋半徑內道路 |
| `get_parcel_map_context` | 整合地號+道路+可比交易的地圖顯示資料 |

#### 估價工具 (`internal/mcp/valuation_tools.go`)
| Tool | 說明 |
|------|------|
| `estimate_land_value` | 熊/熱/牛三象限估價 + 信心度 |
| `estimate_property_value` | 土地+建物估價 |
| `explain_valuation` | 人類可讀的估價說明 |

#### 資料溯源工具 (`internal/mcp/provenance_tools.go`)
| Tool | 說明 |
|------|------|
| `get_data_snapshot` | 資料快照資訊（來源、版本、記錄數、狀態） |
| `get_data_provenance` | 任意結果的完整溯源鏈 |

### 5 個 MCP Resources (`internal/mcp/resources.go`)
- `realestate://snapshot/{snapshot_id}` — 資料快照 metadata
- `realestate://transaction/{transaction_id}` — 交易 provenance
- `realestate://parcel/{parcel_id}` — 地號幾何 + 所有權
- `realestate://valuation/{valuation_id}` — 完整估價結果
- `realestate://algorithm/{version}` — 演算法設定 + 權重

### 3 個 MCP Prompts (`internal/mcp/prompts.go`)
| Prompt | 用途 |
|--------|------|
| `prompt_explain_valuation` | 呼叫 `estimate_land_value` 後解釋估價方法論 |
| `prompt_analyze_comparables` | 結構化可比交易分析報告 |
| `prompt_debug_transaction` | 診斷意外的查詢結果 |

### 資料管線 (`internal/importpipeline/pipeline.go`)
1. **下載** — 從 MOI 取得 CSV (`--auto` 自動抓取最新 URL)
2. **驗證校驗和** — SHA256 驗證原始檔案
3. **解析** — CSV → 中介資料列 (`internal/parser/`)
4. **Enrich** — 從檔名推導縣市/鄉鎮
5. **正規化** — 清洗 + 標準化欄位 (`internal/normalizer/`)
6. **驗證** — 資料品質檢查 (`internal/validator/`)
7. **去重複** — 移除重複交易
8. **匯入** — 透過 `BEGIN`/`COMMIT`/`ROLLBACK` 進行交易性批次插入
9. **鎖定** — 快照狀態切換為 LOCKED (不可變)

### 關鍵原則
- **確定性**: 查詢雜湊 = `canonicalize(snapshot_id, query_params, algorithm_version, config_version)`
- **AI 隔離**: `ProhibitedFields` 驗證所有工具輸入，拒絕 `sql`、`where`、`postgis`、`valuation_formula`、`weights` (P4)
- **Artifact 鎖定**: migrations 002-004 建立資料庫層級 trigger，強制執行快照/設定/原始資料的不可變性 (P5)
- **資料溯源**: 所有結果包含 `source`、`snapshot_id`、`source_record_hash`、`import_batch_id` (P6)

## 專案結構

```
tw-prop-mcp/
├── cmd/realestate-mcp/main.go       # 入口點：CLI flags、環境變數、OTel、伺服器
├── internal/
│   ├── clock/                        # 時鐘抽象化 (測試用)
│   ├── comparable/                   # 可比引擎核心
│   │   └── engine.go                 # 篩選 + 評分 (硬編碼權重)
│   ├── config/                       # 伺服器設定 + DB 連線
│   ├── domain/                       # 領域模型
│   │   ├── transaction.go
│   │   ├── parcel.go
│   │   ├── valuation.go
│   │   ├── comparable.go
│   │   ├── statistics.go
│   │   ├── parcel_road_access.go
│   │   ├── road_segment.go
│   │   ├── provenance.go
│   │   └── snapshot.go
│   ├── downloader/                   # MOI 資料下載 + 封存
│   │   ├── downloader.go             # HTTP 下載器 (具重試)
│   │   ├── discover.go               # 自動發現最新下載 URL
│   │   ├── checksum.go               # SHA256 校驗和
│   │   ├── archive.go                # 封存管理
│   │   └── snapshot.go               # 冪等快照邏輯
│   ├── gis/                          # GIS 轉換 + 適配
│   │   ├── transform.go              # EPSG:3826 ↔ 4326 轉換
│   │   ├── adapter.go                # PostGIS 幾何適配器
│   │   └── importer.go               # 幾何匯入工具
│   ├── importpipeline/               # 完整資料匯入管線
│   │   └── pipeline.go               # 9 階段管線 (下載→鎖定)
│   ├── mcp/                          # MCP server、tools、resources、prompts
│   │   ├── server.go                 # Server bootstrap、HTTP/stdio 傳輸
│   │   ├── transaction_tools.go
│   │   ├── parcel_tools.go
│   │   ├── comparable_tools.go
│   │   ├── gis_tools.go
│   │   ├── valuation_tools.go
│   │   ├── provenance_tools.go
│   │   ├── resources.go              # 5 個 realestate:// resources
│   │   ├── prompts.go                # 3 個 prompt 模板
│   │   ├── errors.go                 # ProhibitedFields、McpError
│   │   ├── instrument.go             # Provenance 注入、查詢雜湊
│   │   └── observability.go          # Metrics、tracing、request logging
│   ├── normalizer/                   # CSV 正規化
│   ├── parser/                       # CSV 解析
│   ├── provenance/                   # 雜湊生成 + 服務
│   ├── repository/                   # 資料存取 (sqlc 生成)
│   ├── service/                      # 商業邏輯
│   └── statistics/                  # 統計引擎
├── migrations/                        # golang-migrate SQL migrations
├── sqlc.yaml                         # SQLC 配置
├── Dockerfile                        # 多階段建置 (golang:1.26-alpine → alpine)
├── Dockerfile.frontend               # 前端建置 (node:20-alpine → nginx)
├── Makefile                          # Build、test、lint、migrate 目標
├── frontend/                         # React + TypeScript + Google Maps
├── tests/                            # 測試套件
│   ├── contract/                     # MCP contract tests
│   ├── e2e/                          # 端對端接受測試
│   ├── integration/                  # 整合測試 (PostgreSQL)
│   ├── isolation/                    # AI 注入防證測試
│   ├── reproducibility/              # 確定性 + 查詢雜湊測試
│   └── golden/                       # Golden dataset (迴歸測試)
├── scripts/verify.sh                 # 16 步驟自動化驗證
├── SPEC.md                           # 系統規格 (P1-P6 原則)
├── DATA_MODEL.md                     # 資料模型與資料庫 schema
├── MCP_API.md                        # MCP tools、resources、錯誤模型
├── GIS_SPEC.md                       # 座標系、道路臨接
├── VALUATION_SPEC.md                 # 可比、統計、估價規格
├── IMPLEMENTATION_PLAN.md            # 18 階段實作計畫
└── go.mod
```

## 環境需求

### Runtime
- **Go**: 1.26+
- **PostgreSQL**: 16+ (需安裝 PostGIS 3.5 擴充套件)
- **作業系統**: 任意 (Docker 建議用于 PostgreSQL)

### 外部服務
- **內政部實價登錄**: `https://plvr.land.moi.gov.tw/` — 資料來源
- **OpenTelemetry Collector** (選用): 透過 `OTEL_EXPORTER_OTLP_ENDPOINT` 收集追蹤/指標
- **Google Maps API** (前端專用): 前端地圖渲染所需

## 安裝

### 從原始碼建置

```bash
# 1. 克隆
git clone <repo-url>
cd tw-prop-mcp

# 2. 確認 Go 1.26+
go version  # 應顯示 1.26.x

# 3. 建置
make build  # 輸出到 bin/realestate-mcp
# 或: go build -o bin/realestate-mcp ./cmd/realestate-mcp
```

### Docker 建置

```bash
# 主要伺服器 (Go + Alpine、非 root 使用者)
docker build -t tw-prop-mcp .

# 前端 (React + nginx)
docker build -f Dockerfile.frontend -t tw-prop-mcp-frontend frontend/
```

### 使用 Docker Compose 啟動

```bash
# 1. 從模板建立 .env
cp env.example .env
# 編輯 .env — 填入您的 GOOGLE_MAPS_API_KEY

# 2. 啟動所有服務 (postgres → mcp-server → frontend)
docker compose up -d --build

# 3. 檢查狀態
docker compose ps
```

| 服務 | URL | 說明 |
|------|-----|------|
| PostgreSQL | `localhost:5432` | PostgreSQL 16 + PostGIS |
| MCP Server | `localhost:8080` | Streamable HTTP on `/mcp`，metrics on `/metrics` |
| Frontend | `localhost:80` | React + nginx (代理 `/mcp` → MCP Server) |

## 配置

### 伺服器環境變數

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `MCP_TRANSPORT` | `http` | 傳輸方式：`stdio` 或 `http` |
| `MCP_HTTP_ADDR` | `:8080` | HTTP 監聽位址 (HTTP 模式) |
| `DATABASE_URL` | — | PostgreSQL 連接字串 (`postgresql://user:pass@host:5432/db`) |
| `DEFAULT_SNAPSHOT_VERSION` | `latest` | 預設查詢快照 ID |
| `ALGORITHM_VERSION` | `comparable-v2.0` | 查詢雜湊使用的演算法版本 |
| `CONFIGURATION_VERSION` | `v2.0` | 查詢雜湊使用的設定版本 |
| `DATA_IMPORT_URL` | — | 資料匯入的直接下載 URL |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OpenTelemetry OTLP HTTP 端點 (例如 `http://localhost:4318`) |
| `LOG_LEVEL` | `info` | 日誌等級 (info/debug/warn/error) |

### CLI Flags

| Flag | Env Var | 預設值 | 說明 |
|------|---------|--------|------|
| `--transport` | `MCP_TRANSPORT` | `http` | `stdio` 或 `http` |
| `--addr` | `MCP_HTTP_ADDR` | `:8080` | HTTP 監聽位址 |
| `--snapshot-id` | `DEFAULT_SNAPSHOT_VERSION` | `latest` | 預設資料快照 |
| `--algorithm` | `ALGORITHM_VERSION` | `comparable-v2.0` | 演算法版本 |
| `--data-url` | `DATA_IMPORT_URL` | — | 直接下載 URL |
| `--auto` | — | `false` | 自動從 MOI 落地頁抓取最新下載 URL |

## 快速開始

### 1. 啟動 PostgreSQL + PostGIS

```bash
docker run -d --name postgres \
  -e POSTGRES_DB=prop \
  -e POSTGRES_USER=prop \
  -e POSTGRES_PASSWORD=prop_dev_only \
  -p 5432:5432 \
  postgis/postgis:16-3.5

# 執行 migrations
go run ./cmd/migrate up
# 或: migrate -path migrations -database "postgresql://prop:prop_dev_only@localhost:5432/prop?sslmode=disable" up
```

### 2. 匯入資料

```bash
# 自動從 MOI 落地頁抓取最新 URL 並匯入
export DATABASE_URL=postgresql://prop:prop_dev_only@localhost:5432/prop
./bin/realestate-mcp --auto --snapshot-id 2025Q1
```

### 3. 啟動 MCP 伺服器

```bash
# HTTP 模式 (遠端客戶端)
export MCP_TRANSPORT=http
export MCP_HTTP_ADDR=:8080
export DATABASE_URL=postgresql://prop:prop_dev_only@localhost:5432/prop
./bin/realestate-mcp

# Stdio 模式 (Claude Desktop)
export MCP_TRANSPORT=stdio
export DATABASE_URL=postgresql://prop:prop_dev_only@localhost:5432/prop
./bin/realestate-mcp
```

### 4. 查詢交易資料

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

### 工具 Schema

所有工具使用 `mcpapi.AddTool` 泛型提供型別安全的輸入/輸出。每次回應都包含 `metadata`：
- `query_hash` — 確定性雜湊值
- `snapshot_id` — 資料來源快照版本
- `algorithm_version` — 使用的演算法
- `configuration_version` — 使用的設定版本

### 錯誤模型 (`internal/mcp/errors.go`)

| 代碼 | 說明 |
|------|------|
| `INVALID_ARGUMENT` | 參數驗證失敗 |
| `PARCEL_NOT_FOUND` | 地號 UUID 不存在 |
| `TRANSACTION_NOT_FOUND` | 交易 UUID 不存在 |
| `DATA_NOT_AVAILABLE` | 查詢條件無資料 |
| `GIS_NOT_AVAILABLE` | PostGIS 幾何運算失敗 |
| `SNAPSHOT_NOT_FOUND` | 快照 ID 不存在 |
| `VALUATION_NOT_AVAILABLE` | 可比交易不足，無法估價 |
| `SOURCE_UNAVAILABLE` | 官方資料源不可達 |
| `INTERNAL_ERROR` | 伺服器內部錯誤 |

### AI 隔離 (P4)

所有工具處理器在執行商業邏輯前，會透過 `ProhibitedFields` 驗證原始參數：

```go
blockedFields := []string{"sql", "where", "postgis", "valuation_formula", "weights", "expression", "raw_sql"}
```

遭拒請求會回傳 `INVALID_ARGUMENT` 錯誤。此機制防止 LLM 提示注入升級為 SQL 查詢或客製計算。

## 資料模型

核心實體（見 `internal/domain/`）：

- **Transaction**: `total_price`、`unit_price` 使用 `int64` (無浮點貨幣型別)，`section`/`land_number` 作為地號身份識別
- **Parcel**: 幾何資料使用 EPSG:3826 (PostGIS)，輸出轉為 EPSG:4326
- **ValuationResult**: Bear (P25)、Base (P50)、Bull (P75) + Confidence 等級 (HIGH/MEDIUM/LOW/INSUFFICIENT)
- **ComparableCandidate**: 帶分數的交易資料，包含 `area_similarity`、`distance_meters`、`time_weight`、`zoning_match`、`land_use_match`、`road_access_match`

### 地號身份識別 (4-key)
一個地號由 `county + district + section + land_number` 唯一識別。

### 統計計算
`statistics/engine.go` 提供：
- 百分位數：P0/P10/P25/中位數/平均/P75/P90/P100
- 離群值：IQR 方法 (可設定 k 因子)
- 面積換算：1 坪 = 3.305785 平方公尺

## 估價引擎

`internal/valuation/engine.go` 實作以下流程：

1. **可比篩選**: 同縣 → 鄉鎮 → 段 → 建物類型
2. **距離評分**: 道路距離 + 地號中心點距離
3. **時間加權**: 指數衰減 `exp(-lambda * 距離交易日天數)`
4. **面積相似度**: `min(面積A, 面積B) / max(面積A, 面積B)`
5. **離群值移除**: IQR 方法 (k=1.5) 過濾可比交易單價
6. **熊/熱/牛估價**: 依據 P25、P50 (加權中位數)、P75 計算
7. **信心度**: HIGH (≥5 個可比)、MEDIUM (≥2 個)、LOW (<2 個) 或 INSUFFICIENT

## 錯誤處理

- 所有 MCP tool 錯誤回傳具結造的 `McpError` (含錯誤代碼 + retryable 標記)
- 資料庫錯誤透過 `ImportPipelineError` 附加階段資訊
- 網路錯誤 (HTTP 5xx、逾時) 在匯入管線中標記为可重��試

## 日誌與可觀測性

### Prometheus Metrics (`internal/mcp/observability.go`)
- `mcp_requests_total` — 各工具請求計數器
- `mcp_request_duration_seconds` — 各工具耗時直方圖
- `transaction_query_total` — 交易查詢計數器
- `gis_query_total` — GIS 查詢計數器
- `comparable_query_total` — 可比查詢計數器
- `valuation_query_total` — 估價查詢計數器
- `data_import_total` — 匯入成功/失敗計數器
- `snapshot_locked_total` — 快照鎖定計數器

### OpenTelemetry
- 透過 `OTEL_EXPORTER_OTLP_ENDPOINT` (預設 `http://localhost:4318`) 設定 OTLP HTTP exporter
- 使用 `BatchSpanProcessor` 緩衝匯出
- 服務名稱: `tw-prop-mcp`
- 未設定 endpoint 時自動降grade 為 no-op tracer

### 結構化日誌
- 每請求紀錄 `request_id`、`tool_name`、`snapshot_id`、`query_hash`
- 匯入管線記錄各階段結構化日誌

### HTTP 端點 (HTTP 模式)
- `/mcp` — MCP Streamable HTTP 端點
- `/healthz` — liveness probe
- `/readyz` — readiness probe
- `/metrics` — Prometheus metrics

> **已知限制**: `/readyz` 回傳靜態 OK，不檢查資料庫連線 — [NEEDS VERIFICATION]

## 測試

```bash
# 單元測試 (無需 PostgreSQL)
go test ./internal/... -count=1

# 排除 config tests (需要 PostgreSQL container)
make test

# 整合測試 (需要 PostgreSQL + PostGIS)
make test-integration

# E2E 接受測試
go test -tags=e2e ./tests/e2e/... -count=1

# Contract 測試
go test ./tests/contract/... -count=1

# Isolation 測試 (AI 注入防證)
go test ./tests/isolation/... -count=1

# Reproducibility 測試 (確定性驗證)
go test ./tests/reproducibility/... -count=1

# Artifact lock 測試
go test -tags=integration ./tests/artifact_lock/... -count=1

# 基準測試 (真實 MOI 資料)
go test -bench=. -benchmem ./internal/importpipeline/...

# 完整驗證套件
bash scripts/verify.sh  # 16 步驟驗證
```

### 測試統計
- 單元測試: 244 個通過 (2 個 config 測試在無 PostgreSQL container 時失敗)
- 整合測試: 10 個通過
- E2E: 7 個通過
- Contract: 12 個通過
- AI Isolation: 11 個通過
- Reproducibility: 16 個通過
- Artifact Lock: 10 個通過
- 基準測試: 7 個通過

## 建置

```bash
make build          # go build -o bin/realestate-mcp ./cmd/realestate-mcp
make vet            # go vet ./...
make lint           # golangci-lint run ./... || go vet ./...
make sqlc           # 重新生成 sqlc 程式碼
make tidy           # go mod tidy
```

## 部署

### Docker Compose (推薦)

```bash
# 一鍵啟動開發環境：postgres + MCP server + frontend
cp env.example .env
# 編輯 .env — 填入您的 GOOGLE_MAPS_API_KEY
docker compose up -d --build
```

服務依序啟動 (postgres → mcp-server → frontend)，並透過健康檢查控管依賴順序。

### Docker (單獨執行)

```bash
# 多階段建置 (golang:1.26-alpine → alpine:latest, 非 root)
docker build -t tw-prop-mcp .

# 執行
docker run -p 8080:8080 \
  -e DATABASE_URL=postgresql://prop:prop_dev_only@localhost:5432/prop \
  -e MCP_TRANSPORT=http \
  tw-prop-mcp
```

### 前端 (React + Google Maps)

```bash
# 建置與提供服務
docker build -f Dockerfile.frontend -t tw-prop-mcp-frontend . \
  --build-arg VITE_GOOGLE_MAPS_API_KEY=your_key_here
docker run -p 80:80 tw-prop-mcp-frontend
```

### 健康檢查
- `GET /healthz` — liveness (永遠回傳 200)
- `GET /readyz` — readiness (回傳 `{"status":"ready"}`)
- `GET /metrics` — Prometheus metrics

> **注意**: `/readyz` 不驗證資料庫連線，僅回傳靜態 OK — 這是已知缺限。

## 開發

### 新增 MCP Tool

1. 定義具有型別安全的輸入/輸出結構
2. 實作符合 `mcpapi.ToolHandler` 簽名的 handler 函數
3. 透過 `mcpapi.AddTool(srv, &mcpapi.Tool{...}, handler)` 註冊到對應的 `*_tools.go`
4. 透過 `instrument.go` 加入 provenance 注入
5. 添加到 `tests/contract/contract_test.go`

### 程式碼規範
- `gofmt` / `goimports` 格式化
- `golangci-lint` 靜態分析
- 測試覆蓋率目標: ≥ 80%

### Git 規範
- Conventional Commits (`feat:`, `fix:`, `docs:` 等)
- 一個任務一個 commit
- `main` 分支受保護

## 已知限制

- **僅支援單一快照**: 目前 `--auto` 只抓取最新一期 MOI 資料，每次覆蓋快照。歷史資料回補需手動指定歷史 URL
- **單一快照鎖定**: 目前導入管線會覆蓋快照。多快照保留需指定不同的 import batch ID
- **`/readyz` 健康檢查**: 不驗證資料庫連線 — 僅回傳靜態 OK [NEEDS VERIFICATION]
- **`DATABASE_URL` 必要**: 無資料庫連線不啟動
- **前端整合未驗證**: React 前端通過 `tsc --noEmit` 類型檢查，但 CI 不執行瀏覽器整合測試
- **PostGIS 依賴**: 所有 GIS 操作需要 PostGIS 擴充套件。未安裝時 `check_road_access` 等工具可能失敗
- **無請求限流**: MCP server 不實作請求速率限制。AI Agent 發出大量並發請求可能壓垪資料庫

## 授權

本專案採 **Apache License 2.0** 授權。詳見 [`LICENSE`](LICENSE)。

> 本專案僅供個人研究與教育用途使用。資料來源為內政部實價登錄，請遵守臺灣相關法令規範。
