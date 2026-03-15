package envelopededup

import (
	"testing"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestKeyPrefersEnvelopeID(t *testing.T) {
	t.Parallel()

	got := Key(&envelopev1.TelemetryEnvelope{
		EnvelopeId: "env-1",
		DeviceId:   "device-1",
		MessageId:  "message-1",
	})
	if got != "env:env-1" {
		t.Fatalf("Key() = %q, want env:env-1", got)
	}
}

func TestKeyFallsBackToDeviceAndMessageID(t *testing.T) {
	t.Parallel()

	got := Key(&envelopev1.TelemetryEnvelope{
		DeviceId:  "device-1",
		MessageId: "message-1",
	})
	if got != "msg:device-1:message-1" {
		t.Fatalf("Key() = %q, want msg:device-1:message-1", got)
	}
}

func TestKeyReturnsEmptyForInsufficientIdentity(t *testing.T) {
	t.Parallel()

	if got := Key(&envelopev1.TelemetryEnvelope{}); got != "" {
		t.Fatalf("Key() = %q, want empty", got)
	}
}
