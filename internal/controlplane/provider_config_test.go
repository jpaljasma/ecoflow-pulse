package controlplane

import (
	"context"
	"testing"
)

func TestMemoryStoreProviderCredentialConfigRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	created, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "dev-user",
		Provider:    ProviderPecron,
		AccessKey:   "owner@example.test",
		SecretKey:   "super-secret",
		Config: map[string]any{
			"region": "us",
		},
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateProviderCredential() error = %v", err)
	}
	if got := created.Config["region"]; got != "us" {
		t.Fatalf("created config region = %v, want us", got)
	}

	listed, err := store.ListProviderCredentials(context.Background(), ListProviderCredentialsInput{
		UserSubject: "dev-user",
		Provider:    ProviderPecron,
	})
	if err != nil {
		t.Fatalf("ListProviderCredentials() error = %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed credentials = %d, want 1", len(listed))
	}
	if listed[0].AccessKey != "" || listed[0].SecretKey != "" {
		t.Fatalf("listed credential exposed secret material: %#v", listed[0])
	}
	if got := listed[0].Config["region"]; got != "us" {
		t.Fatalf("listed config region = %v, want us", got)
	}

	fetched, err := store.GetProviderCredential(context.Background(), "dev-user", created.ID)
	if err != nil {
		t.Fatalf("GetProviderCredential() error = %v", err)
	}
	if fetched.SecretKey != "super-secret" {
		t.Fatalf("internal fetch did not return secret material")
	}
	if got := fetched.Config["region"]; got != "us" {
		t.Fatalf("fetched config region = %v, want us", got)
	}
}

func TestMemoryStoreAnkerSolixProviderCredentialConfigRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	created, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "dev-user",
		Provider:    ProviderAnkerSolix,
		AccessKey:   "owner@example.test",
		SecretKey:   "super-secret",
		Config: map[string]any{
			"server":  "com",
			"country": "US",
		},
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateProviderCredential() error = %v", err)
	}

	listed, err := store.ListProviderCredentials(context.Background(), ListProviderCredentialsInput{
		UserSubject: "dev-user",
		Provider:    ProviderAnkerSolix,
	})
	if err != nil {
		t.Fatalf("ListProviderCredentials() error = %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed credentials = %d, want 1", len(listed))
	}
	if listed[0].AccessKey != "" || listed[0].SecretKey != "" {
		t.Fatalf("listed credential exposed secret material: %#v", listed[0])
	}
	if listed[0].AccessKeyMask == "" {
		t.Fatalf("listed credential did not include masked account metadata")
	}
	if got := listed[0].Config["server"]; got != "com" {
		t.Fatalf("listed config server = %v, want com", got)
	}
	if got := listed[0].Config["country"]; got != "US" {
		t.Fatalf("listed config country = %v, want US", got)
	}

	fetched, err := store.GetProviderCredential(context.Background(), "dev-user", created.ID)
	if err != nil {
		t.Fatalf("GetProviderCredential() error = %v", err)
	}
	if fetched.AccessKey != "owner@example.test" || fetched.SecretKey != "super-secret" {
		t.Fatalf("internal fetch did not return credential material")
	}
	if got := fetched.Config["server"]; got != "com" {
		t.Fatalf("fetched config server = %v, want com", got)
	}
	if got := fetched.Config["country"]; got != "US" {
		t.Fatalf("fetched config country = %v, want US", got)
	}
}

func TestMemoryStoreIngestAssignmentIncludesCredentialConfig(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	created, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "dev-user",
		Provider:    ProviderPecron,
		AccessKey:   "owner@example.test",
		SecretKey:   "super-secret",
		Config: map[string]any{
			"region": "eu",
		},
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateProviderCredential() error = %v", err)
	}
	store.PutProviderDevice(ProviderDevice{
		ID:                 "pdev-1",
		DeviceID:           "device-1",
		Provider:           ProviderPecron,
		ProviderDeviceID:   "p11vxg:device-key",
		CredentialID:       created.ID,
		ProductName:        "Pecron E1000LFP",
		Model:              "E1000LFP",
		IsActive:           true,
		IngestDesiredState: "active",
	})

	assignments, err := store.ListIngestAssignments(context.Background(), ListIngestAssignmentsInput{
		Provider:   ProviderPecron,
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("ListIngestAssignments() error = %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("assignments = %d, want 1", len(assignments))
	}
	if got := assignments[0].CredentialConfig["region"]; got != "eu" {
		t.Fatalf("assignment config region = %v, want eu", got)
	}
}

func TestMemoryStoreAnkerSolixIngestAssignmentIncludesCredentialConfig(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	created, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "dev-user",
		Provider:    ProviderAnkerSolix,
		AccessKey:   "owner@example.test",
		SecretKey:   "super-secret",
		Config: map[string]any{
			"server":  "eu",
			"country": "DE",
		},
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateProviderCredential() error = %v", err)
	}
	store.PutProviderDevice(ProviderDevice{
		ID:                 "pdev-1",
		DeviceID:           "device-1",
		Provider:           ProviderAnkerSolix,
		ProviderDeviceID:   "A1783:device-sn",
		CredentialID:       created.ID,
		ProductName:        "Anker SOLIX C2000 Gen 2",
		Model:              "A1783",
		IsActive:           true,
		IngestDesiredState: "active",
	})

	assignments, err := store.ListIngestAssignments(context.Background(), ListIngestAssignmentsInput{
		Provider:   ProviderAnkerSolix,
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("ListIngestAssignments() error = %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("assignments = %d, want 1", len(assignments))
	}
	if got := assignments[0].CredentialConfig["server"]; got != "eu" {
		t.Fatalf("assignment config server = %v, want eu", got)
	}
	if got := assignments[0].CredentialConfig["country"]; got != "DE" {
		t.Fatalf("assignment config country = %v, want DE", got)
	}
}
