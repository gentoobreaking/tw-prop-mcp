-- name: GetTransactionByID :one
SELECT * FROM transaction WHERE id = $1 LIMIT 1;

-- name: SearchTransactions :many
SELECT * FROM transaction
WHERE county = $1
  AND district = $2
  AND ($3::text = '' OR section = $3)
  AND ($4::text = '' OR land_number = $4)
  AND ($5::date IS NULL OR transaction_date >= $5)
  AND ($6::date IS NULL OR transaction_date <= $6)
ORDER BY transaction_date DESC, id ASC
LIMIT $7 OFFSET $8;

-- name: BatchInsertTransactions :copyfrom
INSERT INTO transaction (
    snapshot_id, import_batch_id, transaction_id, transaction_date, transaction_type,
    county, district, section, land_number, transaction_target,
    total_price, unit_price, land_area_sqm, building_area_sqm,
    urban_zoning, non_urban_zoning, land_use_category,
    building_type, floor, age, parking_area_sqm, parking_price, source_record_hash
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17,
    $18, $19, $20, $21, $22, $23
);

-- name: GetTransactionStats :one
SELECT
    COUNT(*)::bigint AS cnt,
    COALESCE(MIN(total_price),0)::bigint AS min_price,
    COALESCE(MAX(total_price),0)::bigint AS max_price,
    COALESCE(AVG(total_price),0)::bigint AS avg_price
FROM transaction
WHERE county = $1 AND district = $2 AND section = $3;

-- name: GetTransactionPercentiles :one
-- percentile_cont for total_price (p25/p50/p75) and area medians by county/district/section.
SELECT
    COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY total_price), 0)::float8 AS median_price,
    COALESCE(percentile_cont(0.25) WITHIN GROUP (ORDER BY total_price), 0)::float8 AS p25_price,
    COALESCE(percentile_cont(0.75) WITHIN GROUP (ORDER BY total_price), 0)::float8 AS p75_price,
    COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY land_area_sqm), 0)::float8 AS median_land_area,
    COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY building_area_sqm), 0)::float8 AS median_building_area
FROM transaction
WHERE county = $1 AND district = $2 AND section = $3;
