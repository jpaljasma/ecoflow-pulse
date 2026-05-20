package pecron

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
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
	WriteTimeout   time.Duration
	TLSConfig      *tls.Config
	BufferSize     int
}

type MQTTSubscriber struct {
	cfg       MQTTConfig
	conn      net.Conn
	msgs      chan ecoflowmqtt.Message
	writeMu   sync.Mutex
	closeOnce sync.Once
	done      chan struct{}
	packetID  atomic.Uint32
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
		done: make(chan struct{}),
	}, nil
}

func (s *MQTTSubscriber) Connect(ctx context.Context) error {
	if s == nil {
		return errors.New("subscriber is required")
	}
	connectTimeout, err := timeoutWithinContext(ctx, s.cfg.ConnectTimeout)
	if err != nil {
		return err
	}
	tlsConfig := s.cfg.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	conn, err := paho.NewWebsocket("wss://"+s.cfg.Address+s.cfg.Path, tlsConfig, connectTimeout, nil, nil)
	if err != nil {
		return fmt.Errorf("connect pecron mqtt websocket: %w", err)
	}
	s.writeMu.Lock()
	s.conn = conn
	s.writeMu.Unlock()
	cleanup := func() {
		s.writeMu.Lock()
		if s.conn == conn {
			s.conn = nil
		}
		s.writeMu.Unlock()
		_ = conn.Close()
	}
	connect := newPecronConnectPacket(s.cfg)
	if err := s.writeControlPacket(ctx, connect, s.writeTimeout()); err != nil {
		cleanup()
		return fmt.Errorf("connect pecron mqtt: %w", err)
	}
	packet, err := s.readControlPacket(ctx, s.cfg.ConnectTimeout)
	if err != nil {
		cleanup()
		return fmt.Errorf("connect pecron mqtt: %w", err)
	}
	connack, ok := packet.(*packets.ConnackPacket)
	if !ok {
		cleanup()
		return fmt.Errorf("connect pecron mqtt: expected CONNACK, got %T", packet)
	}
	if connack.ReturnCode != packets.Accepted {
		cleanup()
		if connErr := packets.ConnErrors[connack.ReturnCode]; connErr != nil {
			return fmt.Errorf("connect pecron mqtt: %w", connErr)
		}
		return fmt.Errorf("connect pecron mqtt: broker rejected connection with code %d", connack.ReturnCode)
	}
	s.startPingLoop()
	return nil
}

func (s *MQTTSubscriber) Subscribe(ctx context.Context, topic string, qos byte) error {
	return s.SubscribeMultiple(ctx, []string{topic}, qos)
}

func (s *MQTTSubscriber) SubscribeMultiple(ctx context.Context, topics []string, qos byte) error {
	if s == nil || s.currentConn() == nil {
		return errors.New("subscriber is not connected")
	}
	cleanTopics := make([]string, 0, len(topics))
	qoss := make([]byte, 0, len(topics))
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		cleanTopics = append(cleanTopics, topic)
		qoss = append(qoss, qos)
	}
	if len(cleanTopics) == 0 {
		return errors.New("at least one topic is required")
	}
	msgID := s.nextMessageID()
	subscribe := packets.NewControlPacket(packets.Subscribe).(*packets.SubscribePacket)
	subscribe.MessageID = msgID
	subscribe.Topics = cleanTopics
	subscribe.Qoss = qoss
	if err := s.writeControlPacket(ctx, subscribe, s.writeTimeout()); err != nil {
		return fmt.Errorf("subscribe pecron mqtt: %w", err)
	}
	for {
		packet, err := s.readControlPacket(ctx, s.cfg.ReadTimeout)
		if err != nil {
			return fmt.Errorf("subscribe pecron mqtt: %w", err)
		}
		switch msg := packet.(type) {
		case *packets.SubackPacket:
			if msg.MessageID != msgID {
				continue
			}
			if len(msg.ReturnCodes) < len(cleanTopics) {
				return fmt.Errorf("subscribe pecron mqtt: broker returned %d topic acknowledgements for %d topics", len(msg.ReturnCodes), len(cleanTopics))
			}
			for _, code := range msg.ReturnCodes {
				if code == 0x80 {
					return errors.New("subscribe pecron mqtt: subscription rejected")
				}
				if code > 2 {
					return fmt.Errorf("subscribe pecron mqtt: unexpected subscription return code %d", code)
				}
			}
			return nil
		default:
			if err := s.handleIncomingPacket(ctx, packet); err != nil {
				return fmt.Errorf("subscribe pecron mqtt: %w", err)
			}
		}
	}
}

