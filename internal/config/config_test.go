package config

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestConfigService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Start postgres:16-alpine
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("prop"),
		postgres.WithUsername("prop"),
		postgres.WithPassword("prop_dev_only"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}
	defer func() {
		_ = pgContainer.Terminate(ctx)
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	// Run migrations
	migrationFiles := []string{
		"../../migrations/000001_init.up.sql",
		"../../migrations/000002_snapshot_lock.up.sql",
		"../../migrations/000003_config_locks.up.sql",
	}
	for _, f := range migrationFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration: %v", err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			t.Fatalf("run migration %s: %v", f, err)
		}
	}

	svc := NewConfigService(pool)

	t.Run("GetActiveConfig_v2_0", func(t *testing.T) {
		cs, err := svc.GetActiveConfig(ctx)
		if err != nil {
			t.Fatalf("GetActiveConfig: %v", err)
		}
		if cs.Version != "v2.0" {
			t.Errorf("expected version v2.0, got %s", cs.Version)
		}
		if cs.Provenance.Source != "CONFIG_DB" {
			t.Errorf("expected provenance source CONFIG_DB, got %s", cs.Provenance.Source)
		}
		if cs.Provenance.SourceVersion != "v2.0" {
			t.Errorf("expected provenance source_version v2.0, got %s", cs.Provenance.SourceVersion)
		}
	})

	t.Run("GetConfig_v2_0", func(t *testing.T) {
		cs, err := svc.GetConfig(ctx, "v2.0")
		if err != nil {
			t.Fatalf("GetConfig: %v", err)
		}
		if cs.Version != "v2.0" {
			t.Errorf("expected version v2.0, got %s", cs.Version)
		}
	})

	t.Run("GetConfig_NotFound", func(t *testing.T) {
		_, err := svc.GetConfig(ctx, "v99.0")
		if err == nil {
			t.Error("expected error for non-existent version")
		}
	})

	t.Run("GetAlgorithmVersion", func(t *testing.T) {
		av, err := svc.GetAlgorithmVersion(ctx, "comparable-v2.0")
		if err != nil {
			t.Fatalf("GetAlgorithmVersion: %v", err)
		}
		if av.Version != "comparable-v2.0" {
			t.Errorf("expected comparable-v2.0, got %s", av.Version)
		}
		if av.Provenance.Source != "ALGO_DB" {
			t.Errorf("expected provenance source ALGO_DB, got %s", av.Provenance.Source)
		}
	})

	t.Run("CreateConfig", func(t *testing.T) {
		newConfig := Config{
			AreaSimilarityPct:            35,
			Lambda:                       0.06,
			DistanceScale:                600,
			WArea:                        0.25,
			WDistance:                    0.25,
			WTime:                        0.15,
			WZoning:                      0.10,
			WLandUse:                     0.10,
			WRoad:                        0.15,
			IQRK:                         1.5,
			MinimumRequiredComparables:   4,
			OutlierMethod:                "IQR",
		}
		cs, err := svc.CreateConfig(ctx, newConfig)
		if err != nil {
			t.Fatalf("CreateConfig: %v", err)
		}
		if cs.Version == "v2.0" {
			t.Error("new version should not be v2.0")
		}
		if cs.Provenance.Source != "CONFIG_DB" {
			t.Errorf("expected provenance CONFIG_DB, got %s", cs.Provenance.Source)
		}

		// Verify it's now the active config
		active, err := svc.GetActiveConfig(ctx)
		if err != nil {
			t.Fatalf("GetActiveConfig after create: %v", err)
		}
		if active.Version != cs.Version {
			t.Errorf("new config should be active, got %s", active.Version)
		}
	})

	t.Run("ParseConfig", func(t *testing.T) {
		cs, err := svc.GetActiveConfig(ctx)
		if err != nil {
			t.Fatalf("GetActiveConfig: %v", err)
		}
		cfg, err := ParseConfig(cs)
		if err != nil {
			t.Fatalf("ParseConfig: %v", err)
		}
		if cfg.AreaSimilarityPct <= 0 {
			t.Errorf("parsed config should have valid AreaSimilarityPct")
		}
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		valid := Config{
			AreaSimilarityPct: 30, Lambda: 0.05, DistanceScale: 500,
			WArea: 0.30, WDistance: 0.20, WTime: 0.15,
			WZoning: 0.15, WLandUse: 0.10, WRoad: 0.10,
			IQRK: 1.5, MinimumRequiredComparables: 3, OutlierMethod: "IQR",
		}
		if err := ValidateConfig(&valid); err != nil {
			t.Errorf("valid config should pass: %v", err)
		}

		invalid := Config{AreaSimilarityPct: 0}
		if err := ValidateConfig(&invalid); err == nil {
			t.Error("invalid config should fail validation")
		}

		negativeWeight := valid
		negativeWeight.WArea = -0.1
		if err := ValidateConfig(&negativeWeight); err == nil {
			t.Error("negative weight should fail validation")
		}

		invalidMethod := valid
		invalidMethod.OutlierMethod = "UNKNOWN"
		if err := ValidateConfig(&invalidMethod); err == nil {
			t.Error("invalid outlier method should fail validation")
		}
	})

	t.Run("GetConfigAsStruct", func(t *testing.T) {
		cfg, err := svc.GetConfigAsStruct(ctx)
		if err != nil {
			t.Fatalf("GetConfigAsStruct: %v", err)
		}
		if cfg.AreaSimilarityPct != 30 {
			t.Errorf("expected AreaSimilarityPct 30, got %d", cfg.AreaSimilarityPct)
		}
	})

	t.Run("GetAlgorithmWeights", func(t *testing.T) {
		weights, err := svc.GetAlgorithmWeights(ctx, "comparable-v2.0")
		if err != nil {
			t.Fatalf("GetAlgorithmWeights: %v", err)
		}
		if weights["W_area"] != 0.30 {
			t.Errorf("expected W_area 0.30, got %v", weights["W_area"])
		}
		if weights["W_distance"] != 0.20 {
			t.Errorf("expected W_distance 0.20, got %v", weights["W_distance"])
		}
	})

	t.Run("CreateAlgorithmVersion", func(t *testing.T) {
		newVersion := "comparable-test-" + uuid.New().String()[:8]
		av, err := svc.CreateAlgorithmVersion(ctx, newVersion, "Test Algo", "Test description", map[string]interface{}{
			"W_area": 0.4, "W_distance": 0.3, "W_time": 0.2, "W_zoning": 0.1,
		})
		if err != nil {
			t.Fatalf("CreateAlgorithmVersion: %v", err)
		}
		if av.Version != newVersion {
			t.Errorf("expected version %s, got %s", newVersion, av.Version)
		}
		if av.Provenance.Source != "ALGO_DB" {
			t.Errorf("expected provenance ALGO_DB, got %s", av.Provenance.Source)
		}
	})
}

