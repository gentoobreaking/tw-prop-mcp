# Chapter 4 — GIS_SPEC.md
# GIS / 地籍 / 道路 / 地圖

---

## 4.1 GIS Objective

GIS 系統不是單純「把地圖畫出來」。

它必須回答：

```text
這塊地在哪裡？
形狀如何？
面積多少？
附近有哪些道路？
是否臨路？
距離道路多少？
道路寬度？
附近交易在哪？
```

---

# 4.2 GIS Sources

主要考慮：

```text
國土測繪圖資服務雲
地籍圖資網路便民服務系統
政府 GIS/Open Data
```

國土測繪圖資服務雲本身整合地籍圖、正射影像、土地使用等多種圖資，適合作為 GIS overlay 的官方來源。

地籍圖資網路便民服務系統亦提供依地號、地址等方式查詢地籍位置與相關圖資。

---

# 4.3 GIS Architecture

```text
Official GIS
     │
     ▼
GIS Adapter
     │
     ▼
Normalize Geometry
     │
     ▼
PostGIS
     │
     ├── Parcel
     ├── Road
     ├── Zoning
     └── POI
```

---

# 4.4 Coordinate System

系統內部統一：

```text
EPSG:3826
```

對外 API 可以：

```text
EPSG:4326
```

因此：

```text
API coordinates
      ↓
4326
      ↓
PostGIS transform
      ↓
3826
```

---

# 4.5 Parcel Geometry

核心：

```sql
ST_Intersects()
ST_Within()
ST_Contains()
ST_Distance()
ST_DWithin()
ST_Area()
ST_Centroid()
```

所有大量 spatial query 必須由 PostGIS 執行。

禁止：

```text
SELECT all geometry
↓
Go memory
↓
calculate distance
```

---

# 4.6 Road Access Algorithm

臨路判定不能只靠：

```text
distance < X
```

至少需要：

```text
parcel boundary
+
road geometry
+
distance
+
intersection
```

定義：

### ROAD_ADJACENT

土地邊界與道路 geometry 有直接接觸或在設定 tolerance 內。

### ROAD_NEARBY

道路在指定距離內，但無法證明土地直接臨路。

### NO_ROAD_DETECTED

指定搜尋範圍沒有道路。

### UNKNOWN

GIS source 不足。

---

# 4.7 Road Width

道路寬度來源分為：

```text
OFFICIAL
GIS_DERIVED
UNKNOWN
```

禁止從衛星圖「猜」道路寬度後當成官方資料。

---

# 4.8 Google Maps Integration

Google Maps 不作為：

```text
official cadastral source
```

而作為：

```text
visualization
satellite context
street view
navigation context
```

Google Maps JavaScript API 支援 interactive map、satellite/hybrid map 與 Street View。

架構：

```text
MCP
 │
 ├── parcel geometry
 ├── centroid
 ├── road geometry
 └── transaction locations
        │
        ▼
Frontend
        │
        ├── NLSC GIS layer
        ├── Google Satellite
        └── Google Street View
```

---

# 4.9 Street View

Street View 只提供：

```text
visual verification
```

不得將：

```text
Street View visible road
```

直接轉成：

```text
official road width
```

Google 官方 API 可依座標尋找附近 Street View panorama。

---

# 4.10 GIS Output

```json
{
  "parcel": {
    "geometry": {},
    "centroid": {}
  },
  "road_access": {
    "status": "ROAD_ADJACENT",
    "distance_m": 0,
    "road_width_m": 6,
    "width_source": "OFFICIAL"
  },
  "map_context": {
    "latitude": 23.56,
    "longitude": 119.50
  }
}
```

---
