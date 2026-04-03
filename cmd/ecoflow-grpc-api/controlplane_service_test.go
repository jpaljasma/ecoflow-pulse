package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	controlplanev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/controlplane/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type staticDiscoverer struct {
	devices  []controlplane.ProviderDevice
	cert     ecoflow.GeneralInfoMQTTCertification
	certErr  error
	discover func(controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error)
}

func (d staticDiscoverer) DiscoverDevices(_ context.Context, cred controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error) {
	if d.discover != nil {
		return d.discover(cred)
	}
	return d.devices, nil
}

func (d staticDiscoverer) GetMQTTCertification(context.Context, controlplane.ProviderCredential, string) (ecoflow.GeneralInfoMQTTCertification, error) {
	if d.certErr != nil {
		return ecoflow.GeneralInfoMQTTCertification{}, d.certErr
	}
	return d.cert, nil
}

type staticProbeSubscriber struct {
	connectErr   error
	subscribeErr error
	readErr      error
	msg          ecoflowmqtt.Message
}

func (s *staticProbeSubscriber) Connect(context.Context) error {
	return s.connectErr
}

func (s *staticProbeSubscriber) Subscribe(context.Context, string, byte) error {
	return s.subscribeErr
}

func (s *staticProbeSubscriber) ReadMessage(context.Context) (ecoflowmqtt.Message, error) {
	if s.readErr != nil {
		return ecoflowmqtt.Message{}, s.readErr
	}
	return s.msg, nil
}

func (s *staticProbeSubscriber) Close() error {
	return nil
}

func newControlPlaneServiceForTest() (*ControlPlaneService, *controlplane.MemoryStore) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := controlplane.NewMemoryStore()
	store.EnsureUser("dev-user")
	registry := provideradapter.NewRegistry()
	registry.RegisterProvider(controlplane.ProviderEcoFlow)
	registry.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		devices: []controlplane.ProviderDevice{},
		cert: ecoflow.GeneralInfoMQTTCertification{
			URL:                 "ssl://mqtt.example.com",
			Port:                "8883",
			CertificateAccount:  "acct",
			CertificatePassword: "pass",
		},
	})
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

func TestGetCurrentUserProvisionsFromClaimsAndReturnsAuthorization(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	device, err := store.CreateDevice(context.Background(), controlplane.CreateDeviceInput{
		UserSubject: "dev-user",
		EcoflowSN:   "DEMOD2M00001057",
		ProductName: "Kitchen Delta 2 Max",
		Model:       "DELTA 2 Max",
	})
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	if device.DeviceID == "" {
		t.Fatalf("expected created device id")
	}

	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{
		Subject:       "dev-user",
		Email:         "dev@example.com",
		EmailVerified: true,
		DisplayName:   "Dev User",
		GivenName:     "Dev",
		FamilyName:    "User",
		AvatarURL:     "https://example.com/avatar.png",
		Locale:        "en-US",
		AuthMethod:    "google",
		Roles:         []string{"viewer"},
	})
	resp, err := svc.GetCurrentUser(ctx, &controlplanev1.GetCurrentUserRequest{UserSubject: "dev-user"})
	if err != nil {
		t.Fatalf("get current user failed: %v", err)
	}
	if got := resp.GetUser().GetDisplayName(); got != "Dev User" {
		t.Fatalf("display name=%q want Dev User", got)
	}
	if got := resp.GetUser().GetAvatarUrl(); got != "https://example.com/avatar.png" {
		t.Fatalf("avatar_url=%q want provider value", got)
	}
	if got := resp.GetUser().GetAuthMethod(); got != "google" {
		t.Fatalf("auth_method=%q want google", got)
	}
	if got := resp.GetAuthorization().GetDeviceCount(); got != 1 {
		t.Fatalf("device count=%d want 1", got)
	}
	if got := resp.GetAuthorization().GetTokenRoles(); len(got) != 1 || got[0] != "viewer" {
		t.Fatalf("token roles=%v want [viewer]", got)
	}
}

