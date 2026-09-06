# 台湾实价登录 MCP 服务器

> 基于内政部实价登录数据（来自台湾不动产交易登记）的 MCP 服务器。具备确定性、可重现性与 AI 隔离特性。

## 概述

`tw-prop-mcp` 通过 [Model Context Protocol (MCP)](https://spec.modelcontextprotocol.io/) 提供台湘官方不动产交易数据。客户端（Claude、Cursor、自定义 Agent）可以通过 **17 个有类型的工具** 查询交易数据、地号信息、GIS 几何、可比交易分析、土地估价与数据溯源 - 所有查询都基于 PostgreSQL + PostGIS，并具有查询哈希验证与 AI 隔离机制。

设计原则 (参见 [SPEC.md](SPEC.md))：

- **P1 确定性 (Deterministic)**: 相同输入（快照 + 参数 + 算法版本 + 配置版本）→ 必定产生相同结果 (查询哈希验证)
- **P2 原始数据不可变 (Immutable)**: 官方原始数据为只读存档；快照一旦锁定不可修改
- **P4 AI 隔离 (AI Isolation)**: 工具参数为结构化格式 - 拒绝 SQL、PostGIS 表达式与估价公式
- **P5 制品锁定 (Artifact Locking)**: 数据库层约束阻止修改已锁定的快照、算法、配置
- **P6 数据溯源 (Provenance)**: 每个结果都可追溯 → Transaction → Snapshot → 官方数据源

## 架构

```
                 ┌─────────────────────┐
                 │    MCP 客户端       │  (Claude, Cursor, 自定义 Agent)
                 └─────────┬───────────┘
                           │  MCP over HTTP (SSE) 或 stdio
                           ▼
                 ┌─────────────────────┐
                 │  cmd/realestate-mcp │  CLI 参数、环境变量、OTel 初始化
                 └─────────┬───────────┘
                           │ 初始化
                           ▼
                 ┌─────────────────────┐
                 │  internal/mcp/      │  17 个 tools · 5 个 resources · 3 个 prompts
                 │  *_tools.go         │  AI 隔离 · 数据溯源注入
                 └────┬──────────┬─────┘
                      │         │
                 使用  │         │ 使用
                      ▼         ▼
         ┌─────────────────┐ ┌──────────────────┐
         │  服务层         │ │  数据访问层      │
         │  valuation/     │ │  repository/     │
         │  statistics/    │ │  (sqlc 生成)     │
         └────────┬────────┘ └────────┬────────┘
                  │ 使用            使用
                  ▼                 ▼
         ┌─────────────────┐ ┌──────────────────┐
         │  PostgreSQL 16  │ │  PostGIS 3.6     │
         │  + PostGIS      │ │  EPSG:3826↔4326  │
         └─────────────────┘ └──────────────────┘
```

### 组件

| 层级 | 包 | 职责 |
|------|---------|------|
| 入口点 | `cmd/realestate-mcp/main.go` | CLI flags、环境变量解析、OTel 初始化、服务器启动 |
| MCP 接口 | `internal/mcp/` | 17 个 tools、5 个 resources、3 个 prompts、AI 隔离、可观测性、错误模型 |
| 服务层 | `internal/service/`, `internal/valuation/`, `internal/statistics/` | 商业逻辑: 可比评分、统计、道路临接判断 |
| 数据访问 | `internal/repository/` | pgx/v5 + sqlc 生成的查询 |
| 领域模型 | `internal/domain/` | 核心类型: Transaction、Parcel、Valuation、Provenance |
| 数据导入 | `internal/downloader/`, `internal/importpipeline/` | MOI 数据下载→解析→正规化→验证→导入 |
| GIS | `internal/gis/` | 坐标系转换 (EPSG:3826 ↔ 4326) |
| 前端 | `frontend/` | React + TypeScript - Leaflet(默认)或 Google Maps, MCP 客户端 |

## 快速开始

```bash
# 1. 创建 .env (秘密信息永远不提交)
cp env.example .env
# 编辑 .env - 可选设置 GOOGLE_MAPS_API_KEY (切换到 Google Maps 提供者时)

# 2. 构建并启动所有服务
docker compose up -d --build

# 3. 验证
curl http://localhost/healthz     # {"status":"ok"}
open http://localhost/            # 前端地图界面

# 4. 检查 PostGIS 扩展
docker exec tw-prop-postgres psql -U prop -d prop -c "SELECT extname,extversion FROM pg_extension WHERE extname='postgis';"
```

## MCP Tools

### 交易工具
| Tool | 说明 |
|------|-------------|
| `search_transactions` | 按照县市/区/段、价格范围、日期范围搜索 |
| `get_transaction` | 单笔交易详情 (按 UUID) |
| `get_transaction_statistics` | 区域统计指标 (Min/P25/中位数/平均/P75/P90/最大值) |

### 地号工具
| Tool | 说明 |
|------|-------------|
| `get_parcel` | 按县市/区/段/地号获取地号详情 |
| `search_parcels` | 按段名+地号搜索 |

### 可比交易工具
| Tool | 说明 |
|------|-------------|
| `find_comparable_transactions` | 查找并评分可比交易 |
| `score_comparable_transactions` | 对指定交易进行可比评分 |

### GIS 工具
| Tool | 说明 |
|------|-------------|
| `get_parcel_geometry` | EPSG:4326 WKT 几何 |
| `get_parcel_location` | 中心点、包围盒、地图上下文 |
| `check_road_access` | 道路临接分类 (ROAD_ADJACENT/ROAD_NEARBY/NO_ROAD_DETECTED/UNKNOWN) |
| `find_nearby_roads` | 搜索半径内的道路 |
| `get_parcel_map_context` | 地号+道路+可比交易的地图显示数据 |

### 估价工具
| Tool | 说明 |
|------|-------------|
| `estimate_land_value` | 熊/基础/牛三象区估价 + 置信度 |
| `estimate_property_value` | 土地+建物估价 |
| `explain_valuation` | 人类可读的估价说明 |

### 数据溯源工具
| Tool | 说明 |
|------|-------------|
| `get_data_snapshot` | 快照元数据 (来源、版本、记录数) |
| `get_data_provenance` | 任意结果的完整溯源链 |

参见 [MCP_API.md](MCP_API.md) 获取完整输入/输出模式。

## MCP Resources
- `realestate://snapshot/{snapshot_id}` — 数据集快照元数据
- `realestate://transaction/{transaction_id}` — 交易溯源
- `realestate://parcel/{parcel_id}` — 地号几何 + 所有权
- `realestate://valuation/{valuation_id}` — 完整估价结果
- `realestate://algorithm/{version}` — 算法配置 + 权重

## MCP Prompts
- `prompt_explain_valuation` — 在 `estimate_land_value` 后解释估价方法论
- `prompt_analyze_comparables` — 结构化可比交易分析报告
- `prompt_debug_transaction` — 诊断意外的查询结果

## 数据管道

导入管道 (参见 [SPEC.md](SPEC.md) §P2):

1. **下载** — 从 MOI 获取 CSV
2. **验证校验和** — SHA256 验证
3. **解析** — CSV → 中间数据行
4. **丰富** — 从文件名推导县市/区
5. **正规化** — 清洗 + 标准化字段
6. **验证** — 数据质量检查
7. **去重** — 删除重复交易
8. **导入** — 事务性批量插入 (`BEGIN`/`COMMIT`)
9. **锁定** — 快照状态切换为 LOCKED (不可变)

```
make migrate       # 运行数据库迁移
make seed          # 加载示例数据 (开发用)
make verify        # 运行 16 步自动化验证套件
```

## 坐标系

- **存储**: EPSG:3826 (台湾区域 2, TWD97) - PostGIS 原生
- **API 输出**: EPSG:4326 (WGS84) - 通过 `ST_Transform(geometry, 4326)`
- 转换层位于 `internal/gis/transform.go`

## 错误模型

MCP 工具错误遵循结构化格式:
```json
{
  "error": {
    "code": "PARCEL_NOT_FOUND",
    "message": "...",
    "retryable": false
  }
}
```

错误码: `INVALID_ARGUMENT`, `PARCEL_NOT_FOUND`, `TRANSACTION_NOT_FOUND`, `DATA_NOT_AVAILABLE`, `GIS_NOT_AVAILABLE`, `SNAPSHOT_NOT_FOUND`, `VALUATION_NOT_AVAILABLE`, `SOURCE_UNAVAILABLE`, `INTERNAL_ERROR`.

## 测试

```bash
# 全部测试
make test

# 测试类别
go test ./tests/isolation/...    # AI 注入防御 (P4)
go test ./tests/reproducibility/ # 确定性 + 查询哈希
go test ./tests/contract/...     # MCP 契约测试
go test ./tests/integration/...  # PostgreSQL 集成
go test ./tests/e2e/...          # 端对端接受测试
```

测试覆盖率最低要求 80%。详见 `scripts/verify.sh` 获取完整的 16 步验证流程。

## 构建

```bash
make build    # Go 二进制: bin/realestate-mcp
make docker   # 构建 Docker 镜像
```

前端:
```bash
cd frontend
npm ci
npm run dev    # http://localhost:5173
npm run build  # 生产构建到 dist/
```

## 部署

### Docker Compose (开发)
```bash
docker compose up -d --build
```

### Kubernetes/OpenShift (生产)
参见 [SPEC.md](SPEC.md) - 需要 PostgreSQL 16+ with PostGIS 3.6+, 持久卷, OpenTelemetry 收集器.

## 故障排除

**地号未在地图上显示**：
- 确保示例数据已加载: `docker compose exec tw-prop-postgres psql -U prop -d prop -c "SELECT COUNT(*) FROM parcel;"`
- 检查 PostGIS: `docker exec tw-prop-postgres psql -U prop -d prop -c "SELECT extname FROM pg_extension;"`
- 验证 MCP 连接: `curl http://localhost/healthz`

**MCP 连接错误**:
- 前端通过 `/mcp` 连接 (nginx 代理 - 同源)
- 直接服务器: `http://localhost:8080/mcp`
- 开发时使用 `MCP_TRANSPORT=stdio`

## 限制

- 示例数据必须单独加载 (`make seed` 或 SQL 导入) - 生产环境需要通过管道导入完整的 MOI 数据集
- Google Maps 提供者需要有效的 API 密钥; Leaflet + OSM 为默认
- Kubernetes/OpenShift 部署清单不包含在仓库中 - 仅在 SPEC.md 中文档记录
- 前端地号搜索 UI 尚未实现 (硬编码在 `useMCP.ts` 中)

## 开发

```bash
# 前提: Go 1.26+, PostgreSQL 16+, Docker
git clone <repo-url>
cd tw-prop-mcp
make deps      # 安装 Go 工具链
make migrate   # 数据库迁移
make seed      # 示例数据
make test      # 全部测试
```

## 许可

本项目采用 Apache License 2.0 授权 - 详见 [LICENSE](LICENSE)。

数据来源：台湾内政部实价登录 (實價登錄)。使用本数据需遵守台湾相关法律法规。
