package controlplane

import (
	"context"
	"testing"
)

func TestMemoryStoreCredentialCRUD(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("user-1")

	created, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "user-1",
		Provider:    ProviderEcoFlow,
		AccessKey:   "AK-1111-2222",
		SecretKey:   "SK-1111-2222",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected credential id")
	}

	listed, err := store.ListProviderCredentials(context.Background(), ListProviderCredentialsInput{
		UserSubject: "user-1",
		Provider:    ProviderEcoFlow,
	})
	if err != nil {
		t.Fatalf("list credentials failed: %v", err)
	}
	if got := len(listed); got != 1 {
		t.Fatalf("expected 1 credential, got %d", got)
	}
	fetched, err := store.GetProviderCredential(context.Background(), "user-1", created.ID)
	if err != nil {
		t.Fatalf("get credential failed: %v", err)
	}
	if fetched.AccessKey != "AK-1111-2222" {
		t.Fatalf("access key mismatch: got %q", fetched.AccessKey)
	}
	if fetched.SecretKey != "SK-1111-2222" {
		t.Fatalf("secret key mismatch: got %q", fetched.SecretKey)
	}

	updated, err := store.SetProviderCredentialActive(context.Background(), SetProviderCredentialActiveInput{
		UserSubject:  "user-1",
		CredentialID: created.ID,
		IsActive:     false,
	})
	if err != nil {
		t.Fatalf("set active failed: %v", err)
	}
	if updated.IsActive {
		t.Fatalf("expected inactive credential after update")
	}
}

func TestMemoryStoreListProviderDevices(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.PutProviderDevice(ProviderDevice{
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "R351ZABAPH331057",
		CanonicalSN:        "R351ZABAPH331057",
		IsActive:           true,
		IngestDesiredState: "active",
	})
	store.PutProviderDevice(ProviderDevice{
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "Y711ZABA9H2P0294",
		CanonicalSN:        "Y711ZABA9H2P0294",
		IsActive:           false,
		IngestDesiredState: "paused",
	})

	activeOnly, err := store.ListProviderDevices(context.Background(), ListProviderDevicesInput{
		Provider:   ProviderEcoFlow,
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("list provider devices failed: %v", err)
	}
	if got := len(activeOnly); got != 1 {
		t.Fatalf("expected 1 active device, got %d", got)
	}
}

func TestMemoryStoreProviderDeviceCapabilitiesAndMetadata(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	device, err := store.UpsertProviderDevice(context.Background(), UpsertProviderDeviceInput{
		DeviceID:         "dev-1",
		Provider:         ProviderEcoFlow,
		ProviderDeviceID: "R351ZABAPH331057",
		CredentialID:     "cred-1",
		ProductName:      "Kitchen Delta 2 Max",
		Model:            "DELTA 2 Max",
		Capabilities: map[string]any{
			"battery_pack_count": float64(2),
			"pv_ports":           float64(2),
		},
		Metadata: map[string]any{
			"backup_soc_pct": float64(21),
			"ac_always_on":   true,
		},
		IsActive:           true,
		IngestDesiredState: "active",
	})
	if err != nil {
		t.Fatalf("upsert provider device failed: %v", err)
	}
	device.Capabilities["tamper"] = true
	device.Metadata["tamper"] = true

	listed, err := store.ListProviderDevices(context.Background(), ListProviderDevicesInput{
		Provider: ProviderEcoFlow,
	})
	if err != nil {
		t.Fatalf("list provider devices failed: %v", err)
	}
	if got := len(listed); got != 1 {
		t.Fatalf("expected 1 provider device, got %d", got)
	}
	if listed[0].Capabilities["tamper"] != nil {
		t.Fatalf("capabilities map should be cloned on return")
	}
	if listed[0].Metadata["tamper"] != nil {
		t.Fatalf("metadata map should be cloned on return")
	}
	if got := listed[0].Capabilities["battery_pack_count"]; got != float64(2) {
		t.Fatalf("unexpected battery_pack_count=%v", got)
	}
	if got := listed[0].Metadata["backup_soc_pct"]; got != float64(21) {
		t.Fatalf("unexpected backup_soc_pct=%v", got)
	}

	updated, err := store.UpsertProviderDevice(context.Background(), UpsertProviderDeviceInput{
		DeviceID:           "dev-1",
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "R351ZABAPH331057",
		CredentialID:       "cred-1",
		ProductName:        "Kitchen Delta 2 Max",
		Model:              "DELTA 2 Max",
		IsActive:           true,
		IngestDesiredState: "active",
	})
	if err != nil {
		t.Fatalf("upsert provider device preserve failed: %v", err)
	}
	if got := updated.Capabilities["battery_pack_count"]; got != float64(2) {
		t.Fatalf("expected capabilities preserved, got %v", got)
	}
	if got := updated.Metadata["backup_soc_pct"]; got != float64(21) {
		t.Fatalf("expected metadata preserved, got %v", got)
	}
}

func TestMemoryStoreGetProviderDeviceByDeviceIDPrefersActive(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.PutProviderDevice(ProviderDevice{
		DeviceID:           "dev-1",
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "SN-paused",
		CanonicalSN:        "SN-paused",
		ProductName:        "Device Paused",
		Model:              "DELTA 2 Max",
		IsActive:           true,
		IngestDesiredState: "paused",
	})
	store.PutProviderDevice(ProviderDevice{
		DeviceID:           "dev-1",
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "SN-active",
		CanonicalSN:        "SN-active",
		ProductName:        "Device Active",
		Model:              "DELTA 2 Max",
		IsActive:           true,
		IngestDesiredState: "active",
	})

	row, err := store.GetProviderDeviceByDeviceID(context.Background(), "dev-1")
	if err != nil {
		t.Fatalf("get provider device by device id failed: %v", err)
	}
	if row.ProviderDeviceID != "SN-ACTIVE" {
		t.Fatalf("expected active provider device, got %q", row.ProviderDeviceID)
	}
}

func TestMemoryStoreListIngestAssignmentsActiveOnly(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("user-1")

	credActive, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "user-1",
		Provider:    ProviderEcoFlow,
		AccessKey:   "AK-active",
		SecretKey:   "SK-active",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create active credential failed: %v", err)
	}
	credInactive, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "user-1",
		Provider:    ProviderEcoFlow,
		AccessKey:   "AK-inactive",
		SecretKey:   "SK-inactive",
		IsActive:    false,
	})
	if err != nil {
		t.Fatalf("create inactive credential failed: %v", err)
	}

	store.PutProviderDevice(ProviderDevice{
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "SN-active",
		CredentialID:       credActive.ID,
		IsActive:           true,
		IngestDesiredState: "active",
	})
	store.PutProviderDevice(ProviderDevice{
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "SN-paused",
		CredentialID:       credActive.ID,
		IsActive:           true,
		IngestDesiredState: "paused",
	})
	store.PutProviderDevice(ProviderDevice{
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "SN-disabled",
		CredentialID:       credActive.ID,
		IsActive:           false,
		IngestDesiredState: "active",
	})
	store.PutProviderDevice(ProviderDevice{
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "SN-cred-off",
		CredentialID:       credInactive.ID,
		IsActive:           true,
		IngestDesiredState: "active",
	})

	assignments, err := store.ListIngestAssignments(context.Background(), ListIngestAssignmentsInput{
		Provider:   ProviderEcoFlow,
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("list ingest assignments failed: %v", err)
	}
	if got := len(assignments); got != 2 {
		t.Fatalf("expected 2 active assignments, got %d", got)
	}
	for _, a := range assignments {
		if a.ProviderDeviceID == "SN-paused" || a.ProviderDeviceID == "SN-disabled" {
			t.Fatalf("unexpected assignment in active-only result: %s", a.ProviderDeviceID)
		}
	}
}

