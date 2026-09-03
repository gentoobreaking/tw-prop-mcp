-- T028: Drop triggers and function
DROP TRIGGER IF EXISTS lock_algorithm_version ON algorithm_version;
DROP TRIGGER IF EXISTS lock_configuration_snapshot ON configuration_snapshot;
DROP FUNCTION IF EXISTS raise_artifact_locked();