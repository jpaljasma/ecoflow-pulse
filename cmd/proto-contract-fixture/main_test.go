package main

import (
	"encoding/base64"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestSampleEnvelopeRoundTripDecode(t *testing.T) {
	env := sampleEnvelope()
	wire, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal sample envelope: %v", err)
	}
	decoded, err := decodeEnvelope(base64.StdEncoding.EncodeToString(wire))
	if err != nil {
		t.Fatalf("decodeEnvelope() error = %v", err)
	}
	if decoded.EnvelopeID != env.GetEnvelopeId() || decoded.DeviceID != env.GetDeviceId() {
		t.Fatalf("decoded envelope mismatch: %+v", decoded)
	}
	if decoded.PayloadUTF8 == "" || decoded.PayloadBase64 == "" {
		t.Fatalf("expected payload fields to be populated: %+v", decoded)
	}
}
