package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

const (
	defaultBindAddr       = "127.0.0.1:19090"
	defaultRequestTimeout = 5 * time.Second
)

type config struct {
	bindAddr       string
	source         string
	provider       string
	subject        telemetrybus.SubjectConfig
	publishOptions telemetrybus.NATSEnvelopePublisherOptions
}

type ingestServer struct {
	log       *slog.Logger
	cfg       config
	publisher telemetrybus.EnvelopePublisher
	nowFn     func() time.Time
	accepted  atomic.Uint64
	rejected  atomic.Uint64
}

type ingestRequest struct {
	DeviceID      string         `json:"device_id"`
	SerialNumber  string         `json:"serial_number"`
	ObservedUnixM int64          `json:"observed_unix_ms"`
	MessageID     string         `json:"message_id"`
	Metrics       requestMetrics `json:"metrics"`
}

type requestMetrics struct {
	SOC         *float64 `json:"soc"`
	PVW         *float64 `json:"pv_w"`
	LoadW       *float64 `json:"load_w"`
	AcW         *float64 `json:"ac_w"`
	DcW         *float64 `json:"dc_w"`
	BatteryInW  *float64 `json:"battery_in_w"`
	BatteryOutW *float64 `json:"battery_out_w"`
	TempC       *float64 `json:"temp_c"`
}

type ingestResponse struct {
	Accepted         bool   `json:"accepted"`
	EnvelopeID       string `json:"envelope_id"`
	DeviceID         string `json:"device_id"`
	SerialNumber     string `json:"serial_number"`
	Shard            uint32 `json:"shard"`
	ShardCount       uint32 `json:"shard_count"`
	IngestedUnixMS   int64  `json:"ingested_unix_ms"`
	PublishLatencyMS int64  `json:"publish_latency_ms"`
}

