package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/dbpool"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
)

type smokeConfig struct {
	email        string
	password     string
	region       string
	configJSON   string
	dbDSN        string
	credentialID string
	targetSuffix string
	timeout      time.Duration
	mqttEnabled  bool
	mqttQoS      uint
}

func main() {
	cfg := parseFlags()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	credential, err := resolvePecronCredential(ctx, cfg)
	if err != nil {
		log.Fatalf("config failed: %v", err)
	}
	adapter := provideradapter.NewPecronAdapter(provideradapter.PecronAdapterConfig{})

	devices, err := adapter.DiscoverDevices(ctx, credential)
	if err != nil {
		log.Fatalf("discover failed: %v", err)
	}
	fmt.Printf("discover: count=%d region=%s\n", len(devices), credentialRegion(credential.Config))
	printDiscoveredDevices(devices)
	if len(devices) == 0 {
		return
	}

	target, err := selectTargetDevice(devices, cfg.targetSuffix)
	if err != nil {
		log.Fatalf("target selection failed: %v", err)
	}
	fmt.Printf(
		"target: provider_device=%s canonical_sn=%s model=%q online=%v\n",
		maskProviderDeviceID(target.ProviderDeviceID),
		maskCanonicalSN(target.CanonicalSN),
		target.Model,
		target.Metadata["online"],
	)

	runSnapshotSmoke(ctx, adapter, credential, target.ProviderDeviceID)
	if cfg.mqttEnabled {
		runMQTTSmoke(ctx, adapter, credential, target.ProviderDeviceID, byte(cfg.mqttQoS))
	}
}

func parseFlags() smokeConfig {
	cfg := smokeConfig{
		email:        strings.TrimSpace(os.Getenv("PECRON_EMAIL")),
		password:     os.Getenv("PECRON_PASSWORD"),
		region:       strings.TrimSpace(os.Getenv("PECRON_REGION")),
		configJSON:   strings.TrimSpace(os.Getenv("PECRON_CONFIG")),
		dbDSN:        strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN")),
		credentialID: strings.TrimSpace(os.Getenv("PECRON_CREDENTIAL_ID")),
		targetSuffix: strings.TrimSpace(os.Getenv("PECRON_TARGET_SUFFIX")),
		timeout:      90 * time.Second,
		mqttEnabled:  true,
		mqttQoS:      1,
	}
	flag.StringVar(&cfg.email, "email", cfg.email, "Pecron account email; defaults to PECRON_EMAIL")
	flag.StringVar(&cfg.region, "region", cfg.region, "Pecron cloud region (us, eu, cn); defaults to PECRON_REGION or us")
	flag.StringVar(&cfg.configJSON, "config-json", cfg.configJSON, "raw provider config JSON; defaults to PECRON_CONFIG")
	flag.StringVar(&cfg.dbDSN, "db-dsn", cfg.dbDSN, "control-plane Postgres DSN used when Pecron credentials are not provided in env")
	flag.StringVar(&cfg.credentialID, "credential-id", cfg.credentialID, "specific Pecron credential ID to load from DB; defaults to PECRON_CREDENTIAL_ID")
	flag.StringVar(&cfg.targetSuffix, "target-suffix", cfg.targetSuffix, "provider device ID or canonical SN suffix to probe")
	flag.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "overall smoke timeout")
	flag.BoolVar(&cfg.mqttEnabled, "mqtt", cfg.mqttEnabled, "run the Pecron MQTT live-feed probe")
	flag.UintVar(&cfg.mqttQoS, "mqtt-qos", cfg.mqttQoS, "MQTT QoS for the direct live-feed probe; must be 0 or 1")
	flag.Parse()
	return cfg
}

