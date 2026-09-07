package downloader

import (
	"context"
)

// MOI open data bulk download — 直接就是 lvr_landcsv.zip，無需掃頁面 (Boss 指正)
const openDataZipURL = "https://plvr.land.moi.gov.tw/Download?type=zip&fileName=lvr_landcsv.zip"

// AutoDiscoverLatestURL 直接回傳 DownloadOpenData 的 zip URL
// 不再掃 landing page 的 GetFile，單純直連，避免 JS 渲染複雜度
func AutoDiscoverLatestURL(_ context.Context) (string, error) {
	return openDataZipURL, nil
}
