package archiveworker

import (
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/proto"
)

func TestProcessDeliveryFlushByMaxRecords(t *testing.T) {
	t.Parallel()

	store := &fakeObjectStore{}
	now := time.Date(2026, time.February, 24, 18, 0, 0, 0, time.UTC)
	worker := newTestWorker(store, now)
	worker.cfg.MaxRecordsPerPart = 2
	worker.cfg.FlushInterval = 5 * time.Minute

	d1 := newFakeDelivery(t, envelope(1, "env-1", 1000))
	d2 := newFakeDelivery(t, envelope(1, "env-2", 2000))
	if err := worker.processDelivery(context.Background(), d1); err != nil {
		t.Fatalf("process first delivery failed: %v", err)
	}
	if err := worker.processDelivery(context.Background(), d2); err != nil {
		t.Fatalf("process second delivery failed: %v", err)
	}

	if len(store.requests) != 1 {
		t.Fatalf("archive object write count mismatch: got=%d want=1", len(store.requests))
	}
	req := store.requests[0]
	if !strings.Contains(req.Key, "/shard=001/") {
		t.Fatalf("unexpected archive key shard path: %s", req.Key)
	}
	ids := decodeEnvelopeIDs(t, req.Body)
	if len(ids) != 2 || ids[0] != "env-1" || ids[1] != "env-2" {
		t.Fatalf("decoded envelope ids mismatch: got=%v", ids)
	}
	if d1.acked != 1 || d2.acked != 1 {
		t.Fatalf("expected both deliveries acked, got d1=%d d2=%d", d1.acked, d2.acked)
	}
	if d1.nacked != 0 || d2.nacked != 0 {
		t.Fatalf("unexpected nack count d1=%d d2=%d", d1.nacked, d2.nacked)
	}
}

func TestDefaultConfigFailureAlertDefaults(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.FailureAlertWindow != 10*time.Minute {
		t.Fatalf("failure alert window mismatch: got=%s want=10m", cfg.FailureAlertWindow)
	}
	if cfg.FailureAlertThreshold != 6 {
		t.Fatalf("failure alert threshold mismatch: got=%d want=6", cfg.FailureAlertThreshold)
	}
	if cfg.FailureAlertCooldown != 5*time.Minute {
		t.Fatalf("failure alert cooldown mismatch: got=%s want=5m", cfg.FailureAlertCooldown)
	}
}

func TestProcessDeliveryFlushByMaxRecordsWritesManifest(t *testing.T) {
	t.Parallel()

	store := &fakeObjectStore{}
	manifest := &fakeManifestStore{}
	now := time.Date(2026, time.February, 24, 18, 0, 0, 0, time.UTC)
	worker := newTestWorker(store, now)
	worker.manifestStore = manifest
	worker.cfg.MaxRecordsPerPart = 2
	worker.cfg.FlushInterval = 5 * time.Minute

	env1 := envelope(1, "env-1", 1000)
	env1.Labels = map[string]string{
		"provider": "ecoflow",
	}
	env2 := envelope(1, "env-2", 2000)
	env2.Labels = map[string]string{
		"provider": "ecoflow",
	}
	d1 := newFakeDelivery(t, env1)
	d2 := newFakeDelivery(t, env2)
	if err := worker.processDelivery(context.Background(), d1); err != nil {
		t.Fatalf("process first delivery failed: %v", err)
	}
	if err := worker.processDelivery(context.Background(), d2); err != nil {
		t.Fatalf("process second delivery failed: %v", err)
	}

	if len(manifest.records) != 1 {
		t.Fatalf("manifest upsert count mismatch: got=%d want=1", len(manifest.records))
	}
	record := manifest.records[0]
	if record.Provider != "ecoflow" {
		t.Fatalf("manifest provider mismatch: got=%q want=ecoflow", record.Provider)
	}
	if record.RecordCount != 2 {
		t.Fatalf("manifest record count mismatch: got=%d want=2", record.RecordCount)
	}
	if len(record.DeviceIDs) != 1 || record.DeviceIDs[0] != "device-1" {
		t.Fatalf("manifest device ids mismatch: got=%v", record.DeviceIDs)
	}
	if len(record.ProviderDeviceIDs) != 1 || record.ProviderDeviceIDs[0] != "R351ZABAPH331057" {
		t.Fatalf("manifest provider device ids mismatch: got=%v", record.ProviderDeviceIDs)
	}
}

