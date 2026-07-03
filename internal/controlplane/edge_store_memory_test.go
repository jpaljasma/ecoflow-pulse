package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryStoreEdgeCollectorEnrollmentAndHeartbeat(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("user-1")

	collector, err := store.CreateEdgeCollector(context.Background(), CreateEdgeCollectorInput{
		UserSubject:    "user-1",
		DisplayName:    "Pi 5",
		SetupTokenHash: "setup-hash",
	})
	if err != nil {
		t.Fatalf("create edge collector: %v", err)
	}
	if collector.IsActive {
		t.Fatalf("collector should start inactive")
	}
	pending, err := store.GetEdgeCollectorBySetupTokenHash(context.Background(), "setup-hash")
	if err != nil {
		t.Fatalf("get pending edge collector: %v", err)
	}
	if pending.ID != collector.ID || pending.UserID == "" {
		t.Fatalf("unexpected pending collector: %+v", pending)
	}

	if _, err := store.AuthenticateEdgeCollector(context.Background(), AuthenticateEdgeCollectorInput{CollectorSecretHash: "secret-hash"}); !errors.Is(err, ErrEdgeCollectorNotFound) {
		t.Fatalf("auth before enrollment error=%v want not found", err)
	}

	enrolled, err := store.EnrollEdgeCollector(context.Background(), EnrollEdgeCollectorInput{
		SetupTokenHash:      "setup-hash",
		CollectorSecretHash: "secret-hash",
		CollectorVersion:    "v0.1.0",
		Hostname:            "pulse-edge-pi",
	})
	if err != nil {
		t.Fatalf("enroll edge collector: %v", err)
	}
	if !enrolled.IsActive || enrolled.SetupTokenHash != "" || enrolled.CollectorSecretHash == "" {
		t.Fatalf("unexpected enrolled collector: %+v", enrolled)
	}
	if _, err := store.GetEdgeCollectorBySetupTokenHash(context.Background(), "setup-hash"); !errors.Is(err, ErrEdgeCollectorNotFound) {
		t.Fatalf("get pending after enrollment error=%v want not found", err)
	}

	authed, err := store.AuthenticateEdgeCollector(context.Background(), AuthenticateEdgeCollectorInput{CollectorSecretHash: "secret-hash"})
	if err != nil {
		t.Fatalf("authenticate edge collector: %v", err)
	}
	if authed.ID != collector.ID {
		t.Fatalf("auth collector id=%q want %q", authed.ID, collector.ID)
	}

	heartbeat, err := store.UpdateEdgeCollectorHeartbeat(context.Background(), UpdateEdgeCollectorHeartbeatInput{
		CollectorID:      collector.ID,
		CollectorVersion: "v0.1.1",
	})
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if heartbeat.CollectorVersion != "v0.1.1" || heartbeat.LastHeartbeatAt.IsZero() {
		t.Fatalf("heartbeat did not update collector: %+v", heartbeat)
	}
}

func TestMemoryStoreEdgeDiscoveryRequiresApprovalBeforeLinkedLookup(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("user-1")
	collector, err := store.CreateEdgeCollector(context.Background(), CreateEdgeCollectorInput{
		UserSubject:    "user-1",
		SetupTokenHash: "setup-hash",
	})
	if err != nil {
		t.Fatalf("create collector: %v", err)
	}
	collector, err = store.EnrollEdgeCollector(context.Background(), EnrollEdgeCollectorInput{
		SetupTokenHash:      "setup-hash",
		CollectorSecretHash: "secret-hash",
	})
	if err != nil {
		t.Fatalf("enroll collector: %v", err)
	}

	source, err := store.UpsertEdgeDeviceSource(context.Background(), UpsertEdgeDeviceSourceInput{
		CollectorID:      collector.ID,
		Provider:         ProviderEcoFlow,
		Transport:        "ble",
		ProviderDeviceID: "DEMOEDGE0001",
		DisplayName:      "Demo edge device",
		Model:            "EcoFlow RIVER 3 Plus",
		RSSIDBm:          -59,
		ObservedAt:       time.Date(2026, 5, 28, 13, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert edge source: %v", err)
	}
	if source.Status != "pending" {
		t.Fatalf("status=%q want pending", source.Status)
	}
	if _, err := store.GetLinkedEdgeDeviceSource(context.Background(), GetLinkedEdgeDeviceSourceInput{
		CollectorID:      collector.ID,
		Provider:         ProviderEcoFlow,
		Transport:        "ble",
		ProviderDeviceID: "DEMOEDGE0001",
	}); !errors.Is(err, ErrEdgeDeviceSourceNotFound) {
		t.Fatalf("linked lookup before approval error=%v want source not found", err)
	}

	approved, err := store.ApproveEdgeDeviceSource(context.Background(), ApproveEdgeDeviceSourceInput{
		UserSubject: "user-1",
		SourceID:    source.ID,
	})
	if err != nil {
		t.Fatalf("approve source: %v", err)
	}
	if approved.Source.Status != "linked" || approved.Source.LinkedDeviceID == "" {
		t.Fatalf("unexpected approved source: %+v", approved.Source)
	}
	if approved.Device.EcoflowSN != "DEMOEDGE0001" {
		t.Fatalf("device sn=%q", approved.Device.EcoflowSN)
	}

	linked, err := store.GetLinkedEdgeDeviceSource(context.Background(), GetLinkedEdgeDeviceSourceInput{
		CollectorID:      collector.ID,
		Provider:         ProviderEcoFlow,
		Transport:        "ble",
		ProviderDeviceID: "DEMOEDGE0001",
	})
	if err != nil {
		t.Fatalf("linked lookup: %v", err)
	}
	if linked.LinkedDeviceID != approved.Device.DeviceID {
		t.Fatalf("linked device id=%q want %q", linked.LinkedDeviceID, approved.Device.DeviceID)
	}
}
