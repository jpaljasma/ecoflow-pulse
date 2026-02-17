package main

import "testing"

func TestBuildClientIDDeterministicCRC32(t *testing.T) {
	tests := []struct {
		name string
		sn   string
		want string
	}{
		{
			name: "d2m",
			sn:   "R351ZABAPH331057",
			want: "ecoflow-pulse-29f3a572",
		},
		{
			name: "dpu",
			sn:   "Y711ZABA9H2P0294",
			want: "ecoflow-pulse-b416aca2",
		},
		{
			name: "empty",
			sn:   "",
			want: "ecoflow-pulse-00000000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildClientID(tt.sn); got != tt.want {
				t.Fatalf("buildClientID(%q) = %q, want %q", tt.sn, got, tt.want)
			}
		})
	}
}

func TestBuildClientIDStableForWhitespace(t *testing.T) {
	a := buildClientID(" R351ZABAPH331057 ")
	b := buildClientID("R351ZABAPH331057")
	if a != b {
		t.Fatalf("expected whitespace-trimmed stable id, got %q vs %q", a, b)
	}
}
