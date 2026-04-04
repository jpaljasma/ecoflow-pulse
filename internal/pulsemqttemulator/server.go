package pulsemqttemulator

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultMQTTAddr        = ":8883"
	defaultPublishInterval = 5 * time.Second
	dpuXPackEnergyWh       = 6144
	dpuXChargeLossWatts    = 110.0
	dpuXBatteryCapacityWh  = 4 * dpuXPackEnergyWh
	dpuXChargeMaxSOC       = 95.0
	dpuXChargeResumeSOC    = 93.0
	dpuXBackupReserveSOC   = 40.0
	dpuXACInputLimitWatts  = 1800.0
	dpuXReserveTopUpWatts  = 650.0
	dpuXACInputVolts       = 230.0
	dpuXSimulationStep     = 2 * time.Minute

	businessCodeOK            = "0"
	businessCodeAccessInvalid = "8513"
	businessMessageSuccess    = "Success"
	businessMessageAccessKey  = "accessKey is invalid"

	packetTypeConnect    = 1
	packetTypeConnAck    = 2
	packetTypePublish    = 3
	packetTypeSubscribe  = 8
	packetTypeSubAck     = 9
	packetTypePingReq    = 12
	packetTypePingResp   = 13
	packetTypeDisconnect = 14
)

type DeviceConfig struct {
	SN                  string
	DeviceName          string
	ProductName         string
	BatteryPackCount    int
	BatteryPackEnergyWh float64
}

type Config struct {
	HTTPAddr            string
	MQTTAddr            string
	BrokerAdvertiseHost string
	BrokerAdvertisePort string
	AccessKey           string
	SecretKey           string
	MQTTUsername        string
	MQTTPassword        string
	PublishInterval     time.Duration
	Device              DeviceConfig
	Logger              *slog.Logger
}

type Server struct {
	cfg Config
	log *slog.Logger

	httpServer   *http.Server
	httpListener net.Listener
	mqttListener net.Listener

	mu      sync.RWMutex
	clients map[*mqttClient]struct{}
	state   *deviceState

	tickCancel context.CancelFunc
	tickDone   chan struct{}
	closeOnce  sync.Once
}

type mqttClient struct {
	server          *Server
	conn            net.Conn
	reader          *bufio.Reader
	writer          *bufio.Writer
	idleReadTimeout time.Duration

	mu     sync.RWMutex
	packet sync.Mutex
	topics map[string]struct{}

	closeOnce sync.Once
}

type deviceState struct {
	mu   sync.Mutex
	tick int
}

type telemetrySnapshot struct {
	Tick            int
	ObservedUnixMS  int64
	OverallSOC      float64
	PackSOCs        []int
	PackTemps       []int
	PackWatts       []float64
	PackHeatTimes   []int
	RemainTimeMin   int
	WattsInSum      float64
	WattsOutSum     float64
	InLvMpptVol     float64
	InLvMpptAmp     float64
	InHvMpptVol     float64
	InHvMpptAmp     float64
	ACInputWatts    float64
	ACOutputWatts   float64
	DCOutputWatts   float64
	USBOutputWatts  float64
	BMSInputWatts   float64
	BMSOutputWatts  float64
	BatAmp          float64
	BatVol          float64
	PDTemp          int
	FanState        int
	BMSModeSet      int
	BatteryHeatMode int
	ChargeMaxSOC    int
	DischargeMinSOC int
	BackupSOC       int
}

type batterySimState struct {
	SOC          float64
	SolarLimited bool
}

func NewServer(cfg Config) (*Server, error) {
	cfg = normalizeConfig(cfg)
	if strings.TrimSpace(cfg.AccessKey) == "" {
		return nil, errors.New("access key is required")
	}
	if strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, errors.New("secret key is required")
	}
	if strings.TrimSpace(cfg.MQTTUsername) == "" {
		return nil, errors.New("mqtt username is required")
	}
	if strings.TrimSpace(cfg.MQTTPassword) == "" {
		return nil, errors.New("mqtt password is required")
	}
	if strings.TrimSpace(cfg.Device.SN) == "" {
		return nil, errors.New("device sn is required")
	}
	return &Server{
		cfg:      cfg,
		log:      cfg.Logger,
		clients:  map[*mqttClient]struct{}{},
		state:    &deviceState{},
		tickDone: make(chan struct{}),
	}, nil
}

func (s *Server) Start() error {
	if s == nil {
		return errors.New("server is required")
	}

	httpListener, err := net.Listen("tcp", s.cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen http: %w", err)
	}
	tlsConfig, err := generateTLSConfig(s.cfg.BrokerAdvertiseHost)
	if err != nil {
		_ = httpListener.Close()
		return fmt.Errorf("generate mqtt tls config: %w", err)
	}
	mqttRawListener, err := net.Listen("tcp", s.cfg.MQTTAddr)
	if err != nil {
		_ = httpListener.Close()
		return fmt.Errorf("listen mqtt: %w", err)
	}
	mqttListener := tls.NewListener(mqttRawListener, tlsConfig)

	s.httpListener = httpListener
	s.mqttListener = mqttListener
	s.httpServer = &http.Server{
		Handler:           s.httpHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if serveErr := s.httpServer.Serve(httpListener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.log.Warn("pulse mqtt emulator http server stopped", slog.String("error", serveErr.Error()))
		}
	}()
	go s.acceptMQTT()

	tickCtx, tickCancel := context.WithCancel(context.Background())
	s.tickCancel = tickCancel
	go s.publishLoop(tickCtx)

	s.log.Info("pulse mqtt emulator started",
		slog.String("http_addr", httpListener.Addr().String()),
		slog.String("mqtt_addr", mqttListener.Addr().String()),
		slog.String("advertised_broker", s.BrokerAddress()),
		slog.String("device_sn", strings.ToUpper(strings.TrimSpace(s.cfg.Device.SN))),
	)
	return nil
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	var closeErr error
	s.closeOnce.Do(func() {
		if s.tickCancel != nil {
			s.tickCancel()
		}
		select {
		case <-s.tickDone:
		case <-time.After(2 * time.Second):
		}
		if s.httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			closeErr = errors.Join(closeErr, s.httpServer.Shutdown(ctx))
			cancel()
		}
		if s.httpListener != nil {
			closeErr = errors.Join(closeErr, s.httpListener.Close())
		}
		if s.mqttListener != nil {
			closeErr = errors.Join(closeErr, s.mqttListener.Close())
		}
		s.mu.RLock()
		clients := make([]*mqttClient, 0, len(s.clients))
		for client := range s.clients {
			clients = append(clients, client)
		}
		s.mu.RUnlock()
		for _, client := range clients {
			closeErr = errors.Join(closeErr, client.Close())
		}
	})
	return closeErr
}

func (s *Server) BaseURL() string {
	if s == nil || s.httpListener == nil {
		return ""
	}
	return "http://" + s.httpListener.Addr().String()
}

