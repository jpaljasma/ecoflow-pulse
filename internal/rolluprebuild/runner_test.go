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
	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
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

func TestProcessObjectGroupPrefersLatestReplayEnvelopeForHistoricalSample(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC)
	deviceID := "019cec1d-9a84-7e55-8018-27353cbc79da"
	envOld := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-old",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "PULSEDPUX24K001",
		ObservedTimeUnixMs: base.UnixMilli(),
		DeviceTimeUnixMs:   base.UnixMilli(),
		IngestedTimeUnixMs: base.UnixMilli(),
		Labels: map[string]string{
			"provider":           "pulsemqtt",
			"provider_device_id": "PULSEDPUX24K001",
		},
		Payload:         []byte(`{"typeCode":"quota","addr":"hs_yj751_pd_appshow_addr","cmdId":1,"cmdFunc":2,"params":{"inHvMpptPwr":60}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		SourceKind:      envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA,
	})
	envReplay := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-replay",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "PULSEDPUX24K001",
		MessageId:          "pulse-replay-1",
		ObservedTimeUnixMs: base.UnixMilli(),
		DeviceTimeUnixMs:   base.UnixMilli(),
		IngestedTimeUnixMs: base.Add(30 * time.Minute).UnixMilli(),
		Labels: map[string]string{
			"provider":           "pulsemqtt",
			"provider_device_id": "PULSEDPUX24K001",
		},
		Payload:         []byte(`{"typeCode":"quota","addr":"hs_yj751_pd_appshow_addr","cmdId":1,"cmdFunc":2,"params":{"inHvMpptPwr":240}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		SourceKind:      envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA,
	})
	envTail := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-tail",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "PULSEDPUX24K001",
		MessageId:          "pulse-tail-1",
		ObservedTimeUnixMs: base.Add(30 * time.Second).UnixMilli(),
		DeviceTimeUnixMs:   base.Add(30 * time.Second).UnixMilli(),
		IngestedTimeUnixMs: base.Add(30*time.Minute + 30*time.Second).UnixMilli(),
		Labels: map[string]string{
			"provider":           "pulsemqtt",
			"provider_device_id": "PULSEDPUX24K001",
		},
		Payload:         []byte(`{"typeCode":"quota","addr":"hs_yj751_pd_backend_addr","cmdId":1,"cmdFunc":2,"params":{"wattsOutSum":10}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		SourceKind:      envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA,
	})

	runner, err := NewRunner(
		nil,
		&runnerTestManifestStore{},
		&runnerTestObjectReader{bodiesByObjectKey: map[string][]byte{
			"raw/yyyy=2026/mm=04/dd=03/hh=10/shard=001/part-00001.pb.zst": encodeRebuildFrames(t, envOld),
			"raw/yyyy=2026/mm=04/dd=03/hh=19/shard=001/part-00002.pb.zst": encodeRebuildFrames(t, envReplay, envTail),
		}},
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
		orderObjectsForRebuild([]replaycli.ManifestObject{
			{
				ObjectBucket:      "pulse-telemetry-raw",
				ObjectKey:         "raw/yyyy=2026/mm=04/dd=03/hh=10/shard=001/part-00001.pb.zst",
				Provider:          "pulsemqtt",
				ProviderDeviceIDs: []string{"PULSEDPUX24K001"},
				Shard:             1,
				PartitionHour:     time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC),
			},
			{
				ObjectBucket:      "pulse-telemetry-raw",
				ObjectKey:         "raw/yyyy=2026/mm=04/dd=03/hh=19/shard=001/part-00002.pb.zst",
				Provider:          "pulsemqtt",
				ProviderDeviceIDs: []string{"PULSEDPUX24K001"},
				Shard:             1,
				PartitionHour:     time.Date(2026, time.April, 3, 19, 0, 0, 0, time.UTC),
			},
		}),
		&objectsProcessed,
		&missingObjects,
		2,
		&messagesDecoded,
		&messagesApplied,
		&quotaMessages,
		base.Add(time.Minute).UnixMilli(),
	)
	if result.err != nil {
		t.Fatalf("processObjectGroup failed: %v", result.err)
	}
	if result.messagesApplied != 2 {
		t.Fatalf("messagesApplied = %d, want 2 after replay dedupe", result.messagesApplied)
	}
	if len(result.minuteRows) != 1 {
		t.Fatalf("minute row count = %d, want 1", len(result.minuteRows))
	}
	if got := result.minuteRows[0].SolarGeneratedWh; got < 1.9 || got > 2.1 {
		t.Fatalf("solar_generated_wh = %v, want replay-preferred 240W window", got)
	}
}

func TestOrderObjectsForRebuildPreservesCrossShardSolarContinuity(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 15, 13, 29, 0, 0, time.UTC)
	objects := []replaycli.ManifestObject{
		{
			ObjectBucket: "pulse-telemetry-raw",
			ObjectKey:    "raw/yyyy=2026/mm=03/dd=15/hh=13/shard=043/part-00002.pb.zst",
			Shard:        43,
			TSMinUnixMS:  base.Add(5 * time.Minute).UnixMilli(),
			TSMaxUnixMS:  base.Add(10 * time.Minute).UnixMilli(),
		},
		{
			ObjectBucket: "pulse-telemetry-raw",
			ObjectKey:    "raw/yyyy=2026/mm=03/dd=15/hh=13/shard=039/part-00001.pb.zst",
			Shard:        39,
			TSMinUnixMS:  base.UnixMilli(),
			TSMaxUnixMS:  base.Add(5 * time.Minute).UnixMilli(),
		},
	}

	got := orderObjectsForRebuild(objects)
	if len(got) != 2 {
		t.Fatalf("ordered object count = %d, want 2", len(got))
	}
	if got[0].ObjectKey != objects[1].ObjectKey {
		t.Fatalf("first object = %s, want earliest object %s", got[0].ObjectKey, objects[1].ObjectKey)
	}
	if got[1].ObjectKey != objects[0].ObjectKey {
		t.Fatalf("second object = %s, want latest object %s", got[1].ObjectKey, objects[0].ObjectKey)
	}
}

func TestProcessObjectGroupCarriesSolarAcrossArchiveObjectBoundaries(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 15, 13, 29, 30, 0, time.UTC)
	deviceID := "019cec1d-9a84-7e55-8018-27353cbc79da"
	envA := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-a",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "Y711ZABA9H2P0294",
		MessageId:          "msg-a",
		ObservedTimeUnixMs: base.UnixMilli(),
		DeviceTimeUnixMs:   base.UnixMilli(),
		IngestedTimeUnixMs: base.UnixMilli(),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "Y711ZABA9H2P0294",
		},
		Payload:         []byte(`{"params":{"inLvMpptPwr":240}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
	})
	envB := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-b",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "Y711ZABA9H2P0294",
		MessageId:          "msg-b",
		ObservedTimeUnixMs: base.Add(10 * time.Minute).UnixMilli(),
		DeviceTimeUnixMs:   base.Add(10 * time.Minute).UnixMilli(),
		IngestedTimeUnixMs: base.Add(10 * time.Minute).UnixMilli(),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "Y711ZABA9H2P0294",
		},
		Payload:         []byte(`{"params":{"wattsOutSum":10}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
	})

	objects := orderObjectsForRebuild([]replaycli.ManifestObject{
		{
			ObjectBucket:      "pulse-telemetry-raw",
			ObjectKey:         "raw/yyyy=2026/mm=03/dd=15/hh=13/shard=043/part-00002.pb.zst",
			Provider:          "ecoflow",
			ProviderDeviceIDs: []string{"Y711ZABA9H2P0294"},
			Shard:             43,
			TSMinUnixMS:       envBObserved(envB),
			TSMaxUnixMS:       envBObserved(envB),
		},
		{
			ObjectBucket:      "pulse-telemetry-raw",
			ObjectKey:         "raw/yyyy=2026/mm=03/dd=15/hh=13/shard=039/part-00001.pb.zst",
			Provider:          "ecoflow",
			ProviderDeviceIDs: []string{"Y711ZABA9H2P0294"},
			Shard:             39,
			TSMinUnixMS:       envBObserved(envA),
			TSMaxUnixMS:       envBObserved(envA),
		},
	})
	reader := &runnerTestObjectReader{
		bodiesByObjectKey: map[string][]byte{
			objects[0].ObjectKey: encodeRebuildFrames(t, envA),
			objects[1].ObjectKey: encodeRebuildFrames(t, envB),
		},
	}
	runner, err := NewRunner(nil, &runnerTestManifestStore{}, reader, &PostgresWriter{}, 100, 1)
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
		objects,
		&objectsProcessed,
		&missingObjects,
		len(objects),
		&messagesDecoded,
		&messagesApplied,
		&quotaMessages,
		base.Add(10*time.Minute).UnixMilli(),
	)
	if result.err != nil {
		t.Fatalf("processObjectGroup failed: %v", result.err)
	}
	if len(result.minuteRows) == 0 {
		t.Fatalf("expected minute rows from cross-object replay")
	}
	var found bool
	for _, row := range result.minuteRows {
		if row.BucketStart.Equal(time.Date(2026, time.March, 15, 13, 39, 0, 0, time.UTC)) {
			found = true
			if row.SolarGeneratedWh <= 0 {
				t.Fatalf("expected carried solar_generated_wh in %s, got %v", row.BucketStart.Format(time.RFC3339), row.SolarGeneratedWh)
			}
		}
	}
	if !found {
		t.Fatalf("expected minute row for carried solar bucket")
	}
}

func TestProcessObjectGroupDoesNotSynthesizeTailBucketsWithoutLookaheadSample(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 15, 13, 29, 30, 0, time.UTC)
	deviceID := "019cec1d-9a84-7e55-8018-27353cbc79da"
	env := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-tail",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "Y711ZABA9H2P0294",
		MessageId:          "msg-tail",
		ObservedTimeUnixMs: base.UnixMilli(),
		DeviceTimeUnixMs:   base.UnixMilli(),
		IngestedTimeUnixMs: base.UnixMilli(),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "Y711ZABA9H2P0294",
		},
		Payload:         []byte(`{"params":{"inLvMpptPwr":240}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
	})

	runner, err := NewRunner(
		nil,
		&runnerTestManifestStore{},
		&runnerTestObjectReader{body: encodeRebuildFrames(t, env)},
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
			ObjectKey:         "raw/yyyy=2026/mm=03/dd=15/hh=13/shard=039/part-00001.pb.zst",
			Provider:          "ecoflow",
			ProviderDeviceIDs: []string{"Y711ZABA9H2P0294"},
			Shard:             39,
			TSMinUnixMS:       base.UnixMilli(),
			TSMaxUnixMS:       base.UnixMilli(),
		}},
		&objectsProcessed,
		&missingObjects,
		1,
		&messagesDecoded,
		&messagesApplied,
		&quotaMessages,
		base.Add(10*time.Minute).UnixMilli(),
	)
	if result.err != nil {
		t.Fatalf("processObjectGroup failed: %v", result.err)
	}
	if len(result.minuteRows) != 1 {
		t.Fatalf("expected only the real sample bucket, got %d rows", len(result.minuteRows))
	}
	if !result.minuteRows[0].BucketStart.Equal(time.Date(2026, time.March, 15, 13, 29, 0, 0, time.UTC)) {
		t.Fatalf("unexpected bucket start: %s", result.minuteRows[0].BucketStart.Format(time.RFC3339))
	}
}

