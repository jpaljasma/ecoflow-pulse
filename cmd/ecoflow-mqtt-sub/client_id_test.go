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
			sn:   "DEMOD2M00001057",
			want: "ecoflow-mqtt-31d7d5e0",
		},
		{
			name: "dpu",
			sn:   "DEMODPU0000294",
			want: "ecoflow-mqtt-cf79c985",
		},
		{
			name: "empty",
			sn:   "",
			want: "ecoflow-mqtt-00000000",
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
	a := buildClientID(" DEMOD2M00001057 ")
	b := buildClientID("DEMOD2M00001057")
	if a != b {
		t.Fatalf("expected whitespace-trimmed stable id, got %q vs %q", a, b)
	}
}
