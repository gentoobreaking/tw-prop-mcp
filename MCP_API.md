# Chapter 3 — MCP_API.md
# MCP Interface

---

## 3.1 MCP Design Principle

MCP 是：

```text
AI → deterministic application interface
```

而不是：

```text
AI → database
```

---

# 3.2 Tool Categories

## Transaction

```text
search_transactions
get_transaction
get_transaction_statistics
```

## Parcel

```text
get_parcel
search_parcels
```

## Comparable

```text
find_comparable_transactions
score_comparable_transactions
```

## GIS

```text
get_parcel_geometry
get_parcel_location
check_road_access
find_nearby_roads
get_parcel_map_context
```

## Valuation

```text
estimate_land_value
estimate_property_value
explain_valuation
```

## Provenance

```text
get_data_snapshot
get_data_provenance
```

---

# 3.3 search_transactions

Input：

```json
{
  "county": "澎湖縣",
  "district": "西嶼鄉",
  "section": "竹篙灣段",
  "land_number": "3615",
  "date_from": "2021-01-01",
  "date_to": "2026-01-01"
}
```

Output：

```json
{
  "transactions": [],
  "statistics": {},
  "data_provenance": {}
}
```

---

# 3.4 find_comparable_transactions

Input：

```json
{
  "target": {
    "county": "澎湖縣",
    "district": "西嶼鄉",
    "section": "竹篙灣段",
    "land_number": "3615"
  },
  "filters": {
    "years": 5,
    "area_similarity_pct": 30,
    "same_zoning": true,
    "same_land_use": true,
    "road_access_required": false
  },
  "limit": 20
}
```

---

# 3.5 Comparable Result

```json
{
  "target": {},
  "comparables": [
    {
      "transaction_id": "...",
      "distance_m": 120.3,
      "area_similarity": 0.94,
      "zoning_match": true,
      "land_use_match": true,
      "road_access_match": true,
      "time_score": 0.92,
      "total_score": 0.91
    }
  ],
  "algorithm_version": "comparable-v2.0"
}
```

---

# 3.6 Tool Result Requirements

所有核心 tool response 必須包含：

```json
{
  "data": {},
  "metadata": {
    "algorithm_version": "...",
    "snapshot_id": "...",
    "generated_at": "...",
    "query_hash": "..."
  },
  "data_provenance": {}
}
```

---

# 3.7 Query Hash

將：

```text
input parameters
+
algorithm version
+
configuration version
+
snapshot
```

canonicalize 後產生：

```text
query_hash
```

用途：

```text
reproducibility
audit
cache
regression test
```

---

# 3.8 MCP Resources

除了 Tools，v2.0 可提供 Resources：

```text
realestate://snapshot/{id}

realestate://transaction/{id}

realestate://parcel/{id}

realestate://valuation/{id}

realestate://algorithm/{version}
```

---

# 3.9 Error Model

統一：

```json
{
  "error": {
    "code": "PARCEL_NOT_FOUND",
    "message": "...",
    "retryable": false
  }
}
```

Error codes：

```text
INVALID_ARGUMENT
PARCEL_NOT_FOUND
TRANSACTION_NOT_FOUND
DATA_NOT_AVAILABLE
GIS_NOT_AVAILABLE
SNAPSHOT_NOT_FOUND
VALUATION_NOT_AVAILABLE
SOURCE_UNAVAILABLE
INTERNAL_ERROR
```

---

# 3.10 AI Isolation Rules

禁止 MCP tool 接受：

```text
SQL
raw SQL WHERE
PostGIS expression
arbitrary code
valuation formula
```

例如禁止：

```json
{
  "sql": "SELECT ..."
}
```

必須：

```json
{
  "section": "竹篙灣段",
  "area_min_sqm": 100,
  "area_max_sqm": 300
}
```

由 service layer 轉成 SQL。

---
