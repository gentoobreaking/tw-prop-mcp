# Data Model Specification

## Overview
This document defines the complete data model for the Taiwan Property Valuation MCP Server, including database schema, domain models, and their relationships.

## Database Schema

### Core Tables

#### dataset_snapshot
Stores immutable data snapshots from official sources.

```sql
CREATE TABLE dataset_snapshot (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source              VARCHAR(50) NOT NULL,                    -- 'MOI'
    source_version      VARCHAR(50) NOT NULL,                    -- '2024Q1', '2024Q2', etc.
    downloaded_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at        TIMESTAMPTZ,
    file_name           VARCHAR(255) NOT NULL,
    file_sha256         CHAR(64) NOT NULL,
    record_count        BIGINT NOT NULL DEFAULT 0,
    status              VARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING, IMPORTING, LOCKED, FAILED
    schema_version      VARCHAR(20) NOT NULL DEFAULT '1.0',
    import_started_at   TIMESTAMPTZ,
    import_completed_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE (source, source_version, file_sha256)
);

CREATE INDEX idx_snapshot_status ON dataset_snapshot(status);
CREATE INDEX idx_snapshot_source_version ON dataset_snapshot(source, source_version);
```

#### import_batch
Tracks each import execution for provenance.

```sql
CREATE TABLE import_batch (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id         UUID NOT NULL REFERENCES dataset_snapshot(id),
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    status              VARCHAR(20) NOT NULL DEFAULT 'RUNNING',  -- RUNNING, COMPLETED, FAILED
    records_processed   BIGINT NOT NULL DEFAULT 0,
    records_imported    BIGINT NOT NULL DEFAULT 0,
    records_failed      BIGINT NOT NULL DEFAULT 0,
    error_message       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CHECK (status IN ('RUNNING', 'COMPLETED', 'FAILED'))
);

CREATE INDEX idx_import_batch_snapshot ON import_batch(snapshot_id);
```

#### transaction
Core transaction records from official data.

```sql
CREATE TABLE transaction (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id             UUID NOT NULL REFERENCES dataset_snapshot(id),
    import_batch_id         UUID NOT NULL REFERENCES import_batch(id),
    transaction_id          VARCHAR(50) NOT NULL,              -- Official transaction ID
    transaction_date        DATE NOT NULL,
    transaction_type        VARCHAR(20) NOT NULL,              -- '土地', '建物', '土地建物'
    county                  VARCHAR(20) NOT NULL,
    district                VARCHAR(20) NOT NULL,
    section                 VARCHAR(50),
    land_number             VARCHAR(50),
    transaction_target      VARCHAR(50),                       -- '土地', '建物', '土地建物', '車位'
    total_price             BIGINT NOT NULL,
    unit_price              BIGINT NOT NULL,                   -- 元/平方公尺
    land_area_sqm           NUMERIC(12,4),
    building_area_sqm       NUMERIC(12,4),
    urban_zoning            VARCHAR(50),
    non_urban_zoning        VARCHAR(50),
    land_use_category       VARCHAR(50),
    building_type           VARCHAR(50),
    floor                   VARCHAR(20),
    age                     INTEGER,
    parking_area_sqm        NUMERIC(10,4),
    parking_price           BIGINT,
    source_record_hash      CHAR(64) NOT NULL,                 -- SHA256 of raw CSV row
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE (snapshot_id, source_record_hash)
);

CREATE INDEX idx_txn_snapshot ON transaction(snapshot_id);
CREATE INDEX idx_txn_location ON transaction(county, district, section);
CREATE INDEX idx_txn_date ON transaction(transaction_date);
CREATE INDEX idx_txn_land_number ON transaction(county, district, section, land_number);
CREATE INDEX idx_txn_import_batch ON transaction(import_batch_id);
```

#### transaction_land
Land-specific details for transactions.

