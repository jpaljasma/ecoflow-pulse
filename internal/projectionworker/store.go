package projectionworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	valkey "github.com/valkey-io/valkey-go"
)

const (
	defaultSnapshotKeyPrefix = "pulse:projection"
)

type ValkeySnapshotStoreConfig struct {
	KeyPrefix string
	NowFn     func() time.Time
}

func DefaultValkeySnapshotStoreConfig() ValkeySnapshotStoreConfig {
	return ValkeySnapshotStoreConfig{
		KeyPrefix: defaultSnapshotKeyPrefix,
		NowFn:     time.Now,
	}
}

type LiveSnapshot struct {
	DeviceID        string                `json:"device_id"`
	EcoflowSN       string                `json:"ecoflow_sn"`
	CursorSeq       uint64                `json:"cursor_seq"`
	CursorTsUnixMs  int64                 `json:"cursor_ts_unix_ms"`
	UpdatedAtUnixMs int64                 `json:"updated_at_unix_ms"`
	EnvelopeID      string                `json:"envelope_id"`
	MessageID       string                `json:"message_id"`
	TypeCode        string                `json:"type_code"`
	Shard           uint32                `json:"shard"`
	ShardCount      uint32                `json:"shard_count"`
	SourceKind      envelopev1.SourceKind `json:"source_kind"`
	Metrics         map[string]float64    `json:"metrics"`
}

type SnapshotStore interface {
	ApplyEnvelope(ctx context.Context, env *envelopev1.TelemetryEnvelope) (*LiveSnapshot, error)
	GetSnapshot(ctx context.Context, deviceID string, ecoflowSN string) (*LiveSnapshot, error)
}

type ValkeySnapshotStore struct {
	client    valkey.Client
	keyPrefix string
	nowFn     func() time.Time
}

func NewValkeySnapshotStore(client valkey.Client, cfg ValkeySnapshotStoreConfig) (*ValkeySnapshotStore, error) {
	if client == nil {
		return nil, errors.New("valkey client is required")
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		cfg.KeyPrefix = defaultSnapshotKeyPrefix
	}
	if cfg.NowFn == nil {
		cfg.NowFn = time.Now
	}
	return &ValkeySnapshotStore{
		client:    client,
		keyPrefix: strings.TrimSpace(cfg.KeyPrefix),
		nowFn:     cfg.NowFn,
	}, nil
}

