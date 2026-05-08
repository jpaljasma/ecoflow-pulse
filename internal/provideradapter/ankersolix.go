package provideradapter

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ankersolix"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

const defaultAnkerSolixRealtimeTriggerTimeout = 300 * time.Second

var errAnkerSolixDeviceNotEnableable = errors.New("anker solix device is not enableable")

type ankerSolixCloudClient interface {
	Login(ctx context.Context, email, password string) (ankersolix.Session, error)
	ListBindDevices(ctx context.Context, session ankersolix.Session) ([]ankersolix.Device, error)
	GetMQTTInfo(ctx context.Context, session ankersolix.Session) (ankersolix.MQTTSessionInfo, error)
}

type AnkerSolixClientFactory interface {
	NewClient(cfg ankersolix.Config) (ankerSolixCloudClient, error)
}

type AnkerSolixAdapterConfig struct {
	ClientFactory         AnkerSolixClientFactory
	MQTTSubscriberFactory ankerSolixMQTTSubscriberFactory
	DecodeMQTTMessage     AnkerSolixMessageDecoder
	Now                   func() time.Time
}

type AnkerSolixAdapter struct {
	factory       AnkerSolixClientFactory
	newSubscriber ankerSolixMQTTSubscriberFactory
	decodeMQTT    AnkerSolixMessageDecoder
	now           func() time.Time
}

type runtimeAnkerSolixClientFactory struct{}

func (runtimeAnkerSolixClientFactory) NewClient(cfg ankersolix.Config) (ankerSolixCloudClient, error) {
	return ankersolix.NewClient(cfg, nil), nil
}

type staticAnkerSolixClientFactory struct {
	client ankerSolixCloudClient
}

func StaticAnkerSolixClientFactory(client ankerSolixCloudClient) AnkerSolixClientFactory {
	return staticAnkerSolixClientFactory{client: client}
}

func (f staticAnkerSolixClientFactory) NewClient(ankersolix.Config) (ankerSolixCloudClient, error) {
	if f.client == nil {
		return nil, errors.New("anker solix client is required")
	}
	return f.client, nil
}

type ankerSolixMQTTSubscriber interface {
	Connect(ctx context.Context) error
	Subscribe(ctx context.Context, topic string, qos byte) error
	Publish(ctx context.Context, topic string, payload []byte, qos byte) error
	ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error)
	Close() error
}

type ankerSolixMQTTSubscriberFactory func(cfg ankersolix.MQTTConfig) (ankerSolixMQTTSubscriber, error)

type AnkerSolixDecodedMessage struct {
	Ref    ankersolix.DeviceRef
	Values map[string]any
}

type AnkerSolixMessageDecoder func(topic string, payload []byte) (AnkerSolixDecodedMessage, error)