```sql
CREATE TABLE transaction_land (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id          UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
    land_area_sqm           NUMERIC(12,4) NOT NULL,
    land_use_category       VARCHAR(50),
    urban_zoning            VARCHAR(50),
    non_urban_zoning        VARCHAR(50),
    land_value              BIGINT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_txn_land_txn ON transaction_land(transaction_id);
```

#### transaction_building
Building-specific details for transactions.

```sql
CREATE TABLE transaction_building (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id          UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
    building_area_sqm       NUMERIC(12,4),
    building_type           VARCHAR(50),
    floor                   VARCHAR(20),
    age                     INTEGER,
    parking_area_sqm        NUMERIC(10,4),
    parking_price           BIGINT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_txn_building_txn ON transaction_building(transaction_id);
```

#### parcel
Land parcel master data from official cadastral maps.

```sql
CREATE TABLE parcel (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    county                  VARCHAR(20) NOT NULL,
    district                VARCHAR(20) NOT NULL,
    section                 VARCHAR(50) NOT NULL,
    land_number             VARCHAR(50) NOT NULL,
    area_sqm                NUMERIC(12,4) NOT NULL,
    urban_zoning            VARCHAR(50),
    land_use_category       VARCHAR(50),
    geometry                GEOMETRY(MULTIPOLYGON, 3826) NOT NULL,
    centroid                GEOMETRY(POINT, 3826) GENERATED ALWAYS AS (ST_Centroid(geometry)) STORED,
    source                  VARCHAR(50) NOT NULL,              -- 'NLSC', 'MOI'
    source_version          VARCHAR(50) NOT NULL,
    import_batch_id         UUID REFERENCES import_batch(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE (county, district, section, land_number, source, source_version)
);

CREATE INDEX idx_parcel_location ON parcel(county, district, section);
CREATE INDEX idx_parcel_geometry ON parcel USING GIST(geometry);
CREATE INDEX idx_parcel_source_version ON parcel(source, source_version);
```

#### parcel_geometry_history
Tracks geometry changes over time.

```sql
CREATE TABLE parcel_geometry_history (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id               UUID NOT NULL REFERENCES parcel(id) ON DELETE CASCADE,
    geometry                GEOMETRY(MULTIPOLYGON, 3826) NOT NULL,
    area_sqm                NUMERIC(12,4) NOT NULL,
    source                  VARCHAR(50) NOT NULL,
    source_version          VARCHAR(50) NOT NULL,
    valid_from              DATE NOT NULL,
    valid_to                DATE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_parcel_geom_hist_parcel ON parcel_geometry_history(parcel_id);
CREATE INDEX idx_parcel_geom_hist_date ON parcel_geometry_history(valid_from, valid_to);
```

### GIS Tables

#### road_segment
Road network data for access analysis.

```sql
CREATE TABLE road_segment (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    VARCHAR(100),
    road_class              VARCHAR(20),                       -- '國道', '省道', '縣道', '鄉道', '巷道', '其他'
    width_m                 NUMERIC(6,2),
    width_source            VARCHAR(20) NOT NULL,              -- 'OFFICIAL', 'GIS_DERIVED', 'UNKNOWN'
    geometry                GEOMETRY(MULTILINESTRING, 3826) NOT NULL,
    source                  VARCHAR(50) NOT NULL,
    source_version          VARCHAR(50) NOT NULL,
    import_batch_id         UUID REFERENCES import_batch(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CHECK (width_source IN ('OFFICIAL', 'GIS_DERIVED', 'UNKNOWN'))
);

CREATE INDEX idx_road_geometry ON road_segment USING GIST(geometry);
CREATE INDEX idx_road_class ON road_segment(road_class);
CREATE INDEX idx_road_source_version ON road_segment(source, source_version);
```

#### parcel_road_access
Pre-computed road access for parcels.