func (s *ValkeySnapshotStore) ApplyEnvelope(ctx context.Context, env *envelopev1.TelemetryEnvelope) (*LiveSnapshot, error) {
	if env == nil {
		return nil, errors.New("envelope is required")
	}
	keys, err := s.keysForEnvelope(env)
	if err != nil {
		return nil, err
	}
	existing, err := s.getBySnapshotKey(ctx, keys.snapshot)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if env.GetEnvelopeId() != "" && strings.EqualFold(existing.EnvelopeID, env.GetEnvelopeId()) {
			return existing, nil
		}
		if env.GetIngestedTimeUnixMs() > 0 && existing.CursorTsUnixMs > env.GetIngestedTimeUnixMs() {
			return existing, nil
		}
	}

	mergedMetrics := make(map[string]float64, 64)
	if existing != nil && len(existing.Metrics) > 0 {
		for k, v := range existing.Metrics {
			mergedMetrics[k] = v
		}
	}
	incoming := extractNumericMetrics(env.GetPayload())
	for k, v := range incoming {
		mergedMetrics[k] = v
	}

	seq, err := s.client.Do(ctx, s.client.B().Incr().Key(keys.seq).Build()).ToInt64()
	if err != nil {
		return nil, fmt.Errorf("increment snapshot cursor sequence: %w", err)
	}
	if seq < 0 {
		seq = 0
	}

	nowUnixMs := s.nowFn().UTC().UnixMilli()
	cursorTs := env.GetIngestedTimeUnixMs()
	if cursorTs <= 0 {
		cursorTs = nowUnixMs
	}
	deviceID := strings.TrimSpace(env.GetDeviceId())
	ecoflowSN := strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn()))
	if deviceID == "" && existing != nil {
		deviceID = existing.DeviceID
	}
	if ecoflowSN == "" && existing != nil {
		ecoflowSN = existing.EcoflowSN
	}

	snapshot := &LiveSnapshot{
		DeviceID:        deviceID,
		EcoflowSN:       ecoflowSN,
		CursorSeq:       uint64(seq),
		CursorTsUnixMs:  cursorTs,
		UpdatedAtUnixMs: nowUnixMs,
		EnvelopeID:      strings.TrimSpace(env.GetEnvelopeId()),
		MessageID:       strings.TrimSpace(env.GetMessageId()),
		TypeCode:        strings.TrimSpace(env.GetTypeCode()),
		Shard:           env.GetShard(),
		ShardCount:      env.GetShardCount(),
		SourceKind:      env.GetSourceKind(),
		Metrics:         mergedMetrics,
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal live snapshot: %w", err)
	}
	if err := s.client.Do(
		ctx,
		s.client.B().Set().Key(keys.snapshot).Value(valkey.BinaryString(encoded)).Build(),
	).Error(); err != nil {
		return nil, fmt.Errorf("persist live snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *ValkeySnapshotStore) GetSnapshot(ctx context.Context, deviceID string, ecoflowSN string) (*LiveSnapshot, error) {
	keys, err := s.keysForIdentity(deviceID, ecoflowSN)
	if err != nil {
		return nil, err
	}
	return s.getBySnapshotKey(ctx, keys.snapshot)
}

func (s *ValkeySnapshotStore) getBySnapshotKey(ctx context.Context, key string) (*LiveSnapshot, error) {
	raw, err := s.client.Do(ctx, s.client.B().Get().Key(key).Build()).AsBytes()
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("read snapshot key %q: %w", key, err)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	var snapshot LiveSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("decode live snapshot: %w", err)
	}
	if snapshot.Metrics == nil {
		snapshot.Metrics = map[string]float64{}
	}
	return &snapshot, nil
}

type snapshotKeys struct {
	tag      string
	seq      string
	snapshot string
}

func (s *ValkeySnapshotStore) keysForEnvelope(env *envelopev1.TelemetryEnvelope) (snapshotKeys, error) {
	return s.keysForIdentity(env.GetDeviceId(), env.GetEcoflowSn())
}

func (s *ValkeySnapshotStore) keysForIdentity(deviceID string, ecoflowSN string) (snapshotKeys, error) {
	tag, err := snapshotTag(deviceID, ecoflowSN)
	if err != nil {
		return snapshotKeys{}, err
	}
	base := fmt.Sprintf("%s:{%s}", s.keyPrefix, tag)
	return snapshotKeys{
		tag:      tag,
		seq:      fmt.Sprintf("%s:seq", base),
		snapshot: fmt.Sprintf("%s:snapshot", base),
	}, nil
}

func snapshotTag(deviceID string, ecoflowSN string) (string, error) {
	did := strings.TrimSpace(deviceID)
	sn := strings.TrimSpace(ecoflowSN)
	if did == "" && sn == "" {
		return "", errors.New("snapshot identity requires device_id or ecoflow_sn")
	}
	if did != "" {
		return fmt.Sprintf("did:%s", sanitizeKeySegment(did)), nil
	}
	return fmt.Sprintf("sn:%s", sanitizeKeySegment(strings.ToUpper(sn))), nil
}

func sanitizeKeySegment(in string) string {
	clean := strings.TrimSpace(in)
	clean = strings.ReplaceAll(clean, "{", "_")
	clean = strings.ReplaceAll(clean, "}", "_")
	clean = strings.ReplaceAll(clean, " ", "_")
	return clean
}
