package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

func TestNormalizeRequestRejectsInvalidInput(t *testing.T) {
	_, err := normalizeRequest(ingestRequest{SerialNumber: "SN"})
	if err == nil {
		t.Fatalf("expected missing device_id error")
	}
	_, err = normalizeRequest(ingestRequest{DeviceID: "nope", SerialNumber: "SN"})
	if err == nil {
		t.Fatalf("expected invalid uuid error")
	}
	_, err = normalizeRequest(ingestRequest{DeviceID: uuid.NewString()})
	if err == nil {
		t.Fatalf("expected missing serial_number error")
	}
}

func TestNormalizeRequestSetsDefaultsAndCanonicalFields(t *testing.T) {
	deviceID := uuid.NewString()
	req, err := normalizeRequest(ingestRequest{
		DeviceID:     "  " + deviceID + " ",
		SerialNumber: " demod2m00001057 ",
	})
	if err != nil {
		t.Fatalf("normalizeRequest failed: %v", err)
	}
	if req.DeviceID != deviceID {
		t.Fatalf("device_id mismatch: got=%q want=%q", req.DeviceID, deviceID)
	}
	if req.SerialNumber != "DEMOD2M00001057" {
		t.Fatalf("serial_number mismatch: got=%q", req.SerialNumber)
	}
	if req.ObservedUnixM <= 0 {
		t.Fatalf("expected observed timestamp default")
	}
}

func TestBuildPayloadUsesExplicitValues(t *testing.T) {
	payload, err := buildPayload(requestMetrics{
		SOC:         floatPtr(62.5),
		PVW:         floatPtr(410),
		LoadW:       floatPtr(280),
		AcW:         floatPtr(15),
		DcW:         floatPtr(22),
		BatteryInW:  floatPtr(480),
		BatteryOutW: floatPtr(40),
		TempC:       floatPtr(30),
	})
	if err != nil {
		t.Fatalf("buildPayload failed: %v", err)
	}
	var decoded map[string]map[string]float64
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	params := decoded["params"]
	if params["soc"] != 62.5 {
		t.Fatalf("soc mismatch: got=%v", params["soc"])
	}
	if params["pv1ChargeWatts"] != 410 {
		t.Fatalf("pv mismatch: got=%v", params["pv1ChargeWatts"])
	}
	if params["wattsOutSum"] != 280 {
		t.Fatalf("load mismatch: got=%v", params["wattsOutSum"])
	}
	if params["bmsInputWatts"] != 480 {
		t.Fatalf("battery_in mismatch: got=%v", params["bmsInputWatts"])
	}
	if params["bmsOutputWatts"] != 40 {
		t.Fatalf("battery_out mismatch: got=%v", params["bmsOutputWatts"])
	}
}

func TestBuildEnvelopeSetsRequiredFieldsForRollups(t *testing.T) {
	deviceID := uuid.NewString()
	now := time.UnixMilli(1773000000000).UTC()
	env, err := buildEnvelope(config{
		source:   "k6-loadtest",
		provider: "ecoflow",
		subject:  telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 128},
	}, ingestRequest{
		DeviceID:      deviceID,
		SerialNumber:  "DEMOD2M00001057",
		ObservedUnixM: now.UnixMilli(),
		MessageID:     "msg-1",
	}, now)
	if err != nil {
		t.Fatalf("buildEnvelope failed: %v", err)
	}
	if _, err := uuid.Parse(env.GetEnvelopeId()); err != nil {
		t.Fatalf("expected uuidv7 envelope id: %v", err)
	}
	if env.GetDeviceId() != deviceID {
		t.Fatalf("device id mismatch: got=%q want=%q", env.GetDeviceId(), deviceID)
	}
	if env.GetEcoflowSn() != "DEMOD2M00001057" {
		t.Fatalf("serial mismatch: got=%q", env.GetEcoflowSn())
	}
	if env.GetLabels()["provider"] != "ecoflow" {
		t.Fatalf("provider label mismatch: got=%q", env.GetLabels()["provider"])
	}
	if env.GetPayloadType() != "ecoflow.quota.normalized" {
		t.Fatalf("payload_type mismatch: got=%q", env.GetPayloadType())
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
