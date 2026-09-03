# Taiwan Real-Estate Actual Transaction MCP

> 台灣官方實價登錄為核心的 MCP Server — Deterministic First / AI Isolation / Reproducible / Artifact Locked

## 概述

本專案提供 AI Agent 透過 Model Context Protocol (MCP) 存取：
- 不動產交易查詢 / 統計 / 地號查詢
- Parcel / GIS 幾何 / 道路臨接判定
- Comparable 篩選與評分 / 土地估值（bear/base/bull + confidence）
- 完整溯源（Transaction → Snapshot → Official Source）與 Query Hash 重現性

AI 僅做 `Request interpretation → MCP tool selection → Deterministic engine → Structured result → LLM explanation`，禁止直連 SQL/PostGIS/valuation 計算。

## 技術棧

| 層級 | 技術 |
|---|---|
| Language | Go 1.26 |
| DB | PostgreSQL 16 + PostGIS 3.5 (EPSG:3826 內部, EPSG:4326 輸出) |
| SQL | pgx/v5 + sqlc (禁止 ORM / 動態 SQL) |
| MCP | `github.com/modelcontextprotocol/go-sdk/mcp` |
| HTTP | Chi / net/http |
| Observability | Prometheus + Grafana + OpenTelemetry |

## 快速開始

```bash
# 依賴
go version # 1.26+

# 建置與測試
make tidy
make vet
make test        # 單測（無 DB）
make build

# 整合測試（需 Docker）
make db-up 2>/dev/null || docker compose up -d postgres
make test-integration

# 執行
./bin/realestate-mcp --version
./bin/realestate-mcp
```

## 專案結構

```
tw-prop-mcp/
├── cmd/realestate-mcp/main.go
├── internal/
│   ├── mcp/ transaction_tools.go, parcel_tools.go, comparable_tools.go, gis_tools.go, valuation_tools.go
│   ├── domain/ transaction.go, parcel.go, geometry.go, road.go, comparable.go, valuation.go
│   ├── service/ transaction.go, parcel.go, comparable.go, gis.go, valuation.go
│   ├── repository/ transaction.go, parcel.go, gis.go, valuation.go
│   ├── ingestion/ downloader.go, parser.go, normalizer.go, validator.go, snapshot.go
│   ├── gis/ geometry.go, parcel.go, road.go, distance.go
│   ├── valuation/ comparable.go, statistics.go, outlier.go, scoring.go
│   └── provenance/
├── migrations/
├── sql/
├── tests/ unit, integration, gis, valuation, contract
├── SPEC.md / DATA_MODEL.md / MCP_API.md / GIS_SPEC.md / VALUATION_SPEC.md / IMPLEMENTATION_PLAN.md
├── Dockerfile (golang:1.26-alpine3.24)
└── Makefile
```

## 文件

- `SPEC.md` — 系統總體規格（P1-P6 原則）
- `DATA_MODEL.md` — 資料模型與 Pipeline
- `MCP_API.md` — MCP Tools / Resources / Error Model
- `GIS_SPEC.md` — 座標系 / Parcel Geometry / Road Access / Google Maps
- `VALUATION_SPEC.md` — Comparable / Statistics / Valuation
- `IMPLEMENTATION_PLAN.md` — 18 階段實作順序

## 核心原則

- **Official Data First** — 內政部實價登錄批次檔為唯一交易來源
- **Raw Immutable** — `raw/source_snapshot/{manifest,checksum,original_file}` 不可改
- **Deterministic** — 同 `snapshot+params+algorithm+config` 恆得同結果（`query_hash`）
- **AI Isolation** — Tool 參數結構化，拒 `SQL/PostGIS/formula`
- **Artifact Lock** — `snapshot/algorithm/config` 鎖定後 DB trigger 禁 UPDATE/DELETE
- **Provenance** — 每筆結果含 `source, snapshot_id, record_hash, import_batch_id, algorithm_version`

---
## License

本專案採用 **Apache License 2.0** 授權。

- 完整授權條款見 [`LICENSE`](LICENSE)（專案根目錄）
- Apache-2.0 官方條款：<https://www.apache.org/licenses/LICENSE-2.0>
- 版權與貢獻者資訊以 LICENSE 檔案為準

> 本專案為研究/查詢用途，授權條款不構成任何投資買賣或保證；
> 使用/修改/再散佈前請詳閱 LICENSE 全文。

本專案僅供個人研究與教育用途。資料來源（內政部實價登錄,官方圖資來源）之使用請遵守各平台之服務條款。

