package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcpapi "github.com/modelcontextprotocol/go-sdk/mcp"
)
// --- Freshness Tools ---

func registerFreshnessTools(srv *mcpapi.Server, s *Server) {
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_data_freshness",
			Description: "Get data freshness: latest LOCKED snapshot time, age, stale status, and next MOI release window (1/11/21) for UI display.",
		},
		instrument(s, "get_data_freshness", "", getDataFreshnessHandler(s)),
	)
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "trigger_data_refresh",
			Description: "Manually trigger data refresh if stale. Runs in background and returns immediately; poll get_data_freshness for completion.",
		},
		instrument(s, "trigger_data_refresh", "", triggerDataRefreshHandler(s)),
	)
	mcpapi.AddTool(srv,
		&mcpapi.Tool{
			Name:        "get_import_progress",
			Description: "Get current import progress for polling after trigger_data_refresh. Returns stage, percent, and message for progress bar.",
		},
		instrument(s, "get_import_progress", "", getImportProgressHandler(s)),
	)
}

type getDataFreshnessOutput struct {
	LatestSnapshotID    *string `json:"latest_snapshot_id,omitempty"`
	LatestSnapshotAt    *string `json:"latest_snapshot_at,omitempty"`
	LatestImportAt      *string `json:"latest_import_completed_at,omitempty"`
	Source              *string `json:"source,omitempty"`
	SourceVersion       *string `json:"source_version,omitempty"`
	Status              *string `json:"status,omitempty"`
	RecordCount         *int64  `json:"record_count,omitempty"`
	ParcelCount         int     `json:"parcel_count"`
	TransactionCount    int     `json:"transaction_count"`
	IsStale             bool    `json:"is_stale"`
	StaleReason         string  `json:"stale_reason"`
	FreshnessDays       int     `json:"freshness_days"`
	NextReleaseWindow   string  `json:"next_release_window"`
	AgeHours            *float64 `json:"age_hours,omitempty"`
}

func getDataFreshnessHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input struct{}) (*mcpapi.CallToolResult, getDataFreshnessOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input struct{}) (*mcpapi.CallToolResult, getDataFreshnessOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), getDataFreshnessOutput{}, nil
		}
		if s.Pool == nil {
			return mcpErrorResult(NewError(ErrorCodeDataNotAvailable, "no database configured")), getDataFreshnessOutput{}, nil
		}
		out := getDataFreshnessOutput{}

		// Freshness via scheduler if available
		if s.Scheduler != nil {
			stale, reason := s.Scheduler.IsStale(ctx)
			out.IsStale = stale
			out.StaleReason = reason
		} else {
			// Fallback: query directly
			var importCompletedAt *time.Time
			err := s.Pool.QueryRow(ctx, `SELECT import_completed_at FROM dataset_snapshot WHERE status='LOCKED' ORDER BY import_completed_at DESC LIMIT 1`).Scan(&importCompletedAt)
			if err != nil {
				out.IsStale = true
				out.StaleReason = "no LOCKED snapshot"
			} else if importCompletedAt != nil {
				age := time.Since(*importCompletedAt)
				out.IsStale = age > 10*24*time.Hour
				out.StaleReason = fmt.Sprintf("age %s", age.Round(time.Second))
			}
		}

		// Latest snapshot details
		var id, source, sourceVersion, status, fileName, fileSHA string
		var recordCount int64
		var downloadedAt, importCompletedAt *time.Time
		err := s.Pool.QueryRow(ctx, `SELECT id::text, source, source_version, status, file_name, file_sha256, record_count, downloaded_at, import_completed_at FROM dataset_snapshot WHERE status='LOCKED' ORDER BY import_completed_at DESC LIMIT 1`).Scan(&id, &source, &sourceVersion, &status, &fileName, &fileSHA, &recordCount, &downloadedAt, &importCompletedAt)
		if err == nil {
			out.LatestSnapshotID = &id
			out.Source = &source
			out.SourceVersion = &sourceVersion
			out.Status = &status
			out.RecordCount = &recordCount
			if downloadedAt != nil {
				ts := downloadedAt.Format(time.RFC3339)
				out.LatestSnapshotAt = &ts
			}
			if importCompletedAt != nil {
				ts := importCompletedAt.Format(time.RFC3339)
				out.LatestImportAt = &ts
				ageHours := time.Since(*importCompletedAt).Hours()
				out.AgeHours = &ageHours
			}
		}
		// Counts
		_ = s.Pool.QueryRow(ctx, `SELECT count(*) FROM parcel`).Scan(&out.ParcelCount)
		_ = s.Pool.QueryRow(ctx, `SELECT count(*) FROM transaction`).Scan(&out.TransactionCount)

		// Freshness days from scheduler config or default 10
		out.FreshnessDays = 10
		// Next release window: 1, 11, 21
		now := time.Now()
		day := now.Day()
		var nextWindow string
		for _, d := range []int{1, 11, 21} {
			if day < d {
				nextWindow = fmt.Sprintf("%d-%d", d, d+2)
				break
			}
			if day >= d && day <= d+2 {
				nextWindow = fmt.Sprintf("%d-%d (current window)", d, d+2)
				break
			}
		}
		if nextWindow == "" {
			nextWindow = "1-3 (next month)"
		}
		out.NextReleaseWindow = nextWindow

		return nil, out, nil
	}
}

