//go:build !unix

package edgefiles

import (
	"errors"
	"fmt"
	"os"
)

// OpenPrivateOutputFile truncates/creates path after rejecting a visible final symlink.
func OpenPrivateOutputFile(path string) (*os.File, error) {
	cleanPath, err := PreparePrivateOutputPath(path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(cleanPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("output file must not be a symlink")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat output file: %w", err)
	}
	file, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create output file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure output file: %w", err)
	}
	return file, nil
}
