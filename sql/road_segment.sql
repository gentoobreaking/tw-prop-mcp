-- name: BatchInsertRoadSegments :copyfrom
INSERT INTO road_segment (
    name, road_class, width_m, width_source,
    geometry, source, source_version, import_batch_id
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8
);

-- name: GetRoadSegmentByID :one
SELECT * FROM road_segment WHERE id = $1 LIMIT 1;

-- name: SearchRoadSegments :many
SELECT * FROM road_segment
WHERE ($1::text IS NULL OR name ILIKE '%' || $1 || '%')
  AND ($2::text IS NULL OR road_class = $2)
  AND ($3::text IS NULL OR source = $3)
ORDER BY name
LIMIT $4 OFFSET $5;

-- name: FindRoadsNearGeometry :many
SELECT r.*, ST_Distance(r.geometry, ST_GeomFromText($1, 3826)) AS distance_m
FROM road_segment r
WHERE ST_DWithin(r.geometry, ST_GeomFromText($1, 3826), $2)
ORDER BY distance_m ASC
LIMIT $3;