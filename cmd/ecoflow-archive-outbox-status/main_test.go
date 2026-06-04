package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportsPendingOutboxEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pending.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write pending outbox entry: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--dir", dir, "--fail-on-pending"}, func(string) string { return "" }, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code=%d want 2 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "pending=1") {
		t.Fatalf("stdout missing pending count: %q", stdout.String())
	}
}

func TestRunUsesEnvDir(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer

	code := run(nil, func(key string) string {
		if key == "ARCHIVE_UPLOAD_OUTBOX_DIR" {
			return dir
		}
		return ""
	}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("exit code=%d want 0", code)
	}
	if !strings.Contains(stdout.String(), "pending=0 dir="+dir) {
		t.Fatalf("stdout mismatch: %q", stdout.String())
	}
}
