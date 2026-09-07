package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	// 直接讀取您手動下載到本機的 zip 檔案
	zipFileName := "lvr_landcsv.zip"
	outputDir := "./lvr_data"

	fmt.Printf("🚀 偵測到本地壓縮檔，開始解壓 %s ...\n", zipFileName)

	// 1. 開啟本地的 zip 檔案
	r, err := zip.OpenReader(zipFileName)
	if err != nil {
		fmt.Printf("❌ 無法讀取檔案！請確認您是否已將下載的 '%s' 放到與此程式相同的資料夾下。\n", zipFileName)
		fmt.Printf("錯誤訊息: %v\n", err)
		return
	}
	defer r.Close()

	// 2. 建立目的地資料夾
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		fmt.Printf("❌ 建立資料夾失敗: %v\n", err)
		return
	}

	// 3. 遍歷壓縮包裡面的所有 CSV
	for _, file := range r.File {
		extractedFilePath := filepath.Join(outputDir, file.Name)
		fmt.Printf("📦 解壓中 -> %s\n", extractedFilePath)

		// 開啟壓縮包內的文件
		fileReader, err := file.Open()
		if err != nil {
			fmt.Printf("❌ 無法開啟壓縮檔內文件 %s: %v\n", file.Name, err)
			continue
		}

		// 建立本機實體檔案
		targetFile, err := os.OpenFile(extractedFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fileReader.Close()
			fmt.Printf("❌ 無法建立本機檔案 %s: %v\n", extractedFilePath, err)
			continue
		}

		// 複製資料
		_, err = io.Copy(targetFile, fileReader)
		targetFile.Close()
		fileReader.Close()
		if err != nil {
			fmt.Printf("❌ 寫入檔案失敗 %s: %v\n", extractedFilePath, err)
			continue
		}
	}

	fmt.Println("🎉 全數解壓完畢！所有縣市的 CSV 檔案已成功儲存至 ./lvr_data 資料夾。")
}

