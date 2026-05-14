package provideradapter

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
	valkey "github.com/valkey-io/valkey-go"
)

type fakePecronClient struct {
	loginEmail       string
	loginPassword    string
	loginCalls       int
	listDevicesCalls int
	devices          []pecron.Device
	kv               map[string]any
	err              error
}

func (f *fakePecronClient) Login(_ context.Context, email, password string) (pecron.Session, error) {
	f.loginCalls++
	f.loginEmail = email
	f.loginPassword = password
	if f.err != nil {
		return pecron.Session{}, f.err
	}
	return pecron.Session{AccessToken: "token", UserID: "uid-1"}, nil
}

func (f *fakePecronClient) ListDevices(context.Context, pecron.Session) ([]pecron.Device, error) {
	f.listDevicesCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.devices, nil
}

func (f *fakePecronClient) ProductTSL(context.Context, pecron.Session, string) ([]pecron.TSLProperty, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakePecronClient) DeviceKV(_ context.Context, _ pecron.Session, _ pecron.DeviceRef) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.kv, nil
}

func TestPecronAdapterDiscoverDevicesMapsE1000LFP(t *testing.T) {
	t.Parallel()

	client := &fakePecronClient{
		devices: []pecron.Device{{
			ProductKey:  pecron.ProductKeyE1000LFP,
			DeviceKey:   "aabbccddeeff",
			DeviceName:  "Garage Pecron",
			ProductName: "E1000LFP",
			Online:      true,
		}},
	}
	adapter := NewPecronAdapter(PecronAdapterConfig{
		ClientFactory: StaticPecronClientFactory(client),
	})
	cred := controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderPecron,
		AccessKey: "owner@example.test",
		SecretKey: "password",
		Config:    map[string]any{"region": "us"},
		IsActive:  true,
	}

	devices, err := adapter.DiscoverDevices(context.Background(), cred)
	if err != nil {
		t.Fatalf("DiscoverDevices() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want 1", len(devices))
	}
	if devices[0].ProviderDeviceID != "p11vxg:aabbccddeeff" {
		t.Fatalf("provider device id = %q", devices[0].ProviderDeviceID)
	}
	if devices[0].CanonicalSN != "PECRON-P11VXG-AABBCCDDEEFF" {
		t.Fatalf("canonical sn = %q", devices[0].CanonicalSN)
	}
	if devices[0].Metadata["region"] != "us" {
		t.Fatalf("region metadata = %#v", devices[0].Metadata["region"])
	}
	if client.loginEmail != "owner@example.test" || client.loginPassword != "password" {
		t.Fatalf("credential material not passed to client")
	}
}

func TestPecronAdapterSnapshotNormalizesTelemetry(t *testing.T) {
	t.Parallel()

	client := &fakePecronClient{
		devices: []pecron.Device{{
			ProductKey:  pecron.ProductKeyE1000LFP,
			DeviceKey:   "aabbccddeeff",
			DeviceName:  "Garage Pecron",
			ProductName: "E1000LFP",
			Online:      true,
		}},
		kv: map[string]any{
			"battery_percentage": 75,
			"total_input_power":  120,
		},
	}
	adapter := NewPecronAdapter(PecronAdapterConfig{
		ClientFactory: StaticPecronClientFactory(client),
	})
	cred := controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderPecron,
		AccessKey: "owner@example.test",
		SecretKey: "password",
		Config:    map[string]any{"region": "us"},
		IsActive:  true,
	}

	device, snapshot, err := adapter.GetDeviceTelemetrySnapshot(context.Background(), cred, "p11vxg:aabbccddeeff")
	if err != nil {
		t.Fatalf("GetDeviceTelemetrySnapshot() error = %v", err)
	}
	if device.Model != "E1000LFP" {
		t.Fatalf("model = %q", device.Model)
	}
	if got := snapshot.Params["soc"]; got != float64(75) {
		t.Fatalf("snapshot soc = %#v", got)
	}
	if got := snapshot.Params["wattsInSum"]; got != float64(120) {
		t.Fatalf("snapshot wattsInSum = %#v", got)
	}
}

func TestPecronAdapterMQTTSessionCarriesRegionBrokerFallbacks(t *testing.T) {
	t.Parallel()

	client := &fakePecronClient{
		devices: []pecron.Device{{
			ProductKey:  pecron.ProductKeyE1000LFP,
			DeviceKey:   "aabbccddeeff",
			DeviceName:  "Garage Pecron",
			ProductName: "E1000LFP",
			Online:      true,
		}},
	}
	adapter := NewPecronAdapter(PecronAdapterConfig{
		ClientFactory: StaticPecronClientFactory(client),
	})
	session, err := adapter.MQTTSession(context.Background(), controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderPecron,
		AccessKey: "owner@example.test",
		SecretKey: "password",
		Config:    map[string]any{"region": "eu"},
		IsActive:  true,
	}, "p11vxg:aabbccddeeff")
	if err != nil {
		t.Fatalf("MQTTSession() error = %v", err)
	}
	want := []string{"iot-south.acceleronix.io:8443", "iot-south.quecteleu.com:8443"}
	got := session.BrokerAddresses()
	if len(got) != len(want) {
		t.Fatalf("broker addresses = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("broker addresses = %#v, want %#v", got, want)
		}
	}
}

