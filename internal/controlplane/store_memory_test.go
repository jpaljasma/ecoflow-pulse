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
