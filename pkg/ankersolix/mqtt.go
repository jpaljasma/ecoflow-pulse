package ankersolix

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

type MQTTSessionInfo struct {
	UserID          string `json:"user_id"`
	AppName         string `json:"app_name"`
	ThingName       string `json:"thing_name"`
	CertificateID   string `json:"certificate_id"`
	EndpointAddress string `json:"endpoint_addr"`
	RootCAPEM       string `json:"aws_root_ca1_pem"`
	CertificatePEM  string `json:"certificate_pem"`
	PrivateKeyPEM   string `json:"private_key"`
}

func (i *MQTTSessionInfo) Normalize() {
	i.UserID = strings.TrimSpace(i.UserID)
	i.AppName = strings.TrimSpace(i.AppName)
	if i.AppName == "" {
		i.AppName = "anker_power"
	}
	i.ThingName = strings.TrimSpace(i.ThingName)
	i.CertificateID = strings.TrimSpace(i.CertificateID)
	i.EndpointAddress = normalizeMQTTHost(i.EndpointAddress)
}

func (i MQTTSessionInfo) ClientID(suffix string) string {
	thing := strings.TrimSpace(i.ThingName)
	if thing == "" {
		thing = strings.TrimSpace(i.UserID) + "-" + strings.TrimSpace(i.AppName)
	}
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		suffix = "00001"
	}
	if thing == "" {
		return ""
	}
	return thing + "_" + suffix
}