func TestPecronAdapterMQTTSessionUsesEncryptedSharedCache(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	valkeyClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{server.Addr()},
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("new valkey client: %v", err)
	}
	t.Cleanup(valkeyClient.Close)
	keyring, err := valkeycache.NewKeyring("v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x42}, 32)})
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}
	clock := time.Date(2026, time.May, 14, 12, 0, 0, 0, time.UTC)
	cacheClient, err := valkeycache.New(valkeyClient, valkeycache.Options{
		Prefix:    "pulse",
		Namespace: "provider-mqtt-session",
		Keyring:   keyring,
		Now:       func() time.Time { return clock },
	})
	if err != nil {
		t.Fatalf("new cache client: %v", err)
	}
	sessionCache := NewMQTTSessionCache(cacheClient, MQTTSessionCacheConfig{
		IdleTTL:  time.Minute,
		MaxAge:   time.Hour,
		LocalTTL: time.Second,
		Now:      func() time.Time { return clock },
	})

	client := &fakePecronClient{
		devices: []pecron.Device{{
			ProductKey:  pecron.ProductKeyE1000LFP,
			DeviceKey:   "aabbccddeeff",
			DeviceName:  "Garage Pecron",
			ProductName: "E1000LFP",
			Online:      true,
		}},
	}
	adapter := NewPecronAdapter(PecronAdapterConfig{
		ClientFactory:    StaticPecronClientFactory(client),
		MQTTSessionCache: sessionCache,
		Now:              func() time.Time { return clock },
	})
	credential := controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderPecron,
		AccessKey: "owner@example.test",
		SecretKey: "password",
		Config:    map[string]any{"region": "us"},
		IsActive:  true,
	}

	first, err := adapter.MQTTSession(context.Background(), credential, "p11vxg:aabbccddeeff")
	if err != nil {
		t.Fatalf("first MQTTSession() error = %v", err)
	}
	if client.loginCalls != 1 || client.listDevicesCalls != 1 {
		t.Fatalf("provider calls after first session: login=%d list=%d, want 1/1", client.loginCalls, client.listDevicesCalls)
	}
	clock = clock.Add(time.Second)
	client.err = errors.New("provider should not be called after encrypted session cache warmup")
	second, err := adapter.MQTTSession(context.Background(), credential, "p11vxg:aabbccddeeff")
	if err != nil {
		t.Fatalf("second MQTTSession() error = %v", err)
	}
	if client.loginCalls != 1 || client.listDevicesCalls != 1 {
		t.Fatalf("provider calls after cache hit: login=%d list=%d, want 1/1", client.loginCalls, client.listDevicesCalls)
	}
	if second.Token != first.Token {
		t.Fatalf("cached token = %q, want %q", second.Token, first.Token)
	}
	if second.ClientID == first.ClientID {
		t.Fatalf("cached MQTT client ID was not refreshed: %q", second.ClientID)
	}
	for _, key := range server.Keys() {
		raw, err := server.Get(key)
		if err != nil {
			t.Fatalf("miniredis get %q: %v", key, err)
		}
		if strings.Contains(raw, first.Token) {
			t.Fatalf("cache key %q stored plaintext provider token: %q", key, raw)
		}
	}
}

func TestPecronAdapterValidationGuards(t *testing.T) {
	t.Parallel()

	adapter := NewPecronAdapter(PecronAdapterConfig{ClientFactory: StaticPecronClientFactory(&fakePecronClient{})})
	tests := []struct {
		name       string
		credential controlplane.ProviderCredential
		wantErr    error
	}{
		{
			name:       "wrong provider",
			credential: controlplane.ProviderCredential{Provider: controlplane.ProviderEcoFlow, AccessKey: "email", SecretKey: "password", IsActive: true},
			wantErr:    ErrUnsupportedProvider,
		},
		{
			name:       "inactive",
			credential: controlplane.ProviderCredential{Provider: controlplane.ProviderPecron, AccessKey: "email", SecretKey: "password"},
			wantErr:    ErrInactiveCredential,
		},
		{
			name:       "missing credentials",
			credential: controlplane.ProviderCredential{Provider: controlplane.ProviderPecron, Config: map[string]any{"region": "us"}, IsActive: true},
			wantErr:    ErrMissingCredentialMaterial,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := adapter.DiscoverDevices(context.Background(), tc.credential)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("DiscoverDevices() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
