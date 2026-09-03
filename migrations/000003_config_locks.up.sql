-- T028: Lock algorithm_version and configuration_snapshot tables
-- BEFORE UPDATE OR DELETE triggers to enforce artifact locking

-- Function to raise exception on locked table modifications
CREATE OR REPLACE FUNCTION raise_artifact_locked()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'artifact locked: % is immutable', TG_TABLE_NAME;
    RETURN NULL;
END;
$$;

-- Lock algorithm_version
DROP TRIGGER IF EXISTS lock_algorithm_version ON algorithm_version;
CREATE TRIGGER lock_algorithm_version
BEFORE UPDATE OR DELETE ON algorithm_version
FOR EACH ROW EXECUTE FUNCTION raise_artifact_locked();

-- Lock configuration_snapshot
DROP TRIGGER IF EXISTS lock_configuration_snapshot ON configuration_snapshot;
CREATE TRIGGER lock_configuration_snapshot
BEFORE UPDATE OR DELETE ON configuration_snapshot
FOR EACH ROW EXECUTE FUNCTION raise_artifact_locked();