func TestProcessDeliveryInvalidEnvelopeTerms(t *testing.T) {
	t.Parallel()

	store := &fakeObjectStore{}
	worker := newTestWorker(store, time.Unix(1, 0).UTC())
	d := &fakeDelivery{
		subject: "pulse.telemetry.ingest.s001",
		data:    []byte("not-protobuf"),
	}

	if err := worker.processDelivery(context.Background(), d); err == nil {
		t.Fatalf("expected error for invalid envelope")
	}
	if d.termed != 1 {
		t.Fatalf("expected invalid envelope to be terminated once, got=%d", d.termed)
	}
	if d.acked != 0 || d.nacked != 0 {
		t.Fatalf("unexpected ack/nak for invalid envelope: ack=%d nak=%d", d.acked, d.nacked)
	}
}

func TestProcessDeliverySkipsNormalizedQuotaEnvelope(t *testing.T) {
	t.Parallel()

	store := &fakeObjectStore{}
	manifest := &fakeManifestStore{}
	worker := newTestWorker(store, time.Unix(10, 0).UTC())
	worker.manifestStore = manifest
	d := newFakeDelivery(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "quota-1",
		DeviceId:           "device-1",
		EcoflowSn:          "R351ZABAPH331057",
		Shard:              1,
		ShardCount:         128,
		IngestedTimeUnixMs: 1000,
		Source:             "quota",
		PayloadType:        "ecoflow.quota.normalized",
		PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		Payload:            []byte(`{"typeCode":"quota","params":{"soc":42}}`),
	})

	if err := worker.processDelivery(context.Background(), d); err != nil {
		t.Fatalf("process quota delivery failed: %v", err)
	}
	if d.acked != 1 {
		t.Fatalf("expected quota delivery acked once, got=%d", d.acked)
	}
	if d.nacked != 0 || d.termed != 0 {
		t.Fatalf("unexpected nack/term counts: nak=%d term=%d", d.nacked, d.termed)
	}
	if len(store.requests) != 0 {
		t.Fatalf("expected no archive writes for quota delivery, got=%d", len(store.requests))
	}
	if len(manifest.records) != 0 {
		t.Fatalf("expected no manifest writes for quota delivery, got=%d", len(manifest.records))
	}
}

func TestProcessDeliveryStoreFailureNacks(t *testing.T) {
	t.Parallel()

	store := &fakeObjectStore{err: errors.New("write failed")}
	worker := newTestWorker(store, time.Unix(10, 0).UTC())
	worker.cfg.MaxRecordsPerPart = 1
	d := newFakeDelivery(t, envelope(2, "env-err", 1500))

	if err := worker.processDelivery(context.Background(), d); err == nil {
		t.Fatalf("expected process error when object store write fails")
	}
	if d.nacked != 1 {
		t.Fatalf("expected one nack, got=%d", d.nacked)
	}
	if d.acked != 0 {
		t.Fatalf("unexpected ack count=%d", d.acked)
	}
	if len(worker.segments) != 0 {
		t.Fatalf("segments should be cleared after failed flush, got=%d", len(worker.segments))
	}
}

func TestProcessDeliveryManifestFailureNacks(t *testing.T) {
	t.Parallel()

	store := &fakeObjectStore{}
	manifest := &fakeManifestStore{err: errors.New("manifest write failed")}
	worker := newTestWorker(store, time.Unix(10, 0).UTC())
	worker.manifestStore = manifest
	worker.cfg.MaxRecordsPerPart = 1
	env := envelope(2, "env-err", 1500)
	env.Labels = map[string]string{
		"provider": "ecoflow",
	}
	d := newFakeDelivery(t, env)

	if err := worker.processDelivery(context.Background(), d); err == nil {
		t.Fatalf("expected process error when manifest write fails")
	}
	if d.nacked != 1 {
		t.Fatalf("expected one nack, got=%d", d.nacked)
	}
	if d.acked != 0 {
		t.Fatalf("unexpected ack count=%d", d.acked)
	}
	if len(store.requests) != 1 {
		t.Fatalf("expected one object write before manifest failure, got=%d", len(store.requests))
	}
	if len(manifest.records) != 1 {
		t.Fatalf("expected one manifest attempt, got=%d", len(manifest.records))
	}
	if len(worker.segments) != 0 {
		t.Fatalf("segments should be cleared after failed manifest flush, got=%d", len(worker.segments))
	}
}

