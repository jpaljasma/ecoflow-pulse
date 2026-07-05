package edgefiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparePrivateOutputPathCreatesMissingParentPrivate(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "pulse-edge")
	path, err := PreparePrivateOutputPath(filepath.Join(parent, "raw.jsonl"))
	if err != nil {
		t.Fatalf("PreparePrivateOutputPath failed: %v", err)
	}
	if path == "" {
		t.Fatal("clean path is empty")
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("parent mode=%#o want 0700", got)
	}
}

func TestPreparePrivateOutputPathRejectsWritableParent(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := os.Chmod(parent, 0o777); err != nil {
		t.Fatalf("loosen parent: %v", err)
	}
	_, err := PreparePrivateOutputPath(filepath.Join(parent, "raw.jsonl"))
	if err == nil {
		t.Fatal("expected writable parent to be rejected")
	}
	if !strings.Contains(err.Error(), "group or world writable") {
		t.Fatalf("error=%v", err)
	}
}

func TestPreparePrivateDirectoryCreatesPrivateDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "outbox")
	cleanDir, err := PreparePrivateDirectory(dir)
	if err != nil {
		t.Fatalf("PreparePrivateDirectory failed: %v", err)
	}
	if cleanDir != dir {
		t.Fatalf("dir=%q want %q", cleanDir, dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("dir mode=%#o want 0700", got)
	}
}

func TestPreparePrivateDirectoryRejectsWritableDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "outbox")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatalf("loosen dir: %v", err)
	}
	_, err := PreparePrivateDirectory(dir)
	if err == nil {
		t.Fatal("expected writable dir to be rejected")
	}
	if !strings.Contains(err.Error(), "group or world writable") {
		t.Fatalf("error=%v", err)
	}
}

func TestOpenPrivateOutputFileRejectsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.jsonl")
	if err := os.WriteFile(target, []byte("keep\n"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "raw.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	file, err := OpenPrivateOutputFile(link)
	if err == nil {
		_ = file.Close()
		t.Fatal("expected symlink output file to be rejected")
	}
	body, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read target: %v", readErr)
	}
	if string(body) != "keep\n" {
		t.Fatalf("target was modified: %q", body)
	}
}