func (s *MQTTSubscriber) Publish(ctx context.Context, topic string, payload []byte, qos byte) error {
	if s == nil || s.currentConn() == nil {
		return errors.New("subscriber is not connected")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("topic is required")
	}
	if qos > 1 {
		return errors.New("pecron mqtt publish only supports qos 0 or 1")
	}
	publish := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	publish.TopicName = topic
	publish.Payload = append([]byte(nil), payload...)
	publish.Qos = qos
	if qos > 0 {
		publish.MessageID = s.nextMessageID()
	}
	if err := s.writeControlPacket(ctx, publish, s.writeTimeout()); err != nil {
		return fmt.Errorf("publish pecron mqtt: %w", err)
	}
	if qos == 0 {
		return nil
	}
	for {
		packet, err := s.readControlPacket(ctx, s.cfg.ReadTimeout)
		if err != nil {
			return fmt.Errorf("publish pecron mqtt: %w", err)
		}
		switch msg := packet.(type) {
		case *packets.PubackPacket:
			if msg.MessageID == publish.MessageID {
				return nil
			}
		default:
			if err := s.handleIncomingPacket(ctx, packet); err != nil {
				return fmt.Errorf("publish pecron mqtt: %w", err)
			}
		}
	}
}

func (s *MQTTSubscriber) ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error) {
	if s == nil {
		return ecoflowmqtt.Message{}, errors.New("subscriber is required")
	}
	timeout := s.cfg.ReadTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	select {
	case msg := <-s.msgs:
		return msg, nil
	default:
	}
	for {
		packet, err := s.readControlPacket(ctx, timeout)
		if err != nil {
			return ecoflowmqtt.Message{}, err
		}
		if msg, ok, err := s.messageFromPacket(ctx, packet); ok || err != nil {
			return msg, err
		}
	}
}

func (s *MQTTSubscriber) Disconnect() error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		disconnect := packets.NewControlPacket(packets.Disconnect).(*packets.DisconnectPacket)
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		_ = s.writeControlPacket(ctx, disconnect, 250*time.Millisecond)
		s.writeMu.Lock()
		conn := s.conn
		s.conn = nil
		s.writeMu.Unlock()
		if conn != nil {
			err = conn.Close()
		}
	})
	return err
}

func (s *MQTTSubscriber) Close() error { return s.Disconnect() }

func newPecronConnectPacket(cfg MQTTConfig) *packets.ConnectPacket {
	connect := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	connect.ProtocolName = "MQTT"
	connect.ProtocolVersion = 4
	connect.CleanSession = true
	connect.Keepalive = keepAliveSeconds(cfg.KeepAlive)
	connect.ClientIdentifier = cfg.ClientID
	// Pecron's broker expects the Python Paho wire shape: username flag set
	// with a zero-length username and the access token carried as password.
	connect.UsernameFlag = true
	connect.Username = ""
	connect.PasswordFlag = true
	connect.Password = []byte(cfg.Token)
	return connect
}

