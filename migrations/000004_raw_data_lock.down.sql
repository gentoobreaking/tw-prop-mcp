-- T020: Down migration — remove raw data locking triggers
DROP TRIGGER IF EXISTS trg_transaction_lock ON "transaction";
DROP TRIGGER IF EXISTS trg_valuation_lock ON valuation_result;
DROP TRIGGER IF EXISTS trg_parcel_lock ON parcel;
DROP TRIGGER IF EXISTS trg_parcel_geometry_lock ON parcel_geometry;
DROP TRIGGER IF EXISTS trg_road_lock ON road_segment;
DROP FUNCTION IF EXISTS prevent_locked_snapshot_data_change();
DROP FUNCTION IF EXISTS prevent_locked_batch_data_change();
