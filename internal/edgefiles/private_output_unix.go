//go:build unix

package edgefiles

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// OpenPrivateOutputFile truncates/creates path without following a final symlink.
func OpenPrivateOutputFile(path string) (*os.File, error) {
	cleanPath, err := PreparePrivateOutputPath(path)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open(cleanPath, unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, fmt.Errorf("output file must not be a symlink: %w", err)
		}
		return nil, fmt.Errorf("create output file: %w", err)
	}
	file := os.NewFile(uintptr(fd), cleanPath)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New("create output file: invalid file descriptor")
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure output file: %w", err)
	}
	return file, nil
}
