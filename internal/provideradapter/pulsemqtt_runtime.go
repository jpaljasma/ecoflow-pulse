package provideradapter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

const (
	defaultPulseMQTTBaseURL    = "http://pulse-services-pulse-mqtt-emulator.pulse-services.svc.cluster.local:8080"
	defaultPulseMQTTBrokerHost = "pulse-services-pulse-mqtt-emulator.pulse-services.svc.cluster.local"
	pulseMQTTCABundlePath      = "/mqtt-ca.pem"
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
	tlsConfig, err := pulseMQTTTLSConfigFromEnv(context.Background(), cfg.BaseURL, log)
	if err != nil {
		return nil, err
	}
	return NewEcoFlowCompatibleAdapter(
		controlplane.ProviderPulseMQTT,
		NewDefaultEcoFlowClientFactory(cfg),
		tlsConfig,
	), nil
}

func pulseMQTTTLSConfigFromEnv(ctx context.Context, baseURL string, log *slog.Logger) (*tls.Config, error) {
	serverName := strings.TrimSpace(runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_TLS_SERVER_NAME", defaultPulseMQTTBrokerHost))
	caPEMEnv := strings.TrimSpace(runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_TLS_CA_PEM", ""))
	caPEM := caPEMEnv
	if caPEM == "" {
		var err error
		caPEM, err = fetchPulseMQTTCABundle(ctx, baseURL)
		if err != nil {
			return nil, err
		}
	}

	if log != nil {
		log.Debug("pulse mqtt emulator tls trust configured",
			slog.String("server_name", serverName),
			slog.Bool("ca_from_env", caPEMEnv != ""),
		)
	}
	pool, err := systemCertPool()
	if err != nil {
		return nil, err
	}
	if !pool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, fmt.Errorf("parse pulse mqtt emulator ca bundle")
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    pool,
		ServerName: serverName,
	}, nil
}

func fetchPulseMQTTCABundle(ctx context.Context, baseURL string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", errors.New("pulse mqtt emulator base url is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+pulseMQTTCABundlePath, nil)
	if err != nil {
		return "", fmt.Errorf("build pulse mqtt ca request: %w", err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch pulse mqtt ca bundle: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch pulse mqtt ca bundle: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read pulse mqtt ca bundle: %w", err)
	}
	caPEM := strings.TrimSpace(string(body))
	if caPEM == "" {
		return "", errors.New("pulse mqtt ca bundle is empty")
	}
	return caPEM, nil
}

func systemCertPool() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system cert pool: %w", err)
	}
	if pool == nil {
		pool = x509.NewCertPool()
	}
	return pool, nil
}
