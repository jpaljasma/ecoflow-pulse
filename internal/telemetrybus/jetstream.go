package telemetrybus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	defaultIngestStreamName      = "PULSE_TELEMETRY_INGEST"
	defaultIngestStreamReplicas  = 3
	defaultIngestStreamMaxAge    = 12 * time.Hour
	defaultIngestStreamStorage   = nats.FileStorage
	defaultIngestStreamRetention = nats.LimitsPolicy
)

type JetStreamIngestBootstrapConfig struct {
	Enabled bool
	// StreamName is the target stream for ingest subjects.
	StreamName string
	// Replicas controls stream replica factor.
	Replicas int
	// MaxAge bounds retention horizon for ingest backlog.
	MaxAge time.Duration
	// MaxBytes bounds aggregate stream bytes. 0 means broker default.
	MaxBytes int64
	// Storage controls file/memory persistence.
	Storage nats.StorageType
	// Retention controls stream retention behavior.
	Retention nats.RetentionPolicy
}

func DefaultJetStreamIngestBootstrapConfig() JetStreamIngestBootstrapConfig {
	return JetStreamIngestBootstrapConfig{
		Enabled:    true,
		StreamName: defaultIngestStreamName,
		Replicas:   defaultIngestStreamReplicas,
		MaxAge:     defaultIngestStreamMaxAge,
		MaxBytes:   0,
		Storage:    defaultIngestStreamStorage,
		Retention:  defaultIngestStreamRetention,
	}
}

func (c JetStreamIngestBootstrapConfig) normalized() JetStreamIngestBootstrapConfig {
	out := c
	if strings.TrimSpace(out.StreamName) == "" {
		out.StreamName = defaultIngestStreamName
	}
	if out.Replicas <= 0 {
		out.Replicas = defaultIngestStreamReplicas
	}
	if out.MaxAge <= 0 {
		out.MaxAge = defaultIngestStreamMaxAge
	}
	if out.Storage == nats.StorageType(0) {
		out.Storage = defaultIngestStreamStorage
	}
	if out.Retention == nats.RetentionPolicy(0) {
		out.Retention = defaultIngestStreamRetention
	}
	return out
}

func EnsureJetStreamIngestStream(
	ctx context.Context,
	conn *nats.Conn,
	subjectCfg SubjectConfig,
	cfg JetStreamIngestBootstrapConfig,
) error {
	if conn == nil {
		return errors.New("nats connection is required")
	}
	cfg = cfg.normalized()
	if !cfg.Enabled {
		return nil
	}
	js, err := conn.JetStream()
	if err != nil {
		return fmt.Errorf("init jetstream context: %w", err)
	}
	return ensureJetStreamIngestStreamWithManager(ctx, js, subjectCfg, cfg)
}

type jetStreamManager interface {
	StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	UpdateStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
}

func ensureJetStreamIngestStreamWithManager(
	ctx context.Context,
	js jetStreamManager,
	subjectCfg SubjectConfig,
	cfg JetStreamIngestBootstrapConfig,
) error {
	if js == nil {
		return errors.New("jetstream manager is required")
	}
	subjectCfg = subjectCfg.Normalized()
	cfg = cfg.normalized()
	desired := desiredIngestStreamConfig(subjectCfg, cfg)
	opts := []nats.JSOpt{nats.Context(ctx)}

	info, err := js.StreamInfo(desired.Name, opts...)
	if err != nil {
		if errors.Is(err, nats.ErrStreamNotFound) {
			if _, addErr := js.AddStream(&desired, opts...); addErr != nil {
				return fmt.Errorf("add ingest stream %q: %w", desired.Name, addErr)
			}
			return nil
		}
		return fmt.Errorf("read ingest stream info %q: %w", desired.Name, err)
	}
	if info == nil || info.Config.Name == "" {
		return fmt.Errorf("ingest stream info %q empty", desired.Name)
	}
	if !ingestStreamConfigMatches(info.Config, desired) {
		if _, updateErr := js.UpdateStream(&desired, opts...); updateErr != nil {
			return fmt.Errorf("update ingest stream %q: %w", desired.Name, updateErr)
		}
	}
	return nil
}

func desiredIngestStreamConfig(subjectCfg SubjectConfig, cfg JetStreamIngestBootstrapConfig) nats.StreamConfig {
	subjectCfg = subjectCfg.Normalized()
	cfg = cfg.normalized()
	return nats.StreamConfig{
		Name:      strings.TrimSpace(cfg.StreamName),
		Subjects:  []string{IngestWildcardSubject(subjectCfg)},
		Retention: cfg.Retention,
		MaxAge:    cfg.MaxAge,
		MaxBytes:  cfg.MaxBytes,
		Storage:   cfg.Storage,
		Replicas:  cfg.Replicas,
	}
}

func ingestStreamConfigMatches(current, desired nats.StreamConfig) bool {
	if strings.TrimSpace(current.Name) != strings.TrimSpace(desired.Name) {
		return false
	}
	if current.Retention != desired.Retention {
		return false
	}
	if current.MaxAge != desired.MaxAge {
		return false
	}
	if current.MaxBytes != desired.MaxBytes {
		return false
	}
	if current.Storage != desired.Storage {
		return false
	}
	if current.Replicas != desired.Replicas {
		return false
	}
	if len(current.Subjects) != len(desired.Subjects) {
		return false
	}
	for i := range desired.Subjects {
		if strings.TrimSpace(current.Subjects[i]) != strings.TrimSpace(desired.Subjects[i]) {
			return false
		}
	}
	return true
}
