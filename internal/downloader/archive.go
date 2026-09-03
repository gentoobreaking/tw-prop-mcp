package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RawArchiveStore manages raw/source_snapshot/{snapshotID}/ storage.
// Implements P2 Raw Data Immutable: written files become read-only and never overwritten.
type RawArchiveStore struct {
	RootDir string
}

// NewRawArchiveStore creates a store rooted at root (e.g., "raw").
func NewRawArchiveStore(root string) *RawArchiveStore {
	return &RawArchiveStore{RootDir: root}
}

// ArchiveManifest is written as manifest.json inside each snapshot archive.
type ArchiveManifest struct {
	SnapshotID   string            `json:"snapshot_id"`
	Source       string            `json:"source"`
	SourceVersion string           `json:"source_version"`
	FileName     string            `json:"file_name"`
	FileSHA256   string            `json:"file_sha256"`
	OriginalFile string            `json:"original_file"`
	DownloadedAt time.Time         `json:"downloaded_at"`
	Extra        map[string]any    `json:"extra,omitempty"`
}

// StoreRawArchive atomically stores srcPath into raw/source_snapshot/{snapshotID}/
// Layout: {manifest.json, original_file (preserving basename), checksum.sha256, downloaded_at.txt, source_metadata.json}
// Uses tmp dir + rename for atomicity, chmod 444 for original file, and refuses second write.
func (s *RawArchiveStore) StoreRawArchive(srcPath, snapshotID string, metadata map[string]any) (string, error) {
	if srcPath == "" {
		return "", fmt.Errorf("srcPath empty")
	}
	if snapshotID == "" {
		return "", fmt.Errorf("snapshotID empty")
	}
	if _, err := os.Stat(srcPath); err != nil {
		return "", fmt.Errorf("source file missing: %w", err)
	}

	baseDir := filepath.Join(s.RootDir, "source_snapshot", snapshotID)

	// Refuse second write: if destination already exists, fail.
	if _, err := os.Stat(baseDir); err == nil {
		return "", fmt.Errorf("archive already exists: %s (immutable)", baseDir)
	}

	tmpDir := baseDir + ".tmp"
	// Clean any leftover tmp from prior failed attempt.
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir tmp: %w", err)
	}
	// Ensure cleanup on error before rename.
	cleanupOnErr := true
	defer func() {
		if cleanupOnErr {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	// Compute checksum of src file.
	sha, err := ComputeFileSHA256(srcPath)
	if err != nil {
		return "", err
	}

	// Copy original file preserving basename.
	origName := filepath.Base(srcPath)
	if metadata != nil {
		if fn, ok := metadata["file_name"].(string); ok && fn != "" {
			origName = filepath.Base(fn)
		}
	}
	dstOriginal := filepath.Join(tmpDir, origName)
	if err := copyFile(srcPath, dstOriginal); err != nil {
		return "", fmt.Errorf("copy original: %w", err)
	}
	// Set original file read-only.
	if err := os.Chmod(dstOriginal, 0o444); err != nil {
		return "", fmt.Errorf("chmod original: %w", err)
	}

	// checksum.sha256: "<hex>  <filename>\n"
	checksumPath := filepath.Join(tmpDir, "checksum.sha256")
	if err := os.WriteFile(checksumPath, []byte(sha+"  "+origName+"\n"), 0o444); err != nil {
		return "", fmt.Errorf("write checksum: %w", err)
	}

	// downloaded_at.txt
	now := time.Now().UTC()
	downloadedPath := filepath.Join(tmpDir, "downloaded_at.txt")
	if err := os.WriteFile(downloadedPath, []byte(now.Format(time.RFC3339)+"\n"), 0o444); err != nil {
		return "", fmt.Errorf("write downloaded_at: %w", err)
	}

	// source_metadata.json
	metaPath := filepath.Join(tmpDir, "source_metadata.json")
	metaBytes := []byte("{}")
	if metadata != nil {
		// Enrich metadata with computed fields if missing.
		if _, ok := metadata["file_sha256"]; !ok {
			metadata["file_sha256"] = sha
		}
		if _, ok := metadata["downloaded_at"]; !ok {
			metadata["downloaded_at"] = now.Format(time.RFC3339)
		}
		if b, err := json.MarshalIndent(metadata, "", "  "); err == nil {
			metaBytes = b
		} else {
			return "", fmt.Errorf("marshal metadata: %w", err)
		}
	}
	if err := os.WriteFile(metaPath, metaBytes, 0o444); err != nil {
		return "", fmt.Errorf("write metadata: %w", err)
	}

	// manifest.json
	manifest := ArchiveManifest{
		SnapshotID:    snapshotID,
		FileSHA256:    sha,
		OriginalFile:  origName,
		DownloadedAt:  now,
		Extra:         nil,
	}
	if metadata != nil {
		if v, ok := metadata["source"].(string); ok {
			manifest.Source = v
		}
		if v, ok := metadata["source_version"].(string); ok {
			manifest.SourceVersion = v
		}
		if v, ok := metadata["file_name"].(string); ok {
			manifest.FileName = v
		} else {
			manifest.FileName = origName
		}
		// Stash full metadata as Extra for debugging.
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o444); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}

	// Ensure parent exists for rename.
	if err := os.MkdirAll(filepath.Dir(baseDir), 0o755); err != nil {
		return "", fmt.Errorf("mkdir parent: %w", err)
	}
	// Atomic rename.
	if err := os.Rename(tmpDir, baseDir); err != nil {
		return "", fmt.Errorf("rename archive: %w", err)
	}
	cleanupOnErr = false

	// Ensure final original file is still 444 after rename (rename preserves).
	// Also make other files read-only (already 444).

	return baseDir, nil
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// ArchiveDir returns the expected archive directory for a snapshot.
func (s *RawArchiveStore) ArchiveDir(snapshotID string) string {
	return filepath.Join(s.RootDir, "source_snapshot", snapshotID)
}
