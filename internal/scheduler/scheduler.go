package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tw-prop-mcp/internal/downloader"
	"tw-prop-mcp/internal/importpipeline"
	"tw-prop-mcp/internal/repository"
)

// Config holds scheduler configuration from env.
type Config struct {
	Enabled         bool
	FreshnessDays   int
	RefreshInterval time.Duration
	DataImportURL   string
	AutoDiscover    bool
}

// ConfigFromEnv reads scheduler config from environment.
// Defaults handle MOI 1/11/21 release with weekend/holiday delay:
//   - Freshness 10 days (release cycle) to avoid false stale, but
//   - Interval 6h so that delayed release (next working day) is caught within hours,
//     not missed by a single daily fetch.
func ConfigFromEnv() Config {
	enabled := true
	if v := os.Getenv("AUTO_REFRESH_ENABLED"); v == "false" || v == "0" {
		enabled = false
	}
	freshnessDays := 10
	if v := os.Getenv("DATA_FRESHNESS_DAYS"); v != "" {
		var d int
		if _, err := fmt.Sscanf(v, "%d", &d); err == nil && d >= 0 {
			freshnessDays = d
		}
	}
	interval := 6 * time.Hour
	if v := os.Getenv("DATA_REFRESH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		}
	}
	autoDiscover := true
	if v := os.Getenv("AUTO_DISCOVER"); v == "false" || v == "0" {
		autoDiscover = false
	}
	return Config{
		Enabled:         enabled,
		FreshnessDays:   freshnessDays,
		RefreshInterval: interval,
		DataImportURL:   os.Getenv("DATA_IMPORT_URL"),
		AutoDiscover:    autoDiscover,
	}
}

// isReleaseWindow reports whether today is within 3 days after a scheduled
// MOI release (1, 11, 21) — to handle weekend/holiday delay to next working day.
// During window we log at higher level and the caller can decide to be more aggressive.
func isReleaseWindow(t time.Time) bool {
	day := t.Day()
	// Window: 1-3, 11-13, 21-23 (3-day grace for delay)
	for _, d := range []int{1, 11, 21} {
		if day >= d && day <= d+2 {
			return true
		}
	}
	// Also handle month-end spill: if 21+3 crosses month, still window at month start is covered by 1-3
	return false
}

// Scheduler periodically checks data freshness and triggers import.
type Scheduler struct {
	pool   *pgxpool.Pool
	config Config
	logger *slog.Logger
	mu     sync.Mutex

	progressMu sync.RWMutex
	progress   ImportProgress
}
// ImportProgress tracks current import for frontend polling.
type ImportProgress struct {
	Running   bool       `json:"running"`
	Stage     string     `json:"stage"`
	Percent   int        `json:"percent"`
	Message   string     `json:"message"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	Error     string     `json:"error,omitempty"`
}

// GetProgress returns a copy of current import progress for polling.
func (s *Scheduler) GetProgress() any {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	return s.progress
}
func (s *Scheduler) setProgress(running bool, stage string, percent int, msg string) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	now := time.Now()
	if running && s.progress.StartedAt == nil {
		s.progress.StartedAt = &now
	}
	if !running {
		s.progress.StartedAt = nil
	}
	s.progress.Running = running
	s.progress.Stage = stage
	s.progress.Percent = percent
	s.progress.Message = msg
	s.progress.Error = ""
	s.progress.UpdatedAt = now
}
func (s *Scheduler) setProgressError(errMsg string) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.progress.Running = false
	s.progress.Error = errMsg
	s.progress.UpdatedAt = time.Now()
}

// New creates a Scheduler.
func New(pool *pgxpool.Pool, config Config, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		pool:   pool,
		config: config,
		logger: logger.With("component", "scheduler"),
	}
}

// IsStale reports whether the latest LOCKED snapshot is older than FreshnessDays or missing.
// Also handles MOI 1/11/21 release window: if today is within 3 days after a release
// and the snapshot predates that release, it is stale even if age < FreshnessDays.
func (s *Scheduler) IsStale(ctx context.Context) (bool, string) {
	if s.pool == nil {
		return true, "no DB pool"
	}
	var importCompletedAt *time.Time
	err := s.pool.QueryRow(ctx, `SELECT import_completed_at FROM dataset_snapshot WHERE status='LOCKED' ORDER BY import_completed_at DESC NULLS LAST, created_at DESC LIMIT 1`).Scan(&importCompletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, "no LOCKED snapshot"
		}
		var count int
		_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM dataset_snapshot WHERE status='LOCKED'`).Scan(&count)
		if count == 0 {
			return true, "no LOCKED snapshot (count 0)"
		}
		s.logger.Warn("freshness check query failed", "error", err)
		return true, fmt.Sprintf("query error: %v", err)
	}
	if importCompletedAt == nil {
		return true, "latest LOCKED has nil import_completed_at"
	}
	now := time.Now()
	age := now.Sub(*importCompletedAt)
	threshold := time.Duration(s.config.FreshnessDays) * 24 * time.Hour
	if age > threshold {
		return true, fmt.Sprintf("age %s > threshold %s (import_completed_at=%s)", age.Round(time.Second), threshold, importCompletedAt.Format(time.RFC3339))
	}
	// Release-window check: if we are in 1-3, 11-13, 21-23 and snapshot is before the window start, stale
	if isReleaseWindow(now) {
		// Find the start of current window
		day := now.Day()
		var windowStartDay int
		for _, d := range []int{1, 11, 21} {
			if day >= d && day <= d+2 {
				windowStartDay = d
				break
			}
		}
		if windowStartDay != 0 {
			windowStart := time.Date(now.Year(), now.Month(), windowStartDay, 0, 0, 0, 0, now.Location())
			if importCompletedAt.Before(windowStart) {
				return true, fmt.Sprintf("in release window %d-%d and snapshot %s before window start %s", windowStartDay, windowStartDay+2, importCompletedAt.Format(time.RFC3339), windowStart.Format(time.RFC3339))
			}
		}
	}
	return false, fmt.Sprintf("fresh: age %s <= %s", age.Round(time.Second), threshold)
}

