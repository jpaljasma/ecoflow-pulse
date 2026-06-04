package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/internal/archiveworker"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("ecoflow-archive-outbox-status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := strings.TrimSpace(getenv("ARCHIVE_UPLOAD_OUTBOX_DIR"))
	failOnPending := false
	fs.StringVar(&dir, "dir", dir, "Archive upload outbox directory")
	fs.BoolVar(&failOnPending, "fail-on-pending", false, "Exit with status 2 when pending entries exist")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	count, err := archiveworker.CountFileArchiveUploadOutboxPending(dir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "archive upload outbox status failed: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "pending=%d dir=%s\n", count, dir)
	if failOnPending && count > 0 {
		return 2
	}
	return 0
}
