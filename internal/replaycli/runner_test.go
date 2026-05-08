package replaycli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"google.golang.org/protobuf/proto"
)

func TestRunnerReplayDevicesFiltersAndPublishes(t *testing.T) {
	t.Parallel()

	manifest := &fakeManifestStore{
		deviceObjects: []ManifestObject{
			{
				ObjectBucket: "raw",
				ObjectKey:    "obj-1",
				Shard:        7,
			},
		},
	}

	objReader := &fakeObjectReader{
		byPath: map[string][]byte{
			"raw/obj-1": encodeFramedZstdPayload(t, [][]byte{
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "device-a", EcoflowSn: "SN-A", IngestedTimeUnixMs: 100, Shard: 7, ShardCount: 128}),
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "device-b", EcoflowSn: "demod2m00001057", IngestedTimeUnixMs: 200, Shard: 7, ShardCount: 128}),
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "device-c", EcoflowSn: "SN-C", IngestedTimeUnixMs: 300, Shard: 7, ShardCount: 128}),
			}),
		},
	}
	publisher := &fakePublisher{}

	runner, err := NewRunner(slog.Default(), manifest, objReader, publisher)
	if err != nil {
		t.Fatalf("new replay runner: %v", err)
	}
	defer func() { _ = runner.Close() }()

	report, err := runner.ReplayDevices(context.Background(), ReplayRequest{
		FromUnixMS:        1,
		ToUnixMS:          1000,
		DeviceIDs:         []string{"device-a"},
		ProviderDeviceIDs: []string{"demod2m00001057"},
	})
	if err != nil {
		t.Fatalf("replay devices: %v", err)
	}
	if report.ObjectsMatched != 1 || report.ObjectsProcessed != 1 {
		t.Fatalf("object counts mismatch: matched=%d processed=%d", report.ObjectsMatched, report.ObjectsProcessed)
	}
	if report.MessagesDecoded != 3 {
		t.Fatalf("decoded count mismatch: got=%d want=3", report.MessagesDecoded)
	}
	if report.MessagesPublished != 2 {
		t.Fatalf("published count mismatch: got=%d want=2", report.MessagesPublished)
	}
	if report.MessagesFiltered != 1 {
		t.Fatalf("filtered count mismatch: got=%d want=1", report.MessagesFiltered)
	}
	if got := len(publisher.records); got != 2 {
		t.Fatalf("publisher record count mismatch: got=%d want=2", got)
	}
}

func TestRunnerReplayFleetPublishesAll(t *testing.T) {
	t.Parallel()

	manifest := &fakeManifestStore{
		fleetObjects: []ManifestObject{
			{
				ObjectBucket: "raw",
				ObjectKey:    "obj-fleet",
				Shard:        11,
			},
		},
	}
	objReader := &fakeObjectReader{
		byPath: map[string][]byte{
			"raw/obj-fleet": encodeFramedZstdPayload(t, [][]byte{
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "device-a", IngestedTimeUnixMs: 100, Shard: 11, ShardCount: 128}),
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "device-b", IngestedTimeUnixMs: 110, Shard: 11, ShardCount: 128}),
			}),
		},
	}
	publisher := &fakePublisher{}

	runner, err := NewRunner(slog.Default(), manifest, objReader, publisher)
	if err != nil {
		t.Fatalf("new replay runner: %v", err)
	}
	defer func() { _ = runner.Close() }()

	report, err := runner.ReplayFleet(context.Background(), ReplayRequest{
		FromUnixMS: 1,
		ToUnixMS:   1000,
		Shards:     []uint32{11},
	})
	if err != nil {
		t.Fatalf("replay fleet: %v", err)
	}
	if report.MessagesPublished != 2 {
		t.Fatalf("published count mismatch: got=%d want=2", report.MessagesPublished)
	}
	if manifest.lastFleetQuery == nil || len(manifest.lastFleetQuery.Shards) != 1 || manifest.lastFleetQuery.Shards[0] != 11 {
		t.Fatalf("fleet query shard filter mismatch: %+v", manifest.lastFleetQuery)
	}
}

