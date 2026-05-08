package inference

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/internal/workermetrics"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	defaultConsumerDurable = "inference-device-insights-v1"
	defaultQueueGroup      = "inference-device-insights"
	defaultAckWait         = 30 * time.Second
	defaultMaxAckPending   = 4096
	defaultProcessTimeout  = 3 * time.Second
	defaultDrainTimeout    = 8 * time.Second
)

type Config struct {
	SubjectConfig  SubjectConfig
	StreamName     string
	Durable        string
	QueueGroup     string
	AckWait        time.Duration
	MaxAckPending  int
	ProcessTimeout time.Duration
	DrainTimeout   time.Duration
}

type SubjectConfig = telemetrybus.SubjectConfig

func DefaultWorkerConfig() Config {
	return Config{
		SubjectConfig: SubjectConfig{
			Prefix:     telemetrybus.DefaultSubjectPrefix,
			ShardCount: telemetrybus.DefaultShardCount,
		},
		StreamName:     "PULSE_TELEMETRY_INGEST",
		Durable:        defaultConsumerDurable,
		QueueGroup:     defaultQueueGroup,
		AckWait:        defaultAckWait,
		MaxAckPending:  defaultMaxAckPending,
		ProcessTimeout: defaultProcessTimeout,
		DrainTimeout:   defaultDrainTimeout,
	}
}

func (c Config) normalized() Config {
	out := c
	out.SubjectConfig = out.SubjectConfig.Normalized()
	if strings.TrimSpace(out.StreamName) == "" {
		out.StreamName = "PULSE_TELEMETRY_INGEST"
	}
	if strings.TrimSpace(out.Durable) == "" {
		out.Durable = defaultConsumerDurable
	}
	if strings.TrimSpace(out.QueueGroup) == "" {
		out.QueueGroup = defaultQueueGroup
	}
	if out.AckWait <= 0 {
		out.AckWait = defaultAckWait
	}
	if out.MaxAckPending <= 0 {
		out.MaxAckPending = defaultMaxAckPending
	}
	if out.ProcessTimeout <= 0 {
		out.ProcessTimeout = defaultProcessTimeout
	}
	if out.DrainTimeout <= 0 {
		out.DrainTimeout = defaultDrainTimeout
	}
	return out
}

type Worker struct {
	log       *slog.Logger
	conn      *nats.Conn
	store     Store
	resolver  DeviceContextResolver
	cfg       Config
	subscribe func(js nats.JetStreamContext, handler nats.MsgHandler) (*nats.Subscription, error)
	tracker   *telemetrybus.MsgHandlerTracker
	metrics   *workermetrics.Metrics
}

func NewWorker(log *slog.Logger, conn *nats.Conn, store Store, resolver DeviceContextResolver, cfg Config) (*Worker, error) {
	if conn == nil {
		return nil, errors.New("nats connection is required")
	}
	if store == nil {
		return nil, errors.New("inference store is required")
	}
	if resolver == nil {
		return nil, errors.New("device context resolver is required")
	}
	if log == nil {
		log = slog.Default()
	}
	cfg = cfg.normalized()
	w := &Worker{
		log:      log,
		conn:     conn,
		store:    store,
		resolver: resolver,
		cfg:      cfg,
		tracker:  telemetrybus.NewMsgHandlerTracker(),
	}
	w.subscribe = w.defaultSubscribe
	return w, nil
}

func (w *Worker) Run(ctx context.Context) error {
	js, err := w.conn.JetStream()
	if err != nil {
		return fmt.Errorf("init jetstream context: %w", err)
	}

	w.log.Info("inference worker running",
		slog.String("subject", telemetrybus.IngestWildcardSubject(w.cfg.SubjectConfig)),
		slog.String("stream", w.cfg.StreamName),
		slog.String("queue_group", w.cfg.QueueGroup),
		slog.String("durable", w.cfg.Durable),
		slog.Duration("ack_wait", w.cfg.AckWait),
		slog.Int("max_ack_pending", w.cfg.MaxAckPending),
		slog.Duration("process_timeout", w.cfg.ProcessTimeout),
	)

	return telemetrybus.RunConsumerSupervisor(
		ctx,
		w.log,
		js,
		func(handler nats.MsgHandler) (telemetrybus.QueueSubscription, error) {
			return w.subscribe(js, handler)
		},
		w.tracker.Wrap(w.handleMessage),
		w.tracker,
		telemetrybus.ConsumerSupervisorConfig{
			StreamName:   w.cfg.StreamName,
			Durable:      w.cfg.Durable,
			DrainTimeout: w.cfg.DrainTimeout,
		},
	)
}