func resolvePecronCredential(ctx context.Context, cfg smokeConfig) (controlplane.ProviderCredential, error) {
	hasEmail := strings.TrimSpace(cfg.email) != ""
	hasPassword := strings.TrimSpace(cfg.password) != ""
	if cfg.mqttQoS > 1 {
		return controlplane.ProviderCredential{}, fmt.Errorf("mqtt-qos must be 0 or 1")
	}
	if strings.TrimSpace(cfg.dbDSN) != "" {
		return loadPecronCredentialFromDB(ctx, cfg)
	}
	if hasEmail && hasPassword {
		return buildEnvPecronCredential(cfg)
	}
	if hasEmail || hasPassword {
		return controlplane.ProviderCredential{}, fmt.Errorf("set both PECRON_EMAIL and PECRON_PASSWORD, or set CONTROL_PLANE_DB_DSN to load the active stored Pecron credential")
	}
	return loadPecronCredentialFromDB(ctx, cfg)
}

func buildEnvPecronCredential(cfg smokeConfig) (controlplane.ProviderCredential, error) {
	config, err := providerConfig(nil, cfg)
	if err != nil {
		return controlplane.ProviderCredential{}, err
	}
	credential := controlplane.ProviderCredential{
		Provider:  controlplane.ProviderPecron,
		AccessKey: strings.TrimSpace(cfg.email),
		SecretKey: cfg.password,
		Config:    config,
		IsActive:  true,
	}
	if credential.AccessKey == "" || strings.TrimSpace(credential.SecretKey) == "" {
		return controlplane.ProviderCredential{}, fmt.Errorf("set both PECRON_EMAIL and PECRON_PASSWORD, or set CONTROL_PLANE_DB_DSN to load the active stored Pecron credential")
	}
	return credential, nil
}

func loadPecronCredentialFromDB(ctx context.Context, cfg smokeConfig) (controlplane.ProviderCredential, error) {
	dsn := strings.TrimSpace(cfg.dbDSN)
	if dsn == "" {
		return controlplane.ProviderCredential{}, fmt.Errorf("set PECRON_EMAIL/PECRON_PASSWORD or CONTROL_PLANE_DB_DSN")
	}
	appliedDSN, err := pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return controlplane.ProviderCredential{}, fmt.Errorf("apply postgres search_path: %w", err)
	}
	db, err := sql.Open("pgx", appliedDSN)
	if err != nil {
		return controlplane.ProviderCredential{}, fmt.Errorf("open control-plane database: %w", err)
	}
	defer func() { _ = db.Close() }()
	dbpool.ConfigureSQL(db)
	if err := db.PingContext(ctx); err != nil {
		return controlplane.ProviderCredential{}, fmt.Errorf("ping control-plane database: %w", err)
	}
	credential, err := queryPecronCredential(ctx, db, strings.TrimSpace(cfg.credentialID))
	if err != nil {
		return controlplane.ProviderCredential{}, err
	}
	credential.Config, err = providerConfig(credential.Config, cfg)
	if err != nil {
		return controlplane.ProviderCredential{}, err
	}
	return credential, nil
}

func queryPecronCredential(ctx context.Context, db *sql.DB, credentialID string) (controlplane.ProviderCredential, error) {
	query := `
SELECT
	id::text,
	provider,
	access_key_mask,
	access_key_ciphertext,
	secret_key_ciphertext,
	provider_config,
	is_active
FROM provider_credentials
WHERE provider = $1
  AND is_active = TRUE
  AND (NULLIF($2, '') IS NULL OR id = NULLIF($2, '')::uuid)
ORDER BY updated_at DESC, id DESC
LIMIT 1;
`
	var credential controlplane.ProviderCredential
	var accessKeyBytes []byte
	var secretKeyBytes []byte
	var rawConfig []byte
	if err := db.QueryRowContext(ctx, query, controlplane.ProviderPecron, credentialID).Scan(
		&credential.ID,
		&credential.Provider,
		&credential.AccessKeyMask,
		&accessKeyBytes,
		&secretKeyBytes,
		&rawConfig,
		&credential.IsActive,
	); err != nil {
		if err == sql.ErrNoRows {
			if credentialID != "" {
				return controlplane.ProviderCredential{}, fmt.Errorf("active Pecron credential %s not found", credentialID)
			}
			return controlplane.ProviderCredential{}, fmt.Errorf("active Pecron credential not found")
		}
		return controlplane.ProviderCredential{}, fmt.Errorf("query active Pecron credential: %w", err)
	}
	credential.AccessKey = string(accessKeyBytes)
	credential.SecretKey = string(secretKeyBytes)
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &credential.Config); err != nil {
			return controlplane.ProviderCredential{}, fmt.Errorf("parse stored provider config: %w", err)
		}
	}
	return credential, nil
}

