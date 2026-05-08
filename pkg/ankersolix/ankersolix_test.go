package ankersolix

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

func TestConfigDefaultsValidationAndEndpoints(t *testing.T) {
	cfg, err := ResolveConfig("", "")
	if err != nil {
		t.Fatalf("default config: %v", err)
	}
	if cfg.Server != ServerCOM || cfg.Country != "US" {
		t.Fatalf("default config = %#v, want com/US", cfg)
	}
	if got := cfg.BaseURL(); got != "https://ankerpower-api.anker.com" {
		t.Fatalf("COM endpoint = %q", got)
	}
	cfg, err = ResolveConfig("eu", "de")
	if err != nil {
		t.Fatalf("EU config: %v", err)
	}
	if cfg.Country != "DE" || cfg.BaseURL() != "https://ankerpower-api-eu.anker.com" {
		t.Fatalf("EU config = %#v", cfg)
	}
	if _, err := ResolveConfig("cn", "US"); err == nil {
		t.Fatal("unsupported server accepted")
	}
	if _, err := ResolveConfig("com", "USA"); err == nil {
		t.Fatal("non ISO-2 country accepted")
	}
	client := NewClient(Config{}, nil)
	if client.cfg.Server != ServerCOM || client.cfg.Country != "US" {
		t.Fatalf("client default config = %#v", client.cfg)
	}
}

func TestEncryptPasswordDeterministicWithInjectedKeyAndPKCS7(t *testing.T) {
	serverPriv, err := testP256PrivateKey("01")
	if err != nil {
		t.Fatal(err)
	}
	clientPriv, err := testP256PrivateKey("02")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := EncryptPassword(PasswordEncryptionInput{
		Password:        "correct horse battery staple",
		ClientPrivate:   clientPriv,
		ServerPublicKey: serverPriv.PublicKey().Bytes(),
	})
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	if encrypted.ClientPublicKeyHex != hex.EncodeToString(clientPriv.PublicKey().Bytes()) {
		t.Fatalf("client public key hex mismatch")
	}
	if encrypted.CiphertextBase64 == "" {
		t.Fatal("empty encrypted password")
	}
	again, err := EncryptPassword(PasswordEncryptionInput{
		Password:        "correct horse battery staple",
		ClientPrivate:   clientPriv,
		ServerPublicKey: serverPriv.PublicKey().Bytes(),
	})
	if err != nil {
		t.Fatalf("encrypt password again: %v", err)
	}
	if encrypted.CiphertextBase64 != again.CiphertextBase64 {
		t.Fatalf("deterministic encryption changed: %q != %q", encrypted.CiphertextBase64, again.CiphertextBase64)
	}
	plain, err := decryptPasswordForTest(serverPriv, clientPriv.PublicKey().Bytes(), encrypted.CiphertextBase64)
	if err != nil {
		t.Fatalf("decrypt password: %v", err)
	}
	if plain != "correct horse battery staple" {
		t.Fatalf("decrypted password = %q", plain)
	}
	raw, err := base64.StdEncoding.DecodeString(encrypted.CiphertextBase64)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw)%aesBlockSize != 0 {
		t.Fatalf("ciphertext length %d is not block-aligned", len(raw))
	}
}

func TestTokenHeaders(t *testing.T) {
	cfg := Config{Server: ServerCOM, Country: "US"}
	headers := TokenHeaders(cfg, Session{
		UserID:    "user-123",
		AuthToken: "token-abc",
	}, "GMT-05:00")
	if headers.Get("country") != "US" || headers.Get("timezone") != "GMT-05:00" {
		t.Fatalf("country/timezone headers = %q/%q", headers.Get("country"), headers.Get("timezone"))
	}
	if headers.Get("gtoken") != "bc17c97c3d897d43b7e07ca0266eafa8" {
		t.Fatalf("gtoken = %q", headers.Get("gtoken"))
	}
	if headers.Get("x-auth-token") != "token-abc" {
		t.Fatalf("x-auth-token = %q", headers.Get("x-auth-token"))
	}
}

