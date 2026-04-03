package provideradapter

import (
	"fmt"
	"log/slog"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

// EcoFlowRuntimeConfig loads the shared EcoFlow client profile from env/.env.
// Runtime credentials still come from the per-user control-plane store.
func EcoFlowRuntimeConfig(log *slog.Logger) (ecoflow.Config, error) {
	cfg, err := ecoflow.ConfigFromEnvironment()
	if err != nil {
		return ecoflow.Config{}, fmt.Errorf("load ecoflow config from environment: %w", err)
	}
	cfg.Logging.Debug = false
	cfg.Logging.AdvancedDebugTelemetry = false
	cfg.Logging.DebugLogHeaders = false
	if log != nil {
		cfg.Logging.Logger = log
	}
	return cfg, nil
}

func NewRuntimeEcoFlowAdapter(log *slog.Logger) (*EcoFlowAdapter, error) {
	cfg, err := EcoFlowRuntimeConfig(log)
	if err != nil {
		return nil, err
	}
	return NewEcoFlowAdapter(NewDefaultEcoFlowClientFactory(cfg)), nil
}