func (s *MQTTSubscriber) writeControlPacket(ctx context.Context, packet packets.ControlPacket, timeout time.Duration) error {
	if s == nil {
		return errors.New("subscriber is required")
	}
	var buf bytes.Buffer
	if err := packet.Write(&buf); err != nil {
		return err
	}
	deadline, err := operationDeadline(ctx, timeout)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.conn == nil {
		return errors.New("subscriber is not connected")
	}
	if err := s.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	if _, err := s.conn.Write(buf.Bytes()); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if timeoutError(err) {
			return context.DeadlineExceeded
		}
		return err
	}
	return nil
}

func (s *MQTTSubscriber) readControlPacket(ctx context.Context, timeout time.Duration) (packets.ControlPacket, error) {
	if s == nil {
		return nil, errors.New("subscriber is required")
	}
	conn := s.currentConn()
	if conn == nil {
		return nil, errors.New("subscriber is not connected")
	}
	deadline, err := operationDeadline(ctx, timeout)
	if err != nil {
		return nil, err
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}
	packet, err := packets.ReadPacket(conn)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if timeoutError(err) {
			return nil, context.DeadlineExceeded
		}
		return nil, err
	}
	return packet, nil
}

func (s *MQTTSubscriber) handleIncomingPacket(ctx context.Context, packet packets.ControlPacket) error {
	msg, ok, err := s.messageFromPacket(ctx, packet)
	if err != nil {
		return err
	}
	if ok {
		select {
		case s.msgs <- msg:
		default:
		}
	}
	return err
}

func (s *MQTTSubscriber) messageFromPacket(ctx context.Context, packet packets.ControlPacket) (ecoflowmqtt.Message, bool, error) {
	publish, ok := packet.(*packets.PublishPacket)
	if !ok {
		return ecoflowmqtt.Message{}, false, nil
	}
	if publish.Qos == 1 {
		ack := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
		ack.MessageID = publish.MessageID
		if err := s.writeControlPacket(ctx, ack, s.writeTimeout()); err != nil {
			return ecoflowmqtt.Message{}, false, err
		}
	}
	msg := ecoflowmqtt.Message{
		Topic:     publish.TopicName,
		Payload:   append([]byte(nil), publish.Payload...),
		QoS:       publish.Qos,
		Retain:    publish.Retain,
		Duplicate: publish.Dup,
	}
	return msg, true, nil
}

func (s *MQTTSubscriber) startPingLoop() {
	interval := s.cfg.KeepAlive / 2
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				ping := packets.NewControlPacket(packets.Pingreq).(*packets.PingreqPacket)
				ctx, cancel := context.WithTimeout(context.Background(), s.writeTimeout())
				_ = s.writeControlPacket(ctx, ping, s.writeTimeout())
				cancel()
			}
		}
	}()
}

func (s *MQTTSubscriber) nextMessageID() uint16 {
	for {
		next := uint16(s.packetID.Add(1))
		if next != 0 {
			return next
		}
	}
}

func (s *MQTTSubscriber) writeTimeout() time.Duration {
	if s == nil {
		return 15 * time.Second
	}
	if s.cfg.WriteTimeout > 0 {
		return s.cfg.WriteTimeout
	}
	return 15 * time.Second
}

func keepAliveSeconds(keepAlive time.Duration) uint16 {
	seconds := int64(keepAlive / time.Second)
	if seconds <= 0 {
		return 60
	}
	if seconds > 65535 {
		return 65535
	}
	return uint16(seconds)
}

func timeoutWithinContext(ctx context.Context, fallback time.Duration) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if fallback <= 0 {
		fallback = 10 * time.Second
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, context.DeadlineExceeded
		}
		if remaining < fallback {
			return remaining, nil
		}
	}
	return fallback, nil
}

func operationDeadline(ctx context.Context, fallback time.Duration) (time.Time, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	if fallback <= 0 {
		fallback = 30 * time.Second
	}
	deadline := time.Now().Add(fallback)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if !deadline.After(time.Now()) {
		return time.Time{}, context.DeadlineExceeded
	}
	return deadline, nil
}

func (s *MQTTSubscriber) currentConn() net.Conn {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn
}

func timeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
