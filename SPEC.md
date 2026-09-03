# Taiwan Property Valuation MCP Server - Specification

## 專案概述

本專案實作一個 Model Context Protocol (MCP) Server，提供台灣不動產估價所需的核心功能：
- 實價登錄資料查詢與統計
- 地籍圖資查詢與空間運算
- 道路臨接判定
- 可比交易篩選與評分
- 土地估值引擎
- 完整資料溯源系統

## 核心架構原則

### P1: Deterministic First
- 相同輸入（snapshot + query params + algorithm version + config）→ 必須產生相同輸出
- 所有計算邏輯固化在 Service Layer，禁止 AI 介入計算
- Query Hash 機制確保可重現性驗證

### P2: Raw Data Immutable
- 官方原始下載檔案不可修改（唯讀歸檔）
- 任何正規化、清洗、轉換都產生新的 artifact
- Snapshot 一旦 LOCKED 禁止 UPDATE/DELETE

### P3: Reproducible Build & Query
- Go 1.25+，依賴版本鎖定
- SQL 由 sqlc 生成，禁止動態 SQL
- CI/CD 強制 reproducibility test

### P4: AI Isolation
- AI 僅負責：Request interpretation → MCP tool selection → Structured result → LLM explanation
- MCP Tool 參數結構化，拒絕：SQL、PostGIS expression、arbitrary code、valuation formula
- Service Layer 為唯一轉換 SQL/PostGIS 的路徑

### P5: Artifact Locking
- Raw Data、Snapshot、Algorithm Version、Valuation Config、GIS Source Metadata、Snapshot Manifest 一旦鎖定不可修改
- 資料庫層級 constraint 強制執行

### P6: Provenance Required
- 所有核心資料包含：source, source_version, snapshot_id, source_record_hash, import_batch_id
- Transaction → Snapshot → Official Source 完整追溯
- Valuation → Comparable IDs → Transactions → Snapshot → Official Source 完整追溯
- Query Hash：input params + algorithm version + config version + snapshot → canonicalize → hash

## 技術棧

| 層級 | 技術 |
|------|------|
| 語言 | Go 1.25+ |
| 資料庫 | PostgreSQL 16 + PostGIS 3.5 |
| SQL 生成 | sqlc |
| Migration | golang-migrate |
| MCP SDK | github.com/modelcontextprotocol/go-sdk/mcp |
| GIS | PostGIS (內部 EPSG:3826，對外可輸出 EPSG:4326) |
| 容器 | Docker (golang:1.26-alpine3.24) |
| 部署 | Kubernetes / OpenShift |
| 監控 | Prometheus + Grafana + OpenTelemetry |

## 核心領域模型

### DatasetSnapshot
```go
type DatasetSnapshot struct {
    ID                 string
    Source             string           // "MOI" (內政部)
    SourceVersion      string           // "2024Q1"
    DownloadedAt       time.Time
    PublishedAt        time.Time
    FileName           string
    FileSHA256         string
    RecordCount        int64
    Status             SnapshotStatus   // PENDING, IMPORTING, LOCKED, FAILED
    SchemaVersion      string
    ImportStartedAt    *time.Time
    ImportCompletedAt  *time.Time
}
```

### Transaction
```go
type Transaction struct {
    TransactionID      string
    SnapshotID         string
    TransactionDate    time.Time
    TransactionType    string           // "土地", "建物", "土地建物"
    County             string
    District           string
    Section            string
    LandNumber         string
    TransactionTarget  string
    TotalPrice         int64
    UnitPrice          int64            // 元/平方公尺
    LandAreaSqm        float64
    BuildingAreaSqm    float64
    UrbanZoning        string
    NonUrbanZoning     string
    LandUseCategory    string
    BuildingType       string
    Floor              string
    Age                int
    ParkingArea        float64
    ParkingPrice       int64
    SourceRecordHash   string
}
```

### Parcel
```go
type Parcel struct {
    ParcelID           string
    County             string
    District           string
    Section            string
    LandNumber         string
    AreaSqm            float64
    UrbanZoning        string
    LandUseCategory    string
    Geometry           geom.MultiPolygon // PostGIS, SRID 3826
    Centroid           geom.Point        // PostGIS, SRID 3826
    Source             string
    SourceVersion      string
}
```

### ComparableTransaction
```go
type ComparableTransaction struct {
    Transaction
    Score              float64
    AreaSimilarity     float64
    DistanceMeters     float64
    TimeWeight         float64
    ZoningMatch        bool
    LandUseMatch       bool
    RoadAccessMatch    bool
}
```

