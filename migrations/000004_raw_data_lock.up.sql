-- T020: Raw data locking — P5 Artifact Locking
-- Prevent UPDATE or DELETE on raw data tables when the referenced
-- dataset_snapshot is in 'LOCKED' status.
--
-- Tables with direct snapshot_id: transaction, valuation_result
-- Tables with import_batch_id: parcel, parcel_geometry, road_segment

-- Function for tables that reference snapshot_id directly
CREATE OR REPLACE FUNCTION prevent_locked_snapshot_data_change()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    snap_status TEXT;
BEGIN
    SELECT status INTO snap_status
    FROM dataset_snapshot
    WHERE id = OLD.snapshot_id;

    IF snap_status = 'LOCKED' THEN
        RAISE EXCEPTION 'snapshot locked: % (status=LOCKED) — raw data is immutable', OLD.snapshot_id;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

-- Function for tables that reference snapshot_id via import_batch_id
CREATE OR REPLACE FUNCTION prevent_locked_batch_data_change()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    snap_status TEXT;
BEGIN
    SELECT ds.status INTO snap_status
    FROM dataset_snapshot ds
    JOIN import_batch ib ON ib.snapshot_id = ds.id
    WHERE ib.id = OLD.import_batch_id;

    IF snap_status = 'LOCKED' THEN
        RAISE EXCEPTION 'snapshot locked: raw data is immutable (import_batch %)', OLD.import_batch_id;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

-- transaction table (has snapshot_id directly)
DROP TRIGGER IF EXISTS trg_transaction_lock ON "transaction";
CREATE TRIGGER trg_transaction_lock
BEFORE UPDATE OR DELETE ON "transaction"
FOR EACH ROW EXECUTE FUNCTION prevent_locked_snapshot_data_change();

-- valuation_result table (has snapshot_id directly)
DROP TRIGGER IF EXISTS trg_valuation_lock ON valuation_result;
CREATE TRIGGER trg_valuation_lock
BEFORE UPDATE OR DELETE ON valuation_result
FOR EACH ROW EXECUTE FUNCTION prevent_locked_snapshot_data_change();

-- parcel table (has import_batch_id)
DROP TRIGGER IF EXISTS trg_parcel_lock ON parcel;
CREATE TRIGGER trg_parcel_lock
BEFORE UPDATE OR DELETE ON parcel
FOR EACH ROW EXECUTE FUNCTION prevent_locked_batch_data_change();

-- parcel_geometry table (has import_batch_id)
DROP TRIGGER IF EXISTS trg_parcel_geometry_lock ON parcel_geometry;
CREATE TRIGGER trg_parcel_geometry_lock
BEFORE UPDATE OR DELETE ON parcel_geometry
FOR EACH ROW EXECUTE FUNCTION prevent_locked_batch_data_change();

-- road_segment table (has import_batch_id)
DROP TRIGGER IF EXISTS trg_road_lock ON road_segment;
CREATE TRIGGER trg_road_lock
BEFORE UPDATE OR DELETE ON road_segment
FOR EACH ROW EXECUTE FUNCTION prevent_locked_batch_data_change();
