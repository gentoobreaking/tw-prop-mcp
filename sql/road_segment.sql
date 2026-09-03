-- name: CreateRoadSegment :one
INSERT INTO road_segment (name, road_class, width_m, width_source, geometry, source, source_version, import_batch_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: GetRoadSegmentByID :one
SELECT * FROM road_segment WHERE id = $1 LIMIT 1;

-- name: GetRoadSegmentsByName :many
SELECT * FROM road_segment WHERE name ILIKE '%' || $1 || '%' LIMIT 50;

-- name: ListRoadSegments :many
SELECT * FROM road_segment ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: BatchInsertRoadSegments :copyfrom
INSERT INTO road_segment (id, name, road_class, width_m, width_source, geometry, source, source_version, import_batch_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);