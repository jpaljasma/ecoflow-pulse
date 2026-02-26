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
	defaultGapRepairStreamName      = "PULSE_TELEMETRY_GAPREPAIR"
	defaultGapRepairStreamReplicas  = 3
	defaultGapRepairStreamMaxAge    = 24 * time.Hour
	defaultGapRepairStreamStorage   = nats.FileStorage
	defaultGapRepairStreamRetention = nats.WorkQueuePolicy
)

// JetStreamGapRepairBootstrapConfig controls gap-repair stream bootstrap.
type JetStreamGapRepairBootstrapConfig struct {
	Enabled bool
	// StreamName is the target stream for gap-repair subjects.
	StreamName string
	// Replicas controls stream replica factor.
	Replicas int
	// MaxAge bounds retention horizon for queued gap-repair jobs.
	MaxAge time.Duration
	// MaxBytes bounds aggregate stream bytes. 0 means broker default.
	MaxBytes int64
	// Storage controls file/memory persistence.
	Storage nats.StorageType
	// Retention controls stream retention behavior.
	Retention nats.RetentionPolicy
}

func DefaultJetStreamGapRepairBootstrapConfig() JetStreamGapRepairBootstrapConfig {
	return JetStreamGapRepairBootstrapConfig{
		Enabled:    true,
		StreamName: defaultGapRepairStreamName,
		Replicas:   defaultGapRepairStreamReplicas,
		MaxAge:     defaultGapRepairStreamMaxAge,
		MaxBytes:   0,
		Storage:    defaultGapRepairStreamStorage,
		Retention:  defaultGapRepairStreamRetention,
	}
}

func (c JetStreamGapRepairBootstrapConfig) normalized() JetStreamGapRepairBootstrapConfig {
	out := c
	if strings.TrimSpace(out.StreamName) == "" {
		out.StreamName = defaultGapRepairStreamName
	}
	if out.Replicas <= 0 {
		out.Replicas = defaultGapRepairStreamReplicas
	}
	if out.MaxAge <= 0 {
		out.MaxAge = defaultGapRepairStreamMaxAge
	}
	if out.Storage == nats.StorageType(0) {
		out.Storage = defaultGapRepairStreamStorage
	}
	if out.Retention == nats.RetentionPolicy(0) {
		out.Retention = defaultGapRepairStreamRetention
	}
	return out
}

func EnsureJetStreamGapRepairStream(
	ctx context.Context,
	conn *nats.Conn,
	subjectCfg SubjectConfig,
	cfg JetStreamGapRepairBootstrapConfig,
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
	return ensureJetStreamGapRepairStreamWithManager(ctx, js, subjectCfg, cfg)
}

func ensureJetStreamGapRepairStreamWithManager(
	ctx context.Context,
	js jetStreamManager,
	subjectCfg SubjectConfig,
	cfg JetStreamGapRepairBootstrapConfig,
) error {
	if js == nil {
		return errors.New("jetstream manager is required")
	}
	subjectCfg = subjectCfg.Normalized()
	cfg = cfg.normalized()
	desired := desiredGapRepairStreamConfig(subjectCfg, cfg)
	opts := []nats.JSOpt{nats.Context(ctx)}

	info, err := js.StreamInfo(desired.Name, opts...)
	if err != nil {
		if errors.Is(err, nats.ErrStreamNotFound) {
			if _, addErr := js.AddStream(&desired, opts...); addErr != nil {
				return fmt.Errorf("add gap-repair stream %q: %w", desired.Name, addErr)
			}
			return nil
		}
		return fmt.Errorf("read gap-repair stream info %q: %w", desired.Name, err)
	}
	if info == nil || info.Config.Name == "" {
		return fmt.Errorf("gap-repair stream info %q empty", desired.Name)
	}
	if !gapRepairStreamConfigMatches(info.Config, desired) {
		if _, updateErr := js.UpdateStream(&desired, opts...); updateErr != nil {
			return fmt.Errorf("update gap-repair stream %q: %w", desired.Name, updateErr)
		}
	}
	return nil
}

func desiredGapRepairStreamConfig(subjectCfg SubjectConfig, cfg JetStreamGapRepairBootstrapConfig) nats.StreamConfig {
	subjectCfg = subjectCfg.Normalized()
	cfg = cfg.normalized()
	return nats.StreamConfig{
		Name:      strings.TrimSpace(cfg.StreamName),
		Subjects:  []string{GapRepairWildcardSubject(subjectCfg)},
		Retention: cfg.Retention,
		MaxAge:    cfg.MaxAge,
		MaxBytes:  cfg.MaxBytes,
		Storage:   cfg.Storage,
		Replicas:  cfg.Replicas,
	}
}

func gapRepairStreamConfigMatches(current, desired nats.StreamConfig) bool {
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
