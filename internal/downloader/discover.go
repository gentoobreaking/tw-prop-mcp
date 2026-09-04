package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// MOI landing page URL
const landingPageURL = "https://plvr.land.moi.gov.tw/"

// downloadURLRegex matches the CSV download link pattern on the landing page
var downloadURLRegex = regexp.MustCompile(`GetFile\?type=csv&id=([0-9A-Z]+)`)

// AutoDiscoverLatestURL fetches the MOI landing page and extracts the latest
// CSV download URL. This URL contains a dynamically rotated file ID that
// changes whenever MOI publishes new data.
//
// Usage:
//
//	url, err := AutoDiscoverLatestURL(ctx)
//	if err != nil { ... }
//	// url = "https://plvr.land.moi.gov.tw/Download/GetFile?type=csv&id=XXXX..."
func AutoDiscoverLatestURL(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, landingPageURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "tw-prop-mcp-import/2.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch landing page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("landing page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB limit
	if err != nil {
		return "", fmt.Errorf("read landing page: %w", err)
	}

	matches := downloadURLRegex.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return "", fmt.Errorf("no CSV download link found on landing page")
	}

	fileID := matches[1]
	return fmt.Sprintf("https://plvr.land.moi.gov.tw/Download/GetFile?type=csv&id=%s", fileID), nil
}