func (i MQTTSessionInfo) TLSConfig() (*tls.Config, error) {
	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM([]byte(i.RootCAPEM)); !ok {
		return nil, errors.New("parse anker solix mqtt root CA")
	}
	cert, err := tls.X509KeyPair([]byte(i.CertificatePEM), []byte(i.PrivateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse anker solix mqtt client certificate: %w", err)
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      roots,
		Certificates: []tls.Certificate{cert},
		ServerName:   normalizeMQTTHost(i.EndpointAddress),
	}, nil
}

func SubscribeTopic(info MQTTSessionInfo, ref DeviceRef) string {
	ref = ref.Normalize()
	app := strings.TrimSpace(info.AppName)
	if app == "" {
		app = "anker_power"
	}
	if ref.ProductCode == "" || ref.DeviceSN == "" {
		return ""
	}
	return "dt/" + app + "/" + ref.ProductCode + "/" + ref.DeviceSN + "/#"
}

func PublishTopic(info MQTTSessionInfo, ref DeviceRef) string {
	ref = ref.Normalize()
	app := strings.TrimSpace(info.AppName)
	if app == "" {
		app = "anker_power"
	}
	if ref.ProductCode == "" || ref.DeviceSN == "" {
		return ""
	}
	return "cmd/" + app + "/" + ref.ProductCode + "/" + ref.DeviceSN + "/req"
}

type CommandEnvelopeOptions struct {
	SessionID string
	Now       time.Time
	Seed      string
}

func BuildCommandEnvelope(info MQTTSessionInfo, ref DeviceRef, command []byte, opts CommandEnvelopeOptions) ([]byte, error) {
	ref = ref.Normalize()
	if ref.ProductCode == "" || ref.DeviceSN == "" {
		return nil, errors.New("anker solix command device ref is required")
	}
	if len(command) == 0 {
		return nil, errors.New("anker solix command payload is required")
	}
	if opts.SessionID == "" {
		opts.SessionID = "1234-5678"
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.Seed == "" {
		opts.Seed = "1"
	}
	appName := strings.TrimSpace(info.AppName)
	if appName == "" {
		appName = "anker_power"
	}
	payload := map[string]any{
		"device_sn":  ref.DeviceSN,
		"account_id": strings.TrimSpace(info.UserID),
		"data":       base64.StdEncoding.EncodeToString(command),
	}
	envelope := map[string]any{
		"head": map[string]any{
			"version":    "1.0.0.1",
			"client_id":  "android-" + appName + "-" + strings.TrimSpace(info.UserID) + "-" + strings.TrimSpace(info.CertificateID),
			"sess_id":    opts.SessionID,
			"msg_seq":    1,
			"seed":       opts.Seed,
			"timestamp":  opts.Now.Unix(),
			"cmd_status": 2,
			"cmd":        17,
			"sign_code":  1,
			"device_pn":  ref.ProductCode,
			"device_sn":  ref.DeviceSN,
		},
		"payload": mustCompactJSON(payload),
	}
	return json.Marshal(envelope)
}

func RealtimeTriggerCommand(timeoutSeconds int) []byte {
	if timeoutSeconds < 30 {
		timeoutSeconds = 30
	}
	if timeoutSeconds > 600 {
		timeoutSeconds = 600
	}
	return buildCommandFrame("0057",
		commandTLV("a1", 0x00, []byte{0x22}),
		commandTLV("a2", 0x01, []byte{0x01}),
		commandTLV("a3", 0x03, []byte{byte(timeoutSeconds), byte(timeoutSeconds >> 8), byte(timeoutSeconds >> 16), byte(timeoutSeconds >> 24)}),
	)
}

func StatusRequestCommand() []byte {
	return buildCommandFrame("0040", commandTLV("a1", 0x00, []byte{0x22}))
}

func buildCommandFrame(msgType string, fields ...[]byte) []byte {
	out := []byte{0xff, 0x09, 0x00, 0x00, 0x03, 0x00, 0x0f}
	msgTypeBytes := mustHex(msgType)
	out = append(out, msgTypeBytes...)
	for _, field := range fields {
		out = append(out, field...)
	}
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

func commandTLV(field string, typ byte, value []byte) []byte {
	id := mustHex(field)
	raw := append([]byte{typ}, value...)
	return append([]byte{id[0], byte(len(raw))}, raw...)
}

type MQTTConfig struct {
	Address        string
	ClientID       string
	KeepAlive      time.Duration
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	TLSConfig      *tls.Config
	BufferSize     int
}

type MQTTSubscriber struct {
	cfg             MQTTConfig
	client          mqtt.Client
	msgs            chan ecoflowmqtt.Message
	droppedMessages atomic.Uint64
}

func NewMQTTSubscriber(cfg MQTTConfig) (*MQTTSubscriber, error) {
	cfg.Address = strings.TrimSpace(cfg.Address)
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	if cfg.Address == "" {
		return nil, errors.New("address is required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("client ID is required")
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = 60 * time.Second
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 15 * time.Second
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 16
	}
	return &MQTTSubscriber{cfg: cfg, msgs: make(chan ecoflowmqtt.Message, cfg.BufferSize)}, nil
}

func (s *MQTTSubscriber) Connect(ctx context.Context) error {
	if s == nil {
		return errors.New("subscriber is required")
	}
	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBrokerURL(s.cfg.Address))
	opts.SetClientID(s.cfg.ClientID)
	opts.SetKeepAlive(s.cfg.KeepAlive)
	opts.SetConnectTimeout(s.cfg.ConnectTimeout)
	opts.SetAutoReconnect(false)
	if s.cfg.TLSConfig != nil {
		opts.SetTLSConfig(s.cfg.TLSConfig)
	} else {
		opts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	s.client = mqtt.NewClient(opts)
	return waitPahoToken(ctx, s.client.Connect(), "connect anker solix mqtt")
}

func (s *MQTTSubscriber) Subscribe(ctx context.Context, topic string, qos byte) error {
	if s == nil || s.client == nil {
		return errors.New("subscriber is not connected")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("topic is required")
	}
	return waitPahoToken(ctx, s.client.Subscribe(topic, qos, func(_ mqtt.Client, msg mqtt.Message) {
		out := ecoflowmqtt.Message{
			Topic:     msg.Topic(),
			Payload:   append([]byte(nil), msg.Payload()...),
			QoS:       msg.Qos(),
			Retain:    msg.Retained(),
			Duplicate: msg.Duplicate(),
		}
		s.enqueueMessage(out)
	}), "subscribe anker solix mqtt")
}

func (s *MQTTSubscriber) enqueueMessage(msg ecoflowmqtt.Message) {
	select {
	case s.msgs <- msg:
	default:
		select {
		case <-s.msgs:
			s.droppedMessages.Add(1)
		default:
		}
		select {
		case s.msgs <- msg:
		default:
			s.droppedMessages.Add(1)
		}
	}
}

func (s *MQTTSubscriber) Publish(ctx context.Context, topic string, payload []byte, qos byte) error {
	if s == nil || s.client == nil {
		return errors.New("subscriber is not connected")
	}
	return waitPahoToken(ctx, s.client.Publish(topic, qos, false, payload), "publish anker solix mqtt")
}

func (s *MQTTSubscriber) ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error) {
	if s == nil {
		return ecoflowmqtt.Message{}, errors.New("subscriber is required")
	}
	timer := time.NewTimer(s.cfg.ReadTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ecoflowmqtt.Message{}, ctx.Err()
	case <-timer.C:
		return ecoflowmqtt.Message{}, context.DeadlineExceeded
	case msg := <-s.msgs:
		return msg, nil
	}
}

func (s *MQTTSubscriber) Disconnect() error {
	if s == nil || s.client == nil {
		return nil
	}
	s.client.Disconnect(250)
	return nil
}

func (s *MQTTSubscriber) DroppedMessages() uint64 {
	if s == nil {
		return 0
	}
	return s.droppedMessages.Load()
}

func (s *MQTTSubscriber) Close() error {
	return s.Disconnect()
}

func waitPahoToken(ctx context.Context, token mqtt.Token, action string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		if err := token.Error(); err != nil {
			return fmt.Errorf("%s: %w", action, err)
		}
		return nil
	}
}

func mustCompactJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func mqttBrokerURL(address string) string {
	address = cleanMQTTAddress(address)
	if _, _, err := net.SplitHostPort(address); err == nil {
		return "tls://" + address
	}
	return "tls://" + address + ":8883"
}

func normalizeMQTTHost(address string) string {
	address = cleanMQTTAddress(address)
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return address
}

func cleanMQTTAddress(address string) string {
	address = strings.TrimSpace(address)
	address = strings.TrimPrefix(address, "tls://")
	address = strings.TrimPrefix(address, "ssl://")
	address = strings.TrimPrefix(address, "mqtts://")
	address = strings.TrimSuffix(address, "/")
	return address
}
