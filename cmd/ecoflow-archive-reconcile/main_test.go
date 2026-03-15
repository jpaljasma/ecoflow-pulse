package main

import (
	"testing"
	"time"
)

func TestParseRequiredRFC3339(t *testing.T) {
	t.Parallel()

	got, err := parseRequiredRFC3339("from", "2026-03-14T23:35:40Z")
	if err != nil {
		t.Fatalf("parseRequiredRFC3339() error = %v", err)
	}
	want := time.Date(2026, time.March, 14, 23, 35, 40, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseRequiredRFC3339() = %s, want %s", got, want)
	}
}

func TestParseRequiredRFC3339RejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := parseRequiredRFC3339("from", "not-a-timestamp"); err == nil {
		t.Fatal("expected parseRequiredRFC3339() to fail")
	}
}
