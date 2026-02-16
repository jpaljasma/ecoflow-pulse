package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDeviceInstanceLockPreventsDuplicateSerialProcess(t *testing.T) {
	lockDir := t.TempDir()
	lockA, err := acquireDeviceInstanceLock(lockDir, "SN-123", "Delta")
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	lockPath := lockA.Path()
	defer func() {
		_ = lockA.Close()
	}()

	_, err = acquireDeviceInstanceLock(lockDir, "SN-123", "Delta")
	if err == nil {
		t.Fatal("expected duplicate lock acquisition to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already running") {
		t.Fatalf("unexpected duplicate lock error: %v", err)
	}
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Fatalf("lock file should exist while held: %v", statErr)
	}
}

func TestDeviceInstanceLockCanBeReacquiredAfterClose(t *testing.T) {
	lockDir := t.TempDir()
	lockA, err := acquireDeviceInstanceLock(lockDir, "SN-ABC", "Delta")
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	lockPath := lockA.Path()
	if err := lockA.Close(); err != nil {
		t.Fatalf("close first lock: %v", err)
	}
	if _, statErr := os.Stat(lockPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("lock file should be removed on close: %v", statErr)
	}

	lockB, err := acquireDeviceInstanceLock(lockDir, "SN-ABC", "Delta")
	if err != nil {
		t.Fatalf("reacquire lock: %v", err)
	}
	defer func() {
		_ = lockB.Close()
	}()
	if filepath.Dir(lockB.Path()) != lockDir {
		t.Fatalf("lock dir mismatch: got=%q want=%q", filepath.Dir(lockB.Path()), lockDir)
	}
}

func TestFileAppendSinkConcurrentGoroutinesNoScramble(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.log")
	sink, err := newFileAppendSink(path, 128)
	if err != nil {
		t.Fatalf("newFileAppendSink: %v", err)
	}
	defer func() {
		_ = sink.Close()
	}()

	const workers = 8
	const linesPerWorker = 120

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < linesPerWorker; i++ {
				line := fmt.Sprintf("g%d-m%d\n", worker, i)
				if err := sink.WriteChunk([]byte(line)); err != nil {
					t.Errorf("write error (worker=%d line=%d): %v", worker, i, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	validateStructuredLines(t, path, workers*linesPerWorker, "g", "-m")
}

func TestFileAppendSinkConcurrentMultipleSinksNoScramble(t *testing.T) {
	path := filepath.Join(t.TempDir(), "multi-sink.log")
	sinkA, err := newFileAppendSink(path, 64)
	if err != nil {
		t.Fatalf("newFileAppendSink sinkA: %v", err)
	}
	defer func() {
		_ = sinkA.Close()
	}()
	sinkB, err := newFileAppendSink(path, 64)
	if err != nil {
		t.Fatalf("newFileAppendSink sinkB: %v", err)
	}
	defer func() {
		_ = sinkB.Close()
	}()

	const writesPerSink = 150
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < writesPerSink; i++ {
			if err := sinkA.WriteChunk([]byte(fmt.Sprintf("A-%d\n", i))); err != nil {
				t.Errorf("sinkA write error (%d): %v", i, err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < writesPerSink; i++ {
			if err := sinkB.WriteChunk([]byte(fmt.Sprintf("B-%d\n", i))); err != nil {
				t.Errorf("sinkB write error (%d): %v", i, err)
				return
			}
		}
	}()
	wg.Wait()

	validateStructuredLines(t, path, writesPerSink*2, "", "")
}

func TestFileAppendSinkLargeChunkNoInterleavingAcrossSinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.log")
	sinkA, err := newFileAppendSink(path, 256)
	if err != nil {
		t.Fatalf("newFileAppendSink sinkA: %v", err)
	}
	defer func() { _ = sinkA.Close() }()
	sinkB, err := newFileAppendSink(path, 256)
	if err != nil {
		t.Fatalf("newFileAppendSink sinkB: %v", err)
	}
	defer func() { _ = sinkB.Close() }()

	longA := "A" + strings.Repeat("a", 9000) + "Z\n"
	longB := "B" + strings.Repeat("b", 9000) + "Y\n"
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if err := sinkA.WriteChunk([]byte(longA)); err != nil {
				t.Errorf("sinkA large write error: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if err := sinkB.WriteChunk([]byte(longB)); err != nil {
				t.Errorf("sinkB large write error: %v", err)
				return
			}
		}
	}()
	wg.Wait()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open large log: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 32*1024)
	scanner.Buffer(buf, 128*1024)
	count := 0
	for scanner.Scan() {
		count++
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "A") && strings.HasSuffix(line, "Z"):
			if strings.ContainsRune(line, 'b') {
				t.Fatalf("interleaved line detected in A payload")
			}
		case strings.HasPrefix(line, "B") && strings.HasSuffix(line, "Y"):
			if strings.ContainsRune(line, 'a') {
				t.Fatalf("interleaved line detected in B payload")
			}
		default:
			t.Fatalf("unexpected large line format")
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan large log: %v", err)
	}
	if count != 40 {
		t.Fatalf("large line count mismatch: got=%d want=40", count)
	}
}

func validateStructuredLines(t *testing.T, path string, wantLines int, expectedPrefix string, separator string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)
	lines := 0
	for scanner.Scan() {
		lines++
		line := scanner.Text()
		if expectedPrefix != "" && !strings.HasPrefix(line, expectedPrefix) {
			t.Fatalf("line prefix mismatch: line=%q prefix=%q", line, expectedPrefix)
		}
		if separator != "" && !strings.Contains(line, separator) {
			t.Fatalf("line separator missing: line=%q separator=%q", line, separator)
		}
		for _, r := range line {
			if r == '\n' || r == '\r' {
				t.Fatalf("embedded newline rune in line=%q", line)
			}
		}
		if strings.Contains(line, " ") {
			t.Fatalf("unexpected whitespace in line=%q", line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan log: %v", err)
	}
	if lines != wantLines {
		t.Fatalf("line count mismatch: got=%d want=%d", lines, wantLines)
	}
}
