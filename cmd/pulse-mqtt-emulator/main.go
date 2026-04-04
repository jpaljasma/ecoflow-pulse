package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/pulsemqttemulator"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

const (
	defaultPulseMQTTAccessKey    = "pulse-mqtt-local-ak"
	defaultPulseMQTTSecretKey    = "pulse-mqtt-local-sk"
	defaultPulseMQTTMQTTUser     = "open-pulse-local-account"
	defaultPulseMQTTMQTTPassword = "pulse-mqtt-local-password"
	defaultPulseMQTTDeviceSN     = "PULSEDPUX24K001"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("pulse-mqtt-emulator")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	logCfg.AsyncEnabled = !runtimecfg.Bool("LOG_ASYNC_DISABLED", false)
	logCfg.AsyncQueueSize = runtimecfg.IntMin("LOG_ASYNC_QUEUE_SIZE", logCfg.AsyncQueueSize, 128)
	logCfg.AsyncBypassLevel = pulselog.ParseLevel(runtimecfg.EnvOrDefault("LOG_ASYNC_BYPASS_LEVEL", "warn"), slog.LevelWarn)

	log, asyncLogHandler, err := pulselog.BuildServiceLogger(logCfg)
	if err != nil {
		_, _ = os.Stderr.WriteString("init logger failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		if asyncLogHandler != nil {
			asyncLogHandler.Close()
		}
	}()

	server, err := pulsemqttemulator.NewServer(pulsemqttemulator.Config{
		HTTPAddr:            runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_HTTP_ADDR", ":8080"),
		MQTTAddr:            runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_MQTT_ADDR", ":8883"),
		BrokerAdvertiseHost: runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_BROKER_HOST", "pulse-services-pulse-mqtt-emulator.pulse-services.svc.cluster.local"),
		BrokerAdvertisePort: runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_BROKER_PORT", "8883"),
		AccessKey:           runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_ACCESS_KEY", defaultPulseMQTTAccessKey),
		SecretKey:           runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_SECRET_KEY", defaultPulseMQTTSecretKey),
		MQTTUsername:        runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_MQTT_USERNAME", defaultPulseMQTTMQTTUser),
		MQTTPassword:        runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_MQTT_PASSWORD", defaultPulseMQTTMQTTPassword),
		PublishInterval:     runtimecfg.DurationPositive("PULSE_MQTT_EMULATOR_PUBLISH_INTERVAL", 5*time.Second),
		Device: pulsemqttemulator.DeviceConfig{
			SN:                  strings.ToUpper(strings.TrimSpace(runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_DEVICE_SN", defaultPulseMQTTDeviceSN))),
			DeviceName:          runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_DEVICE_NAME", "DPU-X 24 kWh"),
			ProductName:         runtimecfg.EnvOrDefault("PULSE_MQTT_EMULATOR_DEVICE_MODEL", "DELTA Pro Ultra X"),
			BatteryPackCount:    runtimecfg.IntPositive("PULSE_MQTT_EMULATOR_BATTERY_PACK_COUNT", 4),
			BatteryPackEnergyWh: runtimecfg.Float64NonNegative("PULSE_MQTT_EMULATOR_BATTERY_PACK_ENERGY_WH", 6144),
		},
		Logger: log,
	})
	if err != nil {
		log.Error("init pulse mqtt emulator failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := server.Start(); err != nil {
		log.Error("start pulse mqtt emulator failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = server.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()

	log.Info("pulse mqtt emulator shutting down")
	if err := server.Close(); err != nil {
		log.Warn("close pulse mqtt emulator failed", slog.String("error", err.Error()))
	}
}
