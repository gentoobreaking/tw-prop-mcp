-- Comparable result queries
-- ComparableResultRepository operations

-- name: InsertComparableResult :one
INSERT INTO comparable_result (target_parcel_id, candidate_transaction_id, distance_m, area_similarity, zoning_match, land_use_match, road_access_match, time_score, distance_score, total_score, algorithm_version)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING *;

-- name: ListComparableResults :many
-- Deterministic ordering: total_score DESC, then distance_m ASC, then transaction_id ASC
SELECT * FROM comparable_result WHERE target_parcel_id=$1 ORDER BY total_score DESC, distance_m ASC, candidate_transaction_id ASC LIMIT $2;

-- name: GetComparableResultByID :one
SELECT * FROM comparable_result WHERE id=$1 LIMIT 1;
