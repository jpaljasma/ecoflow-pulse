package ecoflowmqtt

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	packetTypeConnect   = 1
	packetTypeConnAck   = 2
	packetTypePublish   = 3
	packetTypePubAck    = 4
	packetTypeSubscribe = 8
	packetTypeSubAck    = 9
	packetTypePingReq   = 12
	packetTypePingResp  = 13
)

// Config controls transport and authentication for a subscriber session.
type Config struct {
	// Address is the MQTT broker host:port.
	Address string
	// Username is the MQTT username.
	Username string
	// Password is the MQTT password.
	Password string
	// ClientID is the MQTT client identifier.
	ClientID string
	// KeepAlive controls MQTT keepalive heartbeat interval.
	KeepAlive time.Duration
	// ConnectTimeout caps TCP/TLS connect duration.
	ConnectTimeout time.Duration
	// ReadTimeout is applied for packet reads; timeout triggers a ping.
	ReadTimeout time.Duration
	// TLSConfig controls TLS settings; when nil, secure defaults are used.
	TLSConfig *tls.Config
}

// Message is one incoming MQTT PUBLISH packet.
type Message struct {
	Topic     string
	Payload   []byte
	QoS       byte
	Retain    bool
	Duplicate bool
}

// Subscriber is a lightweight MQTT 3.1.1 subscriber client.
type Subscriber struct {
	cfg Config

	mu       sync.Mutex
	conn     net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	packetID uint16
	closed   bool
}

// NewSubscriber validates configuration and returns a subscriber.
func NewSubscriber(cfg Config) (*Subscriber, error) {
	cfg.Address = strings.TrimSpace(cfg.Address)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Password = strings.TrimSpace(cfg.Password)
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	if cfg.Address == "" {
		return nil, errors.New("address is required")
	}
	if cfg.Username == "" {
		return nil, errors.New("username is required")
	}
	if cfg.Password == "" {
		return nil, errors.New("password is required")
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
	return &Subscriber{cfg: cfg}, nil
}

// Connect establishes TLS transport and performs MQTT CONNECT handshake.
func (s *Subscriber) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("subscriber is closed")
	}
	if s.conn != nil {
		return errors.New("subscriber is already connected")
	}

	conn, err := s.dial(ctx)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	if err := writePacket(conn, writer, ctx, byte(packetTypeConnect<<4), buildConnectPacket(s.cfg)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("send CONNECT: %w", err)
	}
	ptype, _, body, err := readPacket(conn, reader, ctx, s.cfg.ReadTimeout)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("read CONNACK: %w", err)
	}
	if ptype != packetTypeConnAck {
		_ = conn.Close()
		return fmt.Errorf("unexpected packet type %d while waiting CONNACK", ptype)
	}
	if len(body) != 2 {
		_ = conn.Close()
		return fmt.Errorf("invalid CONNACK length %d", len(body))
	}
	if body[1] != 0 {
		_ = conn.Close()
		return fmt.Errorf("connect rejected, return code=%d", body[1])
	}

	s.conn = conn
	s.reader = reader
	s.writer = writer
	return nil
}

func (s *Subscriber) dial(ctx context.Context) (net.Conn, error) {
	host, _, err := net.SplitHostPort(s.cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", s.cfg.Address, err)
	}
	tlsConfig := s.cfg.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if strings.TrimSpace(tlsConfig.ServerName) == "" {
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ServerName = host
	}
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: s.cfg.ConnectTimeout},
		Config:    tlsConfig,
	}
	conn, err := dialer.DialContext(ctx, "tcp", s.cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("dial mqtt broker: %w", err)
	}
	return conn, nil
}

