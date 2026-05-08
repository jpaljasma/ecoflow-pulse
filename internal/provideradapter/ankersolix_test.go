package provideradapter

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ankersolix"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

type fakeAnkerSolixClient struct {
	loginEmail    string
	loginPassword string
	devices       []ankersolix.Device
	mqttInfo      ankersolix.MQTTSessionInfo
	err           error
}

func (f *fakeAnkerSolixClient) Login(_ context.Context, email, password string) (ankersolix.Session, error) {
	f.loginEmail = email
	f.loginPassword = password
	if f.err != nil {
		return ankersolix.Session{}, f.err
	}
	return ankersolix.Session{AuthToken: "token", UserID: "uid-1"}, nil
}

func (f *fakeAnkerSolixClient) ListBindDevices(context.Context, ankersolix.Session) ([]ankersolix.Device, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.devices, nil
}

func (f *fakeAnkerSolixClient) GetMQTTInfo(context.Context, ankersolix.Session) (ankersolix.MQTTSessionInfo, error) {
	if f.err != nil {
		return ankersolix.MQTTSessionInfo{}, f.err
	}
	return f.mqttInfo, nil
}

func TestAnkerSolixAdapterDiscoverDevicesMapsSupportedModels(t *testing.T) {
	t.Parallel()

	client := &fakeAnkerSolixClient{
		devices: []ankersolix.Device{{
			ProductCode: "A1783",
			DeviceSN:    "ankersn001",
			AliasName:   "Garage C2000",
			DeviceName:  "Anker SOLIX C2000 Gen 2",
			Online:      true,
		}},
	}
	adapter := NewAnkerSolixAdapter(AnkerSolixAdapterConfig{
		ClientFactory: StaticAnkerSolixClientFactory(client),
	})
	cred := controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderAnkerSolix,
		AccessKey: "owner@example.test",
		SecretKey: "password",
		Config: map[string]any{
			"server":  "com",
			"country": "US",
		},
		IsActive: true,
	}

	devices, err := adapter.DiscoverDevices(context.Background(), cred)
	if err != nil {
		t.Fatalf("DiscoverDevices() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want 1", len(devices))
	}
	if devices[0].Provider != controlplane.ProviderAnkerSolix {
		t.Fatalf("provider = %q", devices[0].Provider)
	}
	if devices[0].ProviderDeviceID != "a1783:ankersn001" {
		t.Fatalf("provider device id = %q", devices[0].ProviderDeviceID)
	}
	if devices[0].CanonicalSN != "ANKER-A1783-ANKERSN001" {
		t.Fatalf("canonical sn = %q", devices[0].CanonicalSN)
	}
	if devices[0].Capabilities["read_only"] != true || devices[0].Capabilities["mqtt_supported"] != true {
		t.Fatalf("capabilities = %#v", devices[0].Capabilities)
	}
	if devices[0].Metadata["server"] != "com" || devices[0].Metadata["country"] != "US" {
		t.Fatalf("metadata = %#v", devices[0].Metadata)
	}
	if client.loginEmail != "owner@example.test" || client.loginPassword != "password" {
		t.Fatalf("credential material not passed to client")
	}
}

