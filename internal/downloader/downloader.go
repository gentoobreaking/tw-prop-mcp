package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultUserAgent  = "tw-prop-mcp/1.0 (+https://github.com/tw-prop-mcp)"
	DefaultMaxRetries = 3
	DefaultTimeout    = 30 * time.Second
)

// Downloader defines the contract for downloading a remote file to local disk.
type Downloader interface {
	Download(ctx context.Context, url, dest string) (string, error)
}

// HTTPDownloader implements Downloader via http GET with retry, timeout, User-Agent and progress logging.
type HTTPDownloader struct {
	Client         *http.Client
	MaxRetries     int
	UserAgent      string
	InitialBackoff time.Duration
	Logger         *slog.Logger
}

// NewHTTPDownloader creates an HTTPDownloader with sane defaults.
func NewHTTPDownloader() *HTTPDownloader {
	return &HTTPDownloader{
		Client: &http.Client{
			Timeout: DefaultTimeout,
		},
		MaxRetries:     DefaultMaxRetries,
		UserAgent:      DefaultUserAgent,
		InitialBackoff: 500 * time.Millisecond,
		Logger:         slog.Default(),
	}
}

// ErrDownloadFailed is returned when download fails after retries.
var ErrDownloadFailed = errors.New("download failed")

// Download fetches url and writes to dest (file). If dest is a directory, uses basename from URL.
// Returns the final file path on success.
func (d *HTTPDownloader) Download(ctx context.Context, rawURL, dest string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("%w: empty url", ErrDownloadFailed)
	}
	if dest == "" {
		return "", fmt.Errorf("%w: empty dest", ErrDownloadFailed)
	}

	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}
	maxRetries := d.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}
	backoff := d.InitialBackoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}
	userAgent := d.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Resolve dest file path: if dest is dir, join; if not existing and no ext, treat as dir.
	// We attempt to detect directory: if path exists and is dir, join with temp name.
	// Caller (SnapshotService) passes a temp file path, so dest is file path.
	finalPath := dest
	if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
		finalPath = filepath.Join(dest, "download")
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("%w: context canceled: %w", ErrDownloadFailed, ctx.Err())
			case <-time.After(backoff * time.Duration(1<<uint(attempt-1))):
			}
		}

		logger.Info("downloading", "url", rawURL, "dest", finalPath, "attempt", attempt+1)

		// Ensure context timeout is respected.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return "", fmt.Errorf("%w: create request: %w", ErrDownloadFailed, err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			// Check context errors to classify.
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return "", fmt.Errorf("%w: %w", ErrDownloadFailed, err)
			}
			// Network error -> retry
			lastErr = fmt.Errorf("%w: attempt %d: %w", ErrDownloadFailed, attempt+1, err)
			logger.Warn("download attempt failed", "attempt", attempt+1, "error", err)
			continue
		}

		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("%w: attempt %d: http status %d", ErrDownloadFailed, attempt+1, resp.StatusCode)
				logger.Warn("download non-2xx", "status", resp.StatusCode, "attempt", attempt+1)
				return
			}
			// Ensure parent dir exists.
			if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
				lastErr = fmt.Errorf("%w: mkdir: %w", ErrDownloadFailed, err)
				return
			}
			f, err := os.Create(finalPath)
			if err != nil {
				lastErr = fmt.Errorf("%w: create file: %w", ErrDownloadFailed, err)
				return
			}
			defer f.Close()
			n, err := io.Copy(f, resp.Body)
			if err != nil {
				// Respect context
				if ctx.Err() != nil {
					lastErr = fmt.Errorf("%w: %w", ErrDownloadFailed, ctx.Err())
				} else {
					lastErr = fmt.Errorf("%w: copy body: %w", ErrDownloadFailed, err)
				}
				_ = os.Remove(finalPath)
				return
			}
			logger.Info("download complete", "path", finalPath, "bytes", n)
			lastErr = nil
		}()

		if lastErr == nil {
			return finalPath, nil
		}
		// If lastErr was due to non-2xx, retry unless client error 4xx (except 429)?
		// For 4xx we still retry limited times except 404.
		// Keep retry loop.
		_ = os.Remove(finalPath)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("%w: unknown error", ErrDownloadFailed)
	}
	return "", lastErr
}

// MockDownloader is a test double for Downloader.
type MockDownloader struct {
	DownloadFunc func(ctx context.Context, url, dest string) (string, error)
	Calls        []MockCall
	Err          error
	Content      []byte
}

type MockCall struct {
	URL  string
	Dest string
}

// Download implements Downloader for MockDownloader.
func (m *MockDownloader) Download(ctx context.Context, url, dest string) (string, error) {
	m.Calls = append(m.Calls, MockCall{URL: url, Dest: dest})
	if m.DownloadFunc != nil {
		return m.DownloadFunc(ctx, url, dest)
	}
	if m.Err != nil {
		return "", m.Err
	}
	// Default: write Content to dest.
	content := m.Content
	if content == nil {
		content = []byte("mock content")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, content, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

var _ Downloader = (*HTTPDownloader)(nil)
var _ Downloader = (*MockDownloader)(nil)
