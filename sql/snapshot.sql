-- name: CreateSnapshot :one
INSERT INTO dataset_snapshot (id, source, source_version, file_name, file_sha256, record_count, status, schema_version, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL) RETURNING *;

-- name: GetSnapshotByID :one
SELECT * FROM dataset_snapshot WHERE id = $1 LIMIT 1;

-- name: ListSnapshots :many
SELECT * FROM dataset_snapshot ORDER BY downloaded_at DESC;

-- name: LockSnapshot :exec
UPDATE dataset_snapshot SET status='LOCKED', import_completed_at=NOW() WHERE id=$1 AND status='IMPORTING';

-- name: CreateImportBatch :one
INSERT INTO import_batch (snapshot_id, status) VALUES ($1, 'RUNNING') RETURNING *;

-- name: CompleteImportBatch :exec
UPDATE import_batch SET status='COMPLETED', completed_at=NOW(), records_imported=$2 WHERE id=$1;