func (s *Server) BrokerAddress() string {
	if s == nil {
		return ""
	}
	host := strings.TrimSpace(s.cfg.BrokerAdvertiseHost)
	if host == "" && s.mqttListener != nil {
		if parsedHost, _, err := net.SplitHostPort(s.mqttListener.Addr().String()); err == nil {
			host = parsedHost
		}
	}
	port := strings.TrimSpace(s.cfg.BrokerAdvertisePort)
	if port == "" && s.mqttListener != nil {
		if _, parsedPort, err := net.SplitHostPort(s.mqttListener.Addr().String()); err == nil {
			port = parsedPort
		}
	}
	if host == "" || port == "" {
		return ""
	}
	return net.JoinHostPort(host, port)
}

func normalizeConfig(cfg Config) Config {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if strings.TrimSpace(cfg.HTTPAddr) == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if strings.TrimSpace(cfg.MQTTAddr) == "" {
		cfg.MQTTAddr = defaultMQTTAddr
	}
	if cfg.PublishInterval <= 0 {
		cfg.PublishInterval = defaultPublishInterval
	}
	if strings.TrimSpace(cfg.Device.DeviceName) == "" {
		cfg.Device.DeviceName = "DPU-X 24 kWh"
	}
	if strings.TrimSpace(cfg.Device.ProductName) == "" {
		cfg.Device.ProductName = "DELTA Pro Ultra X"
	}
	if cfg.Device.BatteryPackCount <= 0 {
		cfg.Device.BatteryPackCount = 4
	}
	if cfg.Device.BatteryPackEnergyWh <= 0 {
		cfg.Device.BatteryPackEnergyWh = 6144
	}
	return cfg
}

func (s *Server) httpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/iot-open/sign/device/list", s.handleDeviceList)
	mux.HandleFunc("/iot-open/sign/certification", s.handleMQTTCertification)
	mux.HandleFunc("/iot-open/sign/device/quota/all", s.handleDeviceQuota)
	mux.HandleFunc("/replay", s.handleReplay)
	return mux
}

func (s *Server) handleDeviceList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizeSignedRequest(w, r) {
		return
	}
	device := map[string]any{
		"sn":          strings.ToUpper(strings.TrimSpace(s.cfg.Device.SN)),
		"deviceName":  s.cfg.Device.DeviceName,
		"productName": s.cfg.Device.ProductName,
		"online":      1,
	}
	writeJSON(w, map[string]any{
		"code":    businessCodeOK,
		"message": businessMessageSuccess,
		"data":    []any{device},
	})
}

func (s *Server) handleMQTTCertification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizeSignedRequest(w, r) {
		return
	}
	host, port := splitAddress(s.BrokerAddress())
	writeJSON(w, map[string]any{
		"code":    businessCodeOK,
		"message": businessMessageSuccess,
		"data": map[string]any{
			"certificateAccount":  s.cfg.MQTTUsername,
			"certificatePassword": s.cfg.MQTTPassword,
			"url":                 host,
			"port":                port,
			"protocol":            "mqtts",
		},
	})
}

func (s *Server) handleDeviceQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizeSignedRequest(w, r) {
		return
	}
	sn := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("sn")))
	if sn == "" || sn != strings.ToUpper(strings.TrimSpace(s.cfg.Device.SN)) {
		writeJSON(w, map[string]any{
			"code":    "404",
			"message": "device not found",
			"data":    map[string]any{},
		})
		return
	}
	writeJSON(w, map[string]any{
		"code":    businessCodeOK,
		"message": businessMessageSuccess,
		"data":    s.currentQuota(),
	})
}

func (s *Server) authorizeSignedRequest(w http.ResponseWriter, r *http.Request) bool {
	accessKey := strings.TrimSpace(r.Header.Get(ecoflow.HeaderAccessKey))
	nonce := strings.TrimSpace(r.Header.Get(ecoflow.HeaderNonce))
	timestampRaw := strings.TrimSpace(r.Header.Get(ecoflow.HeaderTimestamp))
	signature := strings.TrimSpace(r.Header.Get(ecoflow.HeaderSignature))
	if accessKey == "" || nonce == "" || timestampRaw == "" || signature == "" {
		s.writeAccessKeyRejected(w)
		return false
	}
	if subtle.ConstantTimeCompare([]byte(accessKey), []byte(s.cfg.AccessKey)) != 1 {
		s.writeAccessKeyRejected(w)
		return false
	}
	timestampMillis, err := strconv.ParseInt(timestampRaw, 10, 64)
	if err != nil {
		s.writeAccessKeyRejected(w)
		return false
	}
	result, err := ecoflow.NewHMACSHA256Signer().Sign(r.Context(), ecoflow.SignInput{
		Credentials: ecoflow.Credentials{
			AccessKey: s.cfg.AccessKey,
			SecretKey: s.cfg.SecretKey,
		},
		Nonce:           nonce,
		TimestampMillis: timestampMillis,
		Query:           cloneQuery(r.URL.Query()),
		Body:            nil,
	})
	if err != nil {
		s.writeAccessKeyRejected(w)
		return false
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(signature)), []byte(strings.ToLower(result.Signature))) != 1 {
		s.writeAccessKeyRejected(w)
		return false
	}
	return true
}

func (s *Server) writeAccessKeyRejected(w http.ResponseWriter) {
	writeJSON(w, map[string]any{
		"code":    businessCodeAccessInvalid,
		"message": businessMessageAccessKey,
		"data":    map[string]any{},
	})
}

func (s *Server) acceptMQTT() {
	for {
		conn, err := s.mqttListener.Accept()
		if err != nil {
			if isUseOfClosedNetworkConn(err) {
				return
			}
			s.log.Warn("pulse mqtt emulator accept failed", slog.String("error", err.Error()))
			continue
		}
		go s.handleMQTTConn(conn)
	}
}

func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	from, err := parseReplayTime(r.URL.Query().Get("from"))
	if err != nil {
		http.Error(w, "invalid from: "+err.Error(), http.StatusBadRequest)
		return
	}
	to, err := parseReplayTime(r.URL.Query().Get("to"))
	if err != nil {
		http.Error(w, "invalid to: "+err.Error(), http.StatusBadRequest)
		return
	}
	step, err := parseReplayStep(r.URL.Query().Get("step"), time.Minute)
	if err != nil {
		http.Error(w, "invalid step: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !to.After(from) {
		http.Error(w, "to must be after from", http.StatusBadRequest)
		return
	}
	replaySamples := replayTimes(from, to, step)
	if samples := len(replaySamples); samples == 0 || samples > 5_000 {
		http.Error(w, "replay window is too large", http.StatusBadRequest)
		return
	}
	framesPublished, samplesPublished := s.replayRange(replaySamples)
	writeJSON(w, map[string]any{
		"from":             from.Format(time.RFC3339Nano),
		"to":               to.Format(time.RFC3339Nano),
		"step":             step.String(),
		"samplesPublished": samplesPublished,
		"framesPublished":  framesPublished,
		"clients":          s.clientCount(),
	})
}