### ValuationResult
```go
type ValuationResult struct {
    ValuationID        string
    TargetParcelID     string
    SnapshotID         string
    BearValue          int64            // P25 adjusted
    BaseValue          int64            // P50 adjusted (weighted median)
    BullValue          int64            // P75 adjusted
    Confidence         ConfidenceLevel  // HIGH, MEDIUM, LOW, INSUFFICIENT
    ComparableCount    int
    AlgorithmVersion   string
    ConfigurationVersion string
    OutlierMethod      string
    Weights            ValuationWeights
    Statistics         Statistics
    Provenance         DataProvenance
    CreatedAt          time.Time
}
```

## MCP Tools 列表

### Transaction Tools
- `search_transactions` - 依條件搜尋交易
- `get_transaction` - 取得單筆交易詳情
- `get_transaction_statistics` - 取得交易統計指標

### Parcel Tools
- `get_parcel` - 取得地號詳情
- `search_parcels` - 搜尋地號

### Comparable Tools
- `find_comparable_transactions` - 尋找可比交易
- `score_comparable_transactions` - 可比交易評分

### GIS Tools
- `get_parcel_geometry` - 取得地號幾何資料
- `get_parcel_location` - 取得地號位置（centroid, bbox）
- `check_road_access` - 判斷道路臨接
- `find_nearby_roads` - 查詢附近道路
- `get_parcel_map_context` - 取得地圖顯示所需資料

### Valuation Tools
- `estimate_land_value` - 土地估值
- `estimate_property_value` - 不動產估值（土地+建物）
- `explain_valuation` - 估值說明

### Provenance Tools
- `get_data_snapshot` - 取得資料快照資訊
- `get_data_provenance` - 取得資料溯源鏈

## MCP Resources
- `realestate://snapshot/{id}`
- `realestate://transaction/{id}`
- `realestate://parcel/{id}`
- `realestate://valuation/{id}`
- `realestate://algorithm/{version}`

## 錯誤代碼
- `INVALID_ARGUMENT` - 參數錯誤
- `PARCEL_NOT_FOUND` - 地號不存在
- `TRANSACTION_NOT_FOUND` - 交易不存在
- `DATA_NOT_AVAILABLE` - 資料不可用
- `GIS_NOT_AVAILABLE` - GIS 服務不可用
- `SNAPSHOT_NOT_FOUND` - 快照不存在
- `VALUATION_NOT_AVAILABLE` - 無法估值
- `SOURCE_UNAVAILABLE` - 官方資料源不可用
- `INTERNAL_ERROR` - 內部錯誤

## 配置管理

### ValuationConfig (valuation_config 表)
```sql
CREATE TABLE valuation_config (
    version         VARCHAR(50) PRIMARY KEY,
    weights         JSONB NOT NULL,      -- W_area, W_distance, W_time, W_zoning, W_land_use, W_road
    lambda          NUMERIC NOT NULL,    -- Time decay lambda
    distance_scale  NUMERIC NOT NULL,    -- Distance decay scale
    area_threshold  NUMERIC NOT NULL,    -- Area similarity threshold (default 0.3)
    min_comparables INTEGER NOT NULL,    -- Minimum comparables for valuation
    outlier_method  VARCHAR(20) NOT NULL,-- 'IQR' | 'P10_P90' | 'MAD'
    iqr_multiplier  NUMERIC NOT NULL,    -- IQR multiplier (default 1.5)
    created_at      TIMESTAMP NOT NULL,
    locked          BOOLEAN NOT NULL DEFAULT FALSE
);
```

### RoadAccessConfig (road_access_config 表)
```sql
CREATE TABLE road_access_config (
    version            VARCHAR(50) PRIMARY KEY,
    search_radius_m    INTEGER NOT NULL,   -- Search radius for nearby roads
    adjacent_tolerance_m NUMERIC NOT NULL, -- Tolerance for ROAD_ADJACENT
    nearby_max_distance_m NUMERIC NOT NULL,-- Max distance for ROAD_NEARBY
    created_at         TIMESTAMP NOT NULL,
    locked             BOOLEAN NOT NULL DEFAULT FALSE
);
```

## 開發規範

### 代碼風格
- `gofmt` / `goimports` 格式化
- `golangci-lint` 靜態分析
- 測試覆蓋率 ≥ 80%

### Git 規範
- Conventional Commits
- 一個任務一個 commit
- main branch 保護

### 版本發布
- Semantic Versioning
- CHANGELOG.md 維護