func TestGetCurrentUserFallsBackToGivenAndFamilyName(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	device, err := store.CreateDevice(context.Background(), controlplane.CreateDeviceInput{
		UserSubject: "dev-user",
		EcoflowSN:   "Y711ZABA9H2P0294",
	})
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	if device.DeviceID == "" {
		t.Fatalf("expected created device id")
	}

	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{
		Subject:       "dev-user",
		Email:         "jaan@example.com",
		EmailVerified: true,
		GivenName:     "Jaan",
		FamilyName:    "Paljasma",
	})
	resp, err := svc.GetCurrentUser(ctx, &controlplanev1.GetCurrentUserRequest{UserSubject: "dev-user"})
	if err != nil {
		t.Fatalf("get current user failed: %v", err)
	}
	if got := resp.GetUser().GetDisplayName(); got != "Jaan Paljasma" {
		t.Fatalf("display name=%q want Jaan Paljasma", got)
	}
}

func TestUpdateCurrentUserValidatesTimezoneAndPreservesPulseDisplayName(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{
		Subject:     "dev-user",
		DisplayName: "Provider Name",
	})
	if _, err := svc.GetCurrentUser(ctx, &controlplanev1.GetCurrentUserRequest{UserSubject: "dev-user"}); err != nil {
		t.Fatalf("bootstrap current user failed: %v", err)
	}

	if _, err := svc.UpdateCurrentUser(ctx, &controlplanev1.UpdateCurrentUserRequest{
		UserSubject: "dev-user",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for missing timezone, got %v", err)
	}

	if _, err := svc.UpdateCurrentUser(ctx, &controlplanev1.UpdateCurrentUserRequest{
		UserSubject: "dev-user",
		Timezone:    "Mars/Olympus_Mons",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for invalid timezone, got %v", err)
	}

	updated, err := svc.UpdateCurrentUser(ctx, &controlplanev1.UpdateCurrentUserRequest{
		UserSubject:            "dev-user",
		DisplayName:            "Pulse Preferred",
		Timezone:               "America/New_York",
		WeatherLocationEnabled: false,
	})
	if err != nil {
		t.Fatalf("update current user failed: %v", err)
	}
	if got := updated.GetUser().GetDisplayName(); got != "Pulse Preferred" {
		t.Fatalf("display name=%q want Pulse Preferred", got)
	}

	refreshed, err := svc.GetCurrentUser(ctx, &controlplanev1.GetCurrentUserRequest{UserSubject: "dev-user"})
	if err != nil {
		t.Fatalf("refresh current user failed: %v", err)
	}
	if got := refreshed.GetUser().GetDisplayName(); got != "Pulse Preferred" {
		t.Fatalf("display name should stay pulse-owned, got %q", got)
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

func TestSetProviderCredentialActiveValidatesCandidateAsActive(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		discover: func(cred controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error) {
			if !cred.IsActive {
				return nil, provideradapter.ErrInactiveCredential
			}
			return []controlplane.ProviderDevice{
				{
					ID:               "pdev-1",
					Provider:         controlplane.ProviderEcoFlow,
					ProviderDeviceID: "R351ZABAPH331057",
					CredentialID:     cred.ID,
					CanonicalSN:      "R351ZABAPH331057",
					ProductName:      "Delta 2 Max",
				},
			}, nil
		},
		cert: ecoflow.GeneralInfoMQTTCertification{
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
			CertificateAccount:  "acct",
			CertificatePassword: "pass",
		},
	})
	svc.newMQTTSubscriber = func(ecoflowmqtt.Config) (mqttProbeSubscriber, error) {
		return &staticProbeSubscriber{
			msg: ecoflowmqtt.Message{
				Topic:   "/open/acct/R351ZABAPH331057/quota",
				Payload: []byte(`{"ok":true}`),
			},
		}, nil
	}

	createResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK11112222",
		SecretKey:   "SK11112222",
		IsActive:    false,
	})
	if err != nil {
		t.Fatalf("create inactive credential failed: %v", err)
	}

	updateResp, err := svc.SetProviderCredentialActive(context.Background(), &controlplanev1.SetProviderCredentialActiveRequest{
		UserSubject:  "dev-user",
		CredentialId: createResp.GetCredential().GetId(),
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("activate inactive credential failed: %v", err)
	}
	if !updateResp.GetCredential().GetIsActive() {
		t.Fatalf("expected credential to be active")
	}
}

