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
	domains := cfg.loginDomains()
	if len(domains) != 2 {
		t.Fatalf("login domains = %#v, want current plus fallback", domains)
	}
	if domains[0].UserDomain != "C.DM.10351.1" || domains[1].UserDomain != "U.DM.10351.1" {
		t.Fatalf("login domains = %#v", domains)
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

func TestDeviceRefPreservesPecronIdentifierCase(t *testing.T) {
	t.Parallel()

	ref := DeviceRef{ProductKey: "p11u2Q", DeviceKey: "AABBCCDD944C"}
	if got, want := ref.ProviderDeviceID(), "p11u2Q:AABBCCDD944C"; got != want {
		t.Fatalf("provider device id = %q, want %q", got, want)
	}
	if got, want := MQTTChannel(ref), "qdp11u2QAABBCCDD944C"; got != want {
		t.Fatalf("mqtt channel = %q, want %q", got, want)
	}
	parsed, err := ParseProviderDeviceID("p11u2Q:AABBCCDD944C")
	if err != nil {
		t.Fatalf("ParseProviderDeviceID() error = %v", err)
	}
	if parsed != ref {
		t.Fatalf("parsed ref = %#v, want %#v", parsed, ref)
	}
}

func TestTTLVReadPacketMatchesPecronMonitorProtocol(t *testing.T) {
	t.Parallel()

	got := TTLVReadPacket(1)
	want := []byte{0xaa, 0xaa, 0x00, 0x05, 0x12, 0x00, 0x01, 0x00, 0x11}
	if string(got) != string(want) {
		t.Fatalf("TTLV read packet = %x, want %x", got, want)
	}
}
