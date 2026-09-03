-- name: GetParcelByID :one
SELECT * FROM parcel WHERE id = $1 LIMIT 1;

-- name: GetParcelByLandNumber :one
SELECT * FROM parcel WHERE county=$1 AND district=$2 AND section=$3 AND land_number=$4 LIMIT 1;

-- name: SearchParcels :many
SELECT * FROM parcel
WHERE county=$1 AND district=$2
  AND ($3::text IS NULL OR section=$3)
  AND ($4::numeric IS NULL OR area_sqm >= $4)
  AND ($5::numeric IS NULL OR area_sqm <= $5)
ORDER BY section, land_number
LIMIT $6 OFFSET $7;

-- name: GetParcelGeometry :one
SELECT id, geometry, centroid, bbox, area_sqm FROM parcel WHERE id=$1 LIMIT 1;

-- name: FindNearbyRoads :many
SELECT r.*, ST_Distance(p.geometry, r.geometry) AS distance_m
FROM parcel p, road_segment r
WHERE p.id=$1 AND ST_DWithin(p.geometry, r.geometry, $2)
ORDER BY distance_m ASC
LIMIT $3;

-- name: CheckRoadAccess :one
SELECT * FROM parcel_road_access WHERE parcel_id=$1 AND algorithm_version=$2 LIMIT 1;
