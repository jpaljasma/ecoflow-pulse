package provideradapter

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

func TestEcoFlowAdapterGetMQTTCertificationSeededSNsIntegration(t *testing.T) {
	if os.Getenv("ECOFLOW_ADAPTER_INTEGRATION") != "1" {
		t.Skip("set ECOFLOW_ADAPTER_INTEGRATION=1 to run integration test")
	}

	accessKey := strings.TrimSpace(os.Getenv("ECOFLOW_DEV_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("ECOFLOW_DEV_SECRET_KEY"))
	if accessKey == "" || secretKey == "" {
		t.Skip("ECOFLOW_DEV_ACCESS_KEY and ECOFLOW_DEV_SECRET_KEY are required")
	}

	seedSNs := parseSeedSNs(strings.TrimSpace(os.Getenv("ECOFLOW_DEV_SEED_SNS")))
	if len(seedSNs) == 0 {
		seedSNs = []string{"R351ZABAPH331057", "Y711ZABA9H2P0294"}
	}

	cfg := ecoflow.DefaultConfig()
	cfg.Logging.Debug = false
	cfg.Logging.AdvancedDebugTelemetry = false
	cfg.Logging.DebugLogHeaders = false
	cfg.Logging.Logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	adapter := NewEcoFlowAdapter(NewDefaultEcoFlowClientFactory(cfg))
	credential := controlplane.ProviderCredential{
		ID:        "dev-seed",
		Provider:  controlplane.ProviderEcoFlow,
		AccessKey: accessKey,
		SecretKey: secretKey,
		IsActive:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	for _, sn := range seedSNs {
		cert, err := adapter.GetMQTTCertification(ctx, credential, sn)
		if err != nil {
			t.Fatalf("GetMQTTCertification(%q) failed: %v", sn, err)
		}
		if strings.TrimSpace(cert.URL) == "" || strings.TrimSpace(cert.Port) == "" {
			t.Fatalf("empty endpoint data for %q: %#v", sn, cert)
		}
		if strings.TrimSpace(cert.CertificateAccount) == "" || strings.TrimSpace(cert.CertificatePassword) == "" {
			t.Fatalf("empty auth data for %q: %#v", sn, cert)
		}
	}
}

func parseSeedSNs(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		sn := normalizeProviderDeviceID(part)
		if sn == "" {
			continue
		}
		if _, exists := seen[sn]; exists {
			continue
		}
		seen[sn] = struct{}{}
		out = append(out, sn)
	}
	return out
}