// Subscribe sends a SUBSCRIBE request for one topic.
func (s *Subscriber) Subscribe(ctx context.Context, topic string, qos byte) error {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("topic is required")
	}
	if qos > 1 {
		return fmt.Errorf("unsupported qos %d", qos)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return errors.New("subscriber is not connected")
	}

	packetID := s.nextPacketID()
	packet := make([]byte, 0, 2+len(topic)+3)
	packet = appendPacketID(packet, packetID)
	packet = appendMQTTString(packet, topic)
	packet = append(packet, qos)

	if err := writePacket(s.conn, s.writer, ctx, byte(packetTypeSubscribe<<4|0x02), packet); err != nil {
		return fmt.Errorf("send SUBSCRIBE: %w", err)
	}

	for {
		ptype, _, body, err := readPacket(s.conn, s.reader, ctx, s.cfg.ReadTimeout)
		if err != nil {
			return fmt.Errorf("read SUBACK: %w", err)
		}
		switch ptype {
		case packetTypeSubAck:
			if len(body) < 3 {
				return fmt.Errorf("invalid SUBACK length %d", len(body))
			}
			ackID := binary.BigEndian.Uint16(body[:2])
			if ackID != packetID {
				continue
			}
			for _, code := range body[2:] {
				if code == 0x80 {
					return errors.New("subscription rejected")
				}
			}
			return nil
		case packetTypePingResp:
			continue
		default:
			continue
		}
	}
}

// ReadMessage blocks until a PUBLISH packet is received.
func (s *Subscriber) ReadMessage(ctx context.Context) (Message, error) {
	s.mu.Lock()
	conn := s.conn
	reader := s.reader
	writer := s.writer
	s.mu.Unlock()
	if conn == nil || reader == nil || writer == nil {
		return Message{}, errors.New("subscriber is not connected")
	}

	for {
		ptype, flags, body, err := readPacket(conn, reader, ctx, s.cfg.ReadTimeout)
		if err != nil {
			if isTimeout(err) {
				if pingErr := writePacket(conn, writer, ctx, byte(packetTypePingReq<<4), nil); pingErr != nil {
					return Message{}, fmt.Errorf("send PINGREQ: %w", pingErr)
				}
				continue
			}
			return Message{}, err
		}

		switch ptype {
		case packetTypePingResp:
			continue
		case packetTypePublish:
			message, packetID, err := parsePublish(flags, body)
			if err != nil {
				return Message{}, err
			}
			if message.QoS == 1 {
				ack := make([]byte, 0, 2)
				ack = appendPacketID(ack, packetID)
				if err := writePacket(conn, writer, ctx, byte(packetTypePubAck<<4), ack); err != nil {
					return Message{}, fmt.Errorf("send PUBACK: %w", err)
				}
			}
			if message.QoS > 1 {
				return Message{}, fmt.Errorf("unsupported publish qos %d", message.QoS)
			}
			return message, nil
		default:
			continue
		}
	}
}

// Disconnect closes the underlying MQTT transport but keeps the subscriber reusable.
func (s *Subscriber) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	s.reader = nil
	s.writer = nil
	return err
}

// Close closes the underlying MQTT transport.
func (s *Subscriber) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return s.Disconnect()
}

func (s *Subscriber) nextPacketID() uint16 {
	s.packetID++
	if s.packetID == 0 {
		s.packetID = 1
	}
	return s.packetID
}

func buildConnectPacket(cfg Config) []byte {
	flags := byte(0x02)
	if cfg.Username != "" {
		flags |= 0x80
	}
	if cfg.Password != "" {
		flags |= 0x40
	}
	keepAliveSeconds := uint16(cfg.KeepAlive / time.Second)
	if keepAliveSeconds == 0 {
		keepAliveSeconds = 60
	}

	packet := make([]byte, 0, 64)
	packet = appendMQTTString(packet, "MQTT")
	packet = append(packet, 0x04)
	packet = append(packet, flags)
	packet = append(packet, byte(keepAliveSeconds>>8), byte(keepAliveSeconds))
	packet = appendMQTTString(packet, cfg.ClientID)
	packet = appendMQTTString(packet, cfg.Username)
	packet = appendMQTTString(packet, cfg.Password)
	return packet
}

