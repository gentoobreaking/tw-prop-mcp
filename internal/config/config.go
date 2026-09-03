package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tw-prop-mcp/internal/domain"
)

// ConfigService provides access to algorithm versions and configuration snapshots
type ConfigService struct {
	db *pgxpool.Pool
}

// NewConfigService creates a new ConfigService
func NewConfigService(db *pgxpool.Pool) *ConfigService {
	return &ConfigService{db: db}
}

// AlgorithmVersion represents an immutable algorithm version
type AlgorithmVersion struct {
	Version     string          `json:"version"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Weights     json.RawMessage `json:"weights"`
	CreatedAt   time.Time       `json:"created_at"`
	Provenance  domain.Provenance `json:"provenance"`
}

// ConfigurationSnapshot represents an immutable configuration snapshot
type ConfigurationSnapshot struct {
	Version     string          `json:"version"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   time.Time       `json:"created_at"`
	Provenance  domain.Provenance `json:"provenance"`
}

// Config represents the valuation configuration parameters
type Config struct {
	AreaSimilarityPct            int     `json:"area_similarity_pct"`
	Lambda                       float64 `json:"lambda"`
	DistanceScale                float64 `json:"distance_scale"`
	WArea                        float64 `json:"W_area"`
	WDistance                    float64 `json:"W_distance"`
	WTime                        float64 `json:"W_time"`
	WZoning                      float64 `json:"W_zoning"`
	WLandUse                     float64 `json:"W_land_use"`
	WRoad                        float64 `json:"W_road"`
	IQRK                         float64 `json:"IQR_k"`
	MinimumRequiredComparables   int     `json:"minimum_required_comparables"`
	OutlierMethod                string  `json:"outlier_method"`
}

// GetActiveConfig returns the currently active configuration snapshot
func (s *ConfigService) GetActiveConfig(ctx context.Context) (*ConfigurationSnapshot, error) {
	row := s.db.QueryRow(ctx, `
		SELECT version, config, created_at
		FROM configuration_snapshot
		ORDER BY created_at DESC
		LIMIT 1
	`)

	var cs ConfigurationSnapshot
	var configJSON []byte
	if err := row.Scan(&cs.Version, &configJSON, &cs.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active configuration found")
		}
		return nil, fmt.Errorf("get active config: %w", err)
	}
	cs.Config = configJSON
	cs.Provenance = domain.Provenance{
		Source:        "CONFIG_DB",
		SourceVersion: cs.Version,
		RetrievedAt:   time.Now().UTC(),
	}
	return &cs, nil
}

// GetConfig returns a specific configuration snapshot by version
func (s *ConfigService) GetConfig(ctx context.Context, version string) (*ConfigurationSnapshot, error) {
	row := s.db.QueryRow(ctx, `
		SELECT version, config, created_at
		FROM configuration_snapshot
		WHERE version = $1
	`, version)

	var cs ConfigurationSnapshot
	var configJSON []byte
	if err := row.Scan(&cs.Version, &configJSON, &cs.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("configuration version %s not found", version)
		}
		return nil, fmt.Errorf("get config %s: %w", version, err)
	}
	cs.Config = configJSON
	cs.Provenance = domain.Provenance{
		Source:        "CONFIG_DB",
		SourceVersion: cs.Version,
		RetrievedAt:   time.Now().UTC(),
	}
	return &cs, nil
}

// GetAlgorithmVersion returns an algorithm version by name
func (s *ConfigService) GetAlgorithmVersion(ctx context.Context, name string) (*AlgorithmVersion, error) {
	row := s.db.QueryRow(ctx, `
		SELECT version, name, description, weights, created_at
		FROM algorithm_version
		WHERE version = $1
	`, name)

	var av AlgorithmVersion
	var weightsJSON []byte
	if err := row.Scan(&av.Version, &av.Name, &av.Description, &weightsJSON, &av.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("algorithm version %s not found", name)
		}
		return nil, fmt.Errorf("get algorithm version %s: %w", name, err)
	}
	av.Weights = weightsJSON
	av.Provenance = domain.Provenance{
		Source:        "ALGO_DB",
		SourceVersion: av.Version,
		RetrievedAt:   time.Now().UTC(),
	}
	return &av, nil
}

