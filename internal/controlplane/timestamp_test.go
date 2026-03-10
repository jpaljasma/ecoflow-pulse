package controlplane

import (
	"context"
	"testing"
	"time"
)

func TestNormalizeWriteTimeConvertsToUTC(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC-5", -5*60*60)
	input := time.Date(2026, 3, 9, 12, 34, 56, 0, loc)
	got := normalizeWriteTime(input)

	if got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", got.Location())
	}
	if got.Hour() != 17 {
		t.Fatalf("expected normalized UTC hour 17, got %d", got.Hour())
	}
}

func TestMemoryStoreWritesUTCTimestampsFromNonUTCClock(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC-4", -4*60*60)
	fixedNow := time.Date(2026, 3, 9, 9, 15, 0, 0, loc)

	store := NewMemoryStore()
	store.now = func() time.Time { return fixedNow }
	store.EnsureUser("user-1")

	credential, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "user-1",
		Provider:    ProviderEcoFlow,
		AccessKey:   "AK-1111-2222",
		SecretKey:   "SK-1111-2222",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create provider credential failed: %v", err)
	}
	if credential.CreatedAt.Location() != time.UTC || credential.UpdatedAt.Location() != time.UTC {
		t.Fatalf("expected credential timestamps in UTC, got created=%v updated=%v", credential.CreatedAt.Location(), credential.UpdatedAt.Location())
	}

	device, err := store.CreateDevice(context.Background(), CreateDeviceInput{
		UserSubject: "user-1",
		EcoflowSN:   "SN-UTC-0001",
		ProductName: "DELTA 2 Max",
		Model:       "DELTA 2 Max",
	})
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	if device.CreatedAt.Location() != time.UTC || device.UpdatedAt.Location() != time.UTC {
		t.Fatalf("expected device timestamps in UTC, got created=%v updated=%v", device.CreatedAt.Location(), device.UpdatedAt.Location())
	}

	_, err = store.UpsertProviderDevice(context.Background(), UpsertProviderDeviceInput{
		DeviceID:           device.DeviceID,
		Provider:           ProviderEcoFlow,
		ProviderDeviceID:   "SN-UTC-0001",
		CredentialID:       credential.ID,
		ProductName:        "DELTA 2 Max",
		Model:              "DELTA 2 Max",
		IsActive:           true,
		IngestDesiredState: "active",
	})
	if err != nil {
		t.Fatalf("upsert provider device failed: %v", err)
	}
}