func TestUpdateProviderCredentialRebindsAssignments(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		devices: []controlplane.ProviderDevice{
			{
				ID:                 "pdev-1",
				DeviceID:           "device-1",
				Provider:           controlplane.ProviderEcoFlow,
				ProviderDeviceID:   "DPU-001",
				CredentialID:       "cred-placeholder",
				CanonicalSN:        "DPU-001",
				ProductName:        "PowerOcean DPU",
				Model:              "DPU",
				IsActive:           true,
				IngestDesiredState: "active",
			},
		},
		cert: ecoflow.GeneralInfoMQTTCertification{
			URL:                 "ssl://mqtt.example.com",
			Port:                "8883",
			CertificateAccount:  "acct",
			CertificatePassword: "pass",
		},
	})
	svc.newMQTTSubscriber = func(cfg ecoflowmqtt.Config) (mqttProbeSubscriber, error) {
		return &staticProbeSubscriber{
			msg: ecoflowmqtt.Message{
				Topic:   "/open/acct/DPU-001/quota",
				Payload: []byte("{}"),
			},
		}, nil
	}
	activeResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK-active",
		SecretKey:   "SK-active",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create active credential failed: %v", err)
	}
	editedResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK-standby",
		SecretKey:   "SK-standby",
		IsActive:    false,
	})
	if err != nil {
		t.Fatalf("create standby credential failed: %v", err)
	}

	store.PutProviderDevice(controlplane.ProviderDevice{
		ID:                 "pdev-1",
		DeviceID:           "device-1",
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DPU-001",
		CredentialID:       activeResp.GetCredential().GetId(),
		CanonicalSN:        "DPU-001",
		IsActive:           true,
		IngestDesiredState: "active",
	})

	updateResp, err := svc.UpdateProviderCredential(context.Background(), &controlplanev1.UpdateProviderCredentialRequest{
		UserSubject:  "dev-user",
		CredentialId: editedResp.GetCredential().GetId(),
		AccessKey:    "AK-rotated",
		SecretKey:    "SK-rotated",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("update provider credential failed: %v", err)
	}
	if !updateResp.GetCredential().GetIsActive() {
		t.Fatalf("expected updated credential to be active")
	}

	assignments, err := store.ListIngestAssignments(context.Background(), controlplane.ListIngestAssignmentsInput{
		Provider:   controlplane.ProviderEcoFlow,
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("list ingest assignments failed: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("expected 1 ingest assignment, got %d", len(assignments))
	}
	if assignments[0].CredentialID != editedResp.GetCredential().GetId() {
		t.Fatalf("assignment credential=%q want %q", assignments[0].CredentialID, editedResp.GetCredential().GetId())
	}
	if assignments[0].AccessKey != "AK-rotated" || assignments[0].SecretKey != "SK-rotated" {
		t.Fatalf("assignment material mismatch: access=%q secret=%q", assignments[0].AccessKey, assignments[0].SecretKey)
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

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := controlplane.NewMemoryStore()
	store.EnsureUser("dev-user")
	registry := provideradapter.NewRegistry()
	registry.RegisterProvider(controlplane.ProviderEcoFlow)
	svc := NewControlPlaneService(log, store, registry)
	createResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    false,
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

func TestListAvailableProviderDevicesReturnsUnconfiguredOnly(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	credResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	existing, err := store.CreateDevice(context.Background(), controlplane.CreateDeviceInput{
		UserSubject: "dev-user",
		EcoflowSN:   "DEMODEXISTING0001",
		ProductName: "Configured Device",
		Model:       "DELTA 2 Max",
	})
	if err != nil {
		t.Fatalf("create configured device failed: %v", err)
	}
	if _, err := store.UpsertProviderDevice(context.Background(), controlplane.UpsertProviderDeviceInput{
		DeviceID:           existing.DeviceID,
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMODEXISTING0001",
		CredentialID:       credResp.GetCredential().GetId(),
		ProductName:        "Configured Device",
		Model:              "DELTA 2 Max",
		IsActive:           true,
		IngestDesiredState: "active",
	}); err != nil {
		t.Fatalf("upsert configured provider device failed: %v", err)
	}

	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		devices: []controlplane.ProviderDevice{
			{
				Provider:         controlplane.ProviderEcoFlow,
				ProviderDeviceID: "DEMODEXISTING0001",
				CanonicalSN:      "DEMODEXISTING0001",
				ProductName:      "Configured Device",
				Model:            "DELTA 2 Max",
			},
			{
				Provider:         controlplane.ProviderEcoFlow,
				ProviderDeviceID: "DEMODNEWDEVICE02",
				CanonicalSN:      "DEMODNEWDEVICE02",
				ProductName:      "Garage Delta 2",
				Model:            "DELTA 2",
			},
		},
	})

	resp, err := svc.ListAvailableProviderDevices(context.Background(), &controlplanev1.ListAvailableProviderDevicesRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
	})
	if err != nil {
		t.Fatalf("list available provider devices failed: %v", err)
	}
	if !resp.GetHasActiveCredentials() {
		t.Fatalf("expected has_active_credentials=true")
	}
	if got := len(resp.GetDevices()); got != 1 {
		t.Fatalf("available device count=%d want 1", got)
	}
	if got := resp.GetDevices()[0].GetProviderDeviceId(); got != "DEMODNEWDEVICE02" {
		t.Fatalf("provider_device_id=%q want DEMODNEWDEVICE02", got)
	}
}

func TestListAvailableProviderDevicesIncludesInactivePersistedDevices(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	credResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	existing, err := store.CreateDevice(context.Background(), controlplane.CreateDeviceInput{
		UserSubject: "dev-user",
		EcoflowSN:   "DEMODORMANT0001",
		ProductName: "Dormant Device",
		Model:       "DELTA mini",
	})
	if err != nil {
		t.Fatalf("create dormant device failed: %v", err)
	}
	if _, err := store.UpsertProviderDevice(context.Background(), controlplane.UpsertProviderDeviceInput{
		DeviceID:           existing.DeviceID,
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMODORMANT0001",
		CredentialID:       credResp.GetCredential().GetId(),
		ProductName:        "Dormant Device",
		Model:              "DELTA mini",
		IsActive:           false,
		IngestDesiredState: "paused",
	}); err != nil {
		t.Fatalf("upsert dormant provider device failed: %v", err)
	}

	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		devices: []controlplane.ProviderDevice{
			{
				Provider:         controlplane.ProviderEcoFlow,
				ProviderDeviceID: "DEMODORMANT0001",
				CanonicalSN:      "DEMODORMANT0001",
				ProductName:      "Dormant Device",
				Model:            "DELTA mini",
			},
		},
	})

	resp, err := svc.ListAvailableProviderDevices(context.Background(), &controlplanev1.ListAvailableProviderDevicesRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
	})
	if err != nil {
		t.Fatalf("list available provider devices failed: %v", err)
	}
	if got := len(resp.GetDevices()); got != 1 {
		t.Fatalf("available device count=%d want 1", got)
	}
	if got := resp.GetDevices()[0].GetProviderDeviceId(); got != "DEMODORMANT0001" {
		t.Fatalf("provider_device_id=%q want DEMODORMANT0001", got)
	}
}