type triggerDataRefreshInput struct {
	Force *bool `json:"force,omitempty" jsonschema:"Force refresh even if not stale"`
}

type triggerDataRefreshOutput struct {
	Started   bool   `json:"started"`
	Message   string `json:"message"`
	IsStale   bool   `json:"is_stale"`
	Reason    string `json:"reason"`
}

func triggerDataRefreshHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input triggerDataRefreshInput) (*mcpapi.CallToolResult, triggerDataRefreshOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input triggerDataRefreshInput) (*mcpapi.CallToolResult, triggerDataRefreshOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), triggerDataRefreshOutput{}, nil
		}
		if s.Pool == nil {
			return mcpErrorResult(NewError(ErrorCodeDataNotAvailable, "no database configured")), triggerDataRefreshOutput{}, nil
		}
		if s.Scheduler == nil {
			return mcpErrorResult(NewError(ErrorCodeDataNotAvailable, "scheduler not configured")), triggerDataRefreshOutput{}, nil
		}
		force := false
		if input.Force != nil {
			force = *input.Force
		}
		stale, reason := s.Scheduler.IsStale(ctx)
		if !stale && !force {
			return nil, triggerDataRefreshOutput{
				Started: false,
				Message: "data is fresh, no refresh needed (use force=true to override)",
				IsStale: stale,
				Reason:  reason,
			}, nil
		}
		// Run in background with timeout
		go func(force bool) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if force {
				_, _ = s.Scheduler.ForceRefresh(bgCtx)
			} else {
				_, _ = s.Scheduler.CheckAndRefresh(bgCtx)
			}
		}(force)
		return nil, triggerDataRefreshOutput{
			Started: true,
			Message: "refresh started in background; poll get_import_progress for completion",
			IsStale: stale,
			Reason:  reason,
		}, nil
	}
}
type getImportProgressOutput struct {
	Running   bool    `json:"running"`
	Stage     string  `json:"stage"`
	Percent   int     `json:"percent"`
	Message   string  `json:"message"`
	StartedAt *string `json:"started_at,omitempty"`
	UpdatedAt string  `json:"updated_at"`
	Error     string  `json:"error,omitempty"`
}

func getImportProgressHandler(s *Server) func(ctx context.Context, req *mcpapi.CallToolRequest, input struct{}) (*mcpapi.CallToolResult, getImportProgressOutput, error) {
	return func(ctx context.Context, req *mcpapi.CallToolRequest, input struct{}) (*mcpapi.CallToolResult, getImportProgressOutput, error) {
		if mce := checkAIIsolation(req); mce != nil {
			return mcpErrorResult(mce), getImportProgressOutput{}, nil
		}
		if s.Scheduler == nil {
			return nil, getImportProgressOutput{
				Running: false,
				Stage:   "idle",
				Percent: 0,
				Message: "scheduler not configured",
			}, nil
		}
		raw := s.Scheduler.GetProgress()
		// Marshal raw (scheduler.ImportProgress) to JSON then unmarshal to output
		// This avoids import cycle by using JSON as bridge
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, getImportProgressOutput{
				Running: false,
				Stage:   "unknown",
				Percent: 0,
				Message: fmt.Sprintf("%v", raw),
				Error:   err.Error(),
			}, nil
		}
		var out getImportProgressOutput
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, getImportProgressOutput{
				Running: false,
				Stage:   "unknown",
				Percent: 0,
				Message: fmt.Sprintf("%v", raw),
				Error:   err.Error(),
			}, nil
		}
		return nil, out, nil
	}
}
