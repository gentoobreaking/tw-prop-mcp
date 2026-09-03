-- T002: Core schema — PostgreSQL 16 + PostGIS 3.5, EPSG:3826 internal
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- dataset_snapshot
CREATE TABLE dataset_snapshot (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source              VARCHAR(50) NOT NULL,
    source_version      VARCHAR(50) NOT NULL,
    downloaded_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at        TIMESTAMPTZ,
    file_name           VARCHAR(255) NOT NULL,
    file_sha256         CHAR(64) NOT NULL,
    record_count        BIGINT NOT NULL DEFAULT 0,
    status              VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','IMPORTING','LOCKED','FAILED')),
    schema_version      VARCHAR(20) NOT NULL DEFAULT 'v2.0',
    import_started_at   TIMESTAMPTZ,
    import_completed_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source, source_version, file_sha256)
);
CREATE INDEX idx_snapshot_status ON dataset_snapshot(status);
CREATE INDEX idx_snapshot_source_version ON dataset_snapshot(source, source_version);

-- import_batch
CREATE TABLE import_batch (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id         UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    status              VARCHAR(20) NOT NULL DEFAULT 'RUNNING' CHECK (status IN ('RUNNING','COMPLETED','FAILED')),
    records_processed   BIGINT NOT NULL DEFAULT 0,
    records_imported    BIGINT NOT NULL DEFAULT 0,
    records_failed      BIGINT NOT NULL DEFAULT 0,
    record_count        BIGINT NOT NULL DEFAULT 0,
    error_message       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_import_batch_snapshot ON import_batch(snapshot_id);

-- transaction (core)
CREATE TABLE transaction (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id             UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
    import_batch_id         UUID NOT NULL REFERENCES import_batch(id) ON DELETE RESTRICT,
    transaction_id          VARCHAR(50) NOT NULL,
    transaction_date        DATE NOT NULL,
    transaction_type        VARCHAR(20) NOT NULL,
    county                  VARCHAR(20) NOT NULL,
    district                VARCHAR(20) NOT NULL,
    section                 VARCHAR(50),
    land_number             VARCHAR(50),
    transaction_target      VARCHAR(50),
    total_price             BIGINT NOT NULL CHECK (total_price > 0),
    unit_price              BIGINT NOT NULL CHECK (unit_price > 0),
    land_area_sqm           NUMERIC(12,4) CHECK (land_area_sqm IS NULL OR land_area_sqm > 0),
    building_area_sqm       NUMERIC(12,4) CHECK (building_area_sqm IS NULL OR building_area_sqm > 0),
    urban_zoning            VARCHAR(50),
    non_urban_zoning        VARCHAR(50),
    land_use_category       VARCHAR(50),
    building_type           VARCHAR(50),
    floor                   VARCHAR(20),
    age                     INTEGER CHECK (age IS NULL OR age >= 0),
    parking_area_sqm        NUMERIC(10,4) CHECK (parking_area_sqm IS NULL OR parking_area_sqm >= 0),
    parking_price           BIGINT CHECK (parking_price IS NULL OR parking_price >= 0),
    source_record_hash      CHAR(64) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (snapshot_id, source_record_hash),
    UNIQUE (county, district, section, land_number, snapshot_id, transaction_id)
);
CREATE INDEX idx_txn_snapshot ON transaction(snapshot_id);
CREATE INDEX idx_txn_location ON transaction(county, district, section);
CREATE INDEX idx_txn_date ON transaction(transaction_date);
CREATE INDEX idx_txn_land_number ON transaction(county, district, section, land_number);
CREATE INDEX idx_txn_import_batch ON transaction(import_batch_id);

-- transaction_land
CREATE TABLE transaction_land (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id          UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
    land_area_sqm           NUMERIC(12,4) NOT NULL CHECK (land_area_sqm > 0),
    land_use_category       VARCHAR(50),
    urban_zoning            VARCHAR(50),
    non_urban_zoning        VARCHAR(50),
    land_value              BIGINT CHECK (land_value IS NULL OR land_value > 0),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_txn_land_txn ON transaction_land(transaction_id);

-- transaction_building
CREATE TABLE transaction_building (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id          UUID NOT NULL REFERENCES transaction(id) ON DELETE CASCADE,
    building_area_sqm       NUMERIC(12,4) CHECK (building_area_sqm IS NULL OR building_area_sqm > 0),
    building_type           VARCHAR(50),
    floor                   VARCHAR(20),
    age                     INTEGER CHECK (age IS NULL OR age >= 0),
    parking_area_sqm        NUMERIC(10,4) CHECK (parking_area_sqm IS NULL OR parking_area_sqm >= 0),
    parking_price           BIGINT CHECK (parking_price IS NULL OR parking_price >= 0),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_txn_building_txn ON transaction_building(transaction_id);

-- parcel (master)
CREATE TABLE parcel (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    county                  VARCHAR(20) NOT NULL,
    district                VARCHAR(20) NOT NULL,
    section                 VARCHAR(50) NOT NULL,
    land_number             VARCHAR(50) NOT NULL,
    area_sqm                NUMERIC(12,4) NOT NULL CHECK (area_sqm > 0),
    urban_zoning            VARCHAR(50),
    land_use_category       VARCHAR(50),
    geometry                GEOMETRY(MULTIPOLYGON, 3826) NOT NULL CHECK (ST_SRID(geometry)=3826),
    centroid                GEOMETRY(POINT, 3826),
    bbox                    GEOMETRY(POLYGON, 3826),
    source                  VARCHAR(50) NOT NULL,
    source_version          VARCHAR(50) NOT NULL,
    import_batch_id         UUID REFERENCES import_batch(id) ON DELETE RESTRICT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (county, district, section, land_number, source, source_version)
);
CREATE INDEX idx_parcel_location ON parcel(county, district, section);
CREATE INDEX idx_parcel_geometry ON parcel USING GIST(geometry);
CREATE INDEX idx_parcel_centroid ON parcel USING GIST(centroid);
CREATE INDEX idx_parcel_source_version ON parcel(source, source_version);

-- parcel_geometry (history / versioned geometry, complements parcel)
CREATE TABLE parcel_geometry (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id               UUID NOT NULL REFERENCES parcel(id) ON DELETE CASCADE,
    geometry                GEOMETRY(MULTIPOLYGON, 3826) NOT NULL CHECK (ST_SRID(geometry)=3826),
    area_sqm                NUMERIC(12,4) NOT NULL CHECK (area_sqm > 0),
    centroid                GEOMETRY(POINT, 3826),
    bbox                    GEOMETRY(POLYGON, 3826),
    source                  VARCHAR(50) NOT NULL,
    source_version          VARCHAR(50) NOT NULL,
    import_batch_id         UUID REFERENCES import_batch(id) ON DELETE RESTRICT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_parcel_geometry_parcel ON parcel_geometry(parcel_id);
CREATE INDEX idx_parcel_geometry_gist ON parcel_geometry USING GIST(geometry);

-- road_segment
CREATE TABLE road_segment (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    VARCHAR(100),
    road_class              VARCHAR(20),
    width_m                 NUMERIC(6,2) CHECK (width_m IS NULL OR width_m >= 0),
    width_source            VARCHAR(20) NOT NULL CHECK (width_source IN ('OFFICIAL','GIS_DERIVED','UNKNOWN')),
    geometry                GEOMETRY(MULTILINESTRING, 3826) NOT NULL CHECK (ST_SRID(geometry)=3826),
    source                  VARCHAR(50) NOT NULL,
    source_version          VARCHAR(50) NOT NULL,
    import_batch_id         UUID REFERENCES import_batch(id) ON DELETE RESTRICT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_road_geometry ON road_segment USING GIST(geometry);
CREATE INDEX idx_road_class ON road_segment(road_class);
CREATE INDEX idx_road_source_version ON road_segment(source, source_version);

-- parcel_road_access
CREATE TABLE parcel_road_access (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id               UUID NOT NULL REFERENCES parcel(id) ON DELETE CASCADE,
    road_id                 UUID REFERENCES road_segment(id) ON DELETE SET NULL,
    distance_m              NUMERIC(8,2) NOT NULL CHECK (distance_m >= 0),
    nearest_point           GEOMETRY(POINT, 3826) CHECK (nearest_point IS NULL OR ST_SRID(nearest_point)=3826),
    road_width_m            NUMERIC(6,2) CHECK (road_width_m IS NULL OR road_width_m >= 0),
    access_type             VARCHAR(20) NOT NULL CHECK (access_type IN ('ROAD_ADJACENT','ROAD_NEARBY','NO_ROAD_DETECTED','UNKNOWN')),
    source                  VARCHAR(50) NOT NULL,
    algorithm_version       VARCHAR(50) NOT NULL,
    computed_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (parcel_id, algorithm_version)
);
CREATE INDEX idx_parcel_road_access_parcel ON parcel_road_access(parcel_id);
CREATE INDEX idx_parcel_road_access_type ON parcel_road_access(access_type);

-- comparable_result
CREATE TABLE comparable_result (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_parcel_id            UUID NOT NULL REFERENCES parcel(id) ON DELETE RESTRICT,
    candidate_transaction_id    UUID NOT NULL REFERENCES transaction(id) ON DELETE RESTRICT,
    distance_m                  NUMERIC(10,2),
    area_similarity             NUMERIC(6,4),
    zoning_match                BOOLEAN NOT NULL,
    land_use_match              BOOLEAN NOT NULL,
    road_access_match           BOOLEAN NOT NULL,
    time_score                  NUMERIC(6,4),
    distance_score              NUMERIC(6,4),
    total_score                 NUMERIC(6,4) NOT NULL,
    algorithm_version           VARCHAR(50) NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_comparable_target ON comparable_result(target_parcel_id);
CREATE INDEX idx_comparable_candidate ON comparable_result(candidate_transaction_id);

-- valuation_result
CREATE TABLE valuation_result (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_parcel_id        UUID NOT NULL REFERENCES parcel(id) ON DELETE RESTRICT,
    snapshot_id             UUID NOT NULL REFERENCES dataset_snapshot(id) ON DELETE RESTRICT,
    comparable_ids          JSONB NOT NULL DEFAULT '[]'::jsonb,
    algorithm_version       VARCHAR(50) NOT NULL,
    configuration_version   VARCHAR(50) NOT NULL,
    outlier_method          VARCHAR(20) NOT NULL CHECK (outlier_method IN ('IQR','P10_P90','MAD')),
    weights                 JSONB NOT NULL,
    statistics              JSONB NOT NULL,
    bear_value              BIGINT NOT NULL CHECK (bear_value > 0),
    base_value              BIGINT NOT NULL CHECK (base_value > 0),
    bull_value              BIGINT NOT NULL CHECK (bull_value > 0),
    confidence              VARCHAR(20) NOT NULL CHECK (confidence IN ('HIGH','MEDIUM','LOW','INSUFFICIENT')),
    status                  VARCHAR(20) NOT NULL DEFAULT 'COMPLETED' CHECK (status IN ('COMPLETED','INSUFFICIENT_DATA','FAILED')),
    query_hash              CHAR(64) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (query_hash)
);
CREATE INDEX idx_valuation_parcel ON valuation_result(target_parcel_id);
CREATE INDEX idx_valuation_snapshot ON valuation_result(snapshot_id);
CREATE INDEX idx_valuation_query_hash ON valuation_result(query_hash);
CREATE INDEX idx_valuation_comparable_ids ON valuation_result USING GIN(comparable_ids);

-- algorithm_version
CREATE TABLE algorithm_version (
    version                 VARCHAR(50) PRIMARY KEY,
    name                    VARCHAR(100) NOT NULL,
    description             TEXT,
    weights                 JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- configuration_snapshot
CREATE TABLE configuration_snapshot (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version                 VARCHAR(50) NOT NULL UNIQUE,
    config                  JSONB NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default algorithm/config (v2.0) for T028
INSERT INTO algorithm_version (version, name, description, weights) VALUES
('comparable-v2.0', 'Comparable Scoring v2.0', 'W_area=0.30 W_distance=0.20 W_time=0.15 W_zoning=0.15 W_land_use=0.10 W_road=0.10', '{"W_area":0.30,"W_distance":0.20,"W_time":0.15,"W_zoning":0.15,"W_land_use":0.10,"W_road":0.10}'::jsonb),
('valuation-v2.0', 'Valuation v2.0', 'weighted median + IQR outlier', '{"outlier":"IQR","k":1.5}'::jsonb)
ON CONFLICT (version) DO NOTHING;

INSERT INTO configuration_snapshot (version, config) VALUES
('v2.0', '{"area_similarity_pct":30,"lambda":0.05,"distance_scale":500,"W_area":0.30,"W_distance":0.20,"W_time":0.15,"W_zoning":0.15,"W_land_use":0.10,"W_road":0.10,"IQR_k":1.5,"minimum_required_comparables":3,"outlier_method":"IQR"}'::jsonb)
ON CONFLICT (version) DO NOTHING;

-- Coordinate transform sanity check: 4326 (119.5,23.5) -> 3826 distance sanity
-- This is a no-op validation trigger placeholder; real check done in integration test
DO $$ BEGIN
  PERFORM ST_Transform(ST_SetSRID(ST_MakePoint(119.5, 23.5), 4326), 3826);
END $$;