func NewAnkerSolixAdapter(cfg AnkerSolixAdapterConfig) *AnkerSolixAdapter {
	if cfg.ClientFactory == nil {
		cfg.ClientFactory = runtimeAnkerSolixClientFactory{}
	}
	if cfg.MQTTSubscriberFactory == nil {
		cfg.MQTTSubscriberFactory = func(mqttCfg ankersolix.MQTTConfig) (ankerSolixMQTTSubscriber, error) {
			return ankersolix.NewMQTTSubscriber(mqttCfg)
		}
	}
	if cfg.DecodeMQTTMessage == nil {
		cfg.DecodeMQTTMessage = DecodeAnkerSolixMQTTMessage
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &AnkerSolixAdapter{
		factory:       cfg.ClientFactory,
		newSubscriber: cfg.MQTTSubscriberFactory,
		decodeMQTT:    cfg.DecodeMQTTMessage,
		now:           cfg.Now,
	}
}

func NewRuntimeAnkerSolixAdapter() (*AnkerSolixAdapter, error) {
	return NewAnkerSolixAdapter(AnkerSolixAdapterConfig{}), nil
}

func (a *AnkerSolixAdapter) DiscoverDevices(ctx context.Context, credential controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error) {
	client, session, cfg, err := a.sessionForCredential(ctx, credential)
	if err != nil {
		return nil, err
	}
	devices, err := client.ListBindDevices(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("list anker solix devices: %w", err)
	}
	out := make([]controlplane.ProviderDevice, 0, len(devices))
	seen := map[string]struct{}{}
	for i := range devices {
		mapped, ok := mapAnkerSolixDevice(devices[i], credential.ID, cfg)
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

func (a *AnkerSolixAdapter) GetDeviceTelemetrySnapshot(
	ctx context.Context,
	credential controlplane.ProviderCredential,
	providerDeviceID string,
) (controlplane.ProviderDevice, ankersolix.NormalizedTelemetry, error) {
	client, session, cfg, err := a.sessionForCredential(ctx, credential)
	if err != nil {
		return controlplane.ProviderDevice{}, ankersolix.NormalizedTelemetry{}, err
	}
	device, err := a.describeDevice(ctx, client, session, credential.ID, cfg, providerDeviceID)
	if err != nil {
		return controlplane.ProviderDevice{}, ankersolix.NormalizedTelemetry{}, err
	}
	return device, ankersolix.NormalizedTelemetry{
		Capabilities: cloneAnyMap(device.Capabilities),
		Metadata:     cloneAnyMap(device.Metadata),
		ObservedAt:   a.now().UTC(),
	}, nil
}

func (a *AnkerSolixAdapter) MQTTSession(
	ctx context.Context,
	credential controlplane.ProviderCredential,
	providerDeviceID string,
) (AnkerSolixMQTTSession, error) {
	client, session, cfg, err := a.sessionForCredential(ctx, credential)
	if err != nil {
		return AnkerSolixMQTTSession{}, err
	}
	device, err := a.describeDevice(ctx, client, session, credential.ID, cfg, providerDeviceID)
	if err != nil {
		return AnkerSolixMQTTSession{}, err
	}
	if !deviceAnkerSolixEnableable(device) {
		return AnkerSolixMQTTSession{}, errAnkerSolixDeviceNotEnableable
	}
	info, err := client.GetMQTTInfo(ctx, session)
	if err != nil {
		return AnkerSolixMQTTSession{}, fmt.Errorf("get anker solix mqtt info: %w", err)
	}
	tlsConfig, err := info.TLSConfig()
	if err != nil {
		return AnkerSolixMQTTSession{}, fmt.Errorf("build anker solix mqtt tls config: %w", err)
	}
	ref, err := ankersolix.ParseProviderDeviceID(providerDeviceID)
	if err != nil {
		return AnkerSolixMQTTSession{}, err
	}
	return newAnkerSolixMQTTSession(info, ref, tlsConfig), nil
}

func (a *AnkerSolixAdapter) ProbeMQTT(
	ctx context.Context,
	credential controlplane.ProviderCredential,
	providerDeviceID string,
	timeout time.Duration,
) (MQTTProbeResult, error) {
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	session, err := a.MQTTSession(probeCtx, credential, providerDeviceID)
	if err != nil {
		if errors.Is(err, errAnkerSolixDeviceNotEnableable) {
			return MQTTProbeResult{Status: "unsupported_model"}, nil
		}
		return MQTTProbeResult{}, err
	}
	subscriber, connectedAddress, err := a.connectProbeSubscriber(probeCtx, session)
	if err != nil {
		return MQTTProbeResult{Status: mqttProbeStatus(err, "connect_failed")}, nil
	}
	defer func() { _ = subscriber.Close() }()
	if err := publishAnkerSolixRealtimeTrigger(probeCtx, subscriber, session, defaultAnkerSolixRealtimeTriggerTimeout, a.now().UTC()); err != nil {
		return MQTTProbeResult{Status: mqttProbeStatus(err, "publish_failed")}, nil
	}
	for {
		msg, readErr := subscriber.ReadMessage(probeCtx)
		if readErr != nil {
			return MQTTProbeResult{Status: mqttProbeStatus(readErr, "no_messages")}, nil
		}
		if _, err := a.decodeMQTT(strings.TrimSpace(msg.Topic), msg.Payload); err != nil {
			continue
		}
		_ = connectedAddress
		return MQTTProbeResult{
			Success:          true,
			Status:           "ok",
			SampleTopic:      redactAnkerSolixTopic(msg.Topic),
			PayloadBytes:     int64(len(msg.Payload)),
			ObservedAtUnixMS: a.now().UTC().UnixMilli(),
		}, nil
	}
}

func (a *AnkerSolixAdapter) connectProbeSubscriber(ctx context.Context, session AnkerSolixMQTTSession) (ankerSolixMQTTSubscriber, string, error) {
	addresses := session.BrokerAddresses()
	if len(addresses) == 0 {
		return nil, "", errors.New("anker solix mqtt session has no broker addresses")
	}
	var lastErr error
	for _, address := range addresses {
		subscriber, err := a.newSubscriber(ankersolix.MQTTConfig{
			Address:        address,
			ClientID:       session.ClientID,
			KeepAlive:      90 * time.Second,
			ConnectTimeout: 10 * time.Second,
			ReadTimeout:    10 * time.Second,
			TLSConfig:      session.TLSConfig,
			BufferSize:     32,
		})
		if err != nil {
			lastErr = fmt.Errorf("init anker solix mqtt probe subscriber for %s: %w", address, err)
			continue
		}
		if err := subscriber.Connect(ctx); err != nil {
			_ = subscriber.Close()
			lastErr = fmt.Errorf("connect anker solix mqtt probe subscriber %s: %w", address, err)
			continue
		}
		if err := subscribeAnkerSolixTopics(ctx, subscriber, session.Topics, 0); err != nil {
			_ = subscriber.Close()
			lastErr = fmt.Errorf("subscribe anker solix mqtt probe topics on %s: %w", address, err)
			continue
		}
		return subscriber, address, nil
	}
	if lastErr == nil {
		lastErr = errors.New("anker solix mqtt probe subscriber could not connect")
	}
	return nil, "", lastErr
}

func (a *AnkerSolixAdapter) sessionForCredential(
	ctx context.Context,
	credential controlplane.ProviderCredential,
) (ankerSolixCloudClient, ankersolix.Session, ankersolix.Config, error) {
	if controlplane.NormalizeProvider(credential.Provider) != controlplane.ProviderAnkerSolix {
		return nil, ankersolix.Session{}, ankersolix.Config{}, ErrUnsupportedProvider
	}
	if !credential.IsActive {
		return nil, ankersolix.Session{}, ankersolix.Config{}, ErrInactiveCredential
	}
	email := strings.TrimSpace(credential.AccessKey)
	password := strings.TrimSpace(credential.SecretKey)
	if email == "" || password == "" {
		return nil, ankersolix.Session{}, ankersolix.Config{}, ErrMissingCredentialMaterial
	}
	cfg, err := ankersolix.ResolveConfig(
		configString(credential.Config, "server", string(ankersolix.ServerCOM)),
		configString(credential.Config, "country", "US"),
	)
	if err != nil {
		return nil, ankersolix.Session{}, ankersolix.Config{}, err
	}
	client, err := a.factory.NewClient(cfg)
	if err != nil {
		return nil, ankersolix.Session{}, ankersolix.Config{}, fmt.Errorf("create anker solix client: %w", err)
	}
	session, err := client.Login(ctx, email, password)
	if err != nil {
		return nil, ankersolix.Session{}, ankersolix.Config{}, fmt.Errorf("login anker solix cloud: %w", err)
	}
	if strings.TrimSpace(session.AuthToken) == "" {
		return nil, ankersolix.Session{}, ankersolix.Config{}, errors.New("anker solix login returned empty auth token")
	}
	return client, session, cfg, nil
}

func (a *AnkerSolixAdapter) describeDevice(
	ctx context.Context,
	client ankerSolixCloudClient,
	session ankersolix.Session,
	credentialID string,
	cfg ankersolix.Config,
	providerDeviceID string,
) (controlplane.ProviderDevice, error) {
	target, err := ankersolix.ParseProviderDeviceID(providerDeviceID)
	if err != nil {
		return controlplane.ProviderDevice{}, err
	}
	devices, err := client.ListBindDevices(ctx, session)
	if err != nil {
		return controlplane.ProviderDevice{}, fmt.Errorf("list anker solix devices: %w", err)
	}
	targetID := target.ProviderDeviceID()
	for i := range devices {
		mapped, ok := mapAnkerSolixDevice(devices[i], credentialID, cfg)
		if !ok {
			continue
		}
		if strings.EqualFold(mapped.ProviderDeviceID, targetID) {
			return mapped, nil
		}
	}
	return controlplane.ProviderDevice{}, ErrProviderDeviceNotFound
}

type AnkerSolixMQTTSession struct {
	Info         ankersolix.MQTTSessionInfo
	DeviceRef    ankersolix.DeviceRef
	Address      string
	Addresses    []string
	ClientID     string
	Topics       []string
	PublishTopic string
	TLSConfig    *tls.Config
}

func (s AnkerSolixMQTTSession) BrokerAddresses() []string {
	return brokerAddressList(s.Address, s.Addresses)
}

func (s AnkerSolixMQTTSession) RealtimeTriggerCommand(timeout time.Duration, now time.Time) (topic string, payload []byte, qos byte, err error) {
	if timeout <= 0 {
		timeout = defaultAnkerSolixRealtimeTriggerTimeout
	}
	seconds := int(timeout.Round(time.Second) / time.Second)
	if seconds <= 0 {
		seconds = int(defaultAnkerSolixRealtimeTriggerTimeout / time.Second)
	}
	payload, err = ankersolix.BuildCommandEnvelope(s.Info, s.DeviceRef, ankersolix.RealtimeTriggerCommand(seconds), ankersolix.CommandEnvelopeOptions{
		Now: now.UTC(),
	})
	if err != nil {
		return "", nil, 0, err
	}
	return s.PublishTopic, payload, 0, nil
}

func newAnkerSolixMQTTSession(info ankersolix.MQTTSessionInfo, ref ankersolix.DeviceRef, tlsConfig *tls.Config) AnkerSolixMQTTSession {
	address := normalizeAnkerSolixMQTTAddress(info.EndpointAddress)
	return AnkerSolixMQTTSession{
		Info:         info,
		DeviceRef:    ref,
		Address:      address,
		ClientID:     info.ClientID(ref.ProviderDeviceID()),
		Topics:       []string{ankersolix.SubscribeTopic(info, ref)},
		PublishTopic: ankersolix.PublishTopic(info, ref),
		TLSConfig:    tlsConfig,
	}
}

func mapAnkerSolixDevice(device ankersolix.Device, credentialID string, cfg ankersolix.Config) (controlplane.ProviderDevice, bool) {
	ref := device.Ref()
	providerDeviceID := ref.ProviderDeviceID()
	if providerDeviceID == "" || ankersolix.IsExcludedProduct(ref.ProductCode) {
		return controlplane.ProviderDevice{}, false
	}
	capability, known := ankersolix.LookupCapability(ref.ProductCode)
	enableable := known && capability.Enableable()
	supportStatus := "unsupported"
	if known {
		supportStatus = string(capability.Status)
	}
	productName := firstNonEmptyString(device.Alias, device.AliasName, device.Name, device.DeviceName, capability.DisplayName, "Anker SOLIX "+ref.ProductCode)
	model := ref.ProductCode
	capabilities := map[string]any{
		"read_only":      true,
		"mqtt_supported": enableable,
		"support_status": supportStatus,
	}
	if capability.BatteryCapacityWh > 0 {
		capabilities["battery_capacity_wh"] = float64(capability.BatteryCapacityWh)
	}
	if capability.DefaultPVInputs > 0 {
		capabilities["pv_input_count"] = float64(capability.DefaultPVInputs)
	}
	if capability.Family != "" {
		capabilities["family"] = string(capability.Family)
	}
	metadata := map[string]any{
		"server":           string(cfg.Server),
		"country":          cfg.Country,
		"product_code":     ref.ProductCode,
		"device_sn_suffix": suffix(ref.DeviceSN, 6),
		"online":           device.Online,
		"support_status":   supportStatus,
	}
	if !enableable {
		if capability.Status == ankersolix.SupportCompanion {
			metadata["unsupported_reason"] = "companion_only"
		} else {
			metadata["unsupported_reason"] = "needs_sample_or_out_of_scope"
		}
	}
	return controlplane.ProviderDevice{
		Provider:         controlplane.ProviderAnkerSolix,
		ProviderDeviceID: providerDeviceID,
		CredentialID:     credentialID,
		CanonicalSN:      ankersolix.CanonicalSN(ref),
		ProductName:      productName,
		Model:            model,
		Capabilities:     capabilities,
		Metadata:         metadata,
		IsActive:         enableable,
		IngestDesiredState: func() string {
			if enableable {
				return "active"
			}
			return "unsupported"
		}(),
	}, true
}

func deviceAnkerSolixEnableable(device controlplane.ProviderDevice) bool {
	if device.Capabilities["mqtt_supported"] == true {
		return true
	}
	if strings.EqualFold(fmt.Sprint(device.Metadata["support_status"]), string(ankersolix.SupportEnabled)) {
		return true
	}
	return false
}

func normalizeAnkerSolixMQTTAddress(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if strings.Contains(endpoint, ":") {
		return endpoint
	}
	return endpoint + ":8883"
}

func publishAnkerSolixRealtimeTrigger(
	ctx context.Context,
	subscriber ankerSolixMQTTSubscriber,
	session AnkerSolixMQTTSession,
	timeout time.Duration,
	now time.Time,
) error {
	topic, payload, qos, err := session.RealtimeTriggerCommand(timeout, now)
	if err != nil {
		return err
	}
	return subscriber.Publish(ctx, topic, payload, qos)
}

func subscribeAnkerSolixTopics(ctx context.Context, subscriber ankerSolixMQTTSubscriber, topics []string, qos byte) error {
	if len(topics) == 0 {
		return errors.New("anker solix mqtt session has no subscribe topics")
	}
	for _, topic := range topics {
		if err := subscriber.Subscribe(ctx, topic, qos); err != nil {
			return fmt.Errorf("subscribe anker solix mqtt topic %s: %w", redactAnkerSolixTopic(topic), err)
		}
	}
	return nil
}

func DecodeAnkerSolixMQTTMessage(topic string, payload []byte) (AnkerSolixDecodedMessage, error) {
	wrapper, err := ankersolix.DecodeMQTTWrapper(topic, payload)
	if err != nil {
		return AnkerSolixDecodedMessage{}, err
	}
	return AnkerSolixDecodedMessage{
		Ref: ankersolix.DeviceRef{
			ProductCode: wrapper.ProductCode,
			DeviceSN:    wrapper.DeviceSN,
		},
		Values: wrapper.Values,
	}, nil
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func suffix(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}

func brokerAddressList(primary string, fallbacks []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 1+len(fallbacks))
	for _, candidate := range append([]string{primary}, fallbacks...) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func redactAnkerSolixTopic(topic string) string {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) >= 4 && parts[0] == "dt" {
		parts[3] = "redacted"
		return strings.Join(parts, "/")
	}
	return ""
}
