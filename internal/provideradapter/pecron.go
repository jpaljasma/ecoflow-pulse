package provideradapter

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
)

type PecronClientFactory interface {
	NewClient(region pecron.RegionConfig) (pecron.CloudClient, error)
}

type PecronAdapterConfig struct {
	ClientFactory         PecronClientFactory
	MQTTSubscriberFactory pecronMQTTSubscriberFactory
	MQTTSessionCache      *MQTTSessionCache
	Now                   func() time.Time
}

type PecronAdapter struct {
	factory       PecronClientFactory
	newSubscriber pecronMQTTSubscriberFactory
	sessionCache  *MQTTSessionCache
	now           func() time.Time
}

type runtimePecronClientFactory struct{}

func (runtimePecronClientFactory) NewClient(region pecron.RegionConfig) (pecron.CloudClient, error) {
	return pecron.NewClient(region, nil), nil
}

type staticPecronClientFactory struct {
	client pecron.CloudClient
}

func StaticPecronClientFactory(client pecron.CloudClient) PecronClientFactory {
	return staticPecronClientFactory{client: client}
}

func (f staticPecronClientFactory) NewClient(pecron.RegionConfig) (pecron.CloudClient, error) {
	if f.client == nil {
		return nil, errors.New("pecron client is required")
	}
	return f.client, nil
}

type pecronMQTTSubscriber interface {
	Connect(ctx context.Context) error
	Subscribe(ctx context.Context, topic string, qos byte) error
	ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error)
	Close() error
}

type pecronMQTTSubscriberFactory func(cfg pecron.MQTTConfig) (pecronMQTTSubscriber, error)

