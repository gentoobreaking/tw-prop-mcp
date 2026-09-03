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
		return "utf-8-bom"
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