func TestListAvailableProviderDevicesSkipsCredentialsMissingMaterial(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	credMissing, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AKMISSING1234",
		SecretKey:   "SKMISSING1234",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create missing-material credential failed: %v", err)
	}
	credValid, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AKVALID1234",
		SecretKey:   "SKVALID1234",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create valid credential failed: %v", err)
	}
	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		discover: func(cred controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error) {
			if cred.ID == credMissing.GetCredential().GetId() {
				return nil, provideradapter.ErrMissingCredentialMaterial
			}
			if cred.ID != credValid.GetCredential().GetId() {
				t.Fatalf("unexpected credential id %q", cred.ID)
			}
			return []controlplane.ProviderDevice{
				{
					Provider:         controlplane.ProviderEcoFlow,
					ProviderDeviceID: "DEMODNEWDEVICE03",
					CanonicalSN:      "DEMODNEWDEVICE03",
					ProductName:      "Patio River 3",
					Model:            "RIVER 3",
				},
			}, nil
		},
	})

	resp, err := svc.ListAvailableProviderDevices(context.Background(), &controlplanev1.ListAvailableProviderDevicesRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
	})
	if err != nil {
		t.Fatalf("list available provider devices failed: %v", err)
	}
	if !resp.GetHasActiveCredentials() {
		t.Fatalf("expected has_active_credentials=true")
	}
	if got := len(resp.GetDevices()); got != 1 {
		t.Fatalf("available device count=%d want 1", got)
	}
	if got := resp.GetDevices()[0].GetProviderDeviceId(); got != "DEMODNEWDEVICE03" {
		t.Fatalf("provider_device_id=%q want DEMODNEWDEVICE03", got)
	}
}

