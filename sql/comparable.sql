-- name: InsertComparableResult :one
INSERT INTO comparable_result (target_parcel_id, candidate_transaction_id, distance_m, area_similarity, zoning_match, land_use_match, road_access_match, time_score, distance_score, total_score, algorithm_version)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING *;

-- name: ListComparableResults :many
SELECT * FROM comparable_result WHERE target_parcel_id=$1 ORDER BY total_score DESC, distance_m ASC LIMIT $2;

-- name: GetValuationResult :one
SELECT * FROM valuation_result WHERE id=$1 LIMIT 1;

-- name: InsertValuationResult :one
INSERT INTO valuation_result (target_parcel_id, snapshot_id, comparable_ids, algorithm_version, configuration_version, outlier_method, weights, statistics, bear_value, base_value, bull_value, confidence, query_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING *;

-- name: GetConfig :one
SELECT * FROM configuration_snapshot WHERE version=$1 LIMIT 1;

-- name: GetAlgorithmVersion :one
SELECT * FROM algorithm_version WHERE version=$1 LIMIT 1;