func main() {
	logCfg := pulselog.DefaultServiceConfig("loadtest-ingest-bridge")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	logCfg.AsyncEnabled = !runtimecfg.Bool("LOG_ASYNC_DISABLED", false)
	logCfg.AsyncQueueSize = runtimecfg.IntMin("LOG_ASYNC_QUEUE_SIZE", logCfg.AsyncQueueSize, 128)
	logCfg.AsyncBypassLevel = pulselog.ParseLevel(runtimecfg.EnvOrDefault("LOG_ASYNC_BYPASS_LEVEL", "warn"), slog.LevelWarn)

	log, asyncLogHandler, err := pulselog.BuildServiceLogger(logCfg)
	if err != nil {
		_, _ = os.Stderr.WriteString("init logger failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		if asyncLogHandler != nil {
			asyncLogHandler.Close()
		}
	}()

	cfg := loadConfig()

	natsCfg := telemetrybus.DefaultNATSConnConfig(runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-loadtest-ingest-bridge")
	natsCfg.ConnectTimeout = runtimecfg.DurationPositive("NATS_CONNECT_TIMEOUT", natsCfg.ConnectTimeout)
	natsCfg.ReconnectWait = runtimecfg.DurationPositive("NATS_RECONNECT_WAIT", natsCfg.ReconnectWait)
	natsCfg.ReconnectJitter = runtimecfg.DurationPositive("NATS_RECONNECT_JITTER", natsCfg.ReconnectJitter)
	natsCfg.PingInterval = runtimecfg.DurationPositive("NATS_PING_INTERVAL", natsCfg.PingInterval)
	natsCfg.MaxPingsOut = runtimecfg.IntMin("NATS_MAX_PINGS_OUT", natsCfg.MaxPingsOut, 1)
	natsCfg.MaxReconnects = runtimecfg.IntMin("NATS_MAX_RECONNECTS", natsCfg.MaxReconnects, -1)

	natsConn, err := telemetrybus.DialNATS(log, natsCfg)
	if err != nil {
		log.Error("init nats connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer natsConn.Close()

	publisher, err := telemetrybus.NewNATSEnvelopePublisherWithOptions(natsConn, cfg.subject, cfg.publishOptions)
	if err != nil {
		log.Error("init ingest publisher failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if closeErr := publisher.Close(); closeErr != nil {
			log.Warn("close ingest publisher failed", slog.String("error", closeErr.Error()))
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "loadtest-ingest-bridge", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()

	handler := &ingestServer{
		log:       log,
		cfg:       cfg,
		publisher: publisher,
		nowFn:     time.Now,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.handleHealth)
	mux.HandleFunc("/ingest", handler.handleIngest)

	srv := &http.Server{
		Addr:              cfg.bindAddr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn("http server shutdown failed", slog.String("error", err.Error()))
		}
	}()

	log.Info("loadtest ingest bridge starting",
		slog.String("bind_addr", cfg.bindAddr),
		slog.String("provider", cfg.provider),
		slog.String("source", cfg.source),
		slog.String("subject_prefix", cfg.subject.Prefix),
		slog.Uint64("shard_count", uint64(cfg.subject.ShardCount)),
		slog.Bool("publish_use_jetstream", cfg.publishOptions.UseJetStream),
		slog.Duration("publish_timeout", cfg.publishOptions.PublishTimeout),
		slog.Int("publish_max_retries", cfg.publishOptions.PublishMaxRetries),
		slog.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
	)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("http server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("loadtest ingest bridge stopped",
		slog.Uint64("accepted_total", handler.accepted.Load()),
		slog.Uint64("rejected_total", handler.rejected.Load()),
	)
}

func loadConfig() config {
	subject := telemetrybus.SubjectConfig{
		Prefix:     runtimecfg.EnvOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: runtimecfg.Uint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}.Normalized()

	publishOptions := telemetrybus.NATSEnvelopePublisherOptions{}
	publishOptions.StripLabels = runtimecfg.Bool("INGEST_DISABLE_ENVELOPE_LABELS", false)
	publishOptions.UseJetStream = runtimecfg.Bool("INGEST_NATS_USE_JETSTREAM", true)
	publishOptions.PublishTimeout = runtimecfg.DurationPositive("INGEST_NATS_PUBLISH_TIMEOUT", defaultRequestTimeout)
	publishOptions.PublishMaxRetries = runtimecfg.IntMin("INGEST_NATS_PUBLISH_MAX_RETRIES", 3, 0)
	publishOptions.PublishRetryInitialBackoff = runtimecfg.DurationPositive("INGEST_NATS_PUBLISH_RETRY_INITIAL_BACKOFF", 50*time.Millisecond)
	publishOptions.PublishRetryMaxBackoff = runtimecfg.DurationPositive("INGEST_NATS_PUBLISH_RETRY_MAX_BACKOFF", 500*time.Millisecond)
	publishOptions.PublishRetryJitter = runtimecfg.Float64NonNegative("INGEST_NATS_PUBLISH_RETRY_JITTER", 0.20)

	return config{
		bindAddr:       runtimecfg.EnvOrDefault("LOADTEST_INGEST_BIND_ADDR", defaultBindAddr),
		source:         runtimecfg.EnvOrDefault("LOADTEST_INGEST_SOURCE", "k6-loadtest"),
		provider:       strings.ToLower(strings.TrimSpace(runtimecfg.EnvOrDefault("LOADTEST_INGEST_PROVIDER", "ecoflow"))),
		subject:        subject,
		publishOptions: publishOptions,
	}
}

func (s *ingestServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"accepted_total": s.accepted.Load(),
		"rejected_total": s.rejected.Load(),
	})
}

func (s *ingestServer) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.rejected.Add(1)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	bodyReader := io.LimitReader(r.Body, 64*1024)
	defer func() { _ = r.Body.Close() }()

	var req ingestRequest
	if err := json.NewDecoder(bodyReader).Decode(&req); err != nil {
		s.rejected.Add(1)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json", "message": err.Error()})
		return
	}

	normalized, err := normalizeRequest(req)
	if err != nil {
		s.rejected.Add(1)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request", "message": err.Error()})
		return
	}

	start := s.nowFn().UTC()
	envelope, err := buildEnvelope(s.cfg, normalized, start)
	if err != nil {
		s.rejected.Add(1)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "build_envelope_failed", "message": err.Error()})
		return
	}

	ctx := r.Context()
	if s.cfg.publishOptions.PublishTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(r.Context(), s.cfg.publishOptions.PublishTimeout)
		defer cancel()
	}

	if err := telemetrybus.PublishEnvelope(ctx, s.publisher, envelope); err != nil {
		s.rejected.Add(1)
		s.log.Warn("publish envelope failed",
			slog.String("device_id", normalized.DeviceID),
			slog.String("serial_number", normalized.SerialNumber),
			slog.String("error", err.Error()),
		)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "publish_failed", "message": err.Error()})
		return
	}

	s.accepted.Add(1)
	latency := time.Since(start)
	writeJSON(w, http.StatusAccepted, ingestResponse{
		Accepted:         true,
		EnvelopeID:       envelope.GetEnvelopeId(),
		DeviceID:         envelope.GetDeviceId(),
		SerialNumber:     envelope.GetEcoflowSn(),
		Shard:            envelope.GetShard(),
		ShardCount:       envelope.GetShardCount(),
		IngestedUnixMS:   envelope.GetIngestedTimeUnixMs(),
		PublishLatencyMS: latency.Milliseconds(),
	})
}

