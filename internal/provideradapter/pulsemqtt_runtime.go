package provideradapter

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

const (
	defaultPulseMQTTBaseURL    = "http://pulse-services-pulse-mqtt-emulator.pulse-services.svc.cluster.local:8080"
	defaultPulseMQTTBrokerHost = "pulse-services-pulse-mqtt-emulator.pulse-services.svc.cluster.local"
)

func PulseMQTTRuntimeConfig(log *slog.Logger) (ecoflow.Config, error) {
	cfg := ecoflow.DefaultConfig()
	cfg.Environment = ecoflow.EnvironmentDev
	cfg.BaseURL = runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_BASE_URL", defaultPulseMQTTBaseURL)
	cfg.Logging.Debug = false
	cfg.Logging.AdvancedDebugTelemetry = false
	cfg.Logging.DebugLogHeaders = false
	if log != nil {
		cfg.Logging.Logger = log
	}
	return cfg, nil
}

func NewRuntimePulseMQTTAdapter(log *slog.Logger) (*EcoFlowAdapter, error) {
	cfg, err := PulseMQTTRuntimeConfig(log)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := pulseMQTTTLSConfigFromEnv(log)
	if err != nil {
		return nil, err
	}
	return NewEcoFlowCompatibleAdapter(
		controlplane.ProviderPulseMQTT,
		NewDefaultEcoFlowClientFactory(cfg),
		tlsConfig,
	), nil
}

func pulseMQTTTLSConfigFromEnv(log *slog.Logger) (*tls.Config, error) {
	serverName := strings.TrimSpace(runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_TLS_SERVER_NAME", defaultPulseMQTTBrokerHost))
	if caPEM := strings.TrimSpace(runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_TLS_CA_PEM", "")); caPEM != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caPEM)) {
			return nil, fmt.Errorf("parse PULSE_MQTT_EMULATOR_TLS_CA_PEM")
		}
		return &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
			ServerName: serverName,
		}, nil
	}

	if log != nil {
		log.Warn("pulse mqtt emulator tls verification is disabled; configure PULSE_MQTT_EMULATOR_TLS_CA_PEM for broker trust")
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         serverName,
		InsecureSkipVerify: true,
	}, nil
}
