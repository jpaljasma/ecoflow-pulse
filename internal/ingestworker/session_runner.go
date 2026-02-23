package ingestworker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
)

type EcoFlowSessionRunner struct {
	log     *slog.Logger
	adapter *provideradapter.EcoFlowAdapter
}

func NewEcoFlowSessionRunner(log *slog.Logger, adapter *provideradapter.EcoFlowAdapter) *EcoFlowSessionRunner {
	if log == nil {
		log = slog.Default()
	}
	if adapter == nil {
		adapter = provideradapter.NewEcoFlowAdapter(nil)
	}
	return &EcoFlowSessionRunner{
		log:     log,
		adapter: adapter,
	}
}

func (r *EcoFlowSessionRunner) Run(ctx context.Context, a controlplane.IngestAssignment) error {
	if sanitizeProvider(a.Provider) != controlplane.ProviderEcoFlow {
		return fmt.Errorf("unsupported provider in session runner: %s", a.Provider)
	}
	credential := controlplane.ProviderCredential{
		ID:        a.CredentialID,
		Provider:  a.Provider,
		AccessKey: a.AccessKey,
		SecretKey: a.SecretKey,
		IsActive:  a.CredentialIsActive,
	}
	cert, err := r.adapter.GetMQTTCertification(ctx, credential, a.ProviderDeviceID)
	if err != nil {
		return fmt.Errorf("resolve mqtt certification for %s/%s: %w", a.Provider, a.ProviderDeviceID, err)
	}
	r.log.Info("ecoflow ingest session certified",
		slog.String("provider", a.Provider),
		slog.String("provider_device_id", a.ProviderDeviceID),
		slog.String("broker", cert.URL),
		slog.String("port", cert.Port),
	)
	<-ctx.Done()
	return nil
}
