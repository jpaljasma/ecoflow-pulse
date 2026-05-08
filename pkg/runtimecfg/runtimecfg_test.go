package runtimecfg_test

import (
	"os"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("RUNTIMECFG_TEST_STRING", "")
	if got := runtimecfg.EnvOrDefault("RUNTIMECFG_TEST_STRING", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}

	t.Setenv("RUNTIMECFG_TEST_STRING", "  value  ")
	if got := runtimecfg.EnvOrDefault("RUNTIMECFG_TEST_STRING", "fallback"); got != "value" {
		t.Fatalf("expected trimmed value, got %q", got)
	}

	t.Setenv("RUNTIMECFG_TEST_STRING", "   ")
	if got := runtimecfg.EnvOrDefault("RUNTIMECFG_TEST_STRING", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for whitespace-only value, got %q", got)
	}
}

func TestSplitNonEmpty(t *testing.T) {
	got := runtimecfg.SplitNonEmpty("a, ,b,, c")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("unexpected split result: %#v", got)
	}
}

func TestBool(t *testing.T) {
	t.Setenv("RUNTIMECFG_TEST_BOOL", "true")
	if !runtimecfg.Bool("RUNTIMECFG_TEST_BOOL", false) {
		t.Fatal("expected true")
	}

	t.Setenv("RUNTIMECFG_TEST_BOOL", "invalid")
	if !runtimecfg.Bool("RUNTIMECFG_TEST_BOOL", true) {
		t.Fatal("expected fallback true")
	}
}

func TestUint32(t *testing.T) {
	t.Setenv("RUNTIMECFG_TEST_UINT32", "42")
	if got := runtimecfg.Uint32("RUNTIMECFG_TEST_UINT32", 7); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}

	t.Setenv("RUNTIMECFG_TEST_UINT32", "-1")
	if got := runtimecfg.Uint32("RUNTIMECFG_TEST_UINT32", 7); got != 7 {
		t.Fatalf("expected fallback 7, got %d", got)
	}
}

func TestFloat64NonNegative(t *testing.T) {
	t.Setenv("RUNTIMECFG_TEST_FLOAT", "1.25")
	if got := runtimecfg.Float64NonNegative("RUNTIMECFG_TEST_FLOAT", 0.5); got != 1.25 {
		t.Fatalf("expected 1.25, got %.2f", got)
	}

	t.Setenv("RUNTIMECFG_TEST_FLOAT", "-1")
	if got := runtimecfg.Float64NonNegative("RUNTIMECFG_TEST_FLOAT", 0.5); got != 0.5 {
		t.Fatalf("expected fallback 0.5, got %.2f", got)
	}
}

func TestIntParsers(t *testing.T) {
	t.Setenv("RUNTIMECFG_TEST_INT", "10")
	if got := runtimecfg.IntAny("RUNTIMECFG_TEST_INT", 5); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}
	if got := runtimecfg.IntPositive("RUNTIMECFG_TEST_INT", 5); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}
	if got := runtimecfg.IntMin("RUNTIMECFG_TEST_INT", 5, 8); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}

	t.Setenv("RUNTIMECFG_TEST_INT", "0")
	if got := runtimecfg.IntPositive("RUNTIMECFG_TEST_INT", 5); got != 5 {
		t.Fatalf("expected fallback 5, got %d", got)
	}
	if got := runtimecfg.IntMin("RUNTIMECFG_TEST_INT", 5, 1); got != 5 {
		t.Fatalf("expected fallback 5, got %d", got)
	}

	t.Setenv("RUNTIMECFG_TEST_INT", "2")
	if got := runtimecfg.IntRange("RUNTIMECFG_TEST_INT", 5, 0, 2); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}

	t.Setenv("RUNTIMECFG_TEST_INT", "3")
	if got := runtimecfg.IntRange("RUNTIMECFG_TEST_INT", 5, 0, 2); got != 5 {
		t.Fatalf("expected fallback 5 for out-of-range value, got %d", got)
	}
}

func TestInt64Min(t *testing.T) {
	t.Setenv("RUNTIMECFG_TEST_INT64", "9")
	if got := runtimecfg.Int64Min("RUNTIMECFG_TEST_INT64", 3, 4); got != 9 {
		t.Fatalf("expected 9, got %d", got)
	}

	t.Setenv("RUNTIMECFG_TEST_INT64", "2")
	if got := runtimecfg.Int64Min("RUNTIMECFG_TEST_INT64", 3, 4); got != 3 {
		t.Fatalf("expected fallback 3, got %d", got)
	}
}

func TestDurationParsers(t *testing.T) {
	t.Setenv("RUNTIMECFG_TEST_DURATION", "3s")
	if got := runtimecfg.DurationPositive("RUNTIMECFG_TEST_DURATION", time.Second); got != 3*time.Second {
		t.Fatalf("expected 3s, got %s", got)
	}
	if got := runtimecfg.DurationNonNegative("RUNTIMECFG_TEST_DURATION", time.Second); got != 3*time.Second {
		t.Fatalf("expected 3s, got %s", got)
	}

	t.Setenv("RUNTIMECFG_TEST_DURATION", "0s")
	if got := runtimecfg.DurationPositive("RUNTIMECFG_TEST_DURATION", time.Second); got != time.Second {
		t.Fatalf("expected fallback 1s, got %s", got)
	}
	if got := runtimecfg.DurationNonNegative("RUNTIMECFG_TEST_DURATION", time.Second); got != 0 {
		t.Fatalf("expected 0s, got %s", got)
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
