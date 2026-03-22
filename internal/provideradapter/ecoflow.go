package provideradapter

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

var (
	ErrUnsupportedProvider       = errors.New("unsupported provider for adapter")
	ErrInactiveCredential        = errors.New("provider credential is inactive")
	ErrMissingCredentialMaterial = errors.New("provider credential material is missing")
	ErrProviderDeviceNotFound    = errors.New("provider device not found for credential")
	ErrInvalidMQTTCertification  = errors.New("invalid mqtt certification payload")
)

type EcoFlowGeneralInfo interface {
	ListDevices(ctx context.Context) ([]ecoflow.GeneralInfoDevice, ecoflow.Response, error)
	GetMQTTCertification(ctx context.Context) (ecoflow.GeneralInfoMQTTCertification, ecoflow.Response, error)
	GetDeviceAllQuota(ctx context.Context, sn string) (map[string]string, ecoflow.Response, error)
}

type EcoFlowClient interface {
	GeneralInfo() EcoFlowGeneralInfo
}

type EcoFlowClientFactory interface {
	NewClient(accessKey, secretKey string) (EcoFlowClient, error)
}

type defaultEcoFlowClientFactory struct {
	baseConfig ecoflow.Config
}

func NewDefaultEcoFlowClientFactory(baseConfig ecoflow.Config) EcoFlowClientFactory {
	return defaultEcoFlowClientFactory{baseConfig: baseConfig}
}

func (f defaultEcoFlowClientFactory) NewClient(accessKey, secretKey string) (EcoFlowClient, error) {
	creds, err := ecoflow.NewStaticCredentialsProvider(accessKey, secretKey)
	if err != nil {
		return nil, fmt.Errorf("create static credentials provider: %w", err)
	}
	cfg := f.baseConfig
	cfg.CredentialsProvider = creds

	client, err := ecoflow.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create ecoflow client: %w", err)
	}
	return ecoflowClientWrapper{client: client}, nil
}

type ecoflowClientWrapper struct {
	client *ecoflow.Client
}

func (c ecoflowClientWrapper) GeneralInfo() EcoFlowGeneralInfo {
	return c.client.GeneralInfo()
}

type EcoFlowAdapter struct {
	factory EcoFlowClientFactory
}

func NewEcoFlowAdapter(factory EcoFlowClientFactory) *EcoFlowAdapter {
	if factory == nil {
		factory = NewDefaultEcoFlowClientFactory(ecoflow.DefaultConfig())
	}
	return &EcoFlowAdapter{factory: factory}
}

func (a *EcoFlowAdapter) DiscoverDevices(ctx context.Context, credential controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error) {
	generalInfo, err := a.generalInfoForCredential(credential)
	if err != nil {
		return nil, err
	}
	devices, _, err := generalInfo.ListDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ecoflow devices: %w", err)
	}

	seen := make(map[string]struct{}, len(devices))
	out := make([]controlplane.ProviderDevice, 0, len(devices))
	for i := range devices {
		mapped, ok := mapEcoFlowDevice(devices[i], credential.ID)
		if !ok {
			continue
		}
		if _, exists := seen[mapped.ProviderDeviceID]; exists {
			continue
		}
		seen[mapped.ProviderDeviceID] = struct{}{}
		out = append(out, mapped)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ProviderDeviceID < out[j].ProviderDeviceID
	})
	return out, nil
}