// CreateConfig creates a new configuration snapshot with the given weights
func (s *ConfigService) CreateConfig(ctx context.Context, config Config) (*ConfigurationSnapshot, error) {
	// Get the next version number
	var nextVersion string
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTRING(version FROM 2) AS INT)), 1) + 1
		FROM configuration_snapshot
		WHERE version LIKE 'v%'
	`).Scan(&nextVersion)
	if err != nil {
		return nil, fmt.Errorf("get next version: %w", err)
	}
	nextVersion = fmt.Sprintf("v%s", nextVersion)

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO configuration_snapshot (version, config)
		VALUES ($1, $2)
	`, nextVersion, configJSON)
	if err != nil {
		return nil, fmt.Errorf("insert configuration: %w", err)
	}

	return s.GetConfig(ctx, nextVersion)
}

// CreateAlgorithmVersion creates a new algorithm version
func (s *ConfigService) CreateAlgorithmVersion(ctx context.Context, version, name, description string, weights map[string]interface{}) (*AlgorithmVersion, error) {
	weightsJSON, err := json.Marshal(weights)
	if err != nil {
		return nil, fmt.Errorf("marshal weights: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO algorithm_version (version, name, description, weights)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (version) DO NOTHING
	`, version, name, description, weightsJSON)
	if err != nil {
		return nil, fmt.Errorf("insert algorithm version: %w", err)
	}

	return s.GetAlgorithmVersion(ctx, version)
}

// GetConfigAsStruct returns the active config parsed into a struct
func (s *ConfigService) GetConfigAsStruct(ctx context.Context) (*Config, error) {
	cs, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(cs.Config, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

// GetAlgorithmWeights returns the weights for a specific algorithm version
func (s *ConfigService) GetAlgorithmWeights(ctx context.Context, version string) (map[string]interface{}, error) {
	av, err := s.GetAlgorithmVersion(ctx, version)
	if err != nil {
		return nil, err
	}

	var weights map[string]interface{}
	if err := json.Unmarshal(av.Weights, &weights); err != nil {
		return nil, fmt.Errorf("unmarshal weights: %w", err)
	}
	return weights, nil
}

// ParseConfig parses a ConfigurationSnapshot into a Config struct
func ParseConfig(cs *ConfigurationSnapshot) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(cs.Config, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

// ValidateConfig validates the configuration values
func ValidateConfig(cfg *Config) error {
	if cfg.AreaSimilarityPct <= 0 || cfg.AreaSimilarityPct > 100 {
		return fmt.Errorf("area_similarity_pct must be between 1 and 100")
	}
	if cfg.Lambda < 0 {
		return fmt.Errorf("lambda must be non-negative")
	}
	if cfg.DistanceScale <= 0 {
		return fmt.Errorf("distance_scale must be positive")
	}
	
	weights := []float64{cfg.WArea, cfg.WDistance, cfg.WTime, cfg.WZoning, cfg.WLandUse, cfg.WRoad}
	sum := 0.0
	for _, w := range weights {
		if w < 0 {
			return fmt.Errorf("weights must be non-negative")
		}
		sum += w
	}
	if sum == 0 {
		return fmt.Errorf("at least one weight must be positive")
	}
	
	if cfg.IQRK <= 0 {
		return fmt.Errorf("IQR_k must be positive")
	}
	if cfg.MinimumRequiredComparables <= 0 {
		return fmt.Errorf("minimum_required_comparables must be positive")
	}
	if cfg.OutlierMethod != "IQR" && cfg.OutlierMethod != "P10_P90" && cfg.OutlierMethod != "MAD" {
		return fmt.Errorf("outlier_method must be IQR, P10_P90, or MAD")
	}
	return nil
}