package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultAppendChunkBytes = 4 * 1024
	defaultDeviceLockDir    = "logs/locks"
)

type fileAppendSink struct {
	mu        sync.Mutex
	path      string
	file      *os.File
	chunkSize int
}

func newFileAppendSink(path string, chunkSize int) (*fileAppendSink, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("append sink path is empty")
	}
	if chunkSize <= 0 {
		chunkSize = defaultAppendChunkBytes
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create append sink directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open append sink file: %w", err)
	}
	return &fileAppendSink{
		path:      path,
		file:      file,
		chunkSize: chunkSize,
	}, nil
}

func (s *fileAppendSink) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *fileAppendSink) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	var syncErr error
	if err := s.file.Sync(); err != nil {
		syncErr = fmt.Errorf("sync append sink file: %w", err)
	}
	err := s.file.Close()
	s.file = nil
	if syncErr != nil {
		if err != nil {
			return fmt.Errorf("%v; close append sink file: %w", syncErr, err)
		}
		return syncErr
	}
	return err
}

func (s *fileAppendSink) withExclusiveLock(fn func(file *os.File) error) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return errors.New("append sink file is closed")
	}
	if err := lockFileExclusive(s.file); err != nil {
		return fmt.Errorf("lock append sink file: %w", err)
	}
	defer func() {
		_ = unlockFile(s.file)
	}()
	return fn(s.file)
}

func (s *fileAppendSink) WriteChunk(chunk []byte) error {
	if s == nil || len(chunk) == 0 {
		return nil
	}
	return s.withExclusiveLock(func(file *os.File) error {
		return writeChunked(file, chunk, s.chunkSize)
	})
}

func writeChunked(file *os.File, payload []byte, chunkSize int) error {
	if file == nil {
		return errors.New("file is nil")
	}
	if len(payload) == 0 {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = defaultAppendChunkBytes
	}
	writer := bufio.NewWriterSize(file, chunkSize)
	for start := 0; start < len(payload); start += chunkSize {
		end := start + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		if _, err := writer.Write(payload[start:end]); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func lockFileExclusive(file *os.File) error {
	if file == nil {
		return errors.New("file is nil")
	}
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func lockFileExclusiveNonBlocking(file *os.File) error {
	if file == nil {
		return errors.New("file is nil")
	}
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func unlockFile(file *os.File) error {
	if file == nil {
		return nil
	}
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

func isFileLockContention(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}

type deviceInstanceLock struct {
	path string
	file *os.File
}

func acquireDeviceInstanceLock(lockDir string, deviceSN string, productName string) (*deviceInstanceLock, error) {
	deviceSN = strings.TrimSpace(deviceSN)
	if deviceSN == "" {
		return nil, errors.New("device serial is empty")
	}
	lockDir = strings.TrimSpace(lockDir)
	if lockDir == "" {
		lockDir = defaultDeviceLockDir
	}
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("create device lock directory: %w", err)
	}
	lockPath := filepath.Join(lockDir, fmt.Sprintf("mqtt-%s.lock", sanitizeLockValue(deviceSN)))
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open device lock file: %w", err)
	}
	if err := lockFileExclusiveNonBlocking(file); err != nil {
		ownerHint := strings.TrimSpace(readLockOwnerHint(lockPath))
		_ = file.Close()
		if isFileLockContention(err) {
			if ownerHint != "" {
				return nil, fmt.Errorf("device %s is already running (%s): %s", deviceSN, lockPath, ownerHint)
			}
			return nil, fmt.Errorf("device %s is already running (%s)", deviceSN, lockPath)
		}
		return nil, fmt.Errorf("acquire device lock: %w", err)
	}

	owner := []string{
		"pid=" + strconv.Itoa(os.Getpid()),
		"started_at=" + time.Now().Format(time.RFC3339Nano),
		"device_sn=" + deviceSN,
	}
	if product := strings.TrimSpace(productName); product != "" {
		owner = append(owner, "product="+product)
	}
	if err := file.Truncate(0); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("truncate device lock file: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("seek device lock file: %w", err)
	}
	if _, err := file.WriteString(strings.Join(owner, " ") + "\n"); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("write device lock owner: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("sync device lock file: %w", err)
	}

	return &deviceInstanceLock{
		path: lockPath,
		file: file,
	}, nil
}

func cleanupStaleDeviceLocks(lockDir string) error {
	lockDir = strings.TrimSpace(lockDir)
	if lockDir == "" {
		lockDir = defaultDeviceLockDir
	}
	entries, err := os.ReadDir(lockDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read device lock directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".lock") {
			continue
		}
		lockPath := filepath.Join(lockDir, name)
		file, openErr := os.OpenFile(lockPath, os.O_RDWR, 0o644)
		if openErr != nil {
			continue
		}
		lockErr := lockFileExclusiveNonBlocking(file)
		if lockErr != nil {
			_ = file.Close()
			continue
		}
		_ = unlockFile(file)
		_ = file.Close()
		_ = os.Remove(lockPath)
	}
	return nil
}

func (l *deviceInstanceLock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *deviceInstanceLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = unlockFile(l.file)
	err := l.file.Close()
	l.file = nil
	if l.path != "" {
		_ = os.Remove(l.path)
	}
	return err
}

func sanitizeLockValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown"
	}
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, raw)
	if safe == "" {
		return "unknown"
	}
	return safe
}

func readLockOwnerHint(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}