func NewPecronAdapter(cfg PecronAdapterConfig) *PecronAdapter {
	if cfg.ClientFactory == nil {
		cfg.ClientFactory = runtimePecronClientFactory{}
	}
	if cfg.MQTTSubscriberFactory == nil {
		cfg.MQTTSubscriberFactory = func(mqttCfg pecron.MQTTConfig) (pecronMQTTSubscriber, error) {
			return pecron.NewMQTTSubscriber(mqttCfg)
		}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &PecronAdapter{
		factory:       cfg.ClientFactory,
		newSubscriber: cfg.MQTTSubscriberFactory,
		sessionCache:  cfg.MQTTSessionCache,
		now:           cfg.Now,
	}
}

func NewRuntimePecronAdapter() (*PecronAdapter, error) {
	return NewPecronAdapter(PecronAdapterConfig{}), nil
}

func (a *PecronAdapter) SetMQTTSessionCache(cache *MQTTSessionCache) {
	if a != nil {
		a.sessionCache = cache
	}
}

func (a *PecronAdapter) DiscoverDevices(ctx context.Context, credential controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error) {
	client, session, region, err := a.sessionForCredential(ctx, credential)
	if err != nil {
		return nil, err
	}
	devices, err := client.ListDevices(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("list pecron devices: %w", err)
	}
	out := make([]controlplane.ProviderDevice, 0, len(devices))
	seen := map[string]struct{}{}
	for i := range devices {
		mapped, ok := mapPecronDevice(devices[i], credential.ID, region.ID)
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

func (a *PecronAdapter) GetDeviceTelemetrySnapshot(
	ctx context.Context,
	credential controlplane.ProviderCredential,
	providerDeviceID string,
) (controlplane.ProviderDevice, pecron.NormalizedTelemetry, error) {
	client, session, region, err := a.sessionForCredential(ctx, credential)
	if err != nil {
		return controlplane.ProviderDevice{}, pecron.NormalizedTelemetry{}, err
	}
	device, err := a.describeDevice(ctx, client, session, credential.ID, region.ID, providerDeviceID)
	if err != nil {
		return controlplane.ProviderDevice{}, pecron.NormalizedTelemetry{}, err
	}
	ref, err := pecron.ParseProviderDeviceID(device.ProviderDeviceID)
	if err != nil {
		return controlplane.ProviderDevice{}, pecron.NormalizedTelemetry{}, err
	}
	kv, err := client.DeviceKV(ctx, session, ref)
	if err != nil {
		return controlplane.ProviderDevice{}, pecron.NormalizedTelemetry{}, fmt.Errorf("get pecron device attributes: %w", err)
	}
	modelDevice := pecron.Device{
		ProductKey:  ref.ProductKey,
		DeviceKey:   ref.DeviceKey,
		DeviceName:  device.ProductName,
		ProductName: device.Model,
	}
	normalized := pecron.NormalizeTelemetry(modelDevice, kv)
	device.Capabilities = mergeAnyMaps(device.Capabilities, normalized.Capabilities)
	device.Metadata = mergeAnyMaps(device.Metadata, normalized.Metadata)
	return device, normalized, nil
}

func (a *PecronAdapter) MQTTSession(
	ctx context.Context,
	credential controlplane.ProviderCredential,
	providerDeviceID string,
) (pecron.MQTTSession, error) {
	if err := validatePecronCredential(credential); err != nil {
		return pecron.MQTTSession{}, err
	}
	if cached, ok := a.sessionCache.GetPecron(ctx, credential, providerDeviceID); ok {
		return cached, nil
	}
	client, session, region, err := a.sessionForCredential(ctx, credential)
	if err != nil {
		return pecron.MQTTSession{}, err
	}
	device, err := a.describeDevice(ctx, client, session, credential.ID, region.ID, providerDeviceID)
	if err != nil {
		return pecron.MQTTSession{}, err
	}
	ref, err := pecron.ParseProviderDeviceID(device.ProviderDeviceID)
	if err != nil {
		return pecron.MQTTSession{}, err
	}
	clientIDSeed := strings.TrimSpace(session.UserID)
	if clientIDSeed == "" {
		clientIDSeed = ref.DeviceKey
	}
	mqttSession := pecron.MQTTSession{
		Address:   region.MQTTAddress,
		Addresses: region.MQTTBrokerAddresses(),
		Path:      region.MQTTPath,
		Token:     session.AccessToken,
		ClientID:  fmt.Sprintf("qu_%s_%d", clientIDSeed, a.now().UTC().UnixMilli()),
		Topics:    pecron.MQTTSubscribeTopics(ref),
	}
	a.sessionCache.PutPecron(ctx, credential, providerDeviceID, mqttSession, clientIDSeed, session.ExpiresAt)
	return mqttSession, nil
}

type MQTTProbeResult struct {
	Success          bool
	Status           string
	SampleTopic      string
	PayloadBytes     int64
	ObservedAtUnixMS int64
}

func (a *PecronAdapter) ProbeMQTT(
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
		return MQTTProbeResult{}, err
	}
	var lastStatus string
	for _, address := range session.BrokerAddresses() {
		subscriber, err := a.newSubscriber(pecron.MQTTConfig{
			Address:        address,
			Path:           session.Path,
			Token:          session.Token,
			ClientID:       session.ClientID,
			KeepAlive:      90 * time.Second,
			ConnectTimeout: 10 * time.Second,
			ReadTimeout:    10 * time.Second,
		})
		if err != nil {
			return MQTTProbeResult{}, fmt.Errorf("init pecron mqtt probe subscriber: %w", err)
		}
		closed := false
		closeSubscriber := func() {
			if closed {
				return
			}
			_ = subscriber.Close()
			closed = true
		}
		if err := subscriber.Connect(probeCtx); err != nil {
			closeSubscriber()
			lastStatus = mqttProbeStatus(err, "connect_failed")
			continue
		}
		subscribed := false
		for _, topic := range session.Topics {
			if !strings.HasSuffix(topic, "/bus_") {
				continue
			}
			if err := subscriber.Subscribe(probeCtx, topic, 1); err != nil {
				closeSubscriber()
				lastStatus = mqttProbeStatus(err, "subscribe_failed")
				break
			}
			subscribed = true
			break
		}
		if !subscribed {
			closeSubscriber()
			if lastStatus == "" {
				lastStatus = "subscribe_failed"
			}
			continue
		}
		msg, err := subscriber.ReadMessage(probeCtx)
		closeSubscriber()
		if err != nil {
			lastStatus = mqttProbeStatus(err, "no_messages")
			continue
		}
		return MQTTProbeResult{
			Success:          true,
			Status:           "ok",
			SampleTopic:      redactPecronTopic(msg.Topic),
			PayloadBytes:     int64(len(msg.Payload)),
			ObservedAtUnixMS: a.now().UTC().UnixMilli(),
		}, nil
	}
	if lastStatus == "" {
		lastStatus = "connect_failed"
	}
	return MQTTProbeResult{Status: lastStatus}, nil
}

func (a *PecronAdapter) sessionForCredential(
	ctx context.Context,
	credential controlplane.ProviderCredential,
) (pecron.CloudClient, pecron.Session, pecron.RegionConfig, error) {
	if err := validatePecronCredential(credential); err != nil {
		return nil, pecron.Session{}, pecron.RegionConfig{}, err
	}
	email := strings.TrimSpace(credential.AccessKey)
	password := strings.TrimSpace(credential.SecretKey)
	region, err := pecron.ResolveRegion(configString(credential.Config, "region", string(pecron.RegionUS)))
	if err != nil {
		return nil, pecron.Session{}, pecron.RegionConfig{}, err
	}
	client, err := a.factory.NewClient(region)
	if err != nil {
		return nil, pecron.Session{}, pecron.RegionConfig{}, fmt.Errorf("create pecron client: %w", err)
	}
	session, err := client.Login(ctx, email, password)
	if err != nil {
		return nil, pecron.Session{}, pecron.RegionConfig{}, fmt.Errorf("login pecron cloud: %w", err)
	}
	if strings.TrimSpace(session.AccessToken) == "" {
		return nil, pecron.Session{}, pecron.RegionConfig{}, errors.New("pecron login returned empty access token")
	}
	return client, session, region, nil
}

func validatePecronCredential(credential controlplane.ProviderCredential) error {
	if controlplane.NormalizeProvider(credential.Provider) != controlplane.ProviderPecron {
		return ErrUnsupportedProvider
	}
	if !credential.IsActive {
		return ErrInactiveCredential
	}
	if strings.TrimSpace(credential.AccessKey) == "" || strings.TrimSpace(credential.SecretKey) == "" {
		return ErrMissingCredentialMaterial
	}
	return nil
}

func (a *PecronAdapter) describeDevice(
	ctx context.Context,
	client pecron.CloudClient,
	session pecron.Session,
	credentialID string,
	region pecron.Region,
	providerDeviceID string,
) (controlplane.ProviderDevice, error) {
	target, err := pecron.ParseProviderDeviceID(providerDeviceID)
	if err != nil {
		return controlplane.ProviderDevice{}, err
	}
	devices, err := client.ListDevices(ctx, session)
	if err != nil {
		return controlplane.ProviderDevice{}, fmt.Errorf("list pecron devices: %w", err)
	}
	targetID := target.ProviderDeviceID()
	for i := range devices {
		mapped, ok := mapPecronDevice(devices[i], credentialID, region)
		if !ok {
			continue
		}
		if strings.EqualFold(mapped.ProviderDeviceID, targetID) {
			return mapped, nil
		}
	}
	return controlplane.ProviderDevice{}, fmt.Errorf("%w: %s", ErrProviderDeviceNotFound, targetID)
}

func mapPecronDevice(device pecron.Device, credentialID string, region pecron.Region) (controlplane.ProviderDevice, bool) {
	ref := pecron.DeviceRefFromDevice(device)
	providerDeviceID := ref.ProviderDeviceID()
	if providerDeviceID == "" {
		return controlplane.ProviderDevice{}, false
	}
	productName := strings.TrimSpace(device.DeviceName)
	model := strings.TrimSpace(device.ProductName)
	if model == "" && strings.EqualFold(ref.ProductKey, pecron.ProductKeyE1000LFP) {
		model = "E1000LFP"
	}
	if productName == "" {
		productName = model
	}
	if productName == "" {
		productName = "Pecron " + strings.ToUpper(ref.ProductKey)
	}
	capabilities := map[string]any{
		"read_only": true,
	}
	if strings.EqualFold(ref.ProductKey, pecron.ProductKeyE1000LFP) || strings.Contains(strings.ToUpper(model), "E1000") {
		capabilities["battery_capacity_wh"] = 1024
		capabilities["battery_pack_count"] = 1
		capabilities["supports_ac_output"] = true
		capabilities["supports_dc_output"] = true
	}
	return controlplane.ProviderDevice{
		Provider:         controlplane.ProviderPecron,
		ProviderDeviceID: providerDeviceID,
		CredentialID:     credentialID,
		CanonicalSN:      pecron.CanonicalSN(ref),
		ProductName:      productName,
		Model:            model,
		Capabilities:     capabilities,
		Metadata: map[string]any{
			"region":         string(region),
			"product_key":    ref.ProductKey,
			"online":         device.Online,
			"protocol":       strings.TrimSpace(device.Protocol),
			"last_conn_time": strings.TrimSpace(device.LastConnTime),
		},
		IsActive:           true,
		IngestDesiredState: "active",
	}, true
}

func configString(config map[string]any, key string, fallback string) string {
	if len(config) == 0 {
		return fallback
	}
	if value, ok := config[key]; ok {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			return text
		}
	}
	return fallback
}

func mergeAnyMaps(left map[string]any, right map[string]any) map[string]any {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := make(map[string]any, len(left)+len(right))
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func redactPecronTopic(topic string) string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return ""
	}
	parts := strings.Split(topic, "/")
	if len(parts) >= 5 && parts[0] == "q" {
		parts[3] = "redacted"
		return strings.Join(parts, "/")
	}
	return "redacted"
}

func mqttProbeStatus(err error, fallback string) string {
	if err == nil {
		return "ok"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "not authorized"), strings.Contains(lower, "bad username"), strings.Contains(lower, "connect rejected"):
		return "connect_rejected"
	case strings.Contains(lower, "subscription rejected"):
		return "subscribe_rejected"
	case strings.Contains(lower, "deadline"), strings.Contains(lower, "timeout"), strings.Contains(lower, "no_messages"):
		return "no_messages"
	default:
		return fallback
	}
}
