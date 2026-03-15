package rollupworker

import (
	"testing"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestRollupDedupKeyPrefersEnvelopeID(t *testing.T) {
	t.Parallel()

	env := &envelopev1.TelemetryEnvelope{
		EnvelopeId: "env-1",
		DeviceId:   "device-1",
		MessageId:  "message-1",
	}

	if got := rollupDedupKey(env); got != "env:env-1" {
		t.Fatalf("dedup key mismatch: got=%q", got)
	}
}

func TestRollupDedupKeyFallsBackToDeviceAndMessageID(t *testing.T) {
	t.Parallel()

	env := &envelopev1.TelemetryEnvelope{
		DeviceId:  "device-1",
		MessageId: "message-1",
	}

	if got := rollupDedupKey(env); got != "msg:device-1:message-1" {
		t.Fatalf("dedup key mismatch: got=%q", got)
	}
}
