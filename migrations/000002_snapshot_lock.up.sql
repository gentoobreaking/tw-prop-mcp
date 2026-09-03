-- T003: Snapshot locking trigger — P2 Raw Data Immutable
-- Prevent any UPDATE or DELETE on dataset_snapshot once status='LOCKED'

CREATE OR REPLACE FUNCTION prevent_locked_snapshot_update() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'LOCKED' THEN
        RAISE EXCEPTION 'snapshot locked: %', OLD.id;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_snapshot_lock ON dataset_snapshot;
CREATE TRIGGER trg_snapshot_lock
    BEFORE UPDATE OR DELETE ON dataset_snapshot
    FOR EACH ROW EXECUTE FUNCTION prevent_locked_snapshot_update();
