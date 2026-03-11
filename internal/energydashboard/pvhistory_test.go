package energydashboard

import (
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestSummarizePVPortHistoryAggregatesQuotaEnvelopeObservations(t *testing.T) {
	t.Parallel()

	envelopes := []*envelopev1.TelemetryEnvelope{
		{
			DeviceId:           "device-a",
			PayloadType:        quotaPayloadType,
			ObservedTimeUnixMs: time.Date(2026, time.March, 11, 13, 30, 0, 0, time.UTC).UnixMilli(),
			PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
			Payload: []byte(`{"params":{
				"inLvMpptVol": 66.3,
				"inLvMpptAmp": 0.79,
				"pv1ChargeWatts": 52.4,
				"inHvMpptVol": 322.1,
				"inHvMpptAmp": 0.63,
				"pv2ChargeWatts": 201.0
			}}`),
		},
		{
			DeviceId:           "device-a",
			PayloadType:        quotaPayloadType,
			ObservedTimeUnixMs: time.Date(2026, time.March, 11, 13, 31, 0, 0, time.UTC).UnixMilli(),
			PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
			Payload: []byte(`{"params":{
				"inLvMpptVol": 72.5,
				"inLvMpptAmp": 1.15,
				"pv1ChargeWatts": 83.3,
				"inHvMpptVol": 335.9,
				"inHvMpptAmp": 0.75,
				"pv2ChargeWatts": 252.0
			}}`),
		},
	}

	got := SummarizePVPortHistory(envelopes)
	if len(got) != 2 {
		t.Fatalf("port count mismatch: got=%d want=2", len(got))
	}

	byPort := map[string]PVPortHistory{}
	for _, row := range got {
		byPort[row.PortID] = row
	}
	if row := byPort["pv-low"]; row.DeviceID != "device-a" || row.MaxObservedWatts != 83.3 || row.MaxObservedVolts != 72.5 || row.SampleCount != 2 {
		t.Fatalf("low port summary mismatch: %+v", row)
	}
	if row := byPort["pv-high"]; row.DeviceID != "device-a" || row.MaxObservedWatts != 252.0 || row.LastObservedAt.IsZero() {
		t.Fatalf("high port summary mismatch: %+v", row)
	}
}

func TestSummarizePVPortHistoryFallsBackToDerivedWatts(t *testing.T) {
	t.Parallel()

	got := SummarizePVPortHistory([]*envelopev1.TelemetryEnvelope{{
		DeviceId:           "device-a",
		PayloadType:        quotaPayloadType,
		ObservedTimeUnixMs: 1,
		PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		Payload: []byte(`{"params":{
			"inLvMpptVol": 10.5,
			"inLvMpptAmp": 0.2
		}}`),
	}})

	if len(got) != 1 {
		t.Fatalf("derived port count mismatch: got=%d want=1", len(got))
	}
	if got[0].PortID != "pv-low" || got[0].LastObservedWatts != 2.1 {
		t.Fatalf("derived low-port watts mismatch: %+v", got[0])
	}
}

func TestSummarizePVPortHistoryIgnoresNonQuotaPayloads(t *testing.T) {
	t.Parallel()

	got := SummarizePVPortHistory([]*envelopev1.TelemetryEnvelope{{
		DeviceId:    "device-a",
		PayloadType: "ecoflow.status",
		Payload:     []byte(`{"params":{"inLvMpptVol": 50}}`),
	}})

	if len(got) != 0 {
		t.Fatalf("expected empty summaries for non-quota payload, got=%+v", got)
	}
}