func TestAnkerSolixAdapterProbeMQTTUsesProviderTransport(t *testing.T) {
	t.Parallel()

	client := &fakeAnkerSolixClient{
		devices: []ankersolix.Device{{
			ProductCode: "A1783",
			DeviceSN:    "ankersn001",
			AliasName:   "Garage C2000",
		}},
		mqttInfo: ankersolix.MQTTSessionInfo{
			EndpointAddress: "aiot-mqtt.anker.example:8883",
			AppName:         "anker_power",
			ThingName:       "thing-1",
			RootCAPEM:       testAnkerSolixRootPEM,
			CertificatePEM:  testAnkerSolixCertPEM,
			PrivateKeyPEM:   testAnkerSolixKeyPEM,
		},
	}
	subscriber := &fakeAnkerSolixProbeSubscriber{
		reads: []fakeAnkerSolixReadResult{{
			msg: ecoflowmqtt.Message{
				Topic:   "dt/anker_power/A1783/ankersn001/param_info",
				Payload: []byte("decoded by injected probe decoder"),
			},
		}},
	}
	factory := &fakeAnkerSolixProbeSubscriberFactory{subscribers: []ankerSolixMQTTSubscriber{subscriber}}
	adapter := NewAnkerSolixAdapter(AnkerSolixAdapterConfig{
		ClientFactory:         StaticAnkerSolixClientFactory(client),
		MQTTSubscriberFactory: factory.new,
		DecodeMQTTMessage: func(string, []byte) (AnkerSolixDecodedMessage, error) {
			return AnkerSolixDecodedMessage{}, nil
		},
		Now: func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	})

	result, err := adapter.ProbeMQTT(context.Background(), controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderAnkerSolix,
		AccessKey: "owner@example.test",
		SecretKey: "password",
		Config:    map[string]any{"server": "com", "country": "US"},
		IsActive:  true,
	}, "A1783:ankersn001", time.Second)
	if err != nil {
		t.Fatalf("ProbeMQTT() error = %v", err)
	}
	if !result.Success || result.Status != "ok" {
		t.Fatalf("probe result = %#v", result)
	}
	if len(factory.configs) != 1 || factory.configs[0].Address != "aiot-mqtt.anker.example:8883" {
		t.Fatalf("subscriber configs = %#v", factory.configs)
	}
	if got := len(subscriber.published); got != 1 {
		t.Fatalf("published trigger commands = %d, want 1", got)
	}
	if subscriber.closeCalls.Load() != 1 {
		t.Fatalf("expected subscriber close")
	}
}

func TestAnkerSolixAdapterProbeMQTTReportsUnsupportedModelsWithoutTransport(t *testing.T) {
	t.Parallel()

	client := &fakeAnkerSolixClient{
		devices: []ankersolix.Device{{
			ProductCode: "A1753",
			DeviceSN:    "ankersn-unsupported",
			AliasName:   "Unmapped SOLIX",
		}},
	}
	factory := &fakeAnkerSolixProbeSubscriberFactory{}
	adapter := NewAnkerSolixAdapter(AnkerSolixAdapterConfig{
		ClientFactory:         StaticAnkerSolixClientFactory(client),
		MQTTSubscriberFactory: factory.new,
	})

	result, err := adapter.ProbeMQTT(context.Background(), controlplane.ProviderCredential{
		ID:        "cred-1",
		Provider:  controlplane.ProviderAnkerSolix,
		AccessKey: "owner@example.test",
		SecretKey: "password",
		Config:    map[string]any{"server": "com", "country": "US"},
		IsActive:  true,
	}, "A1753:ankersn-unsupported", time.Second)
	if err != nil {
		t.Fatalf("ProbeMQTT() error = %v", err)
	}
	if result.Success || result.Status != "unsupported_model" {
		t.Fatalf("probe result = %#v", result)
	}
	if len(factory.configs) != 0 {
		t.Fatalf("subscriber factory should not be called for unsupported model")
	}
}

