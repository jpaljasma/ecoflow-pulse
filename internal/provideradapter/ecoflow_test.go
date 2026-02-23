package provideradapter

import (
	"context"
	"errors"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

type fakeEcoFlowFactory struct {
	client    EcoFlowClient
	err       error
	accessKey string
	secretKey string
}

func (f *fakeEcoFlowFactory) NewClient(accessKey, secretKey string) (EcoFlowClient, error) {
	f.accessKey = accessKey
	f.secretKey = secretKey
	if f.err != nil {
		return nil, f.err
	}
	return f.client, nil
}

type fakeEcoFlowClient struct {
	generalInfo EcoFlowGeneralInfo
}

func (c fakeEcoFlowClient) GeneralInfo() EcoFlowGeneralInfo {
	return c.generalInfo
}

type fakeGeneralInfo struct {
	devices []ecoflow.GeneralInfoDevice
	listErr error

	cert    ecoflow.GeneralInfoMQTTCertification
	certErr error

	listCalls int
	certCalls int
}

func (g *fakeGeneralInfo) ListDevices(context.Context) ([]ecoflow.GeneralInfoDevice, ecoflow.Response, error) {
	g.listCalls++
	if g.listErr != nil {
		return nil, ecoflow.Response{}, g.listErr
	}
	return g.devices, ecoflow.Response{}, nil
}

func (g *fakeGeneralInfo) GetMQTTCertification(context.Context) (ecoflow.GeneralInfoMQTTCertification, ecoflow.Response, error) {
	g.certCalls++
	if g.certErr != nil {
		return ecoflow.GeneralInfoMQTTCertification{}, ecoflow.Response{}, g.certErr
	}
	return g.cert, ecoflow.Response{}, nil
}

func TestEcoFlowAdapterDiscoverDevicesMapsAndSorts(t *testing.T) {
	t.Parallel()

	generalInfo := &fakeGeneralInfo{
		devices: []ecoflow.GeneralInfoDevice{
			{SN: " y711zaba9h2p0294 ", DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra"},
			{SN: "r351zabaph331057", DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max"},
			{SN: "   "},
		},
	}
	factory := &fakeEcoFlowFactory{client: fakeEcoFlowClient{generalInfo: generalInfo}}
	adapter := NewEcoFlowAdapter(factory)

	cred := controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderEcoFlow,
		AccessKey: "ak",
		SecretKey: "sk",
		IsActive:  true,
	}
	devices, err := adapter.DiscoverDevices(context.Background(), cred)
	if err != nil {
		t.Fatalf("DiscoverDevices() error = %v", err)
	}
	if got := len(devices); got != 2 {
		t.Fatalf("expected 2 discovered devices, got %d", got)
	}
	if devices[0].ProviderDeviceID != "R351ZABAPH331057" {
		t.Fatalf("unexpected first device SN: %q", devices[0].ProviderDeviceID)
	}
	if devices[1].ProviderDeviceID != "Y711ZABA9H2P0294" {
		t.Fatalf("unexpected second device SN: %q", devices[1].ProviderDeviceID)
	}
	if devices[0].CredentialID != "cred-1" || devices[1].CredentialID != "cred-1" {
		t.Fatalf("expected credential ID to be propagated")
	}
	if factory.accessKey != "ak" || factory.secretKey != "sk" {
		t.Fatalf("expected adapter to pass credential material to client factory")
	}
}

func TestEcoFlowAdapterGetMQTTCertificationSuccess(t *testing.T) {
	t.Parallel()

	generalInfo := &fakeGeneralInfo{
		devices: []ecoflow.GeneralInfoDevice{
			{SN: "R351ZABAPH331057"},
		},
		cert: ecoflow.GeneralInfoMQTTCertification{
			CertificateAccount:  "account",
			CertificatePassword: "password",
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
			Protocol:            "mqtts",
		},
	}
	adapter := NewEcoFlowAdapter(&fakeEcoFlowFactory{client: fakeEcoFlowClient{generalInfo: generalInfo}})
	cred := controlplane.ProviderCredential{
		Provider:  controlplane.ProviderEcoFlow,
		AccessKey: "ak",
		SecretKey: "sk",
		IsActive:  true,
	}
	cert, err := adapter.GetMQTTCertification(context.Background(), cred, "r351zabaph331057")
	if err != nil {
		t.Fatalf("GetMQTTCertification() error = %v", err)
	}
	if cert.URL != "mqtt.ecoflow.com" || cert.Port != "8883" {
		t.Fatalf("unexpected certification payload: %#v", cert)
	}
	if generalInfo.listCalls != 1 || generalInfo.certCalls != 1 {
		t.Fatalf("expected one list + one cert call, got list=%d cert=%d", generalInfo.listCalls, generalInfo.certCalls)
	}
}

func TestEcoFlowAdapterGetMQTTCertificationDeviceNotFound(t *testing.T) {
	t.Parallel()

	generalInfo := &fakeGeneralInfo{
		devices: []ecoflow.GeneralInfoDevice{
			{SN: "Y711ZABA9H2P0294"},
		},
	}
	adapter := NewEcoFlowAdapter(&fakeEcoFlowFactory{client: fakeEcoFlowClient{generalInfo: generalInfo}})
	cred := controlplane.ProviderCredential{
		Provider:  controlplane.ProviderEcoFlow,
		AccessKey: "ak",
		SecretKey: "sk",
		IsActive:  true,
	}
	_, err := adapter.GetMQTTCertification(context.Background(), cred, "R351ZABAPH331057")
	if !errors.Is(err, ErrProviderDeviceNotFound) {
		t.Fatalf("expected ErrProviderDeviceNotFound, got %v", err)
	}
	if generalInfo.certCalls != 0 {
		t.Fatalf("expected zero cert calls when SN is not owned")
	}
}

func TestEcoFlowAdapterValidationGuards(t *testing.T) {
	t.Parallel()

	adapter := NewEcoFlowAdapter(&fakeEcoFlowFactory{client: fakeEcoFlowClient{generalInfo: &fakeGeneralInfo{}}})

	tests := []struct {
		name       string
		credential controlplane.ProviderCredential
		wantErr    error
	}{
		{
			name: "unsupported provider",
			credential: controlplane.ProviderCredential{
				Provider:  "victron",
				AccessKey: "ak",
				SecretKey: "sk",
				IsActive:  true,
			},
			wantErr: ErrUnsupportedProvider,
		},
		{
			name: "inactive credential",
			credential: controlplane.ProviderCredential{
				Provider:  controlplane.ProviderEcoFlow,
				AccessKey: "ak",
				SecretKey: "sk",
				IsActive:  false,
			},
			wantErr: ErrInactiveCredential,
		},
		{
			name: "missing key material",
			credential: controlplane.ProviderCredential{
				Provider: controlplane.ProviderEcoFlow,
				IsActive: true,
			},
			wantErr: ErrMissingCredentialMaterial,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := adapter.DiscoverDevices(context.Background(), tc.credential)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestEcoFlowAdapterInvalidMQTTCertificationPayload(t *testing.T) {
	t.Parallel()

	generalInfo := &fakeGeneralInfo{
		devices: []ecoflow.GeneralInfoDevice{{SN: "R351ZABAPH331057"}},
		cert: ecoflow.GeneralInfoMQTTCertification{
			CertificateAccount: "account",
			URL:                "mqtt.ecoflow.com",
			Port:               "8883",
		},
	}
	adapter := NewEcoFlowAdapter(&fakeEcoFlowFactory{client: fakeEcoFlowClient{generalInfo: generalInfo}})
	cred := controlplane.ProviderCredential{
		Provider:  controlplane.ProviderEcoFlow,
		AccessKey: "ak",
		SecretKey: "sk",
		IsActive:  true,
	}
	_, err := adapter.GetMQTTCertification(context.Background(), cred, "R351ZABAPH331057")
	if !errors.Is(err, ErrInvalidMQTTCertification) {
		t.Fatalf("expected ErrInvalidMQTTCertification, got %v", err)
	}
}