func TestProcessObjectGroupSortsOverlappingObjectSamplesByIngestedArrival(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 15, 13, 29, 30, 0, time.UTC)
	deviceID := "019cec1d-9a84-7e55-8018-27353cbc79da"
	late := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-late",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "Y711ZABA9H2P0294",
		MessageId:          "msg-late",
		ObservedTimeUnixMs: base.Add(10 * time.Minute).UnixMilli(),
		DeviceTimeUnixMs:   base.Add(10 * time.Minute).UnixMilli(),
		IngestedTimeUnixMs: base.Add(10 * time.Minute).UnixMilli(),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "Y711ZABA9H2P0294",
		},
		Payload:         []byte(`{"params":{"wattsOutSum":10}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
	})
	early := testEnvelopeFrame(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-early",
		EnvelopeVersion:    1,
		DeviceId:           deviceID,
		EcoflowSn:          "Y711ZABA9H2P0294",
		MessageId:          "msg-early",
		ObservedTimeUnixMs: base.UnixMilli(),
		DeviceTimeUnixMs:   base.UnixMilli(),
		IngestedTimeUnixMs: base.UnixMilli(),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "Y711ZABA9H2P0294",
		},
		Payload:         []byte(`{"params":{"inLvMpptPwr":240}}`),
		PayloadType:     "ecoflow.mqtt.raw",
		PayloadVersion:  1,
		PayloadEncoding: envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
	})

	objects := []replaycli.ManifestObject{
		{
			ObjectBucket:      "pulse-telemetry-raw",
			ObjectKey:         "raw/yyyy=2026/mm=03/dd=15/hh=13/shard=039/part-00001.pb.zst",
			Provider:          "ecoflow",
			ProviderDeviceIDs: []string{"Y711ZABA9H2P0294"},
			Shard:             39,
			TSMinUnixMS:       base.UnixMilli(),
			TSMaxUnixMS:       base.Add(10 * time.Minute).UnixMilli(),
		},
		{
			ObjectBucket:      "pulse-telemetry-raw",
			ObjectKey:         "raw/yyyy=2026/mm=03/dd=15/hh=13/shard=043/part-00002.pb.zst",
			Provider:          "ecoflow",
			ProviderDeviceIDs: []string{"Y711ZABA9H2P0294"},
			Shard:             43,
			TSMinUnixMS:       base.Add(time.Minute).UnixMilli(),
			TSMaxUnixMS:       base.Add(10 * time.Minute).UnixMilli(),
		},
	}
	reader := &runnerTestObjectReader{
		bodiesByObjectKey: map[string][]byte{
			objects[0].ObjectKey: encodeRebuildFrames(t, late),
			objects[1].ObjectKey: encodeRebuildFrames(t, early),
		},
	}
	runner, err := NewRunner(nil, &runnerTestManifestStore{}, reader, &PostgresWriter{}, 100, 1)
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
		objects,
		&objectsProcessed,
		&missingObjects,
		len(objects),
		&messagesDecoded,
		&messagesApplied,
		&quotaMessages,
		base.Add(10*time.Minute).UnixMilli(),
	)
	if result.err != nil {
		t.Fatalf("processObjectGroup failed: %v", result.err)
	}
	var found bool
	for _, row := range result.minuteRows {
		if row.BucketStart.Equal(time.Date(2026, time.March, 15, 13, 39, 0, 0, time.UTC)) {
			found = true
			if row.SolarGeneratedWh <= 0 {
				t.Fatalf("expected overlapping objects to preserve carried solar in %s, got %v", row.BucketStart.Format(time.RFC3339), row.SolarGeneratedWh)
			}
		}
	}
	if !found {
		t.Fatalf("expected minute row for overlapping-object carried solar bucket")
	}
}

func TestRebuildSampleLessUsesIngestedArrivalOrderBeforeObservedTime(t *testing.T) {
	t.Parallel()

	left := &rollupworker.RollupSample{
		Provider:         "ecoflow",
		ProviderDeviceID: "Y711ZABA9H2P0294",
		DeviceID:         "019cec1d-9a84-7e55-8018-27353cbc79da",
		EventUnixMs:      1000,
		IngestedUnixMs:   2000,
	}
	right := &rollupworker.RollupSample{
		Provider:         "ecoflow",
		ProviderDeviceID: "Y711ZABA9H2P0294",
		DeviceID:         "019cec1d-9a84-7e55-8018-27353cbc79da",
		EventUnixMs:      2000,
		IngestedUnixMs:   1000,
	}

	if !rebuildSampleLess(right, left) {
		t.Fatalf("expected earlier ingested sample to sort first")
	}
	if rebuildSampleLess(left, right) {
		t.Fatalf("did not expect later ingested sample to sort first")
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
	err               error
	body              []byte
	bodiesByObjectKey map[string][]byte
}

func (f *runnerTestObjectReader) ReadObject(_ context.Context, _, key string) ([]byte, error) {
	if f.err == nil {
		if len(f.bodiesByObjectKey) > 0 {
			body, ok := f.bodiesByObjectKey[key]
			if !ok {
				return nil, errors.New("object body not found for key")
			}
			return append([]byte(nil), body...), nil
		}
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

func envBObserved(frame []byte) int64 {
	var env envelopev1.TelemetryEnvelope
	if err := proto.Unmarshal(frame, &env); err != nil {
		return 0
	}
	return env.GetObservedTimeUnixMs()
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