func TestDecodeAnkerSolixMQTTMessageHandlesJSONHomeSystemPayloads(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(map[string]any{
		"soc": 62,
		"pp":  650,
	})
	if err != nil {
		t.Fatal(err)
	}
	inner, err := json.Marshal(map[string]any{
		"pn":    "A5101",
		"sn":    "SN-X1",
		"trans": base64.StdEncoding.EncodeToString(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := json.Marshal(map[string]any{"payload": string(inner)})
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeAnkerSolixMQTTMessage("dt/anker_power/A5101/SN-X1/param_info", wrapped)
	if err != nil {
		t.Fatalf("DecodeAnkerSolixMQTTMessage() error = %v", err)
	}
	if decoded.Ref.ProductCode != "A5101" || decoded.Ref.DeviceSN != "SN-X1" {
		t.Fatalf("decoded ref = %#v", decoded.Ref)
	}
	if got := decoded.Values["battery_soc"]; got != float64(62) {
		t.Fatalf("battery_soc = %#v", got)
	}
	if got := decoded.Values["photovoltaic_power"]; got != float64(650) {
		t.Fatalf("photovoltaic_power = %#v", got)
	}
}

func TestAnkerSolixAdapterValidationGuards(t *testing.T) {
	t.Parallel()

	adapter := NewAnkerSolixAdapter(AnkerSolixAdapterConfig{
		ClientFactory: StaticAnkerSolixClientFactory(&fakeAnkerSolixClient{}),
	})
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
			credential: controlplane.ProviderCredential{Provider: controlplane.ProviderAnkerSolix, AccessKey: "email", SecretKey: "password"},
			wantErr:    ErrInactiveCredential,
		},
		{
			name:       "missing credentials",
			credential: controlplane.ProviderCredential{Provider: controlplane.ProviderAnkerSolix, Config: map[string]any{"server": "com", "country": "US"}, IsActive: true},
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

type fakeAnkerSolixProbeSubscriber struct {
	connectErr   error
	subscribeErr error
	publishErr   error
	reads        []fakeAnkerSolixReadResult
	published    []struct {
		topic   string
		payload []byte
		qos     byte
	}
	closeCalls atomic.Int64
}

func (f *fakeAnkerSolixProbeSubscriber) Connect(context.Context) error {
	return f.connectErr
}

func (f *fakeAnkerSolixProbeSubscriber) Subscribe(context.Context, string, byte) error {
	return f.subscribeErr
}

func (f *fakeAnkerSolixProbeSubscriber) Publish(_ context.Context, topic string, payload []byte, qos byte) error {
	f.published = append(f.published, struct {
		topic   string
		payload []byte
		qos     byte
	}{topic: topic, payload: payload, qos: qos})
	return f.publishErr
}

func (f *fakeAnkerSolixProbeSubscriber) ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error) {
	if len(f.reads) > 0 {
		next := f.reads[0]
		f.reads = f.reads[1:]
		if next.err != nil {
			return ecoflowmqtt.Message{}, next.err
		}
		return next.msg, nil
	}
	<-ctx.Done()
	return ecoflowmqtt.Message{}, ctx.Err()
}

func (f *fakeAnkerSolixProbeSubscriber) Close() error {
	f.closeCalls.Add(1)
	return nil
}

type fakeAnkerSolixReadResult struct {
	msg ecoflowmqtt.Message
	err error
}

var (
	testAnkerSolixRootPEM string
	testAnkerSolixCertPEM string
	testAnkerSolixKeyPEM  string
)

func init() {
	var err error
	testAnkerSolixRootPEM, testAnkerSolixCertPEM, testAnkerSolixKeyPEM, err = generateAnkerSolixTestCerts()
	if err != nil {
		panic(err)
	}
}

func generateAnkerSolixTestCerts() (string, string, string, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return "", "", "", err
	}
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", err
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		return "", "", "", err
	}
	rootPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}))
	return rootPEM, certPEM, keyPEM, nil
}

type fakeAnkerSolixProbeSubscriberFactory struct {
	subscribers []ankerSolixMQTTSubscriber
	configs     []ankersolix.MQTTConfig
}

func (f *fakeAnkerSolixProbeSubscriberFactory) new(cfg ankersolix.MQTTConfig) (ankerSolixMQTTSubscriber, error) {
	f.configs = append(f.configs, cfg)
	if len(f.subscribers) == 0 {
		return nil, errors.New("no fake anker solix subscribers")
	}
	next := f.subscribers[0]
	f.subscribers = f.subscribers[1:]
	return next, nil
}
