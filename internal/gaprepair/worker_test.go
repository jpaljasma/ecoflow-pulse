package gaprepair

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	replayv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/replay/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"google.golang.org/protobuf/proto"
)

type fakeReplayRunner struct {
	report   replaycli.ReplayReport
	err      error
	requests []replaycli.ReplayRequest
}

func (f *fakeReplayRunner) ReplayDevices(_ context.Context, request replaycli.ReplayRequest) (replaycli.ReplayReport, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return replaycli.ReplayReport{}, f.err
	}
	return f.report, nil
}

type fakeGapDelivery struct {
	data    []byte
	ackErr  error
	nakErr  error
	termErr error
	acked   int
	nacked  int
	termed  int
}

func (d *fakeGapDelivery) Data() []byte { return d.data }
func (d *fakeGapDelivery) Ack() error {
	d.acked++
	return d.ackErr
}
func (d *fakeGapDelivery) Nak() error {
	d.nacked++
	return d.nakErr
}
func (d *fakeGapDelivery) Term() error {
	d.termed++
	return d.termErr
}

func newTestWorker(runner ReplayRunner, cfg WorkerConfig) *Worker {
	return &Worker{log: slog.Default(), runner: runner, cfg: cfg}
}

func marshalGapRepairRequest(t *testing.T, req *replayv1.GapRepairRequest) []byte {
	t.Helper()
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	return data
}

func TestWorkerHandleDeliveryInvalidPayloadTerms(t *testing.T) {
	t.Parallel()

	w := newTestWorker(&fakeReplayRunner{}, DefaultWorkerConfig())
	d := &fakeGapDelivery{data: []byte("not-proto")}
	w.handleDelivery(d)
	if d.termed != 1 || d.acked != 0 || d.nacked != 0 {
		t.Fatalf("unexpected ack state term=%d ack=%d nak=%d", d.termed, d.acked, d.nacked)
	}
}

func TestWorkerHandleDeliveryInvalidFieldsTerms(t *testing.T) {
	t.Parallel()

	w := newTestWorker(&fakeReplayRunner{}, DefaultWorkerConfig())
	req := &replayv1.GapRepairRequest{Provider: "ecoflow", ProviderDeviceId: "R351ZABAPH331057", FromUnixMs: 2000, ToUnixMs: 1000}
	d := &fakeGapDelivery{data: marshalGapRepairRequest(t, req)}
	w.handleDelivery(d)
	if d.termed != 1 || d.acked != 0 || d.nacked != 0 {
		t.Fatalf("unexpected ack state term=%d ack=%d nak=%d", d.termed, d.acked, d.nacked)
	}
}

func TestWorkerHandleDeliveryRunnerErrorNaks(t *testing.T) {
	t.Parallel()

	runner := &fakeReplayRunner{err: errors.New("boom")}
	w := newTestWorker(runner, DefaultWorkerConfig())
	req := &replayv1.GapRepairRequest{Provider: "ecoflow", ProviderDeviceId: "R351ZABAPH331057", FromUnixMs: 1000, ToUnixMs: 2000}
	d := &fakeGapDelivery{data: marshalGapRepairRequest(t, req)}
	w.handleDelivery(d)
	if d.nacked != 1 || d.acked != 0 || d.termed != 0 {
		t.Fatalf("unexpected ack state term=%d ack=%d nak=%d", d.termed, d.acked, d.nacked)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one runner request, got=%d", len(runner.requests))
	}
}

func TestWorkerHandleDeliveryAcksOnSuccess(t *testing.T) {
	t.Parallel()

	runner := &fakeReplayRunner{}
	w := newTestWorker(runner, DefaultWorkerConfig())
	req := &replayv1.GapRepairRequest{
		Provider:         " ecoflow ",
		ProviderDeviceId: " r351zabaph331057 ",
		FromUnixMs:       1000,
		ToUnixMs:         2000,
		MaxObjects:       17,
	}
	d := &fakeGapDelivery{data: marshalGapRepairRequest(t, req)}
	w.handleDelivery(d)
	if d.acked != 1 || d.nacked != 0 || d.termed != 0 {
		t.Fatalf("unexpected ack state term=%d ack=%d nak=%d", d.termed, d.acked, d.nacked)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one runner request, got=%d", len(runner.requests))
	}
	got := runner.requests[0]
	if got.Provider != "ecoflow" {
		t.Fatalf("provider mismatch: got=%s", got.Provider)
	}
	if len(got.ProviderDeviceIDs) != 1 || got.ProviderDeviceIDs[0] != "R351ZABAPH331057" {
		t.Fatalf("provider_device_ids mismatch: %#v", got.ProviderDeviceIDs)
	}
	if got.MaxObjects != 17 {
		t.Fatalf("max objects mismatch: got=%d", got.MaxObjects)
	}
}

func TestWorkerHandleDeliveryUsesDefaultMaxObjects(t *testing.T) {
	t.Parallel()

	runner := &fakeReplayRunner{}
	cfg := DefaultWorkerConfig()
	cfg.DefaultMaxObjects = 42
	w := newTestWorker(runner, cfg)
	req := &replayv1.GapRepairRequest{Provider: "ecoflow", ProviderDeviceId: "R351ZABAPH331057", FromUnixMs: 1000, ToUnixMs: 2000, MaxObjects: 0}
	d := &fakeGapDelivery{data: marshalGapRepairRequest(t, req)}
	w.handleDelivery(d)
	if d.acked != 1 || d.nacked != 0 || d.termed != 0 {
		t.Fatalf("unexpected ack state term=%d ack=%d nak=%d", d.termed, d.acked, d.nacked)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one runner request, got=%d", len(runner.requests))
	}
	if got := runner.requests[0].MaxObjects; got != 42 {
		t.Fatalf("default max objects mismatch: got=%d", got)
	}
}

func TestNormalizeGapRequest(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeGapRequest(&replayv1.GapRepairRequest{
		Provider:         " ecoflow ",
		ProviderDeviceId: " r351zabaph331057 ",
		FromUnixMs:       1000,
		ToUnixMs:         2000,
		MaxObjects:       -1,
	})
	if err != nil {
		t.Fatalf("normalizeGapRequest returned error: %v", err)
	}
	if normalized.GetProvider() != "ecoflow" {
		t.Fatalf("provider mismatch: got=%s", normalized.GetProvider())
	}
	if normalized.GetProviderDeviceId() != "R351ZABAPH331057" {
		t.Fatalf("provider_device_id mismatch: got=%s", normalized.GetProviderDeviceId())
	}
	if normalized.GetMaxObjects() != 0 {
		t.Fatalf("max_objects expected 0, got=%d", normalized.GetMaxObjects())
	}
}