func (s *Server) handleMQTTConn(conn net.Conn) {
	client := &mqttClient{
		server: s,
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
		topics: map[string]struct{}{},
	}
	if err := s.performMQTTHandshake(client); err != nil {
		_ = client.Close()
		return
	}
	s.addClient(client)
	defer func() {
		s.removeClient(client)
		_ = client.Close()
	}()

	for {
		packetType, _, body, err := readPacket(conn, client.reader, client.idleReadTimeout)
		if err != nil {
			return
		}
		switch packetType {
		case packetTypeSubscribe:
			packetID, topic, err := parseSubscribePacket(body)
			if err != nil {
				return
			}
			if err := client.subscribe(topic); err != nil {
				return
			}
			if err := client.writePacket(byte(packetTypeSubAck<<4), buildSubAckPacket(packetID, 0)); err != nil {
				return
			}
			if topic == s.topicName() {
				if _, err := s.publishSnapshotToClient(client, s.state.current()); err != nil {
					return
				}
			}
		case packetTypePingReq:
			if err := client.writePacket(byte(packetTypePingResp<<4), nil); err != nil {
				return
			}
		case packetTypeDisconnect:
			return
		default:
			// Ignore unsupported packets; the client only relies on CONNECT/SUBSCRIBE/PINGREQ.
		}
	}
}

func (s *Server) performMQTTHandshake(client *mqttClient) error {
	packetType, _, body, err := readPacket(client.conn, client.reader, 10*time.Second)
	if err != nil {
		return err
	}
	if packetType != packetTypeConnect {
		return errors.New("expected mqtt connect packet")
	}
	connect, err := parseConnectPacket(body)
	if err != nil {
		return err
	}
	if connect.ProtocolName != "MQTT" || connect.ProtocolLevel != 0x04 {
		_ = client.writePacket(byte(packetTypeConnAck<<4), []byte{0x00, 0x01})
		return errors.New("unsupported mqtt protocol")
	}
	if subtle.ConstantTimeCompare([]byte(connect.Username), []byte(s.cfg.MQTTUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(connect.Password), []byte(s.cfg.MQTTPassword)) != 1 {
		_ = client.writePacket(byte(packetTypeConnAck<<4), []byte{0x00, 0x05})
		return errors.New("mqtt credentials rejected")
	}
	client.idleReadTimeout = mqttIdleReadTimeout(connect.KeepAliveSeconds, s.cfg.PublishInterval)
	return client.writePacket(byte(packetTypeConnAck<<4), []byte{0x00, 0x00})
}

func (s *Server) publishLoop(ctx context.Context) {
	defer close(s.tickDone)
	ticker := time.NewTicker(s.cfg.PublishInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := s.state.next()
			s.broadcastSnapshot(snapshot)
		}
	}
}

func replayTimes(from, to time.Time, step time.Duration) []time.Time {
	if !to.After(from) || step <= 0 {
		return nil
	}
	out := make([]time.Time, 0, int(to.Sub(from)/step)+2)
	for at := from; at.Before(to); at = at.Add(step) {
		out = append(out, at)
	}
	closing := to.Add(-time.Millisecond)
	if len(out) == 0 || out[len(out)-1].Before(closing) {
		out = append(out, closing)
	}
	return out
}

func (s *Server) replayRange(times []time.Time) (framesPublished int, samplesPublished int) {
	for _, at := range times {
		samplesPublished++
		snapshot := snapshotForTime(TickForTime(at, s.cfg.PublishInterval), at)
		framesPublished += s.broadcastSnapshot(snapshot)
	}
	return framesPublished, samplesPublished
}

func (s *Server) broadcastSnapshot(snapshot telemetrySnapshot) int {
	s.mu.RLock()
	clients := make([]*mqttClient, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.RUnlock()
	framesPublished := 0
	for _, client := range clients {
		published, err := s.publishSnapshotToClient(client, snapshot)
		if err != nil {
			s.log.Warn("pulse mqtt emulator publish failed", slog.String("error", err.Error()))
			s.removeClient(client)
			_ = client.Close()
			continue
		}
		framesPublished += published
	}
	return framesPublished
}

func (s *Server) publishSnapshotToClient(client *mqttClient, snapshot telemetrySnapshot) (int, error) {
	if client == nil || !client.hasTopic(s.topicName()) {
		return 0, nil
	}
	published := 0
	for _, payload := range buildMQTTFrames(snapshot) {
		if err := client.publish(s.topicName(), payload); err != nil {
			return published, err
		}
		published++
	}
	return published, nil
}

func (s *Server) addClient(client *mqttClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client] = struct{}{}
}

func (s *Server) removeClient(client *mqttClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
}

func (s *Server) topicName() string {
	return TopicName(s.cfg.MQTTUsername, s.cfg.Device.SN)
}

func (s *Server) currentQuota() map[string]any {
	return buildQuotaData(s.state.current(), s.cfg.Device)
}

func (c *mqttClient) subscribe(topic string) error {
	if c == nil {
		return errors.New("mqtt client is required")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("topic is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.topics[topic] = struct{}{}
	return nil
}

func (c *mqttClient) hasTopic(topic string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.topics[topic]
	return ok
}

func (c *mqttClient) publish(topic string, payload []byte) error {
	packet := make([]byte, 0, len(topic)+len(payload)+2)
	packet = appendMQTTString(packet, topic)
	packet = append(packet, payload...)
	return c.writePacket(byte(packetTypePublish<<4), packet)
}

func (c *mqttClient) writePacket(header byte, body []byte) error {
	if c == nil {
		return errors.New("mqtt client is required")
	}
	c.packet.Lock()
	defer c.packet.Unlock()
	return writePacket(c.conn, c.writer, 10*time.Second, header, body)
}

func (c *mqttClient) Close() error {
	if c == nil {
		return nil
	}
	var closeErr error
	c.closeOnce.Do(func() {
		closeErr = c.conn.Close()
	})
	return closeErr
}

func (s *deviceState) current() telemetrySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return snapshotForTime(s.tick, time.Now())
}

func (s *deviceState) next() telemetrySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tick++
	return snapshotForTime(s.tick, time.Now())
}

func snapshotForTick(tick int) telemetrySnapshot {
	return snapshotForTime(tick, time.Now())
}

func snapshotForTime(tick int, now time.Time) telemetrySnapshot {
	state := simulatedBatteryStateAt(now)
	return buildInstantSnapshot(tick, now, state)
}

type powerModel struct {
	AvailablePVWatts float64
	AcceptedPVWatts  float64
	ACInputWatts     float64
	ACOutputWatts    float64
	DCOutputWatts    float64
	USBOutputWatts   float64
	BatteryNetWatts  float64
	BatVolts         float64
	PV1Volts         float64
	PV1Amps          float64
	PV2Volts         float64
	PV2Amps          float64
	PackHeatTimes    []int
	BMSModeSet       int
	BatteryHeatMode  int
	PackTemps        []int
	PDTemp           int
	FanState         int
}