func parsePublish(flags byte, body []byte) (Message, uint16, error) {
	topic, offset, err := readMQTTString(body, 0)
	if err != nil {
		return Message{}, 0, fmt.Errorf("decode publish topic: %w", err)
	}
	qos := byte((flags & 0x06) >> 1)
	packetID := uint16(0)
	if qos > 0 {
		if len(body[offset:]) < 2 {
			return Message{}, 0, errors.New("missing publish packet identifier")
		}
		packetID = binary.BigEndian.Uint16(body[offset : offset+2])
		offset += 2
	}
	if offset > len(body) {
		return Message{}, 0, errors.New("invalid publish payload offset")
	}
	payload := append([]byte(nil), body[offset:]...)
	return Message{
		Topic:     topic,
		Payload:   payload,
		QoS:       qos,
		Retain:    flags&0x01 != 0,
		Duplicate: flags&0x08 != 0,
	}, packetID, nil
}

func writePacket(conn net.Conn, writer *bufio.Writer, ctx context.Context, header byte, body []byte) error {
	if err := applyWriteDeadline(conn, ctx, 10*time.Second); err != nil {
		return err
	}
	defer clearWriteDeadline(conn)

	if err := writer.WriteByte(header); err != nil {
		return err
	}
	if err := writeRemainingLength(writer, len(body)); err != nil {
		return err
	}
	if len(body) > 0 {
		if _, err := writer.Write(body); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func readPacket(conn net.Conn, reader *bufio.Reader, ctx context.Context, readTimeout time.Duration) (byte, byte, []byte, error) {
	if err := applyReadDeadline(conn, ctx, readTimeout); err != nil {
		return 0, 0, nil, err
	}
	defer clearReadDeadline(conn)

	header, err := reader.ReadByte()
	if err != nil {
		return 0, 0, nil, err
	}
	remainingLength, err := readRemainingLength(reader)
	if err != nil {
		return 0, 0, nil, err
	}
	body := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return 0, 0, nil, err
	}
	return header >> 4, header & 0x0F, body, nil
}

func applyWriteDeadline(conn net.Conn, ctx context.Context, fallback time.Duration) error {
	deadline := time.Now().Add(fallback)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	return conn.SetWriteDeadline(deadline)
}

func applyReadDeadline(conn net.Conn, ctx context.Context, fallback time.Duration) error {
	deadline := time.Now().Add(fallback)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	return conn.SetReadDeadline(deadline)
}

func clearWriteDeadline(conn net.Conn) {
	_ = conn.SetWriteDeadline(time.Time{})
}

func clearReadDeadline(conn net.Conn) {
	_ = conn.SetReadDeadline(time.Time{})
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func appendPacketID(dst []byte, packetID uint16) []byte {
	return append(dst, byte(packetID>>8), byte(packetID))
}

func appendMQTTString(dst []byte, value string) []byte {
	if len(value) > 0xFFFF {
		value = value[:0xFFFF]
	}
	dst = append(dst, byte(len(value)>>8), byte(len(value)))
	return append(dst, value...)
}

func readMQTTString(data []byte, offset int) (string, int, error) {
	if offset < 0 || offset+2 > len(data) {
		return "", offset, errors.New("missing string length")
	}
	length := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+length > len(data) {
		return "", offset, errors.New("string length exceeds packet")
	}
	return string(data[offset : offset+length]), offset + length, nil
}

func writeRemainingLength(writer *bufio.Writer, value int) error {
	if value < 0 {
		return errors.New("remaining length cannot be negative")
	}
	for {
		digit := byte(value % 128)
		value /= 128
		if value > 0 {
			digit |= 0x80
		}
		if err := writer.WriteByte(digit); err != nil {
			return err
		}
		if value == 0 {
			return nil
		}
	}
}

func readRemainingLength(reader *bufio.Reader) (int, error) {
	multiplier := 1
	value := 0
	for i := 0; i < 4; i++ {
		digit, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		value += int(digit&0x7F) * multiplier
		if digit&0x80 == 0 {
			return value, nil
		}
		multiplier *= 128
	}
	return 0, errors.New("malformed remaining length")
}