func TestTestProviderDeviceMQTTSuccess(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	credResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		devices: []controlplane.ProviderDevice{
			{
				Provider:         controlplane.ProviderEcoFlow,
				ProviderDeviceID: "DEMODMQTT000001",
				CanonicalSN:      "DEMODMQTT000001",
				ProductName:      "MQTT Test Device",
				Model:            "DELTA 2",
			},
		},
		cert: ecoflow.GeneralInfoMQTTCertification{
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
			CertificateAccount:  "open-account",
			CertificatePassword: "secret",
		},
	})
	svc.newMQTTSubscriber = func(cfg ecoflowmqtt.Config) (mqttProbeSubscriber, error) {
		if cfg.Address != "mqtt.ecoflow.com:8883" {
			t.Fatalf("mqtt address=%q want mqtt.ecoflow.com:8883", cfg.Address)
		}
		if cfg.ClientID == ecoflowmqtt.BuildClientIDFromSN("DEMODMQTT000001") {
			t.Fatalf("mqtt probe client id should not reuse the ingest client id")
		}
		return &staticProbeSubscriber{
			msg: ecoflowmqtt.Message{
				Topic:   "/open/open-account/DEMODMQTT000001/quota",
				Payload: []byte(`{"id":1}`),
			},
		}, nil
	}

	resp, err := svc.TestProviderDeviceMQTT(context.Background(), &controlplanev1.TestProviderDeviceMQTTRequest{
		UserSubject:      "dev-user",
		Provider:         controlplane.ProviderEcoFlow,
		CredentialId:     credResp.GetCredential().GetId(),
		ProviderDeviceId: "DEMODMQTT000001",
	})
	if err != nil {
		t.Fatalf("test provider device mqtt failed: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected mqtt success response")
	}
	if got := resp.GetPayloadBytes(); got != int64(len(`{"id":1}`)) {
		t.Fatalf("payload_bytes=%d want %d", got, len(`{"id":1}`))
	}
}

func TestEnableProviderDevicePersistsActiveAssignment(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	credResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		devices: []controlplane.ProviderDevice{
			{
				Provider:         controlplane.ProviderEcoFlow,
				ProviderDeviceID: "DEMODENABLE0001",
				CanonicalSN:      "DEMODENABLE0001",
				ProductName:      "Garage Delta 2",
				Model:            "DELTA 2",
			},
		},
		cert: ecoflow.GeneralInfoMQTTCertification{
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
			CertificateAccount:  "open-account",
			CertificatePassword: "secret",
		},
	})
	svc.newMQTTSubscriber = func(cfg ecoflowmqtt.Config) (mqttProbeSubscriber, error) {
		return &staticProbeSubscriber{
			msg: ecoflowmqtt.Message{
				Topic:   "/open/open-account/DEMODENABLE0001/quota",
				Payload: []byte(`{"id":1}`),
			},
		}, nil
	}

	resp, err := svc.EnableProviderDevice(context.Background(), &controlplanev1.EnableProviderDeviceRequest{
		UserSubject:      "dev-user",
		Provider:         controlplane.ProviderEcoFlow,
		CredentialId:     credResp.GetCredential().GetId(),
		ProviderDeviceId: "DEMODENABLE0001",
	})
	if err != nil {
		t.Fatalf("enable provider device failed: %v", err)
	}
	if got := resp.GetProviderDevice().GetIngestDesiredState(); got != "active" {
		t.Fatalf("ingest_desired_state=%q want active", got)
	}
	listed, err := store.ListProviderDevices(context.Background(), controlplane.ListProviderDevicesInput{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		ActiveOnly:  false,
	})
	if err != nil {
		t.Fatalf("list provider devices failed: %v", err)
	}
	if got := len(listed); got != 1 {
		t.Fatalf("provider device count=%d want 1", got)
	}
	if !listed[0].IsActive {
		t.Fatalf("expected enabled provider device to be active")
	}
	userDevices, err := store.ListUserDevices(context.Background(), controlplane.ListUserDevicesInput{UserSubject: "dev-user"})
	if err != nil {
		t.Fatalf("list user devices failed: %v", err)
	}
	if got := len(userDevices); got != 1 {
		t.Fatalf("user device count=%d want 1", got)
	}
}

