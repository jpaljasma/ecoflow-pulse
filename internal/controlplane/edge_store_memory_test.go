package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
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
	assertUUIDString(t, collector.ID)
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

func TestMemoryStoreRejectsExpiredEdgeCollectorSetupToken(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	store.now = func() time.Time { return baseTime }
	store.EnsureUser("user-1")
	if _, err := store.CreateEdgeCollector(context.Background(), CreateEdgeCollectorInput{
		UserSubject:    "user-1",
		SetupTokenHash: "setup-hash",
	}); err != nil {
		t.Fatalf("create edge collector: %v", err)
	}

	store.now = func() time.Time { return baseTime.Add(edgeCollectorSetupTokenTTL + time.Second) }
	if _, err := store.GetEdgeCollectorBySetupTokenHash(context.Background(), "setup-hash"); !errors.Is(err, ErrEdgeCollectorNotFound) {
		t.Fatalf("get expired setup token error=%v want not found", err)
	}
	if _, err := store.EnrollEdgeCollector(context.Background(), EnrollEdgeCollectorInput{
		SetupTokenHash:      "setup-hash",
		CollectorSecretHash: "secret-hash",
	}); !errors.Is(err, ErrEdgeCollectorNotFound) {
		t.Fatalf("enroll expired setup token error=%v want not found", err)
	}
}

