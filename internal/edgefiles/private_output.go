package edgefiles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PreparePrivateOutputPath creates missing parent directories privately and
// rejects existing parents that are unsafe for truncating service-owned files.
func PreparePrivateOutputPath(path string) (string, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" || cleanPath == "." {
		return "", errors.New("output path must not be empty")
	}
	dir := filepath.Dir(cleanPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
	}
	if err := validatePrivateParentDir(dir); err != nil {
		return "", err
	}
	return cleanPath, nil
}

// PreparePrivateDirectory creates dir privately and rejects existing dirs that
// are unsafe for service-owned buffered telemetry files.
func PreparePrivateDirectory(dir string) (string, error) {
	cleanDir := filepath.Clean(strings.TrimSpace(dir))
	if cleanDir == "" || cleanDir == "." {
		return "", errors.New("directory path must not be empty")
	}
	if err := os.MkdirAll(cleanDir, 0o700); err != nil {
		return "", fmt.Errorf("create private directory: %w", err)
	}
	if err := validatePrivateParentDir(cleanDir); err != nil {
		return "", err
	}
	if err := os.Chmod(cleanDir, 0o700); err != nil {
		return "", fmt.Errorf("secure private directory: %w", err)
	}
	return cleanDir, nil
}

func validatePrivateParentDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("stat output directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output directory must not be a symlink: %s", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("output parent is not a directory: %s", dir)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("output directory must not be group or world writable: %s", dir)
	}
	if !ownedByCurrentUserOrRoot(info) {
		return fmt.Errorf("output directory must be owned by the current user or root: %s", dir)
	}
	return nil
}
