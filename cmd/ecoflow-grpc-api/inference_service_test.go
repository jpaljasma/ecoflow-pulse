package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	inferencev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/inference/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/inference"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeInferenceReader struct {
	deviceInsights inference.DeviceInsights
	fleetInsights  []inference.DeviceInsights
	lastGetFilter  inference.Filter
	lastListFilter inference.Filter
	lastListIDs    []string
}

func (r *fakeInferenceReader) GetDeviceInsights(_ context.Context, deviceID string, filter inference.Filter) (inference.DeviceInsights, error) {
	r.lastGetFilter = filter
	out := r.deviceInsights
	if out.DeviceID == "" {
		out.DeviceID = deviceID
	}
	return out, nil
}

func (r *fakeInferenceReader) ListFleetInsights(_ context.Context, deviceIDs []string, filter inference.Filter) ([]inference.DeviceInsights, error) {
	r.lastListFilter = filter
	r.lastListIDs = append([]string(nil), deviceIDs...)
	out := make([]inference.DeviceInsights, 0, len(r.fleetInsights))
	out = append(out, r.fleetInsights...)
	return out, nil
}

func newTestInferenceService(store controlplane.Store, reader inference.Reader) *InferenceService {
	return NewInferenceService(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		store,
		reader,
	)
}

func newAuthorizedDeviceContext(t *testing.T, subject string) (context.Context, string, *controlplane.MemoryStore) {
	t.Helper()

	store := controlplane.NewMemoryStore()
	store.EnsureUser(subject)
	device, err := store.CreateDevice(context.Background(), controlplane.CreateDeviceInput{
		UserSubject: subject,
		EcoflowSN:   "SN-" + subject,
		ProductName: "DELTA 2 Max",
		Model:       "delta2max",
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: subject})
	return ctx, device.DeviceID, store
}

