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