func simulatedBatteryStateAt(now time.Time) batterySimState {
	localNow := now.Local()
	start := startOfLocalDay(localNow).Add(-24 * time.Hour)
	state := batterySimState{
		SOC:          initialBatterySOC(start),
		SolarLimited: false,
	}
	for at := start; at.Before(localNow); at = at.Add(dpuXSimulationStep) {
		nextAt := at.Add(dpuXSimulationStep)
		if nextAt.After(localNow) {
			nextAt = localNow
		}
		duration := nextAt.Sub(at)
		if duration <= 0 {
			continue
		}
		model := buildPowerModel(at, TickForTime(at, defaultPublishInterval), state, duration)
		state = advanceBatteryState(state, model.BatteryNetWatts, duration)
	}
	return state
}

func buildInstantSnapshot(tick int, now time.Time, state batterySimState) telemetrySnapshot {
	model := buildPowerModel(now, tick, state, dpuXSimulationStep)
	snapshot := telemetrySnapshot{
		Tick:            tick,
		ObservedUnixMS:  now.UTC().UnixMilli(),
		ChargeMaxSOC:    int(dpuXChargeMaxSOC),
		DischargeMinSOC: int(dpuXBackupReserveSOC),
		BackupSOC:       int(dpuXBackupReserveSOC),
		OverallSOC:      clampFloat(state.SOC, dpuXBackupReserveSOC, dpuXChargeMaxSOC),
	}
	snapshot.PackSOCs = derivePackSOCs(snapshot.OverallSOC, tick, 4)
	snapshot.InLvMpptVol = model.PV1Volts
	snapshot.InLvMpptAmp = model.PV1Amps
	snapshot.InHvMpptVol = model.PV2Volts
	snapshot.InHvMpptAmp = model.PV2Amps
	snapshot.ACInputWatts = model.ACInputWatts
	snapshot.ACOutputWatts = model.ACOutputWatts
	snapshot.DCOutputWatts = model.DCOutputWatts
	snapshot.USBOutputWatts = model.USBOutputWatts
	snapshot.BatVol = model.BatVolts
	snapshot.PackHeatTimes = model.PackHeatTimes
	snapshot.BMSModeSet = model.BMSModeSet
	snapshot.BatteryHeatMode = model.BatteryHeatMode
	snapshot.PackTemps = model.PackTemps
	snapshot.PDTemp = model.PDTemp
	snapshot.WattsInSum = model.AcceptedPVWatts + model.ACInputWatts
	snapshot.WattsOutSum = model.ACOutputWatts + model.DCOutputWatts + model.USBOutputWatts
	if model.BatteryNetWatts >= 0 {
		snapshot.BMSInputWatts = model.BatteryNetWatts
		snapshot.BMSOutputWatts = 0
	} else {
		snapshot.BMSInputWatts = 0
		snapshot.BMSOutputWatts = -model.BatteryNetWatts
	}
	if snapshot.BatVol > 0 {
		snapshot.BatAmp = model.BatteryNetWatts / snapshot.BatVol
	}
	snapshot.FanState = model.FanState
	snapshot.RemainTimeMin = estimateRemainTimeMinutes(snapshot, dpuXPackEnergyWh)
	snapshot.PackWatts = distributePackWatts(model.BatteryNetWatts, snapshot.PackSOCs)
	return snapshot
}

func buildPowerModel(now time.Time, tick int, state batterySimState, step time.Duration) powerModel {
	localNow := now.Local()
	solarCurve := daytimeSolarCurve(localNow)
	availablePVWatts := solarAvailableWatts(localNow, tick, solarCurve)
	acOutputWatts := acLoadProfile(localNow, tick, solarCurve)
	dcOutputWatts := dcLoadProfile(tick)
	loadWatts := acOutputWatts + dcOutputWatts
	batVolts := 417.6 + 1.2*solarCurve + 0.8*tickWave(tick, 95, 0.9)
	packHeatTimes, bmsModeSet, batteryHeatMode := preconditioningProfile(tick, 4)
	heatActive := bmsModeSet > 0 || batteryHeatMode > 0
	solarLimited := state.SolarLimited
	if !solarLimited && state.SOC >= dpuXChargeMaxSOC {
		solarLimited = true
	}
	if solarLimited && state.SOC <= dpuXChargeResumeSOC {
		solarLimited = false
	}

	acceptedPVWatts := availablePVWatts
	if solarLimited && acceptedPVWatts > loadWatts {
		acceptedPVWatts = loadWatts
	}

	acInputWatts := 0.0
	batteryNetWatts := acceptedPVWatts - loadWatts
	if batteryNetWatts < 0 {
		maxDischargeWatts := maxReserveDischargeWatts(state.SOC, step)
		requestedDischargeWatts := -batteryNetWatts
		allowedDischargeWatts := math.Min(requestedDischargeWatts, maxDischargeWatts)
		gridAssistWatts := requestedDischargeWatts - allowedDischargeWatts
		batteryNetWatts = -allowedDischargeWatts
		if state.SOC < dpuXBackupReserveSOC {
			gridAssistWatts += reserveTopUpWatts(state.SOC, step)
		}
		if gridAssistWatts > 0 {
			acInputWatts = math.Min(dpuXACInputLimitWatts, gridAssistWatts)
			batteryNetWatts = acceptedPVWatts + acInputWatts - loadWatts
		}
	}
	if batteryNetWatts > 0 {
		batteryNetWatts = math.Max(0, batteryNetWatts-dpuXChargeLossWatts)
	}

	pv1Volts, pv1Amps := pvPortTelemetry(availablePVWatts, acceptedPVWatts, solarCurve, tick, solarLimited)
	baseTemp := 29.4 +
		2.8*solarCurve +
		0.0014*acOutputWatts +
		0.0007*acceptedPVWatts +
		0.9*tickWave(tick, 70, 0.3)
	if heatActive {
		baseTemp += 1.2
	}
	packTemps := []int{
		int(math.Round(baseTemp - 0.6)),
		int(math.Round(baseTemp - 0.1)),
		int(math.Round(baseTemp + 0.2)),
		int(math.Round(baseTemp + 0.6)),
	}
	fanState := 0
	if heatActive ||
		acOutputWatts >= 980 ||
		acceptedPVWatts >= 3000 ||
		((acOutputWatts >= 760 || acceptedPVWatts >= 2400) && windowedPulse(tick, 48, 6, 20) > 0.38) {
		fanState = 1
	}

	return powerModel{
		AvailablePVWatts: availablePVWatts,
		AcceptedPVWatts:  acceptedPVWatts,
		ACInputWatts:     acInputWatts,
		ACOutputWatts:    acOutputWatts,
		DCOutputWatts:    dcOutputWatts,
		USBOutputWatts:   0,
		BatteryNetWatts:  batteryNetWatts,
		BatVolts:         batVolts,
		PV1Volts:         pv1Volts,
		PV1Amps:          pv1Amps,
		PV2Volts:         0,
		PV2Amps:          0,
		PackHeatTimes:    packHeatTimes,
		BMSModeSet:       bmsModeSet,
		BatteryHeatMode:  batteryHeatMode,
		PackTemps:        packTemps,
		PDTemp:           int(math.Round(baseTemp + 1.4)),
		FanState:         fanState,
	}
}

