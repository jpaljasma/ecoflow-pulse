package ecoflowmqtt

import "testing"

func TestBuildClientIDFromSNDeterministicCRC32(t *testing.T) {
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
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildClientIDFromSN(tc.sn); got != tc.want {
				t.Fatalf("BuildClientIDFromSN(%q) = %q, want %q", tc.sn, got, tc.want)
			}
		})
	}
}

func TestBuildClientIDFromSNStableForWhitespace(t *testing.T) {
	a := BuildClientIDFromSN(" R351ZABAPH331057 ")
	b := BuildClientIDFromSN("R351ZABAPH331057")
	if a != b {
		t.Fatalf("expected whitespace-trimmed stable id, got %q vs %q", a, b)
	}
}