func providerConfig(base map[string]any, cfg smokeConfig) (map[string]any, error) {
	config := cloneAnyMap(base)
	if strings.TrimSpace(cfg.configJSON) != "" {
		var override map[string]any
		if err := json.Unmarshal([]byte(cfg.configJSON), &override); err != nil {
			return nil, fmt.Errorf("parse provider config JSON: %w", err)
		}
		for key, value := range override {
			config[key] = value
		}
	}
	if region := strings.TrimSpace(cfg.region); region != "" {
		config["region"] = region
	}
	return config, nil
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func printDiscoveredDevices(devices []controlplane.ProviderDevice) {
	for i, device := range devices {
		fmt.Printf(
			"device[%d]: provider_device=%s canonical_sn=%s name=%q model=%q online=%v\n",
			i,
			maskProviderDeviceID(device.ProviderDeviceID),
			maskCanonicalSN(device.CanonicalSN),
			device.ProductName,
			device.Model,
			device.Metadata["online"],
		)
	}
}

func selectTargetDevice(devices []controlplane.ProviderDevice, suffix string) (controlplane.ProviderDevice, error) {
	if len(devices) == 0 {
		return controlplane.ProviderDevice{}, fmt.Errorf("no Pecron devices discovered")
	}
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	if suffix == "" {
		return devices[0], nil
	}
	for _, device := range devices {
		if strings.HasSuffix(strings.ToLower(device.ProviderDeviceID), suffix) ||
			strings.HasSuffix(strings.ToLower(device.CanonicalSN), suffix) {
			return device, nil
		}
	}
	return controlplane.ProviderDevice{}, fmt.Errorf("no Pecron device ends with suffix %q", suffix)
}

func runSnapshotSmoke(
	ctx context.Context,
	adapter *provideradapter.PecronAdapter,
	credential controlplane.ProviderCredential,
	providerDeviceID string,
) {
	snapshotCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	device, snapshot, err := adapter.GetDeviceTelemetrySnapshot(snapshotCtx, credential, providerDeviceID)
	if err != nil {
		fmt.Printf("snapshot: success=false error=%v\n", err)
		return
	}
	fmt.Printf(
		"snapshot: success=true model=%q params=%d capability_keys=%d metadata_keys=%d sample_params=%s\n",
		device.Model,
		len(snapshot.Params),
		len(snapshot.Capabilities),
		len(snapshot.Metadata),
		sampleKeys(snapshot.Params, 8),
	)
}

func runMQTTSmoke(
	ctx context.Context,
	adapter *provideradapter.PecronAdapter,
	credential controlplane.ProviderCredential,
	providerDeviceID string,
	qos byte,
) {
	probeCtx, cancel := context.WithTimeout(ctx, 24*time.Second)
	defer cancel()
	result, err := adapter.ProbeMQTT(probeCtx, credential, providerDeviceID, 18*time.Second)
	if err != nil {
		fmt.Printf("mqtt_probe: success=false error=%v\n", err)
	} else {
		fmt.Printf(
			"mqtt_probe: success=%v status=%s sample_topic=%s payload_bytes=%d observed_at_unix_ms=%d\n",
			result.Success,
			result.Status,
			result.SampleTopic,
			result.PayloadBytes,
			result.ObservedAtUnixMS,
		)
	}

	session, err := adapter.MQTTSession(ctx, credential, providerDeviceID)
	if err != nil {
		fmt.Printf("mqtt_session: success=false error=%v\n", err)
		return
	}
	fmt.Printf(
		"mqtt_session: success=true brokers=%v path=%s topics=%v\n",
		session.BrokerAddresses(),
		session.Path,
		redactTopics(session.Topics),
	)
	for _, address := range session.BrokerAddresses() {
		trySubscribe(ctx, session, address, qos)
	}
}

func trySubscribe(parent context.Context, session pecron.MQTTSession, address string, qos byte) {
	ctx, cancel := context.WithTimeout(parent, 18*time.Second)
	defer cancel()
	subscriber, err := pecron.NewMQTTSubscriber(pecron.MQTTConfig{
		Address:        address,
		Path:           session.Path,
		Token:          session.Token,
		ClientID:       session.ClientID,
		KeepAlive:      90 * time.Second,
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    6 * time.Second,
	})
	if err != nil {
		fmt.Printf("mqtt_subscribe: broker=%s qos=%d success=false phase=init error=%v\n", address, qos, err)
		return
	}
	defer func() { _ = subscriber.Close() }()
	if err := subscriber.Connect(ctx); err != nil {
		fmt.Printf("mqtt_subscribe: broker=%s qos=%d success=false phase=connect error=%v\n", address, qos, err)
		return
	}
	fmt.Printf("mqtt_subscribe: broker=%s qos=%d phase=connect success=true topics=%v\n", address, qos, redactTopics(session.Topics))
	if err := subscriber.SubscribeMultiple(ctx, session.Topics, qos); err != nil {
		fmt.Printf("mqtt_subscribe: broker=%s qos=%d success=false phase=subscribe error=%v\n", address, qos, err)
		return
	}
	fmt.Printf("mqtt_subscribe: broker=%s qos=%d phase=subscribe success=true\n", address, qos)
	publishTopic := pecron.MQTTPublishTopic(session.Ref)
	if publishTopic != "" {
		if err := subscriber.Publish(ctx, publishTopic, pecron.TTLVReadPacket(1), qos); err != nil {
			fmt.Printf("mqtt_subscribe: broker=%s qos=%d success=false phase=publish error=%v\n", address, qos, err)
			return
		}
		fmt.Printf("mqtt_subscribe: broker=%s qos=%d phase=publish success=true topic=%s\n", address, qos, redactTopic(publishTopic))
	}
	msg, err := subscriber.ReadMessage(ctx)
	if err != nil {
		fmt.Printf("mqtt_subscribe: broker=%s qos=%d success=false phase=read error=%v\n", address, qos, err)
		return
	}
	fmt.Printf("mqtt_subscribe: broker=%s qos=%d success=true phase=read topic=%s payload_bytes=%d\n", address, qos, redactTopic(msg.Topic), len(msg.Payload))
}

func credentialRegion(config map[string]any) string {
	if raw, ok := config["region"]; ok {
		text := strings.TrimSpace(fmt.Sprint(raw))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return "us(default)"
}

func maskProviderDeviceID(value string) string {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return maskTail(value)
	}
	return strings.ToLower(parts[0]) + ":" + maskTail(parts[1])
}

func maskCanonicalSN(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) < 3 {
		return maskTail(value)
	}
	parts[len(parts)-1] = strings.ToUpper(maskTail(parts[len(parts)-1]))
	return strings.Join(parts, "-")
}

func maskTail(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return "..." + value
	}
	return "..." + value[len(value)-4:]
}

func redactTopics(topics []string) []string {
	out := make([]string, 0, len(topics))
	for _, topic := range topics {
		out = append(out, redactTopic(topic))
	}
	return out
}

func redactTopic(topic string) string {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) >= 5 && parts[0] == "q" {
		parts[3] = "redacted"
		return strings.Join(parts, "/")
	}
	return "redacted"
}

func sampleKeys(values map[string]any, limit int) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return strings.Join(keys, ",")
}