func startOfLocalDay(at time.Time) time.Time {
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())
}

func initialBatterySOC(at time.Time) float64 {
	seed := float64((at.YearDay()+17)%29) * 0.31
	base := 77.0 + 4.5*math.Sin(seed+0.6) + 1.8*math.Sin(seed*0.7+1.1)
	return clampFloat(base, 71, 84)
}

func advanceBatteryState(state batterySimState, batteryNetWatts float64, step time.Duration) batterySimState {
	if step <= 0 {
		return state
	}
	next := state
	next.SOC = clampFloat(
		state.SOC+((batteryNetWatts*step.Hours())/dpuXBatteryCapacityWh)*100,
		dpuXBackupReserveSOC,
		dpuXChargeMaxSOC,
	)
	switch {
	case next.SOC >= dpuXChargeMaxSOC:
		next.SolarLimited = true
	case next.SOC <= dpuXChargeResumeSOC:
		next.SolarLimited = false
	default:
		next.SolarLimited = state.SolarLimited
	}
	return next
}

func maxReserveDischargeWatts(soc float64, step time.Duration) float64 {
	if step <= 0 || soc <= dpuXBackupReserveSOC {
		return 0
	}
	reserveWh := ((soc - dpuXBackupReserveSOC) / 100) * dpuXBatteryCapacityWh
	if reserveWh <= 0 {
		return 0
	}
	return reserveWh / step.Hours()
}

func reserveTopUpWatts(soc float64, step time.Duration) float64 {
	if step <= 0 || soc >= dpuXBackupReserveSOC {
		return 0
	}
	reserveGapWh := ((dpuXBackupReserveSOC - soc) / 100) * dpuXBatteryCapacityWh
	if reserveGapWh <= 0 {
		return 0
	}
	return math.Min(dpuXReserveTopUpWatts, reserveGapWh/step.Hours())
}

func derivePackSOCs(overallSOC float64, tick int, count int) []int {
	if count <= 0 {
		return nil
	}
	out := make([]int, 0, count)
	center := float64(count-1) / 2
	for idx := 0; idx < count; idx++ {
		offset := (center-float64(idx))*0.42 + 0.08*tickWave(tick+idx*3, 33, float64(idx)*0.5)
		out = append(out, clampInt(int(math.Round(overallSOC+offset)), 0, 100))
	}
	return out
}

func solarAvailableWatts(now time.Time, tick int, solarCurve float64) float64 {
	if solarCurve <= 0.01 {
		return 0
	}
	envelope := solarCloudCoverage(now, tick, solarCurve)
	ratedArrayWatts := 4130.0
	return clampFloat(ratedArrayWatts*solarCurve*envelope, 0, 4300)
}

func acLoadProfile(now time.Time, tick int, solarCurve float64) float64 {
	hour := float64(now.Hour()) + float64(now.Minute())/60
	baseHouse := 455 +
		55*(0.5+0.5*math.Sin((hour-6.5)/24*2*math.Pi)) +
		42*tickWave(tick, 54, 0.1) +
		28*tickWave(tick, 17, 1.2)
	threeTonAC := 170 +
		280*math.Pow(solarCurve, 0.72) +
		150*(0.5+0.5*tickWave(tick, 46, 0.8))
	washer := 215*windowedPulse(tick+int(hour*5), 168, 92, 20) + 125*windowedPulse(tick+9, 214, 127, 24)
	coffee := 260*windowedPulse(tick+int(hour*7), 720, 78, 9) + 180*windowedPulse(tick+11, 720, 136, 8)
	return clampFloat(baseHouse+threeTonAC+washer+coffee, 450, 1200)
}

func dcLoadProfile(tick int) float64 {
	return clampFloat(
		8.2+
			2.4*tickWave(tick, 31, 0.5)+
			1.7*tickWave(tick, 13, 1.4)+
			0.9*tickWave(tick, 63, 0.2),
		5,
		15,
	)
}

func pvPortTelemetry(availablePVWatts float64, acceptedPVWatts float64, solarCurve float64, tick int, solarLimited bool) (float64, float64) {
	if availablePVWatts <= 1 && solarCurve <= 0.01 {
		return 0, 0
	}
	if solarLimited || acceptedPVWatts <= 1 {
		volts := clampFloat(333+2.5*tickWave(tick, 43, 0.7)+1.4*tickWave(tick, 15, 1.6), 328, 339)
		return volts, 0
	}
	volts := clampFloat(312+4.8*tickWave(tick, 110, 0.2)+2.6*tickWave(tick, 31, 1.1)+1.2*tickWave(tick, 7, 0.6), 304, 320)
	return volts, acceptedPVWatts / volts
}

func acInputVoltage(acInputWatts float64) float64 {
	if acInputWatts <= 0 {
		return 0
	}
	return dpuXACInputVolts
}

func acInputAmps(acInputWatts float64) float64 {
	if acInputWatts <= 0 {
		return 0
	}
	return acInputWatts / dpuXACInputVolts
}

func parseReplayTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("value is required")
	}
	if millis, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.UnixMilli(millis).UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func parseReplayStep(raw string, defaultValue time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue, nil
	}
	step, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	if step <= 0 {
		return 0, errors.New("step must be positive")
	}
	return step, nil
}

func tickWave(tick int, periodTicks float64, phase float64) float64 {
	if periodTicks <= 0 {
		return 0
	}
	return math.Sin((float64(tick)/periodTicks)*2*math.Pi + phase)
}

func daytimeSolarCurve(now time.Time) float64 {
	minutes := now.Hour()*60 + now.Minute()
	const sunriseMinutes = 6 * 60
	const sunsetMinutes = 20 * 60
	if minutes <= sunriseMinutes || minutes >= sunsetMinutes {
		return 0
	}
	progress := float64(minutes-sunriseMinutes) / float64(sunsetMinutes-sunriseMinutes)
	if progress <= 0 || progress >= 1 {
		return 0
	}
	return math.Pow(math.Sin(progress*math.Pi), 1.45)
}

