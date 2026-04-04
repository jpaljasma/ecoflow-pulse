package pulsemqttemulator

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

func TestServerSupportsEcoFlowCompatibleDiscoveryAndMQTT(t *testing.T) {
	t.Parallel()

	server, err := NewServer(Config{
		HTTPAddr:        "127.0.0.1:0",
		MQTTAddr:        "127.0.0.1:0",
		AccessKey:       "pulse-test-ak",
		SecretKey:       "pulse-test-sk",
		MQTTUsername:    "open-pulse-test-account",
		MQTTPassword:    "pulse-test-password",
		PublishInterval: 2 * time.Second,
		Device: DeviceConfig{
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

	cfg := ecoflow.DefaultConfig()
	cfg.BaseURL = server.BaseURL()
	credentialsProvider, err := ecoflow.NewStaticCredentialsProvider("pulse-test-ak", "pulse-test-sk")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}
	cfg.CredentialsProvider = credentialsProvider

	adapter := provideradapter.NewEcoFlowCompatibleAdapter(
		controlplane.ProviderPulseMQTT,
		provideradapter.NewDefaultEcoFlowClientFactory(cfg),
		&tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true},
	)
	credential := controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderPulseMQTT,
		AccessKey: "pulse-test-ak",
		SecretKey: "pulse-test-sk",
		IsActive:  true,
	}

	devices, err := adapter.DiscoverDevices(context.Background(), credential)
	if err != nil {
		t.Fatalf("DiscoverDevices() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("discover count = %d, want 1", len(devices))
	}
	if got := devices[0].Model; got != "DELTA Pro Ultra X" {
		t.Fatalf("device model = %q, want DELTA Pro Ultra X", got)
	}

	_, quota, err := adapter.GetDeviceQuotaSnapshot(context.Background(), credential, devices[0].ProviderDeviceID)
	if err != nil {
		t.Fatalf("GetDeviceQuotaSnapshot() error = %v", err)
	}
	if !strings.Contains(quota["hs_yj751_pd_bp_addr.bpInfo"], "\"bpNo\":4") {
		t.Fatalf("expected 4 battery packs in quota snapshot, got %q", quota["hs_yj751_pd_bp_addr.bpInfo"])
	}

	cert, err := adapter.GetMQTTCertification(context.Background(), credential, devices[0].ProviderDeviceID)
	if err != nil {
		t.Fatalf("GetMQTTCertification() error = %v", err)
	}
	address, topic, err := provideradapter.BuildMQTTAddressAndTopic(cert, devices[0].ProviderDeviceID)
	if err != nil {
		t.Fatalf("BuildMQTTAddressAndTopic() error = %v", err)
	}
	subscriber, err := ecoflowmqtt.NewSubscriber(ecoflowmqtt.Config{
		Address:        address,
		Username:       cert.CertificateAccount,
		Password:       cert.CertificatePassword,
		ClientID:       "pulse-mqtt-emulator-test",
		KeepAlive:      30 * time.Second,
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		TLSConfig:      adapter.MQTTTLSConfig(),
	})
	if err != nil {
		t.Fatalf("NewSubscriber() error = %v", err)
	}
	defer func() { _ = subscriber.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := subscriber.Subscribe(ctx, topic, 0); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	msg, err := subscriber.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if msg.Topic != topic {
		t.Fatalf("message topic = %q, want %q", msg.Topic, topic)
	}
	if !strings.Contains(string(msg.Payload), "\"addr\":\"hs_yj751_pd_appshow_addr\"") {
		t.Fatalf("unexpected mqtt payload: %s", string(msg.Payload))
	}
}

func TestMQTTIdleReadTimeoutHonorsKeepAliveBudget(t *testing.T) {
	t.Parallel()

	got := mqttIdleReadTimeout(90, 5*time.Second)
	if got < 180*time.Second {
		t.Fatalf("mqttIdleReadTimeout() = %s, want at least 180s", got)
	}
}

func TestServerPublishesBrokerCABundle(t *testing.T) {
	t.Parallel()

	server, err := NewServer(Config{
		HTTPAddr:     "127.0.0.1:0",
		MQTTAddr:     "127.0.0.1:0",
		AccessKey:    "pulse-test-ak",
		SecretKey:    "pulse-test-sk",
		MQTTUsername: "open-pulse-test-account",
		MQTTPassword: "pulse-test-password",
		Device: DeviceConfig{
			SN: "PULSEDPUX24K001",
		},
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	resp, err := http.Get(server.BaseURL() + pulseMQTTCABundlePath)
	if err != nil {
		t.Fatalf("GET mqtt ca bundle error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	pemBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if !strings.Contains(string(pemBody), "BEGIN CERTIFICATE") {
		t.Fatalf("ca bundle missing certificate pem: %q", string(pemBody))
	}
}

func TestMQTTPublishPacketCapacityRejectsOverflow(t *testing.T) {
	t.Parallel()

	maxInt := int(^uint(0) >> 1)
	if _, err := mqttPublishPacketCapacity(maxInt, 16); err == nil {
		t.Fatal("mqttPublishPacketCapacity() error = nil, want overflow guard")
	}
}

func TestSnapshotForTickEmulatesDpuXSolarAndLoadProfile(t *testing.T) {
	t.Parallel()

	snapshot := snapshotForTime(0, time.Date(2026, time.March, 29, 13, 10, 0, 0, time.Local))
	if snapshot.InLvMpptVol < 300 || snapshot.InLvMpptVol > 339 {
		t.Fatalf("InLvMpptVol = %.2f, want DPU-X series-string range", snapshot.InLvMpptVol)
	}
	if snapshot.InLvMpptAmp < 6 || snapshot.InLvMpptAmp > 15 {
		t.Fatalf("InLvMpptAmp = %.2f, want series-string current in range", snapshot.InLvMpptAmp)
	}
	if snapshot.WattsInSum < 2500 || snapshot.WattsInSum > 4500 {
		t.Fatalf("WattsInSum = %.2f, want 7x590-style PV production within port limits", snapshot.WattsInSum)
	}
	if snapshot.InHvMpptVol != 0 || snapshot.InHvMpptAmp != 0 {
		t.Fatalf("secondary PV port = %.2fV %.2fA, want idle second port", snapshot.InHvMpptVol, snapshot.InHvMpptAmp)
	}
	if snapshot.ACOutputWatts < 450 || snapshot.ACOutputWatts > 1200 {
		t.Fatalf("ACOutputWatts = %.2f, want appliance profile within requested band", snapshot.ACOutputWatts)
	}
	if snapshot.DCOutputWatts < 5 || snapshot.DCOutputWatts > 15 {
		t.Fatalf("DCOutputWatts = %.2f, want small auxiliary DC load", snapshot.DCOutputWatts)
	}
	if snapshot.BMSInputWatts <= 0 {
		t.Fatalf("BMSInputWatts = %.2f, want battery charging under midday solar", snapshot.BMSInputWatts)
	}
	if snapshot.BackupSOC != 40 || snapshot.DischargeMinSOC != 40 {
		t.Fatalf("backup/discharge reserve = %d/%d, want 40/40", snapshot.BackupSOC, snapshot.DischargeMinSOC)
	}
	if snapshot.ACOutputWatts <= 0 {
		t.Fatalf("ACOutputWatts = %.2f, want DPU-X AC output always on", snapshot.ACOutputWatts)
	}
	if len(snapshot.PackWatts) != len(snapshot.PackSOCs) {
		t.Fatalf("len(PackWatts) = %d, want %d", len(snapshot.PackWatts), len(snapshot.PackSOCs))
	}
	if snapshot.PackWatts[len(snapshot.PackWatts)-1] <= snapshot.PackWatts[0] {
		t.Fatalf("PackWatts = %v, want lower-SOC pack to absorb slightly more charge", snapshot.PackWatts)
	}
}

func TestBuildPowerModelUsesACAssistAtBackupReserve(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.March, 29, 21, 5, 0, 0, time.Local)
	model := buildPowerModel(at, TickForTime(at, defaultPublishInterval), batterySimState{
		SOC: dpuXBackupReserveSOC,
	}, dpuXSimulationStep)
	if model.ACInputWatts <= 0 {
		t.Fatalf("ACInputWatts = %.2f, want AC reserve assist when battery is at backup floor", model.ACInputWatts)
	}
	if model.BatteryNetWatts < -1 {
		t.Fatalf("BatteryNetWatts = %.2f, want reserve floor held by AC assist", model.BatteryNetWatts)
	}
}

func TestSnapshotForTickRandomizesShortPreconditioningBursts(t *testing.T) {
	t.Parallel()

	for tick := 0; tick < 240; tick++ {
		snapshot := snapshotForTime(tick, time.Date(2026, time.March, 29, 13, 10, 0, 0, time.Local))
		if snapshot.BMSModeSet == 0 || snapshot.BatteryHeatMode == 0 {
			continue
		}
		foundPack := false
		for _, heatTime := range snapshot.PackHeatTimes {
			if heatTime == 0 {
				continue
			}
			foundPack = true
			if heatTime < 30 || heatTime > 45 {
				t.Fatalf("tick %d heatTime = %d, want 30-45 seconds", tick, heatTime)
			}
		}
		if !foundPack {
			t.Fatalf("tick %d has system preconditioning without any pack activity", tick)
		}
		return
	}
	t.Fatal("expected at least one preconditioning burst")
}

func TestSnapshotForTimeAddsSmoothCloudCoverageVariability(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 29, 13, 10, 0, 0, time.Local)
	minWatts := math.MaxFloat64
	maxWatts := 0.0
	maxStep := 0.0
	prevWatts := 0.0
	for tick := 0; tick < 240; tick++ {
		snapshot := snapshotForTime(tick, now)
		if snapshot.WattsInSum < minWatts {
			minWatts = snapshot.WattsInSum
		}
		if snapshot.WattsInSum > maxWatts {
			maxWatts = snapshot.WattsInSum
		}
		if tick > 0 {
			maxStep = math.Max(maxStep, math.Abs(snapshot.WattsInSum-prevWatts))
		}
		prevWatts = snapshot.WattsInSum
	}
	if maxWatts-minWatts < 900 {
		t.Fatalf("midday PV spread = %.2fW, want mixed-cloud variability across the day", maxWatts-minWatts)
	}
	if maxStep > 650 {
		t.Fatalf("midday PV step = %.2fW, want cloud transitions without sharp spikes", maxStep)
	}
}

func TestAdvanceBatteryStateAppliesMPPTHysteresisWindow(t *testing.T) {
	t.Parallel()

	charging := advanceBatteryState(batterySimState{SOC: 94.8, SolarLimited: false}, 900, 30*time.Minute)
	if !charging.SolarLimited {
		t.Fatal("expected solar limiting to engage at the 95% window max")
	}
	if charging.SOC != dpuXChargeMaxSOC {
		t.Fatalf("charging SOC = %.2f, want clamp at %.2f", charging.SOC, dpuXChargeMaxSOC)
	}

	resumed := advanceBatteryState(charging, -1800, 20*time.Minute)
	if resumed.SOC > dpuXChargeResumeSOC {
		t.Fatalf("resumed SOC = %.2f, want discharge below resume threshold", resumed.SOC)
	}
	if resumed.SolarLimited {
		t.Fatal("expected solar limiting to clear once SOC drops below 93%")
	}
}

func TestBuildMQTTFramesExposeAcAndDcLoads(t *testing.T) {
	t.Parallel()

	frames := buildMQTTFrames(snapshotForTick(1))
	if len(frames) == 0 {
		t.Fatal("buildMQTTFrames() returned no payloads")
	}
	var appshow struct {
		Addr   string         `json:"addr"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(frames[0], &appshow); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if appshow.Addr != "hs_yj751_pd_appshow_addr" {
		t.Fatalf("addr = %q, want hs_yj751_pd_appshow_addr", appshow.Addr)
	}
	if got := appshow.Params["outAcTtPwr"]; got == nil {
		t.Fatal("outAcTtPwr missing from appshow payload")
	}
	if got := appshow.Params["outAdsPwr"]; got == nil {
		t.Fatal("outAdsPwr missing from appshow payload")
	}
}

func TestMQTTFramesAtExposeStableReplayMetadata(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.April, 3, 12, 5, 0, 0, time.UTC)
	frames := MQTTFramesAt(at, 5*time.Second)
	if len(frames) == 0 {
		t.Fatal("MQTTFramesAt() returned no payloads")
	}
	var payload struct {
		ID   string `json:"id"`
		Time int64  `json:"time"`
	}
	if err := json.Unmarshal(frames[0], &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.ID == "" {
		t.Fatal("expected stable message id in emulator frame")
	}
	if payload.Time != at.UnixMilli() {
		t.Fatalf("payload time = %d, want %d", payload.Time, at.UnixMilli())
	}
}

func TestReplayTimesIncludeClosingSample(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Minute)
	got := replayTimes(from, to, time.Minute)
	if len(got) != 3 {
		t.Fatalf("len(replayTimes) = %d, want 3", len(got))
	}
	if !got[2].Equal(to.Add(-time.Millisecond)) {
		t.Fatalf("closing replay sample = %s, want %s", got[2], to.Add(-time.Millisecond))
	}
}

func TestReplayEndpointPublishesHistoricalFrames(t *testing.T) {
	t.Parallel()

	server, err := NewServer(Config{
		HTTPAddr:        "127.0.0.1:0",
		MQTTAddr:        "127.0.0.1:0",
		AccessKey:       "pulse-test-ak",
		SecretKey:       "pulse-test-sk",
		MQTTUsername:    "open-pulse-test-account",
		MQTTPassword:    "pulse-test-password",
		PublishInterval: time.Hour,
		Device: DeviceConfig{
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

	subscriber, err := ecoflowmqtt.NewSubscriber(ecoflowmqtt.Config{
		Address:        strings.TrimPrefix(server.BrokerAddress(), "[::]"),
		Username:       "open-pulse-test-account",
		Password:       "pulse-test-password",
		ClientID:       "pulse-mqtt-replay-test",
		KeepAlive:      30 * time.Second,
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		TLSConfig:      &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true},
	})
	if err != nil {
		t.Fatalf("NewSubscriber() error = %v", err)
	}
	defer func() { _ = subscriber.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	topic := TopicName("open-pulse-test-account", "PULSEDPUX24K001")
	if err := subscriber.Subscribe(ctx, topic, 0); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if _, err := subscriber.ReadMessage(ctx); err != nil {
		t.Fatalf("ReadMessage() initial snapshot error = %v", err)
	}

	replayFrom := time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC)
	replayTo := replayFrom.Add(2 * time.Minute)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		server.BaseURL()+"/replay?from="+replayFrom.Format(time.RFC3339Nano)+"&to="+replayTo.Format(time.RFC3339Nano)+"&step=1m",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http replay request error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("replay status = %d, want 200", resp.StatusCode)
	}

	for i := 0; i < 8; i++ {
		msg, err := subscriber.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("ReadMessage() replay error = %v", err)
		}
		var payload struct {
			Time int64 `json:"time"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("json.Unmarshal() replay payload error = %v", err)
		}
		if payload.Time == replayFrom.UnixMilli() {
			return
		}
	}
	t.Fatal("expected replay payload with historical time")
}