func (a *EcoFlowAdapter) GetMQTTCertification(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (ecoflow.GeneralInfoMQTTCertification, error) {
	generalInfo, err := a.generalInfoForCredential(credential)
	if err != nil {
		return ecoflow.GeneralInfoMQTTCertification{}, err
	}
	if _, err := a.describeDevice(ctx, generalInfo, credential.ID, providerDeviceID); err != nil {
		return ecoflow.GeneralInfoMQTTCertification{}, err
	}

	cert, _, err := generalInfo.GetMQTTCertification(ctx)
	if err != nil {
		return ecoflow.GeneralInfoMQTTCertification{}, fmt.Errorf("get mqtt certification: %w", err)
	}
	if strings.TrimSpace(cert.CertificateAccount) == "" ||
		strings.TrimSpace(cert.CertificatePassword) == "" ||
		strings.TrimSpace(cert.URL) == "" ||
		strings.TrimSpace(cert.Port) == "" {
		return ecoflow.GeneralInfoMQTTCertification{}, ErrInvalidMQTTCertification
	}
	return cert, nil
}

func (a *EcoFlowAdapter) GetDeviceAllQuota(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (map[string]string, error) {
	_, quota, err := a.GetDeviceQuotaSnapshot(ctx, credential, providerDeviceID)
	if err != nil {
		return nil, err
	}
	return quota, nil
}

func (a *EcoFlowAdapter) GetDeviceQuotaSnapshot(
	ctx context.Context,
	credential controlplane.ProviderCredential,
	providerDeviceID string,
) (controlplane.ProviderDevice, map[string]string, error) {
	generalInfo, err := a.generalInfoForCredential(credential)
	if err != nil {
		return controlplane.ProviderDevice{}, nil, err
	}
	device, err := a.describeDevice(ctx, generalInfo, credential.ID, providerDeviceID)
	if err != nil {
		return controlplane.ProviderDevice{}, nil, err
	}

	quota, _, err := generalInfo.GetDeviceAllQuota(ctx, device.ProviderDeviceID)
	if err != nil {
		return controlplane.ProviderDevice{}, nil, fmt.Errorf("get device all quota: %w", err)
	}
	if quota == nil {
		return device, map[string]string{}, nil
	}
	return device, quota, nil
}

func BuildMQTTAddressAndTopic(
	cert ecoflow.GeneralInfoMQTTCertification,
	providerDeviceID string,
) (address string, topic string, err error) {
	url := strings.TrimSpace(cert.URL)
	port := strings.TrimSpace(cert.Port)
	account := strings.TrimSpace(cert.CertificateAccount)
	if url == "" || port == "" {
		return "", "", errors.New("mqtt certification missing broker url/port")
	}
	if account == "" {
		return "", "", errors.New("mqtt certification missing certificate account")
	}
	sn := normalizeProviderDeviceID(providerDeviceID)
	if sn == "" {
		return "", "", errors.New("provider_device_id is required")
	}
	return fmt.Sprintf("%s:%s", url, port), fmt.Sprintf("/open/%s/%s/quota", account, sn), nil
}

func (a *EcoFlowAdapter) generalInfoForCredential(credential controlplane.ProviderCredential) (EcoFlowGeneralInfo, error) {
	if controlplane.NormalizeProvider(credential.Provider) != controlplane.ProviderEcoFlow {
		return nil, ErrUnsupportedProvider
	}
	if !credential.IsActive {
		return nil, ErrInactiveCredential
	}
	accessKey := strings.TrimSpace(credential.AccessKey)
	secretKey := strings.TrimSpace(credential.SecretKey)
	if accessKey == "" || secretKey == "" {
		return nil, ErrMissingCredentialMaterial
	}

	client, err := a.factory.NewClient(accessKey, secretKey)
	if err != nil {
		return nil, fmt.Errorf("create ecoflow adapter client: %w", err)
	}
	return client.GeneralInfo(), nil
}

func mapEcoFlowDevice(device ecoflow.GeneralInfoDevice, credentialID string) (controlplane.ProviderDevice, bool) {
	sn := normalizeProviderDeviceID(device.SN)
	if sn == "" {
		return controlplane.ProviderDevice{}, false
	}
	productName := strings.TrimSpace(device.DeviceName)
	model := strings.TrimSpace(device.ProductName)
	if productName == "" {
		productName = model
	}
	if productName == "" {
		productName = sn
	}
	return controlplane.ProviderDevice{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   sn,
		CredentialID:       credentialID,
		CanonicalSN:        sn,
		ProductName:        productName,
		Model:              model,
		IsActive:           true,
		IngestDesiredState: "active",
	}, true
}

func (a *EcoFlowAdapter) describeDevice(
	ctx context.Context,
	generalInfo EcoFlowGeneralInfo,
	credentialID string,
	providerDeviceID string,
) (controlplane.ProviderDevice, error) {
	targetSN := normalizeProviderDeviceID(providerDeviceID)
	if targetSN == "" {
		return controlplane.ProviderDevice{}, fmt.Errorf("provider_device_id is required")
	}

	devices, _, err := generalInfo.ListDevices(ctx)
	if err != nil {
		return controlplane.ProviderDevice{}, fmt.Errorf("list ecoflow devices: %w", err)
	}
	for i := range devices {
		mapped, ok := mapEcoFlowDevice(devices[i], credentialID)
		if !ok {
			continue
		}
		if mapped.ProviderDeviceID == targetSN {
			return mapped, nil
		}
	}
	return controlplane.ProviderDevice{}, fmt.Errorf("%w: %s", ErrProviderDeviceNotFound, targetSN)
}

func normalizeProviderDeviceID(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
