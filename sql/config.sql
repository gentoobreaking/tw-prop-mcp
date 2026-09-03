-- Configuration queries
-- Used by config service and provenance lookups

-- name: GetConfig :one
SELECT * FROM configuration_snapshot WHERE version=$1 LIMIT 1;

-- name: GetAlgorithmVersion :one
SELECT * FROM algorithm_version WHERE version=$1 LIMIT 1;
