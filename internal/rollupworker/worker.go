package rollupworker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/internal/workermetrics"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	defaultConsumerDurable = "rollup-timeseries-v1"
	defaultQueueGroup      = "rollup-timeseries"
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

type delivery interface {
	Subject() string
	Data() []byte
	Ack() error
	Nak() error
	Term() error
}

type natsDelivery struct {
	msg *nats.Msg
}

func (d natsDelivery) Subject() string { return d.msg.Subject }
func (d natsDelivery) Data() []byte    { return d.msg.Data }
func (d natsDelivery) Ack() error      { return d.msg.Ack() }
func (d natsDelivery) Nak() error      { return d.msg.Nak() }
func (d natsDelivery) Term() error     { return d.msg.Term() }

func DefaultConfig() Config {
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
	cfg       Config
	subscribe func(js nats.JetStreamContext, handler nats.MsgHandler) (*nats.Subscription, error)
	tracker   *telemetrybus.MsgHandlerTracker
	metrics   *workermetrics.Metrics
}

func New(log *slog.Logger, conn *nats.Conn, store Store, cfg Config) (*Worker, error) {
	if conn == nil {
		return nil, errors.New("nats connection is required")
	}
	if store == nil {
		return nil, errors.New("rollup store is required")
	}
	if log == nil {
		log = slog.Default()
	}
	cfg = cfg.normalized()
	w := &Worker{
		log:     log,
		conn:    conn,
		store:   store,
		cfg:     cfg,
		tracker: telemetrybus.NewMsgHandlerTracker(),
	}
	w.subscribe = w.defaultSubscribe
	return w, nil
}

func (w *Worker) Run(ctx context.Context) error {
	js, err := w.conn.JetStream()
	if err != nil {
		return fmt.Errorf("init jetstream context: %w", err)
	}

	w.log.Info("rollup worker running",
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
	_ = w.processDelivery(context.Background(), natsDelivery{msg: msg})
}

func (w *Worker) processDelivery(ctx context.Context, msg delivery) error {
	if msg == nil {
		return nil
	}
	finish := func(string) {}
	if w.metrics != nil {
		finish = w.metrics.StartMessage()
	}
	outcome := "acked"
	defer func() { finish(outcome) }()
	procCtx, cancel := context.WithTimeout(ctx, w.cfg.ProcessTimeout)
	defer cancel()

	var env envelopev1.TelemetryEnvelope
	if err := proto.Unmarshal(msg.Data(), &env); err != nil {
		outcome = "termed_invalid_proto"
		w.log.Warn("rollup received invalid telemetry envelope; terminating message",
			slog.String("subject", msg.Subject()),
			slog.String("error", err.Error()),
		)
		if termErr := msg.Term(); termErr != nil {
			w.log.Warn("rollup term failed", slog.String("error", termErr.Error()))
		}
		return err
	}

	if err := w.store.ApplyEnvelope(procCtx, &env); err != nil {
		switch {
		case errors.Is(err, ErrNoRollupMetrics):
			outcome = "acked_no_metrics"
			if ackErr := msg.Ack(); ackErr != nil {
				outcome = "ack_failed"
				w.log.Warn("rollup ack failed", slog.String("error", ackErr.Error()))
			}
			return nil
		case errors.Is(err, ErrInvalidRollupEnvelope):
			outcome = "termed_invalid_envelope"
			w.log.Warn("rollup received invalid telemetry sample; terminating message",
				slog.String("subject", msg.Subject()),
				slog.String("device_id", strings.TrimSpace(env.GetDeviceId())),
				slog.String("ecoflow_sn", strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn()))),
				slog.String("error", err.Error()),
			)
			if termErr := msg.Term(); termErr != nil {
				w.log.Warn("rollup term failed", slog.String("error", termErr.Error()))
			}
			return err
		default:
			outcome = "nacked_apply_failed"
			w.log.Warn("rollup apply envelope failed; nacking for retry",
				slog.String("subject", msg.Subject()),
				slog.String("device_id", strings.TrimSpace(env.GetDeviceId())),
				slog.String("ecoflow_sn", strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn()))),
				slog.String("error", err.Error()),
			)
			if nakErr := msg.Nak(); nakErr != nil {
				w.log.Warn("rollup nak failed", slog.String("error", nakErr.Error()))
			}
			return err
		}
	}

	if ackErr := msg.Ack(); ackErr != nil {
		outcome = "ack_failed"
		w.log.Warn("rollup ack failed", slog.String("error", ackErr.Error()))
	}
	return nil
}
