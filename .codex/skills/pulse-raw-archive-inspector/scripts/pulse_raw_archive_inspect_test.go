package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

type failingObjectReader struct{}

func (failingObjectReader) ReadObject(context.Context, string, string) ([]byte, error) {
	return nil, errors.New("read bucket sensitive-bucket key raw/provider/device/object.pb failed")
}

func (failingObjectReader) Close() error {
	return nil
}

func TestShouldScanArchiveHonorsUnmatchedFamilyFilter(t *testing.T) {
	if shouldScanArchive([]string{"missing-family"}, nil) {
		t.Fatal("expected unmatched family filter to skip archive scan")
	}
	if !shouldScanArchive(nil, nil) {
		t.Fatal("expected empty family filter to scan provider archive")
	}
	if !shouldScanArchive([]string{"e1000lfp"}, []string{"00000000-0000-0000-0000-000000000000"}) {
		t.Fatal("expected matched family filter with canonical device id to scan archive")
	}
}

func TestInspectArchiveRedactsReadErrors(t *testing.T) {
	output := captureStderr(t, func() {
		inspectArchive(
			context.Background(),
			failingObjectReader{},
			[]manifestObject{{bucket: "sensitive-bucket", key: "raw/provider/device/object.pb"}},
			config{provider: "pecron"},
			time.Unix(0, 0),
			time.Unix(10, 0),
			nil,
			nil,
		)
	})

	if !strings.Contains(output, "failed to read one archive object") {
		t.Fatalf("expected generic read warning, got %q", output)
	}
	if strings.Contains(output, "sensitive-bucket") || strings.Contains(output, "raw/provider/device/object.pb") {
		t.Fatalf("expected object identifiers to be redacted, got %q", output)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = original
		_ = reader.Close()
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(output)
}