func normalizeRequest(req ingestRequest) (ingestRequest, error) {
	out := req
	out.DeviceID = strings.TrimSpace(out.DeviceID)
	out.SerialNumber = strings.ToUpper(strings.TrimSpace(out.SerialNumber))
	out.MessageID = strings.TrimSpace(out.MessageID)

	if out.DeviceID == "" {
		return ingestRequest{}, errors.New("device_id is required")
	}
	if _, err := uuid.Parse(out.DeviceID); err != nil {
		return ingestRequest{}, errors.New("device_id must be a UUID")
	}
	if out.SerialNumber == "" {
		return ingestRequest{}, errors.New("serial_number is required")
	}
	if out.ObservedUnixM <= 0 {
		out.ObservedUnixM = time.Now().UTC().UnixMilli()
	}
	return out, nil
}

func buildEnvelope(cfg config, req ingestRequest, now time.Time) (*envelopev1.TelemetryEnvelope, error) {
	envelopeID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate envelope id: %w", err)
	}
	messageID := req.MessageID
	if messageID == "" {
		messageID = envelopeID.String()
	}
	payload, err := buildPayload(req.Metrics)
	if err != nil {
		return nil, fmt.Errorf("build payload: %w", err)
	}
	shard := telemetrybus.ShardForDevice(req.DeviceID, cfg.subject.ShardCount)
	ingestedUnixMS := now.UTC().UnixMilli()
	observedUnixMS := req.ObservedUnixM
	if observedUnixMS <= 0 {
		observedUnixMS = ingestedUnixMS
	}
	return &envelopev1.TelemetryEnvelope{
		EnvelopeId:         envelopeID.String(),
		EnvelopeVersion:    1,
		DeviceId:           req.DeviceID,
		EcoflowSn:          req.SerialNumber,
		Shard:              shard,
		ShardCount:         cfg.subject.ShardCount,
		MessageId:          messageID,
		DeviceTimeUnixMs:   observedUnixMS,
		ObservedTimeUnixMs: observedUnixMS,
		IngestedTimeUnixMs: ingestedUnixMS,
		SourceKind:         envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA,
		Source:             cfg.source,
		TypeCode:           "quota",
		PayloadType:        "ecoflow.quota.normalized",
		PayloadVersion:     1,
		PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		Payload:            payload,
		Labels: map[string]string{
			"provider": cfg.provider,
			"source":   cfg.source,
		},
	}, nil
}

func buildPayload(metrics requestMetrics) ([]byte, error) {
	soc := valueOrDefault(metrics.SOC, 55)
	pvW := valueOrDefault(metrics.PVW, 220)
	loadW := valueOrDefault(metrics.LoadW, 180)
	acW := valueOrDefault(metrics.AcW, 40)
	dcW := valueOrDefault(metrics.DcW, 25)
	batteryInW := valueOrDefault(metrics.BatteryInW, maxFloat(0, pvW+acW))
	batteryOutW := valueOrDefault(metrics.BatteryOutW, maxFloat(0, loadW-(pvW+acW)))
	tempC := valueOrDefault(metrics.TempC, 24)

	payload := map[string]any{
		"params": map[string]float64{
			"soc":            soc,
			"pv1ChargeWatts": pvW,
			"wattsOutSum":    loadW,
			"inAcC20Pwr":     acW,
			"carWatts":       dcW,
			"bmsInputWatts":  batteryInW,
			"bmsOutputWatts": batteryOutW,
			"temp":           tempC,
		},
	}
	return json.Marshal(payload)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		_, _ = w.Write([]byte(`{"error":"encode_response_failed"}`))
	}
}

func valueOrDefault(value *float64, fallback float64) float64 {
	if value == nil || !isFinite(*value) {
		return fallback
	}
	return *value
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
