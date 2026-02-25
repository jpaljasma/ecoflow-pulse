package replaycli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/nats-io/nats.go"
)

type NATSPublishTarget string

const (
	NATSPublishTargetReplay NATSPublishTarget = "replay"
	NATSPublishTargetIngest NATSPublishTarget = "ingest"
)

type NATSPublisherConfig struct {
	SubjectConfig telemetrybus.SubjectConfig
	Target        NATSPublishTarget
}

func DefaultNATSPublisherConfig(subjectCfg telemetrybus.SubjectConfig) NATSPublisherConfig {
	return NATSPublisherConfig{
		SubjectConfig: subjectCfg.Normalized(),
		Target:        NATSPublishTargetReplay,
	}
}

type NATSPublisher struct {
	conn       *nats.Conn
	subjectCfg telemetrybus.SubjectConfig
	target     NATSPublishTarget
}

func NewNATSPublisher(conn *nats.Conn, subjectCfg telemetrybus.SubjectConfig) (*NATSPublisher, error) {
	return NewNATSPublisherWithConfig(conn, DefaultNATSPublisherConfig(subjectCfg))
}

func NewNATSPublisherWithConfig(conn *nats.Conn, cfg NATSPublisherConfig) (*NATSPublisher, error) {
	if conn == nil {
		return nil, errors.New("nats connection is required")
	}
	target := normalizePublishTarget(cfg.Target)
	if target == "" {
		return nil, fmt.Errorf("unsupported nats publish target %q", cfg.Target)
	}
	return &NATSPublisher{
		conn:       conn,
		subjectCfg: cfg.SubjectConfig.Normalized(),
		target:     target,
	}, nil
}

func (p *NATSPublisher) Publish(ctx context.Context, shard uint32, payload []byte) error {
	if p == nil || p.conn == nil {
		return errors.New("nats replay publisher is not initialized")
	}
	if len(payload) == 0 {
		return errors.New("replay payload is empty")
	}
	subject := p.subjectForShard(shard)
	if strings.TrimSpace(subject) == "" {
		return errors.New("replay subject is empty")
	}
	if err := p.conn.Publish(subject, payload); err != nil {
		return fmt.Errorf("publish replay envelope subject=%s: %w", subject, err)
	}
	_ = ctx
	return nil
}

func (p *NATSPublisher) subjectForShard(shard uint32) string {
	if p == nil {
		return ""
	}
	switch p.target {
	case NATSPublishTargetIngest:
		return telemetrybus.IngestSubject(p.subjectCfg, shard)
	case NATSPublishTargetReplay:
		return telemetrybus.ReplaySubject(p.subjectCfg, shard)
	default:
		return ""
	}
}

func normalizePublishTarget(target NATSPublishTarget) NATSPublishTarget {
	trimmed := NATSPublishTarget(strings.ToLower(strings.TrimSpace(string(target))))
	switch trimmed {
	case "", NATSPublishTargetReplay:
		return NATSPublishTargetReplay
	case NATSPublishTargetIngest:
		return NATSPublishTargetIngest
	default:
		return ""
	}
}

func (p *NATSPublisher) Close() error {
	if p == nil || p.conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.conn.FlushWithContext(ctx); err != nil {
		return fmt.Errorf("flush replay publisher: %w", err)
	}
	return nil
}
