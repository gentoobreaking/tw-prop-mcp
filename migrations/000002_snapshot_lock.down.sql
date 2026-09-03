DROP TRIGGER IF EXISTS trg_snapshot_lock ON dataset_snapshot;
DROP FUNCTION IF EXISTS prevent_locked_snapshot_update();