func TestRunnerReplayFleetFiltersFramesToRequestedWindow(t *testing.T) {
	t.Parallel()

	manifest := &fakeManifestStore{
		fleetObjects: []ManifestObject{
			{
				ObjectBucket: "raw",
				ObjectKey:    "obj-window",
				Shard:        11,
			},
		},
	}
	objReader := &fakeObjectReader{
		byPath: map[string][]byte{
			"raw/obj-window": encodeFramedZstdPayload(t, [][]byte{
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "before", IngestedTimeUnixMs: 100, Shard: 11, ShardCount: 128}),
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "inside", IngestedTimeUnixMs: 200, Shard: 11, ShardCount: 128}),
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "after", IngestedTimeUnixMs: 300, Shard: 11, ShardCount: 128}),
			}),
		},
	}
	publisher := &fakePublisher{}

	runner, err := NewRunner(slog.Default(), manifest, objReader, publisher)
	if err != nil {
		t.Fatalf("new replay runner: %v", err)
	}
	defer func() { _ = runner.Close() }()

	report, err := runner.ReplayFleet(context.Background(), ReplayRequest{
		FromUnixMS: 150,
		ToUnixMS:   250,
		Shards:     []uint32{11},
	})
	if err != nil {
		t.Fatalf("replay fleet: %v", err)
	}
	if report.MessagesDecoded != 3 {
		t.Fatalf("decoded count mismatch: got=%d want=3", report.MessagesDecoded)
	}
	if report.MessagesPublished != 1 {
		t.Fatalf("published count mismatch: got=%d want=1", report.MessagesPublished)
	}
	if report.MessagesFiltered != 2 {
		t.Fatalf("filtered count mismatch: got=%d want=2", report.MessagesFiltered)
	}
	if got := len(publisher.records); got != 1 {
		t.Fatalf("publisher record count mismatch: got=%d want=1", got)
	}
	var published envelopev1.TelemetryEnvelope
	if err := proto.Unmarshal(publisher.records[0].payload, &published); err != nil {
		t.Fatalf("unmarshal published envelope: %v", err)
	}
	if published.GetDeviceId() != "inside" {
		t.Fatalf("published device mismatch: got=%q want=%q", published.GetDeviceId(), "inside")
	}
}

func TestRunnerReplayStopsOnPublishFailure(t *testing.T) {
	t.Parallel()

	manifest := &fakeManifestStore{
		fleetObjects: []ManifestObject{{ObjectBucket: "raw", ObjectKey: "obj-fail", Shard: 1}},
	}
	objReader := &fakeObjectReader{
		byPath: map[string][]byte{
			"raw/obj-fail": encodeFramedZstdPayload(t, [][]byte{
				mustEnvelopeBytes(t, &envelopev1.TelemetryEnvelope{DeviceId: "device-a", IngestedTimeUnixMs: 100, Shard: 1, ShardCount: 128}),
			}),
		},
	}
	publisher := &fakePublisher{publishErr: errors.New("publish failed")}

	runner, err := NewRunner(slog.Default(), manifest, objReader, publisher)
	if err != nil {
		t.Fatalf("new replay runner: %v", err)
	}
	defer func() { _ = runner.Close() }()

	report, err := runner.ReplayFleet(context.Background(), ReplayRequest{
		FromUnixMS: 1,
		ToUnixMS:   1000,
	})
	if err == nil {
		t.Fatalf("expected replay error on publish failure")
	}
	if report.MessagesFailed != 1 {
		t.Fatalf("message failed count mismatch: got=%d want=1", report.MessagesFailed)
	}
}

func mustEnvelopeBytes(t *testing.T, env *envelopev1.TelemetryEnvelope) []byte {
	t.Helper()
	b, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return b
}

type fakeManifestStore struct {
	deviceObjects   []ManifestObject
	fleetObjects    []ManifestObject
	lastDeviceQuery *DeviceQuery
	lastFleetQuery  *FleetQuery
}

func (f *fakeManifestStore) ListByDevices(_ context.Context, query DeviceQuery) ([]ManifestObject, error) {
	f.lastDeviceQuery = &query
	return append([]ManifestObject(nil), f.deviceObjects...), nil
}

func (f *fakeManifestStore) ListByFleetRange(_ context.Context, query FleetQuery) ([]ManifestObject, error) {
	f.lastFleetQuery = &query
	return append([]ManifestObject(nil), f.fleetObjects...), nil
}

func (f *fakeManifestStore) Close() error { return nil }

type fakeObjectReader struct {
	byPath map[string][]byte
}

func (f *fakeObjectReader) ReadObject(_ context.Context, bucket string, key string) ([]byte, error) {
	if f.byPath == nil {
		return nil, errors.New("no objects configured")
	}
	value, ok := f.byPath[fmt.Sprintf("%s/%s", bucket, key)]
	if !ok {
		return nil, fmt.Errorf("missing object %s/%s", bucket, key)
	}
	return append([]byte(nil), value...), nil
}

func (f *fakeObjectReader) Close() error { return nil }

type fakePublisher struct {
	mu         sync.Mutex
	records    []publishedRecord
	publishErr error
}

type publishedRecord struct {
	shard   uint32
	payload []byte
}

func (f *fakePublisher) Publish(_ context.Context, shard uint32, payload []byte) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, publishedRecord{
		shard:   shard,
		payload: append([]byte(nil), payload...),
	})
	return nil
}

func (f *fakePublisher) Close() error { return nil }