// CheckAndRefresh checks freshness and triggers import if stale. Returns true if refreshed.
func (s *Scheduler) CheckAndRefresh(ctx context.Context) (bool, error) {
	if !s.config.Enabled {
		s.logger.Info("auto refresh disabled, skipping")
		return false, nil
	}
	if !s.mu.TryLock() {
		s.logger.Info("refresh already in progress, skipping")
		return false, nil
	}
	defer s.mu.Unlock()

	s.setProgress(true, "checking", 5, "檢查資料新鮮度")
	stale, reason := s.IsStale(ctx)
	s.logger.Info("freshness check", "stale", stale, "reason", reason, "freshness_days", s.config.FreshnessDays)
	if !stale {
		s.setProgress(false, "idle", 100, "資料已是最新")
		return false, nil
	}

	s.setProgress(true, "discovering", 10, "尋找最新資料")
	url := s.config.DataImportURL
	if url == "" && s.config.AutoDiscover {
		s.logger.Info("auto-discovering latest MOI URL")
		discovered, err := downloader.AutoDiscoverLatestURL(ctx)
		if err != nil {
			s.setProgressError(fmt.Sprintf("auto-discover failed: %v", err))
			return false, fmt.Errorf("auto-discover failed: %w", err)
		}
		url = discovered
		s.logger.Info("auto-discovered URL", "url", url)
		s.setProgress(true, "downloading", 20, "已找到最新資料，準備下載")
	}
	if url == "" {
		s.setProgressError("no DATA_IMPORT_URL and auto-discover disabled")
		return false, fmt.Errorf("no DATA_IMPORT_URL and auto-discover disabled or failed")
	}

	s.setProgress(true, "downloading", 30, "下載中")
	s.logger.Info("starting auto import", "url", url)
	snapshotID := uuid.NewString()
	pipeline := importpipeline.NewImportPipeline(importpipeline.PipelineConfig{
		SnapshotID:  snapshotID,
		DownloadURL: url,
	}, s.logger)

	snapshotRepo := repository.NewSnapshotRepository(s.pool)
	txRepo := repository.NewTransactionRepository(s.pool)
	parcelRepo := repository.NewParcelRepository(s.pool)
	pipeline.SetRepositories(txRepo, parcelRepo, snapshotRepo)
	pipeline.SetDB(s.pool)

	s.setProgress(true, "importing", 50, "匯入中 (解析 → 驗證 → 入庫)")
	result, err := pipeline.ImportFromSource(ctx)
	if err != nil {
		s.logger.Error("auto import failed", "error", err, "snapshot_id", snapshotID)
		s.setProgressError(fmt.Sprintf("import failed: %v", err))
		return false, err
	}
	s.logger.Info("auto import succeeded", "snapshot_id", snapshotID, "transactions", result.TransactionsImported, "parcels", result.ParcelsImported, "duration", result.Duration)
	s.setProgress(false, "done", 100, fmt.Sprintf("完成：%d 筆交易、%d 筆地號", result.TransactionsImported, result.ParcelsImported))
	return true, nil
}