func TestClientLoginBindDevicesMQTTInfoAndErrors(t *testing.T) {
	var seenLogin map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/passport/login":
			if r.Header.Get("country") != "US" {
				t.Fatalf("login country header = %q", r.Header.Get("country"))
			}
			if err := json.NewDecoder(r.Body).Decode(&seenLogin); err != nil {
				t.Fatalf("decode login body: %v", err)
			}
			writeJSON(t, w, map[string]any{
				"code": 0,
				"msg":  "success!",
				"data": map[string]any{
					"user_id":          "user-123",
					"auth_token":       "token-abc",
					"token_expires_at": float64(1893456000),
					"nick_name":        "Pulse",
				},
			})
		case "/power_service/v1/app/get_relate_and_bind_devices":
			writeJSON(t, w, map[string]any{
				"code": 0,
				"data": map[string]any{
					"device_list": []map[string]any{
						{
							"device_sn":     "SN-C2000",
							"product_code":  "A1783",
							"device_name":   "C2000 Gen 2",
							"wifi_online":   1,
							"owner_user_id": "owner-1",
							"alias_name":    "Garage backup",
						},
						{"device_sn": "SN-PLUG", "product_code": "A17X7"},
					},
				},
			})
		case "/app/devicemanage/get_user_mqtt_info":
			writeJSON(t, w, map[string]any{
				"code": 0,
				"data": map[string]any{
					"user_id":          "user-123",
					"app_name":         "anker_power",
					"thing_name":       "thing-1",
					"certificate_id":   "cert-1",
					"endpoint_addr":    "aiot-mqtt.anker.com",
					"aws_root_ca1_pem": testRootPEM,
					"certificate_pem":  testCertPEM,
					"private_key":      testKeyPEM,
				},
			})
		case "/busy":
			writeJSON(t, w, map[string]any{"code": 21105, "msg": "busy"})
		case "/limited":
			w.WriteHeader(http.StatusTooManyRequests)
			writeJSON(t, w, map[string]any{"code": 429, "msg": "slow down"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := Config{Server: ServerCOM, Country: "US", baseURLOverride: server.URL}
	client := NewClient(cfg, &http.Client{Timeout: time.Second})
	client.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	client.encrypt = func(password string) (EncryptedPassword, error) {
		return EncryptedPassword{CiphertextBase64: "encrypted", ClientPublicKeyHex: "client-public"}, nil
	}

	session, err := client.Login(context.Background(), "owner@example.test", "secret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if session.UserID != "user-123" || session.AuthToken != "token-abc" {
		t.Fatalf("session = %#v", session)
	}
	if seenLogin["password"] != "encrypted" || seenLogin["email"] != "owner@example.test" {
		t.Fatalf("login body = %#v", seenLogin)
	}
	devices, err := client.ListBindDevices(context.Background(), session)
	if err != nil {
		t.Fatalf("bind devices: %v", err)
	}
	if len(devices) != 2 || devices[0].ProductCode != "A1783" || !devices[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
	info, err := client.GetMQTTInfo(context.Background(), session)
	if err != nil {
		t.Fatalf("mqtt info: %v", err)
	}
	if info.EndpointAddress != "aiot-mqtt.anker.com" || info.ClientID("") == "" {
		t.Fatalf("mqtt info = %#v", info)
	}
	var busy any
	err = client.request(context.Background(), http.MethodPost, "busy", session, nil, &busy)
	if !IsRetryable(err) {
		t.Fatalf("busy error is not retryable: %v", err)
	}
	err = client.request(context.Background(), http.MethodPost, "limited", session, nil, &busy)
	if !IsRetryable(err) {
		t.Fatalf("429 error is not retryable: %v", err)
	}
}

func TestMQTTTLSConfigTopicsAndWrapperDecode(t *testing.T) {
	info := MQTTSessionInfo{
		UserID:          "user-1",
		AppName:         "anker_power",
		ThingName:       "thing-1",
		CertificateID:   "cert-1",
		EndpointAddress: "aiot-mqtt.anker.com",
		RootCAPEM:       testRootPEM,
		CertificatePEM:  testCertPEM,
		PrivateKeyPEM:   testKeyPEM,
	}
	tlsCfg, err := info.TLSConfig()
	if err != nil {
		t.Fatalf("TLS config: %v", err)
	}
	if len(tlsCfg.Certificates) != 1 || tlsCfg.RootCAs == nil {
		t.Fatalf("TLS config missing certs: %#v", tlsCfg)
	}
	ref := DeviceRef{ProductCode: "A1783", DeviceSN: "SN-C2000"}
	if got := ref.ProviderDeviceID(); got != "a1783:SN-C2000" {
		t.Fatalf("provider device id = %q", got)
	}
	if got := SubscribeTopic(info, ref); got != "dt/anker_power/A1783/SN-C2000/#" {
		t.Fatalf("subscribe topic = %q", got)
	}
	if got := PublishTopic(info, ref); got != "cmd/anker_power/A1783/SN-C2000/req" {
		t.Fatalf("publish topic = %q", got)
	}
	command := RealtimeTriggerCommand(300)
	envelope, err := BuildCommandEnvelope(info, ref, command, CommandEnvelopeOptions{
		SessionID: "1234-5678",
		Now:       time.Unix(1700000000, 0).UTC(),
		Seed:      "abcdef",
	})
	if err != nil {
		t.Fatalf("build command envelope: %v", err)
	}
	var commandEnvelope struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(envelope, &commandEnvelope); err != nil {
		t.Fatalf("decode command envelope: %v", err)
	}
	if !strings.Contains(commandEnvelope.Payload, `"data"`) || !strings.Contains(commandEnvelope.Payload, `"device_sn":"SN-C2000"`) {
		t.Fatalf("command envelope = %s", envelope)
	}

	frame := binaryFrame("0421", tlvString("a2", "SN-C2000"), tlvNumber("a5", typeUI, 87))
	wrapped := mqttWrapper(t, "anker_power", "A1783", "SN-C2000", "data", frame)
	msg, err := DecodeMQTTWrapper("dt/anker_power/A1783/SN-C2000/param_info", wrapped)
	if err != nil {
		t.Fatalf("decode wrapper: %v", err)
	}
	if msg.ProductCode != "A1783" || msg.DeviceSN != "SN-C2000" || msg.MessageType != "0421" {
		t.Fatalf("decoded wrapper = %#v", msg)
	}
	if !bytes.Equal(msg.Data, frame) {
		t.Fatal("decoded data mismatch")
	}
}

func TestMQTTSubscriberKeepsNewestMessageWhenBufferIsFull(t *testing.T) {
	subscriber, err := NewMQTTSubscriber(MQTTConfig{
		Address:    "aiot-mqtt.anker.example:8883",
		ClientID:   "client-1",
		BufferSize: 1,
	})
	if err != nil {
		t.Fatalf("NewMQTTSubscriber() error = %v", err)
	}
	subscriber.enqueueMessage(ecoflowmqtt.Message{Topic: "old"})
	subscriber.enqueueMessage(ecoflowmqtt.Message{Topic: "new"})

	msg, err := subscriber.ReadMessage(context.Background())
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if msg.Topic != "new" {
		t.Fatalf("topic = %q, want newest message", msg.Topic)
	}
	if subscriber.DroppedMessages() != 1 {
		t.Fatalf("dropped messages = %d, want 1", subscriber.DroppedMessages())
	}
}

func TestCapabilityRegistryGating(t *testing.T) {
	for _, code := range []string{"A1722", "A1763", "A1783", "A17C5", "A17E1", "AX170", "A5101"} {
		capability, ok := LookupCapability(code)
		if !ok {
			t.Fatalf("%s missing from registry", code)
		}
		if !capability.Enableable() {
			t.Fatalf("%s should be enableable: %#v", code, capability)
		}
	}
	if capability, ok := LookupCapability("A7320"); !ok || capability.Status != SupportCompanion || capability.Enableable() {
		t.Fatalf("A7320 companion gating = %#v/%v", capability, ok)
	}
	if capability, ok := LookupCapability("A1785"); !ok || capability.Status != SupportNeedsSample || capability.Enableable() {
		t.Fatalf("A1785 unsupported gating = %#v/%v", capability, ok)
	}
	if IsExcludedProduct("A17X7") != true {
		t.Fatal("standalone smart meter should be excluded")
	}
}

func TestDecodeNormalizeFixtures(t *testing.T) {
	tests := []struct {
		name        string
		productCode string
		messageType string
		fields      [][]byte
		wantSOC     float64
		wantIn      float64
		wantOut     float64
		wantPVCount float64
	}{
		{
			name:        "C300",
			productCode: "A1722",
			messageType: "0405",
			fields: [][]byte{
				tlvString("a2", "SN-C300"),
				tlvNumber("bb", typeUI, 79),
				tlvNumber("ac", typeSILE, 120),
				tlvNumber("ad", typeSILE, 80),
				tlvNumber("ae", typeSILE, 32),
			},
			wantSOC: 79, wantIn: 200, wantOut: 32, wantPVCount: 1,
		},
		{
			name:        "C200",
			productCode: "A1725",
			messageType: "0405",
			fields: [][]byte{
				tlvString("a2", "SN-C200"),
				tlvNumber("b7", typeUI, 77),
				tlvNumber("ac", typeSILE, 125),
				tlvNumber("ad", typeSILE, 19),
			},
			wantSOC: 77, wantIn: 125, wantOut: 19, wantPVCount: 1,
		},
		{
			name:        "C1000",
			productCode: "A1761",
			messageType: "0405",
			fields: [][]byte{
				tlvNumber("c1", typeUI, 61),
				tlvNumber("ae", typeSILE, 220),
				tlvNumber("af", typeSILE, 180),
				tlvNumber("b0", typeSILE, 345),
			},
			wantSOC: 61, wantIn: 400, wantOut: 345, wantPVCount: 1,
		},
		{
			name:        "C1000 Gen2",
			productCode: "A1763",
			messageType: "0421",
			fields: [][]byte{
				tlvNumber("a5", typeUI, 88),
				tlvNumber("a6", typeSILE, 456),
				tlvNumber("a7", typeSILE, 210),
				tlvNumber("b2", typeSILE, 37),
			},
			wantSOC: 88, wantIn: 210, wantOut: 456, wantPVCount: 1,
		},
		{
			name:        "C2000 Gen2",
			productCode: "A1783",
			messageType: "0421",
			fields: [][]byte{
				tlvNumber("a5", typeUI, 91),
				tlvNumber("a6", typeSILE, 612),
				tlvNumber("a7", typeSILE, 250),
				tlvNumber("b2", typeSILE, 52),
			},
			wantSOC: 91, wantIn: 250, wantOut: 612, wantPVCount: 1,
		},
		{
			name:        "F2000",
			productCode: "A1780",
			messageType: "0405",
			fields: [][]byte{
				tlvNumber("c1", typeUI, 66),
				tlvNumber("ae", typeSILE, 310),
				tlvNumber("af", typeSILE, 290),
				tlvNumber("b0", typeSILE, 430),
			},
			wantSOC: 66, wantIn: 600, wantOut: 430, wantPVCount: 1,
		},
		{
			name:        "F3000",
			productCode: "A1782",
			messageType: "0421",
			fields: [][]byte{
				tlvNumber("a5", typeUI, 72),
				tlvNumber("a6", typeSILE, 680),
				tlvNumber("a7", typeSILE, 190),
				tlvNumber("b2", typeSILE, 125),
			},
			wantSOC: 72, wantIn: 190, wantOut: 680, wantPVCount: 1,
		},
		{
			name:        "F3800",
			productCode: "A1790",
			messageType: "0405",
			fields: [][]byte{
				tlvNumber("c0", typeUI, 55),
				tlvNumber("af", typeSILE, 350),
				tlvNumber("b0", typeSILE, 150),
				tlvNumber("b2", typeSILE, 900),
			},
			wantSOC: 55, wantIn: 500, wantOut: 900, wantPVCount: 2,
		},
		{
			name:        "Solarbank",
			productCode: "A17C5",
			messageType: "0405",
			fields: [][]byte{
				tlvNumber("a6", typeUI, 64),
				tlvNumber("c6", typeSILE, 125),
				tlvNumber("c7", typeSILE, 115),
				tlvNumber("c5", typeSILE, 480),
				tlvNumber("c4", typeSILE, -95),
			},
			wantSOC: 64, wantIn: 240, wantOut: 480, wantPVCount: 2,
		},
		{
			name:        "E10 Home Backup",
			productCode: "A17E1",
			messageType: "0405",
			fields: [][]byte{
				tlvNumber("a3", typeUI, 81),
				tlvNumber("c6", typeSILE, 300),
				tlvNumber("c7", typeSILE, 280),
				tlvNumber("c5", typeSILE, 1200),
				tlvNumber("c4", typeSILE, 90),
			},
			wantSOC: 81, wantIn: 580, wantOut: 1200, wantPVCount: 2,
		},
		{
			name:        "Home Power Panel",
			productCode: "A17B1",
			messageType: "0500",
			fields: [][]byte{
				tlvNumber("a6", typeUI, 74),
				tlvNumber("ab", typeSILE, 410),
				tlvNumber("af", typeSILE, 1500),
				tlvNumber("c4", typeSILE, -120),
			},
			wantSOC: 74, wantIn: 410, wantOut: 1500, wantPVCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := DecodeBinaryPayload(tt.productCode, binaryFrame(tt.messageType, tt.fields...))
			if err != nil {
				t.Fatalf("decode binary: %v", err)
			}
			telemetry := NormalizeTelemetry(DeviceRef{ProductCode: tt.productCode, DeviceSN: "SN"}, msg.Values)
			assertNumber(t, telemetry.Params, "soc", tt.wantSOC)
			assertNumber(t, telemetry.Params, "wattsInSum", tt.wantIn)
			assertNumber(t, telemetry.Params, "wattsOutSum", tt.wantOut)
			assertNumber(t, telemetry.Capabilities, "pv_input_count", tt.wantPVCount)
			if telemetry.Metadata["support_status"] != string(SupportEnabled) {
				t.Fatalf("metadata support status = %#v", telemetry.Metadata)
			}
		})
	}
}

func TestDecodeNormalizeX1HESJSONFixture(t *testing.T) {
	body := map[string]any{
		"soc":  62,
		"pp":   650,
		"gp":   3705,
		"g2lp": 1409,
		"lp":   1409,
		"bds": []any{
			map[string]any{"sn": "module-1", "soc": 61, "power": -1148},
			map[string]any{"sn": "module-2", "soc": 63, "power": -1148},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := DecodeJSONPayload("A5101", raw)
	if err != nil {
		t.Fatalf("decode JSON payload: %v", err)
	}
	telemetry := NormalizeTelemetry(DeviceRef{ProductCode: "A5101", DeviceSN: "SN-X1"}, msg.Values)
	assertNumber(t, telemetry.Params, "soc", 62)
	assertNumber(t, telemetry.Params, "wattsInSum", 4355)
	assertNumber(t, telemetry.Params, "wattsOutSum", 1409)
	assertNumber(t, telemetry.Capabilities, "battery_module_count", 2)
	assertNumber(t, telemetry.Capabilities, "pv_input_count", 1)
}

func TestZeroPreservationAndPartialMerge(t *testing.T) {
	base := map[string]any{
		"battery_soc":        float64(55),
		"output_power_total": float64(700),
		"pv_1_power":         float64(120),
	}
	next := map[string]any{
		"output_power_total": float64(0),
		"pv_1_power":         float64(0),
		"pv_2_power":         float64(85),
	}
	merged := MergeValues(base, next)
	assertNumber(t, merged, "battery_soc", 55)
	assertNumber(t, merged, "output_power_total", 0)
	assertNumber(t, merged, "pv_1_power", 0)
	assertNumber(t, merged, "pv_2_power", 85)

	telemetry := NormalizeTelemetry(DeviceRef{ProductCode: "A17C5", DeviceSN: "SN"}, merged)
	assertNumber(t, telemetry.Params, "wattsOutSum", 0)
	assertNumber(t, telemetry.Params, "pv1ChargeWatts", 0)
	assertNumber(t, telemetry.Params, "pv2ChargeWatts", 85)
}

func TestNormalizeTelemetryDoesNotExposeRawSensitiveFields(t *testing.T) {
	telemetry := NormalizeTelemetry(DeviceRef{ProductCode: "A1783", DeviceSN: "SN-C2000"}, map[string]any{
		"device_sn":       "SN-C2000",
		"battery_soc":     float64(80),
		"pv_1_power":      float64(120),
		"mystery_counter": float64(5),
	})
	if _, ok := telemetry.Metadata["raw_fields"]; ok {
		t.Fatalf("metadata should not expose raw telemetry values: %#v", telemetry.Metadata)
	}
	fields, ok := telemetry.Metadata["field_names"].([]any)
	if !ok {
		t.Fatalf("metadata field_names = %#v", telemetry.Metadata["field_names"])
	}
	want := []string{"battery_soc", "device_sn", "mystery_counter", "pv_1_power"}
	if len(fields) != len(want) {
		t.Fatalf("field_names = %#v, want %#v", fields, want)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Fatalf("field_names[%d] = %q, want %q", i, fields[i], want[i])
		}
	}
}

func testP256PrivateKey(hexScalar string) (*ecdh.PrivateKey, error) {
	scalar, err := hex.DecodeString(strings.Repeat("0", 64-len(hexScalar)) + hexScalar)
	if err != nil {
		return nil, err
	}
	return ecdh.P256().NewPrivateKey(scalar)
}

func decryptPasswordForTest(serverPrivate *ecdh.PrivateKey, clientPublic []byte, ciphertext string) (string, error) {
	peer, err := ecdh.P256().NewPublicKey(clientPublic)
	if err != nil {
		return "", err
	}
	shared, err := serverPrivate.ECDH(peer)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(shared)
	if err != nil {
		return "", err
	}
	cipher.NewCBCDecrypter(block, shared[:aesBlockSize]).CryptBlocks(raw, raw)
	unpadded, err := pkcs7Unpad(raw, aesBlockSize)
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func assertNumber(t *testing.T, root map[string]any, key string, want float64) {
	t.Helper()
	got, ok := toFloat(root[key])
	if !ok || got != want {
		t.Fatalf("%s = %#v, want %v in %#v", key, root[key], want, root)
	}
}

func mqttWrapper(t *testing.T, appName, pn, sn, field string, payload []byte) []byte {
	t.Helper()
	inner := map[string]any{
		"pn":  pn,
		"sn":  sn,
		field: base64.StdEncoding.EncodeToString(payload),
	}
	innerJSON, err := json.Marshal(inner)
	if err != nil {
		t.Fatal(err)
	}
	outer := map[string]any{
		"head": map[string]any{
			"timestamp": 1700000000,
			"device_pn": pn,
			"device_sn": sn,
		},
		"payload": string(innerJSON),
	}
	raw, err := json.Marshal(outer)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

const (
	typeString byte = 0x00
	typeUI     byte = 0x01
	typeSILE   byte = 0x02
	typeVar    byte = 0x03
)

func binaryFrame(msgType string, fields ...[]byte) []byte {
	body := bytes.Join(fields, nil)
	out := []byte{0xff, 0x09, 0x00, 0x00, 0x03, 0x01, 0x0f}
	msgTypeBytes, _ := hex.DecodeString(msgType)
	out = append(out, msgTypeBytes...)
	out = append(out, 0x01)
	out = append(out, body...)
	out = append(out, 0x00)
	length := len(out)
	out[2] = byte(length)
	out[3] = byte(length >> 8)
	var checksum byte
	for _, b := range out[:len(out)-1] {
		checksum ^= b
	}
	out[len(out)-1] = checksum
	return out
}

func tlvString(field string, value string) []byte {
	id, _ := hex.DecodeString(field)
	raw := append([]byte{typeString}, []byte(value)...)
	return append([]byte{id[0], byte(len(raw))}, raw...)
}

func tlvNumber(field string, typ byte, value int64) []byte {
	id, _ := hex.DecodeString(field)
	var raw []byte
	switch typ {
	case typeUI:
		raw = []byte{typeUI, byte(value)}
	case typeSILE:
		raw = []byte{typeSILE, byte(value), byte(value >> 8)}
	default:
		raw = []byte{typeVar, byte(value), byte(value >> 8), byte(value >> 16), byte(value >> 24)}
	}
	return append([]byte{id[0], byte(len(raw))}, raw...)
}

var (
	testRootPEM string
	testCertPEM string
	testKeyPEM  string
)

func init() {
	var err error
	testRootPEM, testCertPEM, testKeyPEM, err = generateTestCerts()
	if err != nil {
		panic(err)
	}
}

func generateTestCerts() (string, string, string, error) {
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
