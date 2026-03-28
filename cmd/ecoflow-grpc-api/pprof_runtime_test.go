package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewPprofServerFromEnvDisabled(t *testing.T) {
	t.Setenv("GRPC_PPROF_LISTEN_ADDR", "")

	stop, addr, err := newPprofServerFromEnv(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newPprofServerFromEnv() error = %v", err)
	}
	if addr != "disabled" {
		t.Fatalf("addr = %q, want disabled", addr)
	}
	stop()
}

func TestNewPprofServerFromEnvServesProfiles(t *testing.T) {
	t.Setenv("GRPC_PPROF_LISTEN_ADDR", "127.0.0.1:0")

	stop, addr, err := newPprofServerFromEnv(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newPprofServerFromEnv() error = %v", err)
	}
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	for {
		resp, err = http.Get("http://" + addr + "/debug/pprof/goroutine?debug=1")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET pprof goroutine profile failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !strings.Contains(string(body), "goroutine") {
		t.Fatalf("pprof body did not look like a goroutine profile: %q", string(body))
	}
}

func TestNewPprofServerFromEnvRejectsInvalidAddr(t *testing.T) {
	t.Setenv("GRPC_PPROF_LISTEN_ADDR", "not-a-listener")

	_, _, err := newPprofServerFromEnv(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected error for invalid pprof listen addr")
	}
}