// ForceRefresh bypasses IsStale and always downloads the latest zip
func (s *Scheduler) ForceRefresh(ctx context.Context) (bool, error) {
	if !s.config.Enabled {
		s.logger.Info("force refresh disabled, skipping")
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setProgress(true, "discovering", 10, "強制更新：尋找最新資料")
	url := s.config.DataImportURL
	if url == "" && s.config.AutoDiscover {
		discovered, err := downloader.AutoDiscoverLatestURL(ctx)
		if err != nil {
			s.setProgressError(fmt.Sprintf("auto-discover failed: %v", err))
			return false, err
		}
		url = discovered
	}
	if url == "" {
		s.setProgressError("no DATA_IMPORT_URL")
		return false, fmt.Errorf("no DATA_IMPORT_URL")
	}
	s.setProgress(true, "downloading", 30, "強制下載中")
	s.logger.Info("force refresh starting", "url", url)
	snapshotID := uuid.NewString()
	pipeline := importpipeline.NewImportPipeline(importpipeline.PipelineConfig{
		SnapshotID:  snapshotID,
		DownloadURL: url,
	}, s.logger)
	snapshotRepo := repository.NewSnapshotRepository(s.pool)
	txRepo := repository.NewTransactionRepository(s.pool)
	parcelRepo := repository.NewParcelRepository(s.pool)
	pipeline.SetRepositories(txRepo, parcelRepo, snapshotRepo)
	pipeline.SetDB(s.pool)
	s.setProgress(true, "importing", 50, "強制匯入中")
	result, err := pipeline.ImportFromSource(ctx)
	if err != nil {
		s.setProgressError(fmt.Sprintf("force import failed: %v", err))
		return false, err
	}
	s.setProgress(false, "done", 100, fmt.Sprintf("強制完成：%d 筆交易、%d 筆地號", result.TransactionsImported, result.ParcelsImported))
	return true, nil
}

// ImportLocalZip imports a local lvr_landcsv.zip file (manual upload).
func (s *Scheduler) ImportLocalZip(ctx context.Context, zipPath string) (bool, error) {
	if !s.mu.TryLock() {
		return false, fmt.Errorf("import already in progress")
	}
	defer s.mu.Unlock()
	if _, err := os.Stat(zipPath); err != nil {
		return false, fmt.Errorf("zip not found: %w", err)
	}
	s.setProgress(true, "importing", 10, "手動匯入 zip 中")
	snapshotID := uuid.NewString()
	pipeline := importpipeline.NewImportPipeline(importpipeline.PipelineConfig{
		SnapshotID:  snapshotID,
		DownloadURL: zipPath, // local file, download stage will use it directly
	}, s.logger)
	snapshotRepo := repository.NewSnapshotRepository(s.pool)
	txRepo := repository.NewTransactionRepository(s.pool)
	parcelRepo := repository.NewParcelRepository(s.pool)
	pipeline.SetRepositories(txRepo, parcelRepo, snapshotRepo)
	pipeline.SetDB(s.pool)
	s.setProgress(true, "importing", 50, "解析 zip 中")
	result, err := pipeline.ImportFromSource(ctx)
	if err != nil {
		s.setProgressError(fmt.Sprintf("zip import failed: %v", err))
		return false, err
	}
	s.setProgress(false, "done", 100, fmt.Sprintf("zip 匯入完成：%d 筆交易、%d 筆地號", result.TransactionsImported, result.ParcelsImported))
	return true, nil
}

// Start launches the scheduler: immediate check then ticker.
func (s *Scheduler) Start(ctx context.Context) {
	if !s.config.Enabled {
		s.logger.Info("scheduler disabled")
		return
	}
	// Immediate check in background (don't block caller)
	go func() {
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		if _, err := s.CheckAndRefresh(checkCtx); err != nil {
			s.logger.Error("initial freshness check failed", "error", err)
		}
	}()

	// Periodic ticker
	go func() {
		ticker := time.NewTicker(s.config.RefreshInterval)
		defer ticker.Stop()
		s.logger.Info("scheduler started", "interval", s.config.RefreshInterval, "freshness_days", s.config.FreshnessDays)
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("scheduler stopped")
				return
			case <-ticker.C:
				checkCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
				if _, err := s.CheckAndRefresh(checkCtx); err != nil {
					s.logger.Error("scheduled refresh failed", "error", err)
				}
				cancel()
			}
		}
	}()
}
