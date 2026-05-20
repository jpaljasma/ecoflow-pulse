package main

import (
	"reflect"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

func TestProviderConfigMergesExplicitRegion(t *testing.T) {
	t.Parallel()

	got, err := providerConfig(map[string]any{"region": "cn", "stored": "kept"}, smokeConfig{
		configJSON: `{"region":"eu","extra":true}`,
		region:     "us",
	})
	if err != nil {
		t.Fatalf("providerConfig() error = %v", err)
	}
	want := map[string]any{"region": "us", "extra": true, "stored": "kept"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("provider config = %#v, want %#v", got, want)
	}
}

func TestBuildEnvPecronCredentialRequiresCompleteMaterialOrDB(t *testing.T) {
	t.Parallel()

	_, err := buildEnvPecronCredential(smokeConfig{email: "owner@example.test"})
	if err == nil {
		t.Fatal("expected missing password error")
	}
	if got := err.Error(); got != "set both PECRON_EMAIL and PECRON_PASSWORD, or set CONTROL_PLANE_DB_DSN to load the active stored Pecron credential" {
		t.Fatalf("error = %q", got)
	}
}

func TestResolvePecronCredentialRejectsUnsupportedMQTTQoS(t *testing.T) {
	t.Parallel()

	_, err := resolvePecronCredential(t.Context(), smokeConfig{
		email:    "owner@example.test",
		password: "password",
		mqttQoS:  2,
	})
	if err == nil {
		t.Fatal("expected unsupported MQTT QoS to fail")
	}
	if got := err.Error(); got != "mqtt-qos must be 0 or 1" {
		t.Fatalf("error = %q", got)
	}
}

func TestSelectTargetDeviceUsesProviderOrCanonicalSuffix(t *testing.T) {
	t.Parallel()

	devices := []controlplane.ProviderDevice{
		{ProviderDeviceID: "p11vxg:aabbccdd1111", CanonicalSN: "PECRON-P11VXG-AABBCCDD1111"},
		{ProviderDeviceID: "p11vxg:aabbccdd944c", CanonicalSN: "PECRON-P11VXG-AABBCCDD944C"},
	}
	got, err := selectTargetDevice(devices, "944C")
	if err != nil {
		t.Fatalf("selectTargetDevice() error = %v", err)
	}
	if got.ProviderDeviceID != "p11vxg:aabbccdd944c" {
		t.Fatalf("selected provider device = %q", got.ProviderDeviceID)
	}

	got, err = selectTargetDevice(devices, "1111")
	if err != nil {
		t.Fatalf("selectTargetDevice() canonical error = %v", err)
	}
	if got.ProviderDeviceID != "p11vxg:aabbccdd1111" {
		t.Fatalf("selected canonical device = %q", got.ProviderDeviceID)
	}
}

func TestPecronSmokeRedactsIdentifiers(t *testing.T) {
	t.Parallel()

	if got := maskProviderDeviceID("p11vxg:aabbccdd944c"); got != "p11vxg:...944c" {
		t.Fatalf("provider device mask = %q", got)
	}
	if got := maskCanonicalSN("PECRON-P11VXG-AABBCCDD944C"); got != "PECRON-P11VXG-...944C" {
		t.Fatalf("canonical sn mask = %q", got)
	}
	if got := redactTopic("q/2/d/qdp11vxgaabbccdd944c/bus_"); got != "q/2/d/redacted/bus_" {
		t.Fatalf("topic mask = %q", got)
	}
}
