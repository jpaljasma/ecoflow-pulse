package main

import (
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

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

func TestBuildMQTTProbeTargetsDedupesAndSorts(t *testing.T) {
	t.Parallel()

	cert := ecoflow.GeneralInfoMQTTCertification{
		URL:                "mqtt.ecoflow.com",
		Port:               "8883",
		CertificateAccount: "open-account",
	}
	devices := []ecoflow.GeneralInfoDevice{
		{SN: "R634ZABAWH2G1008", DeviceName: "River 3 Plus"},
		{SN: "pr12za1cdhaw0498", DeviceName: "Delta 1000 Air"},
		{SN: "R634ZABAWH2G1008", DeviceName: "River 3 Plus duplicate"},
	}

	address, targets, err := buildMQTTProbeTargets(cert, devices)
	if err != nil {
		t.Fatalf("buildMQTTProbeTargets() error = %v", err)
	}
	if address != "mqtt.ecoflow.com:8883" {
		t.Fatalf("mqtt address = %q, want %q", address, "mqtt.ecoflow.com:8883")
	}
	if len(targets) != 2 {
		t.Fatalf("targets len = %d, want 2", len(targets))
	}
	if got := targets[0].serialNumber; got != "PR12ZA1CDHAW0498" {
		t.Fatalf("targets[0].serialNumber = %q, want %q", got, "PR12ZA1CDHAW0498")
	}
	if got := targets[0].topic; got != "/open/open-account/PR12ZA1CDHAW0498/quota" {
		t.Fatalf("targets[0].topic = %q", got)
	}
	if got := targets[1].serialNumber; got != "R634ZABAWH2G1008" {
		t.Fatalf("targets[1].serialNumber = %q, want %q", got, "R634ZABAWH2G1008")
	}
}

func TestBuildSmokeMQTTClientIDAvoidsDeviceScopedCollision(t *testing.T) {
	t.Parallel()

	targets := []mqttProbeTarget{
		{serialNumber: "PR12ZA1CDHAW0498"},
		{serialNumber: "R634ZABAWH2G1008"},
	}

	clientID := buildSmokeMQTTClientID(targets)
	if clientID == ecoflowmqtt.BuildClientIDFromSN("PR12ZA1CDHAW0498") {
		t.Fatalf("client ID %q should not collide with device-scoped client IDs", clientID)
	}
}
