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

type NATSPublisher struct {
	conn       *nats.Conn
	subjectCfg telemetrybus.SubjectConfig
}

func NewNATSPublisher(conn *nats.Conn, subjectCfg telemetrybus.SubjectConfig) (*NATSPublisher, error) {
	if conn == nil {
		return nil, errors.New("nats connection is required")
	}
	return &NATSPublisher{
		conn:       conn,
		subjectCfg: subjectCfg.Normalized(),
	}, nil
}

func (p *NATSPublisher) Publish(ctx context.Context, shard uint32, payload []byte) error {
	if p == nil || p.conn == nil {
		return errors.New("nats replay publisher is not initialized")
	}
	if len(payload) == 0 {
		return errors.New("replay payload is empty")
	}
	subject := telemetrybus.ReplaySubject(p.subjectCfg, shard)
	if strings.TrimSpace(subject) == "" {
		return errors.New("replay subject is empty")
	}
	if err := p.conn.Publish(subject, payload); err != nil {
		return fmt.Errorf("publish replay envelope subject=%s: %w", subject, err)
	}
	_ = ctx
	return nil
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
