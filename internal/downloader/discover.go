package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// MOI landing page URLs to try (order matters — first success wins).
// The site is JS-heavy and the CSV link may be on different pages
// depending on deployment. We try the landing page plus known download pages.
var landingPageURLs = []string{
	"https://plvr.land.moi.gov.tw/",
	"https://plvr.land.moi.gov.tw/Index",
	"https://plvr.land.moi.gov.tw/Download",
	"https://plvr.land.moi.gov.tw/DownloadHistory",
}

// downloadURLRegex matches the CSV download link pattern on the landing page
var downloadURLRegex = regexp.MustCompile(`GetFile\?type=csv&id=([0-9A-Z]+)`)

// AutoDiscoverLatestURL fetches the MOI landing page and extracts the latest
// CSV download URL. This URL contains a dynamically rotated file ID that
// changes whenever MOI publishes new data.
//
// It tries multiple landing page URLs to handle JS rendering and site moves,
// and handles weekend/holiday delay by returning whatever latest ID is available
// — the scheduler's 6h retry will catch delayed publishes.
func AutoDiscoverLatestURL(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	var lastErr error
	for _, u := range landingPageURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			lastErr = fmt.Errorf("create request %s: %w", u, err)
			continue
		}
		req.Header.Set("User-Agent", "tw-prop-mcp-import/2.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetch %s: %w", u, err)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read %s: %w", u, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s returned status %d", u, resp.StatusCode)
			continue
		}
		matches := downloadURLRegex.FindStringSubmatch(string(body))
		if len(matches) >= 2 {
			fileID := matches[1]
			return fmt.Sprintf("https://plvr.land.moi.gov.tw/Download/GetFile?type=csv&id=%s", fileID), nil
		}
		lastErr = fmt.Errorf("no CSV download link found on %s", u)
	}
	return "", fmt.Errorf("auto-discover failed after %d URLs, last error: %w", len(landingPageURLs), lastErr)
}
