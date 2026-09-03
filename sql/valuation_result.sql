-- Valuation result queries
-- ValuationResultRepository operations

-- name: GetValuationResult :one
SELECT * FROM valuation_result WHERE id=$1 LIMIT 1;

-- name: InsertValuationResult :one
INSERT INTO valuation_result (target_parcel_id, snapshot_id, comparable_ids, algorithm_version, configuration_version, outlier_method, weights, statistics, bear_value, base_value, bull_value, confidence, query_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING *;

-- name: ListValuationResultsByParcel :many
SELECT * FROM valuation_result WHERE target_parcel_id=$1 AND snapshot_id=$2 ORDER BY created_at DESC LIMIT $3;

-- name: GetValuationResultByQueryHash :one
SELECT * FROM valuation_result WHERE query_hash=$1 LIMIT 1;