func solarCloudCoverage(now time.Time, tick int, solarCurve float64) float64 {
	if solarCurve <= 0.01 {
		return 0
	}
	minutes := float64(now.Hour()*60 + now.Minute())
	daySeed := float64((now.YearDay()%37)+1) * 0.23
	cloudDeck := 0.91 +
		0.08*math.Sin(minutes/78+daySeed) +
		0.05*math.Sin(minutes/23+daySeed*1.7)
	driftingBanks := 1 -
		0.21*solarCurve*windowedPulse(tick+int(daySeed*19), 720, 204, 68) -
		0.15*solarCurve*windowedPulse(tick+int(daySeed*11), 720, 336, 54) -
		0.11*solarCurve*windowedPulse(tick+int(daySeed*7), 720, 472, 42)
	cumulusTexture := 1 + math.Pow(solarCurve, 0.8)*
		(0.11*tickWave(tick, 19, 0.4+daySeed)+
			0.07*tickWave(tick, 43, 1.2+daySeed*0.6)+
			0.04*tickWave(tick, 9, 2.1+daySeed*0.2))
	middayFlicker := 1 + math.Pow(solarCurve, 1.45)*
		(0.07*tickWave(tick, 7, daySeed*0.9)*math.Max(0, tickWave(tick, 27, daySeed*0.4))-
			0.12*math.Max(0, tickWave(tick, 13, daySeed*0.7))*math.Max(0, tickWave(tick, 33, daySeed*0.3)))
	return clampFloat(cloudDeck*driftingBanks*cumulusTexture*middayFlicker, 0.34, 1.04)
}