func TestMemoryStoreRevokesEdgeCollectorSetupToken(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("user-1")
	collector, err := store.CreateEdgeCollector(context.Background(), CreateEdgeCollectorInput{
		UserSubject:    "user-1",
		SetupTokenHash: "setup-hash",
	})
	if err != nil {
		t.Fatalf("create edge collector: %v", err)
	}

	revoked, err := store.RevokeEdgeCollectorSetupToken(context.Background(), RevokeEdgeCollectorSetupTokenInput{
		UserSubject: "user-1",
		CollectorID: collector.ID,
	})
	if err != nil {
		t.Fatalf("revoke edge collector setup token: %v", err)
	}
	if revoked.SetupTokenHash != "" || revoked.IsActive {
		t.Fatalf("unexpected revoked collector: %+v", revoked)
	}
	if _, err := store.GetEdgeCollectorBySetupTokenHash(context.Background(), "setup-hash"); !errors.Is(err, ErrEdgeCollectorNotFound) {
		t.Fatalf("get revoked setup token error=%v want not found", err)
	}
	if _, err := store.EnrollEdgeCollector(context.Background(), EnrollEdgeCollectorInput{
		SetupTokenHash:      "setup-hash",
		CollectorSecretHash: "secret-hash",
	}); !errors.Is(err, ErrEdgeCollectorNotFound) {
		t.Fatalf("enroll revoked setup token error=%v want not found", err)
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
	if source.Status != EdgeDeviceSourceStatusPending {
		t.Fatalf("status=%q want pending", source.Status)
	}
	assertUUIDString(t, source.ID)
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
	if approved.Source.Status != EdgeDeviceSourceStatusLinked || approved.Source.LinkedDeviceID == "" {
		t.Fatalf("unexpected approved source: %+v", approved.Source)
	}
	assertUUIDString(t, approved.Device.DeviceID)
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

func TestMemoryStoreRejectsNonPendingEdgeDeviceSourceApproval(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("user-1")
	collector := createActiveEdgeCollectorForTest(t, store)
	source := upsertEdgeDeviceSourceForTest(t, store, collector.ID, "DEMOEDGE0001")

	approved, err := store.ApproveEdgeDeviceSource(context.Background(), ApproveEdgeDeviceSourceInput{
		UserSubject: "user-1",
		SourceID:    source.ID,
	})
	if err != nil {
		t.Fatalf("approve source: %v", err)
	}
	if _, err := store.ApproveEdgeDeviceSource(context.Background(), ApproveEdgeDeviceSourceInput{
		UserSubject: "user-1",
		SourceID:    source.ID,
		DeviceID:    approved.Device.DeviceID,
	}); !errors.Is(err, ErrEdgeDeviceSourceNotPending) {
		t.Fatalf("re-approve linked source error=%v want ErrEdgeDeviceSourceNotPending", err)
	}

	ignored := upsertEdgeDeviceSourceForTest(t, store, collector.ID, "DEMOEDGE0002")
	store.mu.Lock()
	row := store.edgeSources[ignored.ID]
	row.Status = EdgeDeviceSourceStatusIgnored
	store.edgeSources[ignored.ID] = row
	store.mu.Unlock()

	if _, err := store.ApproveEdgeDeviceSource(context.Background(), ApproveEdgeDeviceSourceInput{
		UserSubject: "user-1",
		SourceID:    ignored.ID,
	}); !errors.Is(err, ErrEdgeDeviceSourceNotPending) {
		t.Fatalf("approve ignored source error=%v want ErrEdgeDeviceSourceNotPending", err)
	}
}

func TestMemoryStoreRejectsMismatchedEdgeDeviceSourceTarget(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("user-1")
	collector := createActiveEdgeCollectorForTest(t, store)
	source := upsertEdgeDeviceSourceForTest(t, store, collector.ID, "DEMOEDGE0001")
	target, err := store.CreateDevice(context.Background(), CreateDeviceInput{
		UserSubject: "user-1",
		EcoflowSN:   "OTHEREDGE0002",
		ProductName: "Other device",
	})
	if err != nil {
		t.Fatalf("create target device: %v", err)
	}

	if _, err := store.ApproveEdgeDeviceSource(context.Background(), ApproveEdgeDeviceSourceInput{
		UserSubject: "user-1",
		SourceID:    source.ID,
		DeviceID:    target.DeviceID,
	}); !errors.Is(err, ErrEdgeDeviceSourceTargetMismatch) {
		t.Fatalf("approve mismatched target error=%v want ErrEdgeDeviceSourceTargetMismatch", err)
	}
}

func TestMemoryStoreRejectsAutoClaimingExistingEdgeDeviceSourceSN(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("user-1")
	store.EnsureUser("user-2")
	if _, err := store.CreateDevice(context.Background(), CreateDeviceInput{
		UserSubject: "user-1",
		EcoflowSN:   "DEMOEDGE0001",
		ProductName: "Existing owner device",
	}); err != nil {
		t.Fatalf("create existing device: %v", err)
	}

	collector := createActiveEdgeCollectorForTestForUser(t, store, "user-2")
	source := upsertEdgeDeviceSourceForTest(t, store, collector.ID, "DEMOEDGE0001")

	if _, err := store.ApproveEdgeDeviceSource(context.Background(), ApproveEdgeDeviceSourceInput{
		UserSubject: "user-2",
		SourceID:    source.ID,
	}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("approve existing unowned SN error=%v want ErrPermissionDenied", err)
	}
}

func createActiveEdgeCollectorForTest(t *testing.T, store *MemoryStore) EdgeCollector {
	t.Helper()
	return createActiveEdgeCollectorForTestForUser(t, store, "user-1")
}

func createActiveEdgeCollectorForTestForUser(t *testing.T, store *MemoryStore, userSubject string) EdgeCollector {
	t.Helper()

	collector, err := store.CreateEdgeCollector(context.Background(), CreateEdgeCollectorInput{
		UserSubject:    userSubject,
		DisplayName:    "Pi 5",
		SetupTokenHash: "setup-hash-" + t.Name(),
	})
	if err != nil {
		t.Fatalf("create edge collector: %v", err)
	}
	enrolled, err := store.EnrollEdgeCollector(context.Background(), EnrollEdgeCollectorInput{
		SetupTokenHash:      collector.SetupTokenHash,
		CollectorSecretHash: "secret-hash-" + t.Name(),
	})
	if err != nil {
		t.Fatalf("enroll edge collector: %v", err)
	}
	return enrolled
}

func upsertEdgeDeviceSourceForTest(t *testing.T, store *MemoryStore, collectorID string, providerDeviceID string) EdgeDeviceSource {
	t.Helper()

	source, err := store.UpsertEdgeDeviceSource(context.Background(), UpsertEdgeDeviceSourceInput{
		CollectorID:      collectorID,
		Provider:         ProviderEcoFlow,
		Transport:        "ble",
		ProviderDeviceID: providerDeviceID,
		DisplayName:      "Demo edge device",
		Model:            "EcoFlow RIVER 3 Plus",
		RSSIDBm:          -59,
		ObservedAt:       time.Date(2026, 5, 28, 13, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert edge source: %v", err)
	}
	return source
}

func assertUUIDString(t *testing.T, value string) {
	t.Helper()
	if _, err := uuid.Parse(value); err != nil {
		t.Fatalf("id %q is not UUID-shaped: %v", value, err)
	}
}