func TestFlushDueByInterval(t *testing.T) {
	t.Parallel()

	store := &fakeObjectStore{}
	now := time.Date(2026, time.February, 24, 18, 0, 0, 0, time.UTC)
	worker := newTestWorker(store, now)
	worker.cfg.MaxRecordsPerPart = 10
	worker.cfg.FlushInterval = 30 * time.Second
	d := newFakeDelivery(t, envelope(3, "env-flush", now.UnixMilli()))

	if err := worker.processDelivery(context.Background(), d); err != nil {
		t.Fatalf("process delivery failed: %v", err)
	}
	if d.acked != 0 {
		t.Fatalf("delivery should not be acked before interval flush")
	}

	now = now.Add(31 * time.Second)
	worker.nowFn = func() time.Time { return now }
	if err := worker.flushDue(context.Background()); err != nil {
		t.Fatalf("flushDue failed: %v", err)
	}
	if d.acked != 1 {
		t.Fatalf("delivery should be acked after interval flush, got=%d", d.acked)
	}
	if len(store.requests) != 1 {
		t.Fatalf("expected one stored object after interval flush, got=%d", len(store.requests))
	}
}

type fakeObjectStore struct {
	requests []PutObjectRequest
	err      error
}

func (f *fakeObjectStore) PutObject(_ context.Context, req PutObjectRequest) error {
	if f.err != nil {
		return f.err
	}
	f.requests = append(f.requests, PutObjectRequest{
		Bucket:      req.Bucket,
		Key:         req.Key,
		Body:        append([]byte(nil), req.Body...),
		ContentType: req.ContentType,
		Metadata:    copyMetadata(req.Metadata),
	})
	return nil
}

type fakeManifestStore struct {
	records []ManifestRecord
	err     error
}

func (f *fakeManifestStore) UpsertObjectManifest(_ context.Context, record ManifestRecord) error {
	f.records = append(f.records, record)
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeManifestStore) Close() error {
	return nil
}

type fakeDelivery struct {
	subject string
	data    []byte
	acked   int
	nacked  int
	termed  int
}

func (d *fakeDelivery) Subject() string { return d.subject }
func (d *fakeDelivery) Data() []byte    { return d.data }
func (d *fakeDelivery) Ack() error {
	d.acked++
	return nil
}
func (d *fakeDelivery) Nak() error {
	d.nacked++
	return nil
}
func (d *fakeDelivery) Term() error {
	d.termed++
	return nil
}

func newTestWorker(store ObjectStore, now time.Time) *Worker {
	cfg := DefaultConfig().normalized()
	return &Worker{
		log:        slog.Default(),
		store:      store,
		cfg:        cfg,
		nowFn:      func() time.Time { return now },
		segments:   make(map[string]*archiveSegment),
		partCounts: make(map[string]int),
		failureAlerts: newFailureRateTracker(
			cfg.FailureAlertWindow,
			cfg.FailureAlertThreshold,
			cfg.FailureAlertCooldown,
		),
	}
}

func envelope(shard uint32, id string, ingested int64) *envelopev1.TelemetryEnvelope {
	return &envelopev1.TelemetryEnvelope{
		EnvelopeId:         id,
		DeviceId:           "device-1",
		EcoflowSn:          "R351ZABAPH331057",
		Shard:              shard,
		ShardCount:         128,
		IngestedTimeUnixMs: ingested,
		PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		Payload:            []byte(`{"params":{"wattsOutSum":35}}`),
	}
}

func newFakeDelivery(t *testing.T, env *envelopev1.TelemetryEnvelope) *fakeDelivery {
	t.Helper()
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal test envelope failed: %v", err)
	}
	return &fakeDelivery{
		subject: "pulse.telemetry.ingest.s001",
		data:    data,
	}
}

func decodeEnvelopeIDs(t *testing.T, data []byte) []string {
	t.Helper()
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatalf("init zstd decoder failed: %v", err)
	}
	defer decoder.Close()
	raw, err := decoder.DecodeAll(data, nil)
	if err != nil {
		t.Fatalf("decode zstd payload failed: %v", err)
	}
	ids := make([]string, 0, 4)
	for len(raw) > 0 {
		size, n := binary.Uvarint(raw)
		if n <= 0 {
			t.Fatalf("invalid frame length prefix")
		}
		raw = raw[n:]
		if int(size) > len(raw) {
			t.Fatalf("frame size exceeds buffer: size=%d remaining=%d", size, len(raw))
		}
		frame := raw[:size]
		raw = raw[size:]
		var env envelopev1.TelemetryEnvelope
		if err := proto.Unmarshal(frame, &env); err != nil {
			t.Fatalf("unmarshal frame failed: %v", err)
		}
		ids = append(ids, env.GetEnvelopeId())
	}
	return ids
}
