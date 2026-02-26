package gaprepair

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	replayv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/replay/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type NATSPublisherConfig struct {
	SubjectConfig telemetrybus.SubjectConfig
	UseJetStream  bool
	MsgIDBucket   time.Duration
}

func DefaultNATSPublisherConfig(subjectCfg telemetrybus.SubjectConfig) NATSPublisherConfig {
	return NATSPublisherConfig{
		SubjectConfig: subjectCfg.Normalized(),
		UseJetStream:  true,
		MsgIDBucket:   time.Minute,
	}
}

type NATSPublisher struct {
	conn     *nats.Conn
	js       nats.JetStreamContext
	cfg      NATSPublisherConfig
	bucketMS int64
}

func NewNATSPublisher(conn *nats.Conn, cfg NATSPublisherConfig) (*NATSPublisher, error) {
	if conn == nil {
		return nil, errors.New("nats connection is required")
	}
	cfg = normalizePublisherConfig(cfg)
	var js nats.JetStreamContext
	if cfg.UseJetStream {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		js, err = conn.JetStream(nats.Context(ctx))
		if err != nil {
			return nil, fmt.Errorf("init jetstream context: %w", err)
		}
	}
	return &NATSPublisher{
		conn:     conn,
		js:       js,
		cfg:      cfg,
		bucketMS: cfg.MsgIDBucket.Milliseconds(),
	}, nil
}

func normalizePublisherConfig(cfg NATSPublisherConfig) NATSPublisherConfig {
	out := cfg
	out.SubjectConfig = out.SubjectConfig.Normalized()
	if out.MsgIDBucket <= 0 {
		out.MsgIDBucket = time.Minute
	}
	return out
}

func (p *NATSPublisher) PublishGapRepair(ctx context.Context, req *replayv1.GapRepairRequest) error {
	if p == nil || p.conn == nil {
		return errors.New("gap-repair nats publisher is not initialized")
	}
	if req == nil {
		return errors.New("gap-repair request is required")
	}
	normalized, err := p.normalizeRequest(req)
	if err != nil {
		return err
	}
	payload, err := proto.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal gap-repair request: %w", err)
	}
	subject := telemetrybus.GapRepairSubject(p.cfg.SubjectConfig, normalized.GetShard())
	msg := &nats.Msg{Subject: subject, Data: payload, Header: nats.Header{}}
	msg.Header.Set("Nats-Msg-Id", p.msgID(normalized))

	if p.cfg.UseJetStream {
		if p.js == nil {
			return errors.New("jetstream context is nil for gap-repair publish")
		}
		if _, err := p.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
			return fmt.Errorf("publish gap-repair request via jetstream subject=%s: %w", subject, err)
		}
		return nil
	}
	if err := p.conn.PublishMsg(msg); err != nil {
		return fmt.Errorf("publish gap-repair request subject=%s: %w", subject, err)
	}
	return nil
}

func (p *NATSPublisher) normalizeRequest(req *replayv1.GapRepairRequest) (*replayv1.GapRepairRequest, error) {
	provider := strings.TrimSpace(req.GetProvider())
	providerDeviceID := strings.ToUpper(strings.TrimSpace(req.GetProviderDeviceId()))
	if provider == "" {
		return nil, errors.New("gap-repair request provider is required")
	}
	if providerDeviceID == "" {
		return nil, errors.New("gap-repair request provider_device_id is required")
	}
	from := req.GetFromUnixMs()
	to := req.GetToUnixMs()
	if from <= 0 || to <= 0 || from >= to {
		return nil, errors.New("gap-repair request window is invalid")
	}
	out := proto.Clone(req).(*replayv1.GapRepairRequest)
	out.Provider = provider
	out.ProviderDeviceId = providerDeviceID
	if out.GetDeviceId() != "" {
		out.DeviceId = strings.TrimSpace(out.GetDeviceId())
	}
	if out.GetShardCount() == 0 {
		out.ShardCount = p.cfg.SubjectConfig.ShardCount
	}
	if out.GetShardCount() == 0 {
		out.ShardCount = telemetrybus.DefaultShardCount
	}
	if out.GetShard() >= out.GetShardCount() {
		shard := telemetrybus.ShardForDevice(providerDeviceID, out.GetShardCount())
		out.Shard = shard
	}
	if strings.TrimSpace(out.GetRequestId()) == "" {
		out.RequestId = p.msgID(out)
	}
	out.Reason = strings.TrimSpace(out.GetReason())
	return out, nil
}

func (p *NATSPublisher) msgID(req *replayv1.GapRepairRequest) string {
	if req == nil {
		return "gaprepair:invalid"
	}
	bucketMS := p.bucketMS
	if bucketMS <= 0 {
		bucketMS = int64(time.Minute / time.Millisecond)
	}
	fromBucket := req.GetFromUnixMs() / bucketMS
	toBucket := req.GetToUnixMs() / bucketMS
	base := strings.Join([]string{
		strings.TrimSpace(req.GetProvider()),
		strings.ToUpper(strings.TrimSpace(req.GetProviderDeviceId())),
		strconv.FormatInt(fromBucket, 10),
		strconv.FormatInt(toBucket, 10),
	}, "|")
	h := fnv.New64a()
	_, _ = h.Write([]byte(base))
	return fmt.Sprintf("gaprepair:%016x", h.Sum64())
}

func (p *NATSPublisher) Close() error {
	if p == nil || p.conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.conn.FlushWithContext(ctx); err != nil {
		return fmt.Errorf("flush gap-repair publisher: %w", err)
	}
	return nil
}
