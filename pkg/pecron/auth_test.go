package pecron

import "testing"

func TestResolveRegionNormalizesPecronRegions(t *testing.T) {
	t.Parallel()

	cfg, err := ResolveRegion(" US ")
	if err != nil {
		t.Fatalf("ResolveRegion() error = %v", err)
	}
	if cfg.ID != RegionUS {
		t.Fatalf("region id = %q, want %q", cfg.ID, RegionUS)
	}
	if cfg.BaseURL != "https://iot-api.landecia.com" {
		t.Fatalf("base URL = %q", cfg.BaseURL)
	}
	if cfg.MQTTAddress != "iot-south.landecia.com:8443" {
		t.Fatalf("mqtt address = %q", cfg.MQTTAddress)
	}
	if cfg.MQTTPath != "/ws/v2" {
		t.Fatalf("mqtt path = %q", cfg.MQTTPath)
	}
}

func TestResolveRegionKeepsKnownMQTTBrokerAliases(t *testing.T) {
	t.Parallel()

	cfg, err := ResolveRegion("EU")
	if err != nil {
		t.Fatalf("ResolveRegion() error = %v", err)
	}
	got := cfg.MQTTBrokerAddresses()
	want := []string{
		"iot-south.acceleronix.io:8443",
		"iot-south.quecteleu.com:8443",
	}
	if len(got) != len(want) {
		t.Fatalf("broker addresses = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("broker addresses = %#v, want %#v", got, want)
		}
	}
}

func TestBuildLoginSignature(t *testing.T) {
	t.Parallel()

	got := BuildLoginSignature(
		"owner@example.test",
		"encrypted-password",
		"A1b2C3d4E5f6G7h8",
		"HARsQXfeex8vxyaPRAM8fyjqqVuH2uxAGQ3inJ8XxTiB",
	)
	want := "2a21ff076698b991f336a419a256917ba4363b0e529dd4cf4c2bad3e246ec984"
	if got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
}

func TestEncryptPasswordMatchesPecronAPKDerivation(t *testing.T) {
	t.Parallel()

	got, err := EncryptPassword("battery-staple", "A1b2C3d4E5f6G7h8")
	if err != nil {
		t.Fatalf("EncryptPassword() error = %v", err)
	}
	want := "3sdplu/Bs129i+mVuBFlOg=="
	if got != want {
		t.Fatalf("encrypted password = %q, want %q", got, want)
	}
}
