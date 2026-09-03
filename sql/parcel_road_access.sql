-- name: CreateParcelRoadAccess :one
INSERT INTO parcel_road_access (parcel_id, road_id, distance_m, nearest_point, road_width_m, access_type, source, algorithm_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: GetParcelRoadAccessByID :one
SELECT * FROM parcel_road_access WHERE id = $1 LIMIT 1;

-- name: GetParcelRoadAccessByParcelID :one
SELECT * FROM parcel_road_access WHERE parcel_id = $1 LIMIT 1;

-- name: ListParcelRoadAccess :many
SELECT * FROM parcel_road_access ORDER BY computed_at DESC LIMIT $1 OFFSET $2;

-- name: BatchInsertParcelRoadAccess :copyfrom
INSERT INTO parcel_road_access (parcel_id, road_id, distance_m, nearest_point, road_width_m, access_type, source, algorithm_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: DeleteParcelRoadAccess :exec
DELETE FROM parcel_road_access WHERE id = $1;