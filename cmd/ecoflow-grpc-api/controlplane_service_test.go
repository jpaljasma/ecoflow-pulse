package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	controlplanev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/controlplane/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type staticDiscoverer struct {
	devices []controlplane.ProviderDevice
}

func (d staticDiscoverer) DiscoverDevices(context.Context, controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error) {
	return d.devices, nil
}

func newControlPlaneServiceForTest() (*ControlPlaneService, *controlplane.MemoryStore) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := controlplane.NewMemoryStore()
	store.EnsureUser("dev-user")
	registry := provideradapter.NewRegistry()
	registry.RegisterProvider(controlplane.ProviderEcoFlow)
	return NewControlPlaneService(log, store, registry), store
}

func TestCreateProviderCredentialValidation(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	_, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    "unknown",
		AccessKey:   "abc",
		SecretKey:   "def",
		IsActive:    true,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestCreateProviderCredentialUsesRegistryBackedProviderSupport(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := controlplane.NewMemoryStore()
	store.EnsureUser("dev-user")
	registry := provideradapter.NewRegistry()
	registry.RegisterProvider("victron")
	svc := NewControlPlaneService(log, store, registry)

	resp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    "victron",
		AccessKey:   "AK1234567890",
		SecretKey:   "SK1234567890",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	if got := resp.GetCredential().GetProvider(); got != "victron" {
		t.Fatalf("credential provider=%q want victron", got)
	}
}

func TestCreateProviderCredentialUsesTokenSubject(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"})
	_, err := svc.CreateProviderCredential(ctx, &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "other-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK1234567890",
		SecretKey:   "SK1234567890",
		IsActive:    true,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestCreateAndListProviderCredentials(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	createResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK1234567890",
		SecretKey:   "SK1234567890",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	if createResp.GetCredential().GetAccessKeyMask() == "" {
		t.Fatalf("expected masked access key")
	}

	listResp, err := svc.ListProviderCredentials(context.Background(), &controlplanev1.ListProviderCredentialsRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
	})
	if err != nil {
		t.Fatalf("list credentials failed: %v", err)
	}
	if got := len(listResp.GetCredentials()); got != 1 {
		t.Fatalf("expected 1 credential, got %d", got)
	}
}

func TestSetProviderCredentialActive(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	createResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK11112222",
		SecretKey:   "SK11112222",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	updateResp, err := svc.SetProviderCredentialActive(context.Background(), &controlplanev1.SetProviderCredentialActiveRequest{
		UserSubject:  "dev-user",
		CredentialId: createResp.GetCredential().GetId(),
		IsActive:     false,
	})
	if err != nil {
		t.Fatalf("set active failed: %v", err)
	}
	if updateResp.GetCredential().GetIsActive() {
		t.Fatalf("expected credential to be inactive")
	}
}

func TestDeviceRegistryCreateLinkListAndRBAC(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	store.EnsureUser("other-user")

	created, err := svc.CreateDevice(context.Background(), &controlplanev1.CreateDeviceRequest{
		UserSubject: "dev-user",
		EcoflowSn:   "demod2m00001057",
		ProductName: "Kitchen Delta 2 Max",
		Model:       "DELTA 2 Max",
	})
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	if created.GetDevice().GetRole() != "admin" {
		t.Fatalf("expected creator role=admin, got %q", created.GetDevice().GetRole())
	}

	listDevUser, err := svc.ListUserDevices(context.Background(), &controlplanev1.ListUserDevicesRequest{
		UserSubject: "dev-user",
	})
	if err != nil {
		t.Fatalf("list user devices for dev-user failed: %v", err)
	}
	if got := len(listDevUser.GetDevices()); got != 1 {
		t.Fatalf("expected 1 device for dev-user, got %d", got)
	}

	linked, err := svc.LinkDevice(context.Background(), &controlplanev1.LinkDeviceRequest{
		UserSubject:       "dev-user",
		TargetUserSubject: "other-user",
		DeviceId:          created.GetDevice().GetDeviceId(),
		Role:              "viewer",
	})
	if err != nil {
		t.Fatalf("link device as admin failed: %v", err)
	}
	if linked.GetDevice().GetRole() != "viewer" {
		t.Fatalf("expected linked role=viewer, got %q", linked.GetDevice().GetRole())
	}

	listOtherUser, err := svc.ListUserDevices(context.Background(), &controlplanev1.ListUserDevicesRequest{
		UserSubject: "other-user",
	})
	if err != nil {
		t.Fatalf("list user devices for other-user failed: %v", err)
	}
	if got := len(listOtherUser.GetDevices()); got != 1 {
		t.Fatalf("expected 1 device for other-user, got %d", got)
	}
	if role := listOtherUser.GetDevices()[0].GetRole(); role != "viewer" {
		t.Fatalf("expected other-user role=viewer, got %q", role)
	}

	_, err = svc.LinkDevice(context.Background(), &controlplanev1.LinkDeviceRequest{
		UserSubject:       "other-user",
		TargetUserSubject: "dev-user",
		DeviceId:          created.GetDevice().GetDeviceId(),
		Role:              "admin",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for non-admin link, got %v", err)
	}
}

func TestLinkDeviceValidation(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	_, err := svc.LinkDevice(context.Background(), &controlplanev1.LinkDeviceRequest{
		UserSubject: "dev-user",
		Role:        "owner",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for invalid role, got %v", err)
	}
}

func TestListDevicesGroupedByProvider(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	store.PutProviderDevice(controlplane.ProviderDevice{
		Provider:         controlplane.ProviderEcoFlow,
		ProviderDeviceID: "DEMOD2M00001057",
		CanonicalSN:      "DEMOD2M00001057",
		ProductName:      "Kitchen Delta 2 Max",
		Model:            "DELTA 2 Max",
		Capabilities: map[string]any{
			"battery_pack_count": int64(2),
		},
		Metadata: map[string]any{
			"groups": map[string]any{
				"pd": map[string]any{"soc": int64(83)},
			},
		},
		IsActive:           true,
		IngestDesiredState: "active",
	})
	store.PutProviderDevice(controlplane.ProviderDevice{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMODPU0000294",
		CanonicalSN:        "DEMODPU0000294",
		ProductName:        "DPU A 12 kWh",
		Model:              "DELTA Pro Ultra",
		IsActive:           true,
		IngestDesiredState: "active",
	})

	resp, err := svc.ListDevices(context.Background(), &controlplanev1.ListDevicesRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		ActiveOnly:  true,
	})
	if err != nil {
		t.Fatalf("list devices failed: %v", err)
	}
	if got := len(resp.GetGroups()); got != 1 {
		t.Fatalf("expected 1 provider group, got %d", got)
	}
	if got := len(resp.GetGroups()[0].GetDevices()); got != 2 {
		t.Fatalf("expected 2 devices, got %d", got)
	}
	var found bool
	for _, device := range resp.GetGroups()[0].GetDevices() {
		if device.GetProviderDeviceId() != "DEMOD2M00001057" {
			continue
		}
		found = true
		if device.GetCapabilities().GetFields()["battery_pack_count"].GetNumberValue() != 2 {
			t.Fatalf("expected battery_pack_count in capabilities")
		}
		if device.GetMetadata().GetFields()["groups"].GetStructValue() == nil {
			t.Fatalf("expected grouped metadata in response")
		}
	}
	if !found {
		t.Fatalf("expected to find D2M provider device in response")
	}
}

func TestDiscoverDevicesConfiguredAndUnconfigured(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	createResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}

	unconfigured, err := svc.DiscoverDevices(context.Background(), &controlplanev1.DiscoverDevicesRequest{
		UserSubject:  "dev-user",
		Provider:     controlplane.ProviderEcoFlow,
		CredentialId: createResp.GetCredential().GetId(),
	})
	if err != nil {
		t.Fatalf("discover devices (unconfigured) failed: %v", err)
	}
	if unconfigured.GetAccepted() {
		t.Fatalf("expected accepted=false when discoverer is not configured")
	}

	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		devices: []controlplane.ProviderDevice{
			{
				ID:                 "pdev-1",
				Provider:           controlplane.ProviderEcoFlow,
				ProviderDeviceID:   "DEMOD2M00001057",
				CredentialID:       createResp.GetCredential().GetId(),
				CanonicalSN:        "DEMOD2M00001057",
				ProductName:        "Kitchen Delta 2 Max",
				Model:              "DELTA 2 Max",
				IsActive:           true,
				IngestDesiredState: "active",
			},
		},
	})
	configured, err := svc.DiscoverDevices(context.Background(), &controlplanev1.DiscoverDevicesRequest{
		UserSubject:  "dev-user",
		Provider:     controlplane.ProviderEcoFlow,
		CredentialId: createResp.GetCredential().GetId(),
	})
	if err != nil {
		t.Fatalf("discover devices (configured) failed: %v", err)
	}
	if !configured.GetAccepted() {
		t.Fatalf("expected accepted=true when discoverer is configured")
	}
	if configured.GetDiscoveredCount() != 1 {
		t.Fatalf("expected discovered_count=1, got %d", configured.GetDiscoveredCount())
	}
	if got := len(configured.GetDevices()); got != 1 {
		t.Fatalf("expected 1 discovered provider device in response, got %d", got)
	}

	listUserDevices, err := svc.ListUserDevices(context.Background(), &controlplanev1.ListUserDevicesRequest{
		UserSubject: "dev-user",
	})
	if err != nil {
		t.Fatalf("list user devices after discover failed: %v", err)
	}
	if got := len(listUserDevices.GetDevices()); got != 1 {
		t.Fatalf("expected 1 user device after discover persistence, got %d", got)
	}
	if listUserDevices.GetDevices()[0].GetRole() != "admin" {
		t.Fatalf("expected discover persistence to grant admin role")
	}
}
