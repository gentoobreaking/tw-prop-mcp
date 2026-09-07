package parser

import (
	"bytes"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// DetectEncoding inspects the raw bytes and returns one of:
//   - "utf-8-bom"  (has UTF-8 BOM prefix)
//   - "utf-8"      (valid UTF-8 without BOM)
//   - "big5"       (fallback, likely Big5/unknown single-byte encoding)
func DetectEncoding(data []byte) string {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		// Has UTF-8 BOM
		rest := data[3:]
		// Check if data after BOM is valid UTF-8
		if utf8.Valid(rest) {
			// Check if it looks like Big5 (many high bytes)
			highBytes := 0
			for _, b := range data[3:] {
				if b >= 0x80 {
					highBytes++
				}
			}
			// If more than 5% high bytes, likely Big5 with BOM
			if float64(highBytes)/float64(len(data)-3) > 0.05 {
				return "big5"
			}
			return "utf-8-bom"
		}
		// Has BOM but invalid UTF-8 -> likely Big5 with BOM added
		return "big5"
	}
	if utf8.Valid(data) {
		return "utf-8"
	}
	return "big5"
}

// DecodeReader returns a reader that yields UTF-8 decoded content.
// It auto-detects encoding: if Big5, it decodes via traditionalchinese.Big5.NewDecoder().
// The second return value is the detected encoding string.
func DecodeReader(r io.Reader) (io.Reader, string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}
	enc := DetectEncoding(data)
	switch enc {
	case "utf-8-bom":
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		return bytes.NewReader(data), enc, nil
	case "utf-8":
		return bytes.NewReader(data), enc, nil
	case "big5":
		decoder := traditionalchinese.Big5.NewDecoder()
		decoded, _, err := transform.Bytes(decoder, data)
		if err != nil {
			return nil, enc, err
		}
		return bytes.NewReader(decoded), enc, nil
	default:
		return bytes.NewReader(data), enc, nil
	}
}
