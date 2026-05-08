package gaprepair

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	replayv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/replay/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/logredact"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	defaultWorkerQueueGroup       = "gap-repair-workers"
	defaultWorkerDurable          = "gap-repair-v1"
	defaultWorkerAckWait          = 2 * time.Minute
	defaultWorkerMaxAckPending    = 1024
	defaultWorkerProcessTimeout   = 2 * time.Minute
	defaultWorkerDrainTimeout     = 10 * time.Second
	defaultReplayFailAlertWindow  = 10 * time.Minute
	defaultReplayFailAlertThresh  = 6
	defaultReplayFailAlertBackoff = 5 * time.Minute
)

type delivery interface {
	Data() []byte
	Ack() error
	Nak() error
	Term() error
}

type natsDelivery struct {
	msg *nats.Msg
}

func (d natsDelivery) Data() []byte {
	if d.msg == nil {
		return nil
	}
	return d.msg.Data
}

func (d natsDelivery) Ack() error {
	if d.msg == nil {
		return errors.New("delivery message is nil")
	}
	return d.msg.Ack()
}

func (d natsDelivery) Nak() error {
	if d.msg == nil {
		return errors.New("delivery message is nil")
	}
	return d.msg.Nak()
}

func (d natsDelivery) Term() error {
	if d.msg == nil {
		return errors.New("delivery message is nil")
	}
	return d.msg.Term()
}

type Worker struct {
	log                 *slog.Logger
	conn                *nats.Conn
	runner              ReplayRunner
	cfg                 WorkerConfig
	subjectCfg          telemetrybus.SubjectConfig
	subscribe           func(js nats.JetStreamContext, handler nats.MsgHandler) (*nats.Subscription, error)
	replayFailureAlerts *failureRateTracker
	tracker             *telemetrybus.MsgHandlerTracker
}

func DefaultWorkerConfig() WorkerConfig {
	streamCfg := telemetrybus.DefaultJetStreamGapRepairBootstrapConfig()
	return WorkerConfig{
		StreamName:                  streamCfg.StreamName,
		QueueGroup:                  defaultWorkerQueueGroup,
		Durable:                     defaultWorkerDurable,
		AckWait:                     defaultWorkerAckWait,
		MaxAckPending:               defaultWorkerMaxAckPending,
		ProcessTimeout:              defaultWorkerProcessTimeout,
		DrainTimeout:                defaultWorkerDrainTimeout,
		DefaultMaxObjects:           0,
		ReplayFailureAlertWindow:    defaultReplayFailAlertWindow,
		ReplayFailureAlertThreshold: defaultReplayFailAlertThresh,
		ReplayFailureAlertCooldown:  defaultReplayFailAlertBackoff,
	}
}

func NewWorker(log *slog.Logger, conn *nats.Conn, runner ReplayRunner, cfg WorkerConfig) (*Worker, error) {
	if log == nil {
		log = slog.Default()
	}
	if conn == nil {
		return nil, errors.New("nats connection is required")
	}
	if runner == nil {
		return nil, errors.New("replay runner is required")
	}
	cfg = normalizeWorkerConfig(cfg)
	w := &Worker{log: log, conn: conn, runner: runner, cfg: cfg, tracker: telemetrybus.NewMsgHandlerTracker()}
	w.subscribe = w.defaultSubscribe
	w.replayFailureAlerts = newFailureRateTracker(cfg.ReplayFailureAlertWindow, cfg.ReplayFailureAlertThreshold, cfg.ReplayFailureAlertCooldown)
	return w, nil
}

func normalizeWorkerConfig(cfg WorkerConfig) WorkerConfig {
	out := cfg
	defaults := DefaultWorkerConfig()
	if strings.TrimSpace(out.StreamName) == "" {
		out.StreamName = defaults.StreamName
	}
	if strings.TrimSpace(out.QueueGroup) == "" {
		out.QueueGroup = defaults.QueueGroup
	}
	if strings.TrimSpace(out.Durable) == "" {
		out.Durable = defaults.Durable
	}
	if out.AckWait <= 0 {
		out.AckWait = defaults.AckWait
	}
	if out.MaxAckPending <= 0 {
		out.MaxAckPending = defaults.MaxAckPending
	}
	if out.ProcessTimeout <= 0 {
		out.ProcessTimeout = defaults.ProcessTimeout
	}
	if out.DrainTimeout <= 0 {
		out.DrainTimeout = defaults.DrainTimeout
	}
	if out.DefaultMaxObjects < 0 {
		out.DefaultMaxObjects = 0
	}
	if out.ReplayFailureAlertWindow <= 0 {
		out.ReplayFailureAlertWindow = defaultReplayFailAlertWindow
	}
	if out.ReplayFailureAlertThreshold <= 0 {
		out.ReplayFailureAlertThreshold = defaultReplayFailAlertThresh
	}
	if out.ReplayFailureAlertCooldown <= 0 {
		out.ReplayFailureAlertCooldown = defaultReplayFailAlertBackoff
	}
	return out
}

func (w *Worker) Run(ctx context.Context, subjectCfg telemetrybus.SubjectConfig) error {
	js, err := w.conn.JetStream()
	if err != nil {
		return fmt.Errorf("init jetstream context: %w", err)
	}
	subjectCfg = subjectCfg.Normalized()
	w.subjectCfg = subjectCfg

	w.log.Info("gap-repair worker running",
		slog.String("subject", telemetrybus.GapRepairWildcardSubject(subjectCfg)),
		slog.String("stream", w.cfg.StreamName),
		slog.String("durable", w.cfg.Durable),
		slog.String("queue_group", w.cfg.QueueGroup),
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
		w.tracker.Wrap(w.handleMsg),
		w.tracker,
		telemetrybus.ConsumerSupervisorConfig{
			StreamName:   w.cfg.StreamName,
			Durable:      w.cfg.Durable,
			DrainTimeout: w.cfg.DrainTimeout,
		},
	)
}

