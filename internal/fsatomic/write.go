package fsatomic

import (
	"fmt"
	"os"
	"path/filepath"
)

var replace = replaceFile

// ReplaceFile atomically replaces destination with an already-flushed file
// from the same directory without changing the directory's ACL.
func ReplaceFile(source, destination string) error {
	if source == "" || destination == "" {
		return fmt.Errorf("atomic replace paths must not be empty")
	}
	if filepath.Clean(filepath.Dir(source)) != filepath.Clean(filepath.Dir(destination)) {
		return fmt.Errorf("atomic replace requires source and destination in the same directory")
	}
	return replaceFile(source, destination)
}

// WriteFile replaces path only after the complete temporary file is flushed.
// The temporary file lives in the destination directory so the final rename
// cannot cross volumes.
func WriteFile(path string, data []byte, mode os.FileMode) (err error) {
	if path == "" {
		return fmt.Errorf("atomic file path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create atomic file directory: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		if err := restrictPath(path); err != nil {
			return fmt.Errorf("restrict existing atomic file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing atomic file: %w", err)
	}
	if err := restrictPath(dir); err != nil {
		return fmt.Errorf("restrict atomic file directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".navo-atomic-*.tmp")
	if err != nil {
		return fmt.Errorf("create atomic temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(mode); err != nil {
		return fmt.Errorf("restrict atomic temporary file: %w", err)
	}
	if err := restrictPath(tempPath); err != nil {
		return fmt.Errorf("set atomic temporary file DACL: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write atomic temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("flush atomic temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close atomic temporary file: %w", err)
	}
	if err := replace(tempPath, path); err != nil {
		return fmt.Errorf("replace atomic file: %w", err)
	}
	return nil
}