func clampFloat(value, minValue, maxValue float64) float64 {
	return math.Min(maxValue, math.Max(minValue, value))
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func windowedPulse(tick int, cycleTicks int, start int, length int) float64 {
	if cycleTicks <= 0 || length <= 1 {
		return 0
	}
	position := tick % cycleTicks
	if position < 0 {
		position += cycleTicks
	}
	if position < start || position >= start+length {
		return 0
	}
	progress := float64(position-start) / float64(length-1)
	return math.Sin(progress * math.Pi)
}

func preconditioningProfile(tick int, packCount int) ([]int, int, int) {
	heatTimes := make([]int, packCount)
	if packCount == 0 {
		return heatTimes, 0, 0
	}
	const cycleTicks = 84
	systemActive := false
	for idx := 0; idx < packCount; idx++ {
		start := 10 + ((idx * 17) % 42)
		length := 6 + ((idx + 1) % 4)
		position := tick % cycleTicks
		if position < start || position >= start+length {
			continue
		}
		remainingTicks := start + length - position
		heatTimes[idx] = clampInt(remainingTicks*5, 30, 45)
		systemActive = true
	}
	if !systemActive {
		if windowedPulse(tick, cycleTicks, 58, 7) > 0 {
			systemActive = true
		}
	}
	if !systemActive {
		return heatTimes, 0, 0
	}
	return heatTimes, 1, 1
}

func distributePackWatts(total float64, packSOCs []int) []float64 {
	count := len(packSOCs)
	if count == 0 {
		return nil
	}
	avgSOC := 0.0
	for _, soc := range packSOCs {
		avgSOC += float64(soc)
	}
	avgSOC /= float64(count)
	weights := make([]float64, count)
	weightSum := 0.0
	for idx, soc := range packSOCs {
		delta := avgSOC - float64(soc)
		if total < 0 {
			delta = -delta
		}
		weight := 1 + delta*0.02
		if weight < 0.85 {
			weight = 0.85
		}
		if weight > 1.15 {
			weight = 1.15
		}
		weights[idx] = weight
		weightSum += weight
	}
	remaining := total
	out := make([]float64, 0, count)
	for idx := 0; idx < count; idx++ {
		if idx == count-1 {
			out = append(out, remaining)
			break
		}
		portion := total * (weights[idx] / weightSum)
		out = append(out, portion)
		remaining -= portion
	}
	return out
}

func estimateRemainTimeMinutes(snapshot telemetrySnapshot, packEnergyWh float64) int {
	if len(snapshot.PackSOCs) == 0 || packEnergyWh <= 0 {
		return 0
	}
	totalCapacityWh := float64(len(snapshot.PackSOCs)) * packEnergyWh
	netBatteryWatts := snapshot.BMSInputWatts - snapshot.BMSOutputWatts
	switch {
	case netBatteryWatts > 1:
		targetSOC := float64(snapshot.ChargeMaxSOC)
		if targetSOC <= snapshot.OverallSOC {
			return 0
		}
		remainingWh := totalCapacityWh * (targetSOC - snapshot.OverallSOC) / 100
		if remainingWh <= 0 {
			return 0
		}
		return int((remainingWh / netBatteryWatts) * 60)
	case netBatteryWatts < -1:
		reserveSOC := float64(snapshot.BackupSOC)
		if reserveSOC >= snapshot.OverallSOC {
			return 0
		}
		availableWh := totalCapacityWh * (snapshot.OverallSOC - reserveSOC) / 100
		if availableWh <= 0 {
			return 0
		}
		return int((availableWh / -netBatteryWatts) * 60)
	default:
		return 0
	}
}

func buildQuotaData(snapshot telemetrySnapshot, device DeviceConfig) map[string]any {
	pv1Watts := snapshot.InLvMpptVol * snapshot.InLvMpptAmp
	pv2Watts := snapshot.InHvMpptVol * snapshot.InHvMpptAmp
	bpInfo := make([]map[string]any, 0, len(snapshot.PackSOCs))
	for idx := range snapshot.PackSOCs {
		bpInfo = append(bpInfo, map[string]any{
			"bpSoc":        snapshot.PackSOCs[idx],
			"bpSocMin":     0,
			"bpNo":         idx + 1,
			"bpSocMax":     100,
			"bpPwr":        snapshot.PackWatts[idx],
			"bpChgSta":     chargeState(snapshot.PackWatts[idx]),
			"bpErrCode":    0,
			"remainTime":   snapshot.RemainTimeMin + idx*45,
			"heatTime":     snapshot.PackHeatTimes[idx],
			"bpTemp":       snapshot.PackTemps[idx],
			"bpEnergy":     device.BatteryPackEnergyWh,
			"bpSunnovaBan": 0,
		})
	}
	return map[string]any{
		"hs_yj751_pd_appshow_addr.soc":            snapshot.OverallSOC,
		"hs_yj751_pd_appshow_addr.remainTime":     snapshot.RemainTimeMin,
		"hs_yj751_pd_appshow_addr.wattsInSum":     snapshot.WattsInSum,
		"hs_yj751_pd_appshow_addr.wattsOutSum":    snapshot.WattsOutSum,
		"hs_yj751_pd_appshow_addr.inLvMpptPwr":    pv1Watts,
		"hs_yj751_pd_appshow_addr.inHvMpptPwr":    pv2Watts,
		"hs_yj751_pd_appshow_addr.pv1ChargeWatts": pv1Watts,
		"hs_yj751_pd_appshow_addr.pv2ChargeWatts": pv2Watts,
		"hs_yj751_pd_appshow_addr.inAcC20Pwr":     snapshot.ACInputWatts,
		"hs_yj751_pd_appshow_addr.outAcTtPwr":     snapshot.ACOutputWatts,
		"hs_yj751_pd_appshow_addr.outAdsPwr":      snapshot.DCOutputWatts,
		"hs_yj751_pd_appshow_addr.outAdsVol":      12.8,
		"hs_yj751_pd_appshow_addr.outAdsAmp":      snapshot.DCOutputWatts / 12.8,
		"hs_yj751_pd_appshow_addr.bpNum":          device.BatteryPackCount,
		"hs_yj751_pd_appshow_addr.bpPowerSoc":     snapshot.BackupSOC,
		"hs_yj751_pd_appshow_addr.minAcSoc":       snapshot.BackupSOC,
		"hs_yj751_pd_appshow_addr.showFlag":       2322,
		"hs_yj751_pd_appshow_addr.dcOutState":     1,
		"hs_yj751_pd_appshow_addr.carState":       0,

		"hs_yj751_pd_backend_addr.inLvMpptVol":    snapshot.InLvMpptVol,
		"hs_yj751_pd_backend_addr.inLvMpptAmp":    snapshot.InLvMpptAmp,
		"hs_yj751_pd_backend_addr.inHvMpptVol":    snapshot.InHvMpptVol,
		"hs_yj751_pd_backend_addr.inHvMpptAmp":    snapshot.InHvMpptAmp,
		"hs_yj751_pd_backend_addr.inAcC20Vol":     acInputVoltage(snapshot.ACInputWatts),
		"hs_yj751_pd_backend_addr.inAcC20Amp":     acInputAmps(snapshot.ACInputWatts),
		"hs_yj751_pd_backend_addr.bmsInputWatts":  snapshot.BMSInputWatts,
		"hs_yj751_pd_backend_addr.bmsOutputWatts": snapshot.BMSOutputWatts,
		"hs_yj751_pd_backend_addr.batAmp":         snapshot.BatAmp,
		"hs_yj751_pd_backend_addr.batVol":         snapshot.BatVol,
		"hs_yj751_pd_backend_addr.pdTemp":         snapshot.PDTemp,
		"hs_yj751_pd_backend_addr.fanState":       snapshot.FanState,
		"hs_yj751_pd_backend_addr.acOutFreq":      50,

		"hs_yj751_pd_app_set_info_addr.chgMaxSoc":       snapshot.ChargeMaxSOC,
		"hs_yj751_pd_app_set_info_addr.dsgMinSoc":       snapshot.DischargeMinSOC,
		"hs_yj751_pd_app_set_info_addr.sysBackupSoc":    snapshot.BackupSOC,
		"hs_yj751_pd_app_set_info_addr.acOutFreq":       50,
		"hs_yj751_pd_app_set_info_addr.acOftenOpenFlg":  1,
		"hs_yj751_pd_app_set_info_addr.bmsModeSet":      snapshot.BMSModeSet,
		"hs_yj751_pd_app_set_info_addr.batteryHeatMode": snapshot.BatteryHeatMode,

		"hs_yj751_pd_bp_addr.bpInfo": bpInfo,
	}
}

func buildMQTTFrames(snapshot telemetrySnapshot) [][]byte {
	pv1Watts := snapshot.InLvMpptVol * snapshot.InLvMpptAmp
	pv2Watts := snapshot.InHvMpptVol * snapshot.InHvMpptAmp
	appShow := map[string]any{
		"id":       fmt.Sprintf("%d-1", snapshot.ObservedUnixMS),
		"time":     snapshot.ObservedUnixMS,
		"typeCode": "quota",
		"cmdId":    1,
		"cmdFunc":  2,
		"addr":     "hs_yj751_pd_appshow_addr",
		"params": map[string]any{
			"soc":            snapshot.OverallSOC,
			"remainTime":     snapshot.RemainTimeMin,
			"wattsInSum":     snapshot.WattsInSum,
			"wattsOutSum":    snapshot.WattsOutSum,
			"inLvMpptPwr":    pv1Watts,
			"inHvMpptPwr":    pv2Watts,
			"pv1ChargeWatts": pv1Watts,
			"pv2ChargeWatts": pv2Watts,
			"inAcC20Pwr":     snapshot.ACInputWatts,
			"outAcTtPwr":     snapshot.ACOutputWatts,
			"outAdsPwr":      snapshot.DCOutputWatts,
			"outAdsVol":      12.8,
			"outAdsAmp":      snapshot.DCOutputWatts / 12.8,
			"dcOutState":     1,
			"carState":       0,
			"bpNum":          len(snapshot.PackSOCs),
			"bpPowerSoc":     snapshot.BackupSOC,
			"minAcSoc":       snapshot.BackupSOC,
			"showFlag":       2322,
		},
	}
	backend := map[string]any{
		"id":       fmt.Sprintf("%d-2", snapshot.ObservedUnixMS),
		"time":     snapshot.ObservedUnixMS,
		"typeCode": "quota",
		"cmdId":    2,
		"cmdFunc":  2,
		"addr":     "hs_yj751_pd_backend_addr",
		"params": map[string]any{
			"inLvMpptVol":    snapshot.InLvMpptVol,
			"inLvMpptAmp":    snapshot.InLvMpptAmp,
			"inHvMpptVol":    snapshot.InHvMpptVol,
			"inHvMpptAmp":    snapshot.InHvMpptAmp,
			"inAcC20Vol":     acInputVoltage(snapshot.ACInputWatts),
			"inAcC20Amp":     acInputAmps(snapshot.ACInputWatts),
			"bmsInputWatts":  snapshot.BMSInputWatts,
			"bmsOutputWatts": snapshot.BMSOutputWatts,
			"batAmp":         snapshot.BatAmp,
			"batVol":         snapshot.BatVol,
			"pdTemp":         snapshot.PDTemp,
			"fanState":       snapshot.FanState,
			"acOutFreq":      50,
		},
	}
	appSet := map[string]any{
		"id":       fmt.Sprintf("%d-3", snapshot.ObservedUnixMS),
		"time":     snapshot.ObservedUnixMS,
		"typeCode": "quota",
		"cmdId":    3,
		"cmdFunc":  2,
		"addr":     "hs_yj751_pd_app_set_info_addr",
		"params": map[string]any{
			"chgMaxSoc":       snapshot.ChargeMaxSOC,
			"dsgMinSoc":       snapshot.DischargeMinSOC,
			"sysBackupSoc":    snapshot.BackupSOC,
			"acOutFreq":       50,
			"acOftenOpenFlg":  1,
			"bmsModeSet":      snapshot.BMSModeSet,
			"batteryHeatMode": snapshot.BatteryHeatMode,
		},
	}
	bpInfo := make([]map[string]any, 0, len(snapshot.PackSOCs))
	for idx := range snapshot.PackSOCs {
		bpInfo = append(bpInfo, map[string]any{
			"bpSoc":        snapshot.PackSOCs[idx],
			"bpSocMin":     0,
			"bpNo":         idx + 1,
			"bpSocMax":     100,
			"bpPwr":        snapshot.PackWatts[idx],
			"bpChgSta":     chargeState(snapshot.PackWatts[idx]),
			"bpErrCode":    0,
			"remainTime":   snapshot.RemainTimeMin + idx*45,
			"heatTime":     snapshot.PackHeatTimes[idx],
			"bpTemp":       snapshot.PackTemps[idx],
			"bpEnergy":     6144,
			"bpSunnovaBan": 0,
		})
	}
	batteries := map[string]any{
		"id":       fmt.Sprintf("%d-4", snapshot.ObservedUnixMS),
		"time":     snapshot.ObservedUnixMS,
		"typeCode": "quota",
		"cmdId":    4,
		"cmdFunc":  2,
		"addr":     "hs_yj751_pd_bp_addr",
		"param": map[string]any{
			"bpInfo": bpInfo,
		},
		"params": map[string]any{},
	}

	frames := []map[string]any{appShow, backend, appSet, batteries}
	out := make([][]byte, 0, len(frames))
	for _, frame := range frames {
		payload, err := json.Marshal(frame)
		if err != nil {
			continue
		}
		out = append(out, payload)
	}
	return out
}

func (s *Server) clientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

func chargeState(power float64) int {
	switch {
	case power > 0:
		return 1
	case power < 0:
		return 2
	default:
		return 0
	}
}

type connectPacket struct {
	ProtocolName     string
	ProtocolLevel    byte
	KeepAliveSeconds uint16
	Username         string
	Password         string
}

func parseConnectPacket(body []byte) (connectPacket, error) {
	offset := 0
	protocolName, err := readMQTTString(body, &offset)
	if err != nil {
		return connectPacket{}, err
	}
	if offset+4 > len(body) {
		return connectPacket{}, errors.New("invalid mqtt connect packet")
	}
	protocolLevel := body[offset]
	offset++
	flags := body[offset]
	offset++
	keepAliveSeconds := binary.BigEndian.Uint16(body[offset : offset+2])
	offset += 2
	clientID, err := readMQTTString(body, &offset)
	if err != nil {
		return connectPacket{}, err
	}
	_ = clientID
	packet := connectPacket{
		ProtocolName:     protocolName,
		ProtocolLevel:    protocolLevel,
		KeepAliveSeconds: keepAliveSeconds,
	}
	if flags&0x80 != 0 {
		packet.Username, err = readMQTTString(body, &offset)
		if err != nil {
			return connectPacket{}, err
		}
	}
	if flags&0x40 != 0 {
		packet.Password, err = readMQTTString(body, &offset)
		if err != nil {
			return connectPacket{}, err
		}
	}
	return packet, nil
}

func mqttIdleReadTimeout(keepAliveSeconds uint16, publishInterval time.Duration) time.Duration {
	timeout := 5 * time.Minute
	if keepAliveSeconds > 0 {
		timeout = time.Duration(keepAliveSeconds) * 2 * time.Second
	}
	if publishInterval > 0 {
		minimum := publishInterval * 6
		if minimum > timeout {
			timeout = minimum
		}
	}
	if timeout < 30*time.Second {
		return 30 * time.Second
	}
	return timeout
}

func parseSubscribePacket(body []byte) (uint16, string, error) {
	if len(body) < 5 {
		return 0, "", errors.New("invalid subscribe packet")
	}
	packetID := binary.BigEndian.Uint16(body[:2])
	offset := 2
	topic, err := readMQTTString(body, &offset)
	if err != nil {
		return 0, "", err
	}
	if offset >= len(body) {
		return 0, "", errors.New("invalid subscribe qos")
	}
	return packetID, topic, nil
}

func buildSubAckPacket(packetID uint16, grantedQoS byte) []byte {
	body := make([]byte, 0, 3)
	body = appendPacketID(body, packetID)
	body = append(body, grantedQoS)
	return body
}

func readMQTTString(body []byte, offset *int) (string, error) {
	if *offset+2 > len(body) {
		return "", errors.New("invalid mqtt string length")
	}
	size := int(binary.BigEndian.Uint16(body[*offset : *offset+2]))
	*offset += 2
	if *offset+size > len(body) {
		return "", errors.New("invalid mqtt string body")
	}
	value := string(body[*offset : *offset+size])
	*offset += size
	return value, nil
}

func appendMQTTString(dst []byte, value string) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(len(value)))
	dst = append(dst, buf...)
	dst = append(dst, value...)
	return dst
}

