package provideradapter

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/pulsemqttemulator"
)

func TestPulseMQTTTLSConfigFromEnvFetchesTrustedBrokerCA(t *testing.T) {
	server, err := pulsemqttemulator.NewServer(pulsemqttemulator.Config{
		HTTPAddr:        "127.0.0.1:0",
		MQTTAddr:        "127.0.0.1:0",
		AccessKey:       "pulse-test-ak",
		SecretKey:       "pulse-test-sk",
		MQTTUsername:    "open-pulse-test-account",
		MQTTPassword:    "pulse-test-password",
		PublishInterval: 2 * time.Second,
		Device: pulsemqttemulator.DeviceConfig{
			SN:                  "PULSEDPUX24K001",
			DeviceName:          "DPU-X 24 kWh",
			ProductName:         "DELTA Pro Ultra X",
			BatteryPackCount:    4,
			BatteryPackEnergyWh: 6144,
		},
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	host, _, err := net.SplitHostPort(server.BrokerAddress())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	t.Setenv("PULSE_MQTT_EMULATOR_TLS_CA_PEM", "")
	t.Setenv("PULSE_MQTT_EMULATOR_TLS_SERVER_NAME", host)

	cfg, err := pulseMQTTTLSConfigFromEnv(context.Background(), server.BaseURL(), nil)
	if err != nil {
		t.Fatalf("pulseMQTTTLSConfigFromEnv() error = %v", err)
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("pulseMQTTTLSConfigFromEnv() returned InsecureSkipVerify=true, want verified trust chain")
	}
	if cfg.RootCAs == nil {
		t.Fatal("pulseMQTTTLSConfigFromEnv() returned nil RootCAs")
	}

	conn, err := tls.Dial("tcp", server.BrokerAddress(), cfg)
	if err != nil {
		t.Fatalf("tls.Dial() error = %v", err)
	}
	_ = conn.Close()
}