func TestEnableProviderDeviceRequiresSuccessfulMQTTProbe(t *testing.T) {
	t.Parallel()

	svc, store := newControlPlaneServiceForTest()
	credResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		devices: []controlplane.ProviderDevice{
			{
				Provider:         controlplane.ProviderEcoFlow,
				ProviderDeviceID: "DEMODENABLE0002",
				CanonicalSN:      "DEMODENABLE0002",
				ProductName:      "Dormant River",
				Model:            "RIVER 3 Plus",
			},
		},
		cert: ecoflow.GeneralInfoMQTTCertification{
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
			CertificateAccount:  "open-account",
			CertificatePassword: "secret",
		},
	})
	svc.newMQTTSubscriber = func(cfg ecoflowmqtt.Config) (mqttProbeSubscriber, error) {
		return &staticProbeSubscriber{readErr: context.DeadlineExceeded}, nil
	}

	_, err = svc.EnableProviderDevice(context.Background(), &controlplanev1.EnableProviderDeviceRequest{
		UserSubject:      "dev-user",
		Provider:         controlplane.ProviderEcoFlow,
		CredentialId:     credResp.GetCredential().GetId(),
		ProviderDeviceId: "DEMODENABLE0002",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}

	listed, listErr := store.ListProviderDevices(context.Background(), controlplane.ListProviderDevicesInput{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		ActiveOnly:  false,
	})
	if listErr != nil {
		t.Fatalf("list provider devices failed: %v", listErr)
	}
	if len(listed) != 0 {
		t.Fatalf("expected no provider devices to be persisted, got %d", len(listed))
	}
}

func TestTestProviderDeviceMQTTReturnsTimeoutStatus(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	credResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		cert: ecoflow.GeneralInfoMQTTCertification{
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
			CertificateAccount:  "open-account",
			CertificatePassword: "secret",
		},
	})
	svc.newMQTTSubscriber = func(cfg ecoflowmqtt.Config) (mqttProbeSubscriber, error) {
		return &staticProbeSubscriber{readErr: context.DeadlineExceeded}, nil
	}

	resp, err := svc.TestProviderDeviceMQTT(context.Background(), &controlplanev1.TestProviderDeviceMQTTRequest{
		UserSubject:      "dev-user",
		Provider:         controlplane.ProviderEcoFlow,
		CredentialId:     credResp.GetCredential().GetId(),
		ProviderDeviceId: "DEMODMQTT000002",
	})
	if err != nil {
		t.Fatalf("test provider device mqtt failed: %v", err)
	}
	if resp.GetStatus() != "timeout" {
		t.Fatalf("status=%q want timeout", resp.GetStatus())
	}
}

func TestTestProviderDeviceMQTTMapsProviderNotFound(t *testing.T) {
	t.Parallel()

	svc, _ := newControlPlaneServiceForTest()
	credResp, err := svc.CreateProviderCredential(context.Background(), &controlplanev1.CreateProviderCredentialRequest{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlow,
		AccessKey:   "AK55556666",
		SecretKey:   "SK55556666",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create credential failed: %v", err)
	}
	svc.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{
		certErr: fmt.Errorf("%w", provideradapter.ErrProviderDeviceNotFound),
	})

	_, err = svc.TestProviderDeviceMQTT(context.Background(), &controlplanev1.TestProviderDeviceMQTTRequest{
		UserSubject:      "dev-user",
		Provider:         controlplane.ProviderEcoFlow,
		CredentialId:     credResp.GetCredential().GetId(),
		ProviderDeviceId: "missing",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}
