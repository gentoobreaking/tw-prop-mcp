# Chapter 5 — VALUATION_SPEC.md
# Comparable / Statistics / Valuation

---

# 5.1 Valuation Principle

v2.0 不做：

```text
LLM: 我覺得這塊地值 500 萬
```

而是：

```text
Comparable transactions
        ↓
Filtering
        ↓
Scoring
        ↓
Statistics
        ↓
Adjustment
        ↓
Value range
```

---

# 5.2 Comparable Filtering

Target：

```text
T
```

Candidate：

```text
C1 ... Cn
```

第一階段 hard filters：

```text
same county
same district
same section
```

必要時：

```text
same zoning
same land-use category
```

---

# 5.3 Area Similarity

例如：

```text
target_area = 333.66 坪
```

candidate：

```text
candidate_area
```

定義：

```text
area_ratio =
candidate_area / target_area
```

距離：

```text
area_difference =
abs(candidate_area - target_area)
/
target_area
```

預設：

```text
<= 30%
```

但必須 config 化。

---

# 5.4 Time Weight

交易越接近現在，權重越高。

例如：

```text
age_months = months(now - transaction_date)

time_score =
exp(-lambda * age_months)
```

lambda 不得由 LLM 決定。

由：

```text
valuation_config
```

固定。

---

# 5.5 Spatial Weight

例如：

```text
distance_score =
exp(-distance / distance_scale)
```

其中：

```text
distance_scale
```

是 configuration。

---

# 5.6 Zoning Match

```text
same_zoning = 1
different_zoning = 0
```

---

# 5.7 Land Use Match

```text
same_land_use = 1
different_land_use = 0
```

---

# 5.8 Road Access Match

例如：

```text
target: ROAD_ADJACENT
candidate: ROAD_ADJACENT
```

則：

```text
road_access_score = 1
```

若：

```text
target: ROAD_ADJACENT
candidate: ROAD_NEARBY
```

則降低分數。

---

# 5.9 Comparable Score

第一版：

```text
total_score =
    W_area       * area_score
  + W_distance   * distance_score
  + W_time       * time_score
  + W_zoning     * zoning_score
  + W_land_use   * land_use_score
  + W_road       * road_score
```

權重必須存在於：

```text
valuation_config
```

而不是 hard-code。

---

# 5.10 Outlier Handling

至少提供：

```text
IQR
P10/P90
MAD
```

第一版建議：

```text
IQR
```

例如：

```text
Q1
Q3

IQR = Q3 - Q1

lower = Q1 - 1.5 * IQR
upper = Q3 + 1.5 * IQR
```

---

# 5.11 Statistics

所有 Comparable 必須提供：

```text
count
min
P10
P25
median
mean
P75
P90
max
```

土地單價：

```text
price_per_ping
```

必須統一：

```text
1 坪 = 3.305785 平方公尺
```

---

# 5.12 Base Value

最基本：

```text
base_price_per_ping =
weighted median
```

或：

```text
median comparable unit price
```

第一版預設採：

```text
weighted median
```

原因：

對極端交易較穩健。

---

# 5.13 Valuation Range

產生：

```text
bear_value
base_value
bull_value
```

例如：

```text
bear_value = P25 adjusted
base_value = P50 adjusted
bull_value = P75 adjusted
```

不是直接使用市場最高／最低價格。

---

# 5.14 Confidence

Confidence 不代表：

> 「AI 有多相信」

而代表：

> Comparable 資料品質有多完整。

例如：

```text
HIGH
MEDIUM
LOW
INSUFFICIENT
```

依：

```text
comparable_count
area_similarity
distance
time_range
zoning_match
land_use_match
road_access_match
```

計算。

---

# 5.15 Insufficient Data

如果：

```text
comparable_count < minimum_required
```

不得硬算估值。

回傳：

```json
{
  "status": "INSUFFICIENT_DATA",
  "reason": [
    "not enough comparable transactions"
  ]
}
```

這一點非常重要。

**不能為了讓 AI 有答案而製造答案。**

---

# 5.16 Valuation Provenance

每一個估值必須記錄：

```text
valuation_id

target_parcel

snapshot_id

comparable_ids

algorithm_version

configuration_version

outlier_method

weights

statistics

created_at
```

因此可以重新執行：

```text
same snapshot
+
same config
+
same algorithm
=
same valuation
```

---