func (w *Worker) SetMetrics(metrics *workermetrics.Metrics) {
	if w == nil {
		return
	}
	w.metrics = metrics
}

func (w *Worker) defaultSubscribe(js nats.JetStreamContext, handler nats.MsgHandler) (*nats.Subscription, error) {
	return telemetrybus.QueueSubscribeIngest(js, telemetrybus.IngestQueueSubscribeConfig{
		SubjectConfig: w.cfg.SubjectConfig,
		StreamName:    w.cfg.StreamName,
		Durable:       w.cfg.Durable,
		QueueGroup:    w.cfg.QueueGroup,
		AckWait:       w.cfg.AckWait,
		MaxAckPending: w.cfg.MaxAckPending,
	}, handler)
}

func (w *Worker) handleMessage(msg *nats.Msg) {
	if msg == nil {
		return
	}
	finish := func(string) {}
	if w.metrics != nil {
		finish = w.metrics.StartMessage()
	}
	outcome := "acked"
	defer func() { finish(outcome) }()
	procCtx, cancel := context.WithTimeout(context.Background(), w.cfg.ProcessTimeout)
	defer cancel()

	var env envelopev1.TelemetryEnvelope
	if err := proto.Unmarshal(msg.Data, &env); err != nil {
		outcome = "termed_invalid_proto"
		w.log.Warn("inference received invalid telemetry envelope; terminating message",
			slog.String("subject", msg.Subject),
			slog.String("error", err.Error()),
		)
		if termErr := msg.Term(); termErr != nil {
			w.log.Warn("inference term failed", slog.String("error", termErr.Error()))
		}
		return
	}

	deviceID := strings.TrimSpace(env.GetDeviceId())
	if deviceID == "" {
		outcome = "termed_missing_device_id"
		w.log.Warn("inference received envelope without device_id; terminating message",
			slog.String("subject", msg.Subject),
			slog.String("ecoflow_sn", strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn()))),
		)
		if termErr := msg.Term(); termErr != nil {
			w.log.Warn("inference term failed", slog.String("error", termErr.Error()))
		}
		return
	}

	deviceCtx, err := w.resolver.ResolveDeviceContext(procCtx, deviceID)
	if err != nil {
		if errors.Is(err, controlplane.ErrDeviceNotFound) {
			outcome = "termed_device_not_found"
			w.log.Warn("inference device context not found; terminating message",
				slog.String("device_id", deviceID),
			)
			if termErr := msg.Term(); termErr != nil {
				w.log.Warn("inference term failed", slog.String("error", termErr.Error()))
			}
			return
		}
		outcome = "nacked_resolve_failed"
		w.log.Warn("inference resolve device context failed; nacking for retry",
			slog.String("device_id", deviceID),
			slog.String("error", err.Error()),
		)
		if nakErr := msg.Nak(); nakErr != nil {
			w.log.Warn("inference nak failed", slog.String("error", nakErr.Error()))
		}
		return
	}

	if _, err := w.store.ApplyEnvelope(procCtx, &env, deviceCtx); err != nil {
		outcome = "nacked_apply_failed"
		w.log.Warn("inference apply envelope failed; nacking for retry",
			slog.String("subject", msg.Subject),
			slog.String("device_id", deviceID),
			slog.String("error", err.Error()),
		)
		if nakErr := msg.Nak(); nakErr != nil {
			w.log.Warn("inference nak failed", slog.String("error", nakErr.Error()))
		}
		return
	}

	if ackErr := msg.Ack(); ackErr != nil {
		outcome = "ack_failed"
		w.log.Warn("inference ack failed", slog.String("error", ackErr.Error()))
	}
}
