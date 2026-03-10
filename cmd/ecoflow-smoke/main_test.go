package main

import "testing"

func TestMaskSecret(t *testing.T) {
	if got := maskSecret("abcd"); got != "****" {
		t.Fatalf("short secret mask=%q want=****", got)
	}
	if got := maskSecret("abcdefgh"); got != "ab****gh" {
		t.Fatalf("long secret mask=%q", got)
	}
}

func TestExtractPVInputEntriesFiltersKeys(t *testing.T) {
	quota := map[string]string{
		"mppt.inWatts": "123.4",
		"foo.bar":      "999",
		"inHvMpptPwr":  "77.0",
	}
	got := extractPVInputEntries(quota)
	if len(got) != 2 {
		t.Fatalf("pv entry len=%d want=2", len(got))
	}
}