func TestInferenceGetDeviceInsightsValidation(t *testing.T) {
	t.Parallel()

	svc := newTestInferenceService(nil, nil)
	_, err := svc.GetDeviceInsights(context.Background(), &inferencev1.GetDeviceInsightsRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestInferenceGetDeviceInsightsPendingWithoutReader(t *testing.T) {
	t.Parallel()

	ctx, deviceID, store := newAuthorizedDeviceContext(t, "user-1")
	svc := newTestInferenceService(store, nil)

	resp, err := svc.GetDeviceInsights(ctx, &inferencev1.GetDeviceInsightsRequest{DeviceId: deviceID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.GetInsights().GetStatus(); got != inferencev1.InsightStatus_INSIGHT_STATUS_PENDING {
		t.Fatalf("status mismatch: got=%v want=%v", got, inferencev1.InsightStatus_INSIGHT_STATUS_PENDING)
	}
	if got := resp.GetInsights().GetDeviceId(); got != deviceID {
		t.Fatalf("device id mismatch: got=%q want=%q", got, deviceID)
	}
	if len(resp.GetInsights().GetInsights()) != 0 {
		t.Fatalf("expected no insights, got %d", len(resp.GetInsights().GetInsights()))
	}
}

func TestInferenceGetDeviceInsightsPermissionDenied(t *testing.T) {
	t.Parallel()

	_, deviceID, store := newAuthorizedDeviceContext(t, "owner")
	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "viewer"})
	svc := newTestInferenceService(store, nil)

	_, err := svc.GetDeviceInsights(ctx, &inferencev1.GetDeviceInsightsRequest{DeviceId: deviceID})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestInferenceGetDeviceInsightsUsesReader(t *testing.T) {
	t.Parallel()

	ctx, deviceID, store := newAuthorizedDeviceContext(t, "user-2")
	reader := &fakeInferenceReader{
		deviceInsights: inference.DeviceInsights{
			Status:       inference.StatusReady,
			StatusDetail: "fresh",
			RefreshedAt:  time.UnixMilli(1_741_459_200_000),
			Insights: []inference.Insight{
				{
					ID:           "battery-pack-1",
					DeviceID:     deviceID,
					Kind:         inference.KindBatteryExpansion,
					Title:        "Add an extra battery",
					Summary:      "The device has expansion headroom.",
					Score:        0.92,
					Rank:         1,
					ModelKey:     "battery-expansion",
					ModelVersion: "v1",
					GeneratedAt:  time.UnixMilli(1_741_459_200_000),
					ExpiresAt:    time.UnixMilli(1_741_462_800_000),
					Tags:         []string{"battery", "upsell"},
					Evidence: []inference.Evidence{
						{
							Source:  inference.EvidenceSourceDeviceCapabilities,
							Summary: "Device supports more battery packs.",
							Metrics: map[string]any{"batteryPacks": 1, "maxBatteryPacks": 3},
						},
					},
					Actions: []inference.Action{
						{
							Kind:   inference.ActionKindExternalURL,
							Label:  "Shop battery",
							Target: "https://example.com/battery",
						},
					},
					Attributes: map[string]any{"headroom_packs": 2},
				},
			},
		},
	}
	svc := newTestInferenceService(store, reader)

	resp, err := svc.GetDeviceInsights(ctx, &inferencev1.GetDeviceInsightsRequest{
		DeviceId: deviceID,
		Filter: &inferencev1.InsightFilter{
			Kinds:    []inferencev1.InsightKind{inferencev1.InsightKind_INSIGHT_KIND_BATTERY_EXPANSION},
			MaxItems: 4,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.lastGetFilter.MaxItems != 4 {
		t.Fatalf("max_items mismatch: got=%d want=4", reader.lastGetFilter.MaxItems)
	}
	if len(reader.lastGetFilter.Kinds) != 1 || reader.lastGetFilter.Kinds[0] != inference.KindBatteryExpansion {
		t.Fatalf("unexpected filter kinds: %#v", reader.lastGetFilter.Kinds)
	}
	if got := resp.GetInsights().GetStatus(); got != inferencev1.InsightStatus_INSIGHT_STATUS_READY {
		t.Fatalf("status mismatch: got=%v want=%v", got, inferencev1.InsightStatus_INSIGHT_STATUS_READY)
	}
	if got := len(resp.GetInsights().GetInsights()); got != 1 {
		t.Fatalf("insight count mismatch: got=%d want=1", got)
	}
	insight := resp.GetInsights().GetInsights()[0]
	if got := insight.GetKind(); got != inferencev1.InsightKind_INSIGHT_KIND_BATTERY_EXPANSION {
		t.Fatalf("kind mismatch: got=%v want=%v", got, inferencev1.InsightKind_INSIGHT_KIND_BATTERY_EXPANSION)
	}
	if got := insight.GetEvidence()[0].GetSource(); got != inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_DEVICE_CAPABILITIES {
		t.Fatalf("evidence source mismatch: got=%v", got)
	}
	if got := insight.GetActions()[0].GetKind(); got != inferencev1.InsightActionKind_INSIGHT_ACTION_KIND_EXTERNAL_URL {
		t.Fatalf("action kind mismatch: got=%v", got)
	}
}

func TestInferenceListFleetInsightsUsesVisibleDevicesWhenRequestOmitted(t *testing.T) {
	t.Parallel()

	store := controlplane.NewMemoryStore()
	store.EnsureUser("fleet-user")
	first, err := store.CreateDevice(context.Background(), controlplane.CreateDeviceInput{
		UserSubject: "fleet-user",
		EcoflowSN:   "SN-FLEET-1",
		ProductName: "DELTA Pro Ultra",
		Model:       "dpu",
	})
	if err != nil {
		t.Fatalf("create first device: %v", err)
	}
	second, err := store.CreateDevice(context.Background(), controlplane.CreateDeviceInput{
		UserSubject: "fleet-user",
		EcoflowSN:   "SN-FLEET-2",
		ProductName: "DELTA 2 Max",
		Model:       "d2m",
	})
	if err != nil {
		t.Fatalf("create second device: %v", err)
	}

	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "fleet-user"})
	svc := newTestInferenceService(store, nil)
	resp, err := svc.ListFleetInsights(ctx, &inferencev1.ListFleetInsightsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(resp.GetDevices()); got != 2 {
		t.Fatalf("device count mismatch: got=%d want=2", got)
	}
	gotIDs := map[string]struct{}{}
	for _, device := range resp.GetDevices() {
		gotIDs[device.GetDeviceId()] = struct{}{}
	}
	if _, ok := gotIDs[first.DeviceID]; !ok {
		t.Fatalf("missing first device %q in response", first.DeviceID)
	}
	if _, ok := gotIDs[second.DeviceID]; !ok {
		t.Fatalf("missing second device %q in response", second.DeviceID)
	}
}

func TestInferenceListFleetInsightsRequiresDeviceIDsWithoutUserContext(t *testing.T) {
	t.Parallel()

	svc := newTestInferenceService(controlplane.NewMemoryStore(), nil)
	_, err := svc.ListFleetInsights(context.Background(), &inferencev1.ListFleetInsightsRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestInferenceListFleetInsightsRejectsUnsupportedKinds(t *testing.T) {
	t.Parallel()

	ctx, _, store := newAuthorizedDeviceContext(t, "user-3")
	svc := newTestInferenceService(store, nil)

	_, err := svc.ListFleetInsights(ctx, &inferencev1.ListFleetInsightsRequest{
		DeviceIds: []string{"dev-1"},
		Filter: &inferencev1.InsightFilter{
			Kinds: []inferencev1.InsightKind{inferencev1.InsightKind_INSIGHT_KIND_UNSPECIFIED},
		},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}
