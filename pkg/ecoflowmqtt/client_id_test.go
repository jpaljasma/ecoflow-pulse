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
			sn:   "DEMOD2M00001057",
			want: "ecoflow-pulse-31d7d5e0",
		},
		{
			name: "dpu",
			sn:   "DEMODPU0000294",
			want: "ecoflow-pulse-cf79c985",
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
	a := BuildClientIDFromSN(" DEMOD2M00001057 ")
	b := BuildClientIDFromSN("DEMOD2M00001057")
	if a != b {
		t.Fatalf("expected whitespace-trimmed stable id, got %q vs %q", a, b)
	}
}

func TestBuildClientIDWithNamespaceSeparatesEnvironments(t *testing.T) {
	local := BuildClientIDWithNamespace("local", "DEMOD2M00001057")
	cloud := BuildClientIDWithNamespace("cloud", "DEMOD2M00001057")
	if local == cloud {
		t.Fatalf("expected distinct ids for local and cloud, got shared id %q", local)
	}
}

func TestBuildClientIDWithNamespaceBlankMatchesLegacy(t *testing.T) {
	legacy := BuildClientIDFromSN("DEMOD2M00001057")
	got := BuildClientIDWithNamespace("", "DEMOD2M00001057")
	if got != legacy {
		t.Fatalf("blank namespace should preserve legacy client id, got=%q want=%q", got, legacy)
	}
}
