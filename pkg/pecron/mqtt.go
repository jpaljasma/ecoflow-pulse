package pecron

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

type MQTTConfig struct {
	Address        string
	Path           string
	Token          string
	ClientID       string
	KeepAlive      time.Duration
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	TLSConfig      *tls.Config
	BufferSize     int
}

type MQTTSubscriber struct {
	cfg    MQTTConfig
	client mqtt.Client
	msgs   chan ecoflowmqtt.Message
}

func NewMQTTSubscriber(cfg MQTTConfig) (*MQTTSubscriber, error) {
	cfg.Address = strings.TrimSpace(cfg.Address)
	cfg.Path = strings.TrimSpace(cfg.Path)
	cfg.Token = strings.TrimSpace(cfg.Token)
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	if cfg.Address == "" {
		return nil, errors.New("address is required")
	}
	if cfg.Path == "" {
		cfg.Path = "/ws/v2"
	}
	if cfg.Token == "" {
		return nil, errors.New("token is required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("client ID is required")
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = 60 * time.Second
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 16
	}
	return &MQTTSubscriber{
		cfg:  cfg,
		msgs: make(chan ecoflowmqtt.Message, cfg.BufferSize),
	}, nil
}

func (s *MQTTSubscriber) Connect(ctx context.Context) error {
	if s == nil {
		return errors.New("subscriber is required")
	}
	opts := mqtt.NewClientOptions()
	opts.AddBroker("wss://" + s.cfg.Address + s.cfg.Path)
	opts.SetClientID(s.cfg.ClientID)
	opts.SetUsername("")
	opts.SetPassword(s.cfg.Token)
	opts.SetKeepAlive(s.cfg.KeepAlive)
	opts.SetConnectTimeout(s.cfg.ConnectTimeout)
	opts.SetAutoReconnect(false)
	if s.cfg.TLSConfig != nil {
		opts.SetTLSConfig(s.cfg.TLSConfig)
	} else {
		opts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	s.client = mqtt.NewClient(opts)
	token := s.client.Connect()
	return waitPahoToken(ctx, token, "connect pecron mqtt")
}

func (s *MQTTSubscriber) Subscribe(ctx context.Context, topic string, qos byte) error {
	if s == nil || s.client == nil {
		return errors.New("subscriber is not connected")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("topic is required")
	}
	token := s.client.Subscribe(topic, qos, func(_ mqtt.Client, msg mqtt.Message) {
		out := ecoflowmqtt.Message{
			Topic:     msg.Topic(),
			Payload:   append([]byte(nil), msg.Payload()...),
			QoS:       msg.Qos(),
			Retain:    msg.Retained(),
			Duplicate: msg.Duplicate(),
		}
		select {
		case s.msgs <- out:
		default:
		}
	})
	return waitPahoToken(ctx, token, "subscribe pecron mqtt")
}

func (s *MQTTSubscriber) ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error) {
	if s == nil {
		return ecoflowmqtt.Message{}, errors.New("subscriber is required")
	}
	timeout := s.cfg.ReadTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timer := time.NewTimer(timeout)
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