func TestConfig_LockedTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("prop"),
		postgres.WithUsername("prop"),
		postgres.WithPassword("prop_dev_only"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}
	defer func() {
		_ = pgContainer.Terminate(ctx)
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	// Run migrations including config locks
	migrationFiles := []string{
		"../../migrations/000001_init.up.sql",
		"../../migrations/000002_snapshot_lock.up.sql",
		"../../migrations/000003_config_locks.up.sql",
	}
	for _, f := range migrationFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration: %v", err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			t.Fatalf("run migration %s: %v", f, err)
		}
	}

	t.Run("UpdateAlgorithmVersion_Fails", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE algorithm_version SET name = 'hacked' WHERE version = 'comparable-v2.0'`)
		if err == nil {
			t.Error("UPDATE on algorithm_version should fail")
		}
	})

	t.Run("DeleteAlgorithmVersion_Fails", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM algorithm_version WHERE version = 'comparable-v2.0'`)
		if err == nil {
			t.Error("DELETE on algorithm_version should fail")
		}
	})

	t.Run("UpdateConfigurationSnapshot_Fails", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE configuration_snapshot SET config = '{}' WHERE version = 'v2.0'`)
		if err == nil {
			t.Error("UPDATE on configuration_snapshot should fail")
		}
	})

	t.Run("DeleteConfigurationSnapshot_Fails", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM configuration_snapshot WHERE version = 'v2.0'`)
		if err == nil {
			t.Error("DELETE on configuration_snapshot should fail")
		}
	})

	t.Run("InsertNewConfig_Succeeds", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO configuration_snapshot (version, config)
			VALUES ('v99.9', '{"test": true}'::jsonb)
			ON CONFLICT (version) DO NOTHING
		`)
		if err != nil {
			t.Errorf("INSERT new config should succeed: %v", err)
		}
	})
}