package rolluprebuild

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/proto"
)

func TestDedupeManifestObjectsByBucketAndKey(t *testing.T) {
	t.Parallel()

	objects := []replaycli.ManifestObject{
		{ObjectBucket: "raw", ObjectKey: "a.pb.zst"},
		{ObjectBucket: "raw", ObjectKey: "a.pb.zst"},
		{ObjectBucket: "raw", ObjectKey: "b.pb.zst"},
	}
	got := dedupeManifestObjects(objects)
	if len(got) != 2 {
		t.Fatalf("dedupeManifestObjects count mismatch: got=%d want=2", len(got))
	}
}

func TestRunnerRebuildDevicesFailsBeforeReplacementOnMissingArchiveObject(t *testing.T) {
	t.Parallel()

	runner, err := NewRunner(
		nil,
		&runnerTestManifestStore{deviceObjects: []replaycli.ManifestObject{{
			ObjectBucket:      "raw",
			ObjectKey:         "missing.pb.zst",
			Provider:          "ecoflow",
			ProviderDeviceIDs: []string{"Y711ZABA9H2P0294"},
			Shard:             1,
		}}},
		&runnerTestObjectReader{err: errors.New("The specified key does not exist.")},
		&PostgresWriter{},
		100,
		1,
	)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	_, err = runner.RebuildDevices(context.Background(), replaycli.DeviceQuery{
		Provider:          "ecoflow",
		FromUnixMS:        time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC).UnixMilli(),
		ToUnixMS:          time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC).UnixMilli(),
		ProviderDeviceIDs: []string{"Y711ZABA9H2P0294"},
	})
	if err == nil {
		t.Fatalf("expected rebuild to fail on missing archive object")
	}
	if !strings.Contains(err.Error(), "missing archive objects") {
		t.Fatalf("expected missing-archive error, got: %v", err)
	}
}

func TestProcessObjectGroupDedupesDuplicateEnvelopesDuringRebuild(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 14, 14, 0, 0, 0, time.UTC)
	deviceID := "019cec1d-9a84-7e55-8018-27353cbc79da"
	envA := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-1",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "Y711ZABA9H2P0294",
		MessageId:          "msg-1",
		ObservedTimeUnixMs: base.UnixMilli(),
		DeviceTimeUnixMs:   base.UnixMilli(),
		IngestedTimeUnixMs: base.UnixMilli(),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "Y711ZABA9H2P0294",
		},
		Payload:         []byte(`{"params":{"inLvMpptPwr":120}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
	})
	envDup := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-1",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "Y711ZABA9H2P0294",
		MessageId:          "msg-1",
		ObservedTimeUnixMs: base.UnixMilli(),
		DeviceTimeUnixMs:   base.UnixMilli(),
		IngestedTimeUnixMs: base.UnixMilli(),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "Y711ZABA9H2P0294",
		},
		Payload:         []byte(`{"params":{"inLvMpptPwr":120}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
	})
	envB := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-2",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "Y711ZABA9H2P0294",
		MessageId:          "msg-2",
		ObservedTimeUnixMs: base.Add(30 * time.Second).UnixMilli(),
		DeviceTimeUnixMs:   base.Add(30 * time.Second).UnixMilli(),
		IngestedTimeUnixMs: base.Add(30 * time.Second).UnixMilli(),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "Y711ZABA9H2P0294",
		},
		Payload:         []byte(`{"params":{"inLvMpptPwr":60}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
	})

	runner, err := NewRunner(
		nil,
		&runnerTestManifestStore{},
		&runnerTestObjectReader{body: encodeRebuildFrames(t, envA, envDup, envB)},
		&PostgresWriter{},
		100,
		1,
	)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	var objectsProcessed atomic.Int64
	var missingObjects atomic.Int64
	var messagesDecoded atomic.Int64
	var messagesApplied atomic.Int64
	var quotaMessages atomic.Int64

	result := runner.processObjectGroup(
		context.Background(),
		[]replaycli.ManifestObject{{
			ObjectBucket:      "pulse-telemetry-raw",
			ObjectKey:         "raw/yyyy=2026/mm=03/dd=14/hh=14/shard=001/part-00001.pb.zst",
			Provider:          "ecoflow",
			ProviderDeviceIDs: []string{"Y711ZABA9H2P0294"},
			Shard:             1,
		}},
		&objectsProcessed,
		&missingObjects,
		1,
		&messagesDecoded,
		&messagesApplied,
		&quotaMessages,
		base.Add(time.Minute).UnixMilli(),
	)
	if result.err != nil {
		t.Fatalf("processObjectGroup failed: %v", result.err)
	}
	if result.messagesDecoded != 3 {
		t.Fatalf("messagesDecoded = %d, want 3", result.messagesDecoded)
	}
	if result.messagesApplied != 2 {
		t.Fatalf("messagesApplied = %d, want 2 after dedupe", result.messagesApplied)
	}
	if len(result.minuteRows) != 1 {
		t.Fatalf("minute row count = %d, want 1", len(result.minuteRows))
	}
	if got := result.minuteRows[0].SolarGeneratedWh; got <= 0 || got >= 2 {
		t.Fatalf("unexpected deduped solar_generated_wh = %v", got)
	}
}

type runnerTestManifestStore struct {
	deviceObjects []replaycli.ManifestObject
}

func (f *runnerTestManifestStore) ListByDevices(_ context.Context, _ replaycli.DeviceQuery) ([]replaycli.ManifestObject, error) {
	return append([]replaycli.ManifestObject(nil), f.deviceObjects...), nil
}

func (f *runnerTestManifestStore) ListByFleetRange(_ context.Context, _ replaycli.FleetQuery) ([]replaycli.ManifestObject, error) {
	return nil, nil
}

func (f *runnerTestManifestStore) Close() error {
	return nil
}

type runnerTestObjectReader struct {
	err  error
	body []byte
}

func (f *runnerTestObjectReader) ReadObject(_ context.Context, _, _ string) ([]byte, error) {
	if f.err == nil {
		return append([]byte(nil), f.body...), nil
	}
	return nil, f.err
}

func (f *runnerTestObjectReader) Close() error {
	return nil
}

func testEnvelopeFrame(t *testing.T, env *envelopev1.TelemetryEnvelope) []byte {
	t.Helper()
	frame, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return frame
}

func encodeRebuildFrames(t *testing.T, frames ...[]byte) []byte {
	t.Helper()
	var raw bytes.Buffer
	for _, frame := range frames {
		var sizePrefix [binary.MaxVarintLen64]byte
		n := binary.PutUvarint(sizePrefix[:], uint64(len(frame)))
		if _, err := raw.Write(sizePrefix[:n]); err != nil {
			t.Fatalf("write frame size: %v", err)
		}
		if _, err := raw.Write(frame); err != nil {
			t.Fatalf("write frame body: %v", err)
		}
	}
	var out bytes.Buffer
	encoder, err := zstd.NewWriter(&out)
	if err != nil {
		t.Fatalf("create zstd encoder: %v", err)
	}
	if _, err := encoder.Write(raw.Bytes()); err != nil {
		t.Fatalf("write zstd payload: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("close zstd encoder: %v", err)
	}
	return out.Bytes()
}