```sql
CREATE TABLE parcel_road_access (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id               UUID NOT NULL REFERENCES parcel(id) ON DELETE CASCADE,
    road_id                 UUID REFERENCES road_segment(id) ON DELETE SET NULL,
    distance_m              NUMERIC(8,2) NOT NULL,
    nearest_point           GEOMETRY(POINT, 3826),
    road_width_m            NUMERIC(6,2),
    access_type             VARCHAR(20) NOT NULL,              -- 'ROAD_ADJACENT', 'ROAD_NEARBY', 'NO_ROAD_DETECTED', 'UNKNOWN'
    source                  VARCHAR(50) NOT NULL,
    algorithm_version       VARCHAR(50) NOT NULL,
    computed_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CHECK (access_type IN ('ROAD_ADJACENT', 'ROAD_NEARBY', 'NO_ROAD_DETECTED', 'UNKNOWN')),
    UNIQUE (parcel_id, algorithm_version)
);

CREATE INDEX idx_parcel_road_access_parcel ON parcel_road_access(parcel_id);
CREATE INDEX idx_parcel_road_access_type ON parcel_road_access(access_type);
```

### Valuation Tables

#### valuation_config
Locked configuration for valuation engine.

```sql
CREATE TABLE valuation_config (
    version                 VARCHAR(50) PRIMARY KEY,
    weights                 JSONB NOT NULL,                    -- {"area": 0.3, "distance": 0.2, "time": 0.15, "zoning": 0.15, "land_use": 0.1, "road": 0.1}
    lambda                  NUMERIC(6,4) NOT NULL,             -- Time decay: exp(-lambda * months)
    distance_scale          NUMERIC(8,2) NOT NULL,             -- Distance decay: exp(-distance / scale)
    area_threshold          NUMERIC(4,2) NOT NULL DEFAULT 0.30,
    min_comparables         INTEGER NOT NULL DEFAULT 3,
    outlier_method          VARCHAR(20) NOT NULL DEFAULT 'IQR', -- 'IQR', 'P10_P90', 'MAD'
    iqr_multiplier          NUMERIC(3,1) NOT NULL DEFAULT 1.5,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked                  BOOLEAN NOT NULL DEFAULT FALSE,
    
    CHECK (outlier_method IN ('IQR', 'P10_P90', 'MAD')),
    CHECK (locked = FALSE OR version = (SELECT version FROM valuation_config WHERE locked = TRUE LIMIT 1))
);

-- Only one version can be locked at a time
CREATE UNIQUE INDEX idx_valuation_config_locked ON valuation_config(locked) WHERE locked = TRUE;
```

#### valuation_result
Immutable valuation results.

```sql
CREATE TABLE valuation_result (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_parcel_id        UUID NOT NULL REFERENCES parcel(id),
    snapshot_id             UUID NOT NULL REFERENCES dataset_snapshot(id),
    bear_value              BIGINT NOT NULL,
    base_value              BIGINT NOT NULL,
    bull_value              BIGINT NOT NULL,
    confidence              VARCHAR(20) NOT NULL,              -- 'HIGH', 'MEDIUM', 'LOW', 'INSUFFICIENT'
    comparable_count        INTEGER NOT NULL,
    algorithm_version       VARCHAR(50) NOT NULL,
    configuration_version   VARCHAR(50) NOT NULL REFERENCES valuation_config(version),
    outlier_method          VARCHAR(20) NOT NULL,
    weights                 JSONB NOT NULL,
    statistics              JSONB NOT NULL,                    -- {count, min, p10, p25, median, mean, p75, p90, max}
    provenance              JSONB NOT NULL,                    -- Full provenance chain
    query_hash              CHAR(64) NOT NULL,                 -- SHA256 of canonicalized query
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CHECK (confidence IN ('HIGH', 'MEDIUM', 'LOW', 'INSUFFICIENT')),
    UNIQUE (query_hash)
);

CREATE INDEX idx_valuation_parcel ON valuation_result(target_parcel_id);
CREATE INDEX idx_valuation_snapshot ON valuation_result(snapshot_id);
CREATE INDEX idx_valuation_query_hash ON valuation_result(query_hash);
```

