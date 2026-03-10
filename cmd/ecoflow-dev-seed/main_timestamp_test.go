package main

import (
	"testing"
	"time"
)

func TestNormalizeSeedWriteTimeConvertsToUTC(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC+2", 2*60*60)
	input := time.Date(2026, 3, 9, 12, 0, 0, 0, loc)
	got := normalizeSeedWriteTime(input)

	if got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", got.Location())
	}
	if got.Hour() != 10 {
		t.Fatalf("expected normalized UTC hour 10, got %d", got.Hour())
	}
}
