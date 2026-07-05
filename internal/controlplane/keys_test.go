package controlplane

import "testing"

func TestIsSupportedProvider(t *testing.T) {
	t.Parallel()
	if !IsSupportedProvider("ecoflow") {
		t.Fatalf("expected ecoflow to be supported")
	}
	if !IsSupportedProvider(" EcoFlow ") {
		t.Fatalf("expected provider normalization to pass")
	}
	if !IsSupportedProvider(" EcoFlow_BLE ") {
		t.Fatalf("expected ecoflow_ble to be supported")
	}
	if !IsSupportedProvider("PulseMQTT") {
		t.Fatalf("expected pulsemqtt to be supported")
	}
	if !IsSupportedProvider(" PECRON ") {
		t.Fatalf("expected pecron to be supported")
	}
	if !IsSupportedProvider(" Anker_Solix ") {
		t.Fatalf("expected anker_solix to be supported")
	}
	if IsSupportedProvider("unknown") {
		t.Fatalf("expected unknown provider to be unsupported")
	}
}

func TestMaskAccessKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"ABCD", "A***"},
		{"ABCDEFGH", "AB...GH"},
		{"AK1234567890", "AK12...7890"},
	}
	for _, tc := range cases {
		if got := MaskAccessKey(tc.in); got != tc.want {
			t.Fatalf("MaskAccessKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHashAccessKeyDeterministic(t *testing.T) {
	t.Parallel()
	a := HashAccessKey("AK123456")
	b := HashAccessKey("AK123456")
	if len(a) != 32 {
		t.Fatalf("expected 32-byte hash, got %d", len(a))
	}
	if string(a) != string(b) {
		t.Fatalf("expected deterministic hash output")
	}
}