func appendPacketID(dst []byte, packetID uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, packetID)
	return append(dst, buf...)
}

func readPacket(conn net.Conn, reader *bufio.Reader, readTimeout time.Duration) (byte, byte, []byte, error) {
	if readTimeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			return 0, 0, nil, err
		}
	}
	header, err := reader.ReadByte()
	if err != nil {
		return 0, 0, nil, err
	}
	packetType := header >> 4
	flags := header & 0x0f
	remainingLength, err := readRemainingLength(reader)
	if err != nil {
		return 0, 0, nil, err
	}
	body := make([]byte, remainingLength)
	if _, err := ioReadFull(reader, body); err != nil {
		return 0, 0, nil, err
	}
	return packetType, flags, body, nil
}

func writePacket(conn net.Conn, writer *bufio.Writer, writeTimeout time.Duration, header byte, body []byte) error {
	if writeTimeout > 0 {
		if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
			return err
		}
	}
	packet := []byte{header}
	packet = append(packet, encodeRemainingLength(len(body))...)
	packet = append(packet, body...)
	if _, err := writer.Write(packet); err != nil {
		return err
	}
	return writer.Flush()
}

func readRemainingLength(reader *bufio.Reader) (int, error) {
	multiplier := 1
	value := 0
	for i := 0; i < 4; i++ {
		encodedByte, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		value += int(encodedByte&127) * multiplier
		if encodedByte&128 == 0 {
			return value, nil
		}
		multiplier *= 128
	}
	return 0, errors.New("mqtt remaining length exceeds 4 bytes")
}

func encodeRemainingLength(length int) []byte {
	if length == 0 {
		return []byte{0}
	}
	out := make([]byte, 0, 4)
	for {
		encodedByte := byte(length % 128)
		length /= 128
		if length > 0 {
			encodedByte |= 128
		}
		out = append(out, encodedByte)
		if length == 0 {
			return out
		}
	}
}

func ioReadFull(reader *bufio.Reader, dst []byte) (int, error) {
	read := 0
	for read < len(dst) {
		n, err := reader.Read(dst[read:])
		read += n
		if err != nil {
			return read, err
		}
	}
	return read, nil
}

func splitAddress(address string) (string, string) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", ""
	}
	return host, port
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(payload)
}

func cloneQuery(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, entries := range values {
		out[key] = append([]string(nil), entries...)
	}
	return out
}

func isUseOfClosedNetworkConn(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

func generateTLSConfig(advertiseHost string) (*tls.Config, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "pulse-mqtt-emulator",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	if host := strings.TrimSpace(advertiseHost); host != "" && host != "localhost" {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, publicKey(privateKey), privateKey)
	if err != nil {
		return nil, err
	}
	certificate := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  privateKey,
		Leaf:        template,
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	}, nil
}

func publicKey(privateKey *ecdsa.PrivateKey) any {
	return &privateKey.PublicKey
}

func init() {
	// Ensure ASN.1 is linked for ECDSA private keys in distroless builds.
	_, _ = asn1.Marshal(struct{}{})
}