#### comparable_transaction
Links valuation to comparable transactions with scores.

```sql
CREATE TABLE comparable_transaction (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    valuation_id            UUID NOT NULL REFERENCES valuation_result(id) ON DELETE CASCADE,
    transaction_id          UUID NOT NULL REFERENCES transaction(id),
    score                   NUMERIC(6,4) NOT NULL,
    area_similarity         NUMERIC(6,4),
    distance_m              NUMERIC(8,2),
    time_weight             NUMERIC(6,4),
    zoning_match            BOOLEAN,
    land_use_match          BOOLEAN,
    road_access_match       BOOLEAN,
    rank                    INTEGER NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comparable_valuation ON comparable_transaction(valuation_id);
CREATE INDEX idx_comparable_txn ON comparable_transaction(transaction_id);
```

### Provenance Tables

#### data_provenance
Complete provenance chain for any data entity.

```sql
CREATE TABLE data_provenance (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type             VARCHAR(50) NOT NULL,              -- 'transaction', 'parcel', 'valuation', 'road_segment'
    entity_id               UUID NOT NULL,
    source                  VARCHAR(50) NOT NULL,
    source_version          VARCHAR(50) NOT NULL,
    snapshot_id             UUID REFERENCES dataset_snapshot(id),
    source_record_hash      CHAR(64),
    import_batch_id         UUID REFERENCES import_batch(id),
    parent_provenance_id    UUID REFERENCES data_provenance(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_provenance_entity ON data_provenance(entity_type, entity_id);
CREATE INDEX idx_provenance_snapshot ON data_provenance(snapshot_id);
```

## Domain Model Relationships

```
DatasetSnapshot (1) ←→ (N) ImportBatch
DatasetSnapshot (1) ←→ (N) Transaction
ImportBatch (1) ←→ (N) Transaction
Transaction (1) ←→ (1) TransactionLand
Transaction (1) ←→ (1) TransactionBuilding
DatasetSnapshot (1) ←→ (N) Parcel (via import_batch)
Parcel (1) ←→ (N) ParcelGeometryHistory
Parcel (1) ←→ (1) ParcelRoadAccess
RoadSegment (1) ←→ (N) ParcelRoadAccess
ValuationConfig (1) ←→ (N) ValuationResult
ValuationResult (1) ←→ (N) ComparableTransaction
Transaction (1) ←→ (N) ComparableTransaction
Parcel (1) ←→ (N) DataProvenance
Transaction (1) ←→ (N) DataProvenance
ValuationResult (1) ←→ (N) DataProvenance
```

## Constraints Summary

| Table | Constraint | Purpose |
|-------|------------|---------|
| dataset_snapshot | UNIQUE(source, source_version, file_sha256) | Immutable snapshot deduplication |
| transaction | UNIQUE(snapshot_id, source_record_hash) | Prevent duplicate imports |
| parcel | UNIQUE(county, district, section, land_number, source, source_version) | Parcel identity |
| parcel_road_access | UNIQUE(parcel_id, algorithm_version) | One access result per algorithm version |
| valuation_config | UNIQUE(locked) WHERE locked | Single active config |
| valuation_result | UNIQUE(query_hash) | Reproducibility guarantee |

## Indexing Strategy

- **Spatial**: GIST indexes on all geometry columns
- **Temporal**: B-tree on date/timestamp columns
- **Lookup**: Composite indexes on common query patterns (county+district+section)
- **Provenance**: Indexes on entity_type+entity_id and snapshot_id

## Migration Notes

- All tables use UUID primary keys
- Timestamps use TIMESTAMPTZ
- Numeric precision defined for financial/area data
- Soft deletes NOT used - immutability enforced by constraints
- Locked tables (valuation_config) use partial unique indexes