func (w *Worker) defaultSubscribe(js nats.JetStreamContext, handler nats.MsgHandler) (*nats.Subscription, error) {
	return js.QueueSubscribe(
		telemetrybus.GapRepairWildcardSubject(w.subjectCfg),
		w.cfg.QueueGroup,
		handler,
		nats.BindStream(w.cfg.StreamName),
		nats.Durable(w.cfg.Durable),
		nats.ManualAck(),
		nats.AckWait(w.cfg.AckWait),
		nats.MaxAckPending(w.cfg.MaxAckPending),
	)
}

func (w *Worker) handleMsg(msg *nats.Msg) {
	if msg == nil {
		return
	}
	w.handleDelivery(natsDelivery{msg: msg})
}

func (w *Worker) handleDelivery(msg delivery) {
	var req replayv1.GapRepairRequest
	if err := proto.Unmarshal(msg.Data(), &req); err != nil {
		w.log.Warn("gap-repair invalid request; terminating",
			slog.String("error", err.Error()),
		)
		if termErr := msg.Term(); termErr != nil {
			w.log.Warn("gap-repair term failed", slog.String("error", termErr.Error()))
		}
		return
	}
	normalized, err := normalizeGapRequest(&req)
	if err != nil {
		w.log.Warn("gap-repair invalid request fields; terminating",
			slog.String("request_id", req.GetRequestId()),
			slog.String("error", err.Error()),
		)
		if termErr := msg.Term(); termErr != nil {
			w.log.Warn("gap-repair term failed", slog.String("error", termErr.Error()))
		}
		return
	}

	maxObjects := int(normalized.GetMaxObjects())
	if maxObjects <= 0 {
		maxObjects = w.cfg.DefaultMaxObjects
	}

	ctx, cancel := context.WithTimeout(context.Background(), w.cfg.ProcessTimeout)
	_, runErr := w.runner.ReplayDevices(ctx, replaycli.ReplayRequest{
		Provider:          normalized.GetProvider(),
		FromUnixMS:        normalized.GetFromUnixMs(),
		ToUnixMS:          normalized.GetToUnixMs(),
		ProviderDeviceIDs: []string{normalized.GetProviderDeviceId()},
		MaxObjects:        maxObjects,
	})
	cancel()
	if runErr != nil {
		failCount, failPerMin, spike := w.replayFailureAlerts.Record(time.Now().UTC())
		w.log.Warn("gap-repair replay failed; nacking",
			slog.String("request_id", normalized.GetRequestId()),
			slog.String("provider", normalized.GetProvider()),
			slog.String("provider_device_ref", logredact.Identifier(normalized.GetProviderDeviceId())),
			slog.String("error", runErr.Error()),
			slog.Int("replay_failures_in_window", failCount),
			slog.Float64("replay_failures_per_min", failPerMin),
			slog.Duration("replay_failure_window", w.cfg.ReplayFailureAlertWindow),
		)
		if spike {
			w.log.Warn("gap-repair replay-failure spike detected",
				slog.Int("replay_failures_in_window", failCount),
				slog.Float64("replay_failures_per_min", failPerMin),
				slog.Duration("window", w.cfg.ReplayFailureAlertWindow),
				slog.Int("threshold", w.cfg.ReplayFailureAlertThreshold),
				slog.Duration("cooldown", w.cfg.ReplayFailureAlertCooldown),
				slog.String("provider", normalized.GetProvider()),
				slog.String("provider_device_ref", logredact.Identifier(normalized.GetProviderDeviceId())),
			)
		}
		if nakErr := msg.Nak(); nakErr != nil {
			w.log.Warn("gap-repair nak failed", slog.String("error", nakErr.Error()))
		}
		return
	}
	if err := msg.Ack(); err != nil {
		w.log.Warn("gap-repair ack failed", slog.String("error", err.Error()))
		return
	}
	w.log.Info("gap-repair replay completed",
		slog.String("request_id", normalized.GetRequestId()),
		slog.String("provider", normalized.GetProvider()),
		slog.String("provider_device_ref", logredact.Identifier(normalized.GetProviderDeviceId())),
		slog.Int64("from_unix_ms", normalized.GetFromUnixMs()),
		slog.Int64("to_unix_ms", normalized.GetToUnixMs()),
	)
}

func normalizeGapRequest(in *replayv1.GapRepairRequest) (*replayv1.GapRepairRequest, error) {
	if in == nil {
		return nil, errors.New("request is nil")
	}
	out := proto.Clone(in).(*replayv1.GapRepairRequest)
	out.Provider = strings.TrimSpace(out.GetProvider())
	out.ProviderDeviceId = strings.ToUpper(strings.TrimSpace(out.GetProviderDeviceId()))
	out.DeviceId = strings.TrimSpace(out.GetDeviceId())
	out.Reason = strings.TrimSpace(out.GetReason())
	if out.GetProvider() == "" {
		return nil, errors.New("provider is required")
	}
	if out.GetProviderDeviceId() == "" {
		return nil, errors.New("provider_device_id is required")
	}
	if out.GetFromUnixMs() <= 0 || out.GetToUnixMs() <= 0 || out.GetFromUnixMs() >= out.GetToUnixMs() {
		return nil, errors.New("invalid replay window")
	}
	if out.GetMaxObjects() < 0 {
		out.MaxObjects = 0
	}
	return out, nil
}