func TestMemoryStoreDeviceRegistryRBAC(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.EnsureUser("owner")
	store.EnsureUser("guest")

	created, err := store.CreateDevice(context.Background(), CreateDeviceInput{
		UserSubject: "owner",
		EcoflowSN:   "R351ZABAPH331057",
		ProductName: "Kitchen Delta 2 Max",
		Model:       "DELTA 2 Max",
	})
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	if created.Role != "admin" {
		t.Fatalf("expected owner role=admin, got %q", created.Role)
	}

	_, err = store.LinkDevice(context.Background(), LinkDeviceInput{
		UserSubject:       "guest",
		TargetUserSubject: "owner",
		DeviceID:          created.DeviceID,
		Role:              "viewer",
	})
	if err == nil {
		t.Fatalf("expected non-admin link attempt to fail")
	}

	linked, err := store.LinkDevice(context.Background(), LinkDeviceInput{
		UserSubject:       "owner",
		TargetUserSubject: "guest",
		DeviceID:          created.DeviceID,
		Role:              "viewer",
	})
	if err != nil {
		t.Fatalf("link device failed: %v", err)
	}
	if linked.Role != "viewer" {
		t.Fatalf("expected guest role=viewer, got %q", linked.Role)
	}

	listed, err := store.ListUserDevices(context.Background(), ListUserDevicesInput{
		UserSubject: "guest",
	})
	if err != nil {
		t.Fatalf("list guest devices failed: %v", err)
	}
	if got := len(listed); got != 1 {
		t.Fatalf("expected 1 guest device, got %d", got)
	}
	if listed[0].Role != "viewer" {
		t.Fatalf("expected guest role=viewer, got %q", listed[0].Role)
	}
}
