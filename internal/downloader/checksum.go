package downloader

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrChecksumMismatch is returned when expected checksum does not match actual.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// ComputeFileSHA256 computes hex-encoded SHA256 for file at path.
// Uses io.Copy + sha256.New as required.
func ComputeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for checksum: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeSHA256 computes hex-encoded SHA256 for any reader.
func ComputeSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("hash reader: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum checks that file at path matches expected hex sha256 (case-insensitive).
// Returns ErrChecksumMismatch wrapped on mismatch.
func VerifyChecksum(path, expected string) error {
	if expected == "" {
		return nil
	}
	actual, err := ComputeFileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("%w: expected %s got %s", ErrChecksumMismatch, expected, actual)
	}
	return nil
}
