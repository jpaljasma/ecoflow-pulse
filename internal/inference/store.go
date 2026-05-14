package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
	valkey "github.com/valkey-io/valkey-go"
)

const defaultKeyPrefix = "pulse:inference"
const defaultEnergyComparisonLocalTTL = 30 * time.Second

type Identity struct {
	DeviceID  string
	EcoflowSN string
}

func (i Identity) normalized() Identity {
	return Identity{
		DeviceID:  strings.TrimSpace(i.DeviceID),
		EcoflowSN: strings.ToUpper(strings.TrimSpace(i.EcoflowSN)),
	}
}

type DeviceContext struct {
	DeviceID     string         `json:"device_id"`
	EcoflowSN    string         `json:"ecoflow_sn,omitempty"`
	ProductName  string         `json:"product_name,omitempty"`
	Model        string         `json:"model,omitempty"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type Cursor struct {
	Seq      uint64 `json:"seq"`
	TsUnixMs int64  `json:"ts_unix_ms"`
}

type ReadModel struct {
	Identity        Identity           `json:"identity"`
	Cursor          Cursor             `json:"cursor"`
	UpdatedAtUnixMs int64              `json:"updated_at_unix_ms"`
	EnvelopeID      string             `json:"envelope_id,omitempty"`
	RawMetrics      map[string]float64 `json:"raw_metrics,omitempty"`
	Context         DeviceContext      `json:"context"`
	DeviceInsights  DeviceInsights     `json:"device_insights"`
}

type ValkeyStoreConfig struct {
	KeyPrefix                string
	EnergyComparisonLocalTTL time.Duration
	NowFn                    func() time.Time
}

func DefaultValkeyStoreConfig() ValkeyStoreConfig {
	return ValkeyStoreConfig{
		KeyPrefix: defaultKeyPrefix,
		NowFn:     time.Now,
	}
}

type Store interface {
	Reader
	ApplyEnvelope(ctx context.Context, env *envelopev1.TelemetryEnvelope, deviceCtx DeviceContext) (*ReadModel, error)
	GetReadModel(ctx context.Context, identity Identity) (*ReadModel, error)
}

type ValkeyStore struct {
	client                valkey.Client
	keyPrefix             string
	energyComparisonCache *valkeycache.Client
	nowFn                 func() time.Time
}

func NewValkeyStore(client valkey.Client, cfg ValkeyStoreConfig) (*ValkeyStore, error) {
	if client == nil {
		return nil, errors.New("valkey client is required")
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		cfg.KeyPrefix = defaultKeyPrefix
	}
	if cfg.NowFn == nil {
		cfg.NowFn = time.Now
	}
	energyCache, err := newEnergyComparisonCache(client, cfg)
	if err != nil {
		return nil, err
	}
	return &ValkeyStore{
		client:                client,
		keyPrefix:             strings.TrimSpace(cfg.KeyPrefix),
		energyComparisonCache: energyCache,
		nowFn:                 cfg.NowFn,
	}, nil
}

func (s *ValkeyStore) ApplyEnvelope(ctx context.Context, env *envelopev1.TelemetryEnvelope, deviceCtx DeviceContext) (*ReadModel, error) {
	if env == nil {
		return nil, errors.New("envelope is required")
	}
	identity := Identity{
		DeviceID:  env.GetDeviceId(),
		EcoflowSN: env.GetEcoflowSn(),
	}.normalized()
	keys, err := s.keysForIdentity(identity)
	if err != nil {
		return nil, err
	}
	existing, err := s.getByKey(ctx, keys.readModel)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if env.GetEnvelopeId() != "" && strings.EqualFold(existing.EnvelopeID, env.GetEnvelopeId()) {
			return existing, nil
		}
		if env.GetIngestedTimeUnixMs() > 0 && existing.Cursor.TsUnixMs > env.GetIngestedTimeUnixMs() {
			return existing, nil
		}
	}

	mergedMetrics := make(map[string]float64, 64)
	if existing != nil && len(existing.RawMetrics) > 0 {
		for k, v := range existing.RawMetrics {
			mergedMetrics[k] = v
		}
	}
	for k, v := range extractNumericMetrics(env.GetPayload()) {
		mergedMetrics[k] = v
	}

	now := s.nowFn().UTC()
	nowUnixMs := now.UnixMilli()
	cursorTs := env.GetIngestedTimeUnixMs()
	if cursorTs <= 0 {
		cursorTs = nowUnixMs
	}
	seq, err := s.client.Do(ctx, s.client.B().Incr().Key(keys.seq).Build()).ToInt64()
	if err != nil {
		return nil, fmt.Errorf("increment inference cursor sequence: %w", err)
	}
	if seq < 0 {
		seq = 0
	}

	if strings.TrimSpace(deviceCtx.DeviceID) == "" {
		deviceCtx.DeviceID = identity.DeviceID
	}
	if strings.TrimSpace(deviceCtx.EcoflowSN) == "" {
		deviceCtx.EcoflowSN = identity.EcoflowSN
	}
	if existing != nil {
		if deviceCtx.ProductName == "" {
			deviceCtx.ProductName = existing.Context.ProductName
		}
		if deviceCtx.Model == "" {
			deviceCtx.Model = existing.Context.Model
		}
		if len(deviceCtx.Capabilities) == 0 {
			deviceCtx.Capabilities = cloneAnyMap(existing.Context.Capabilities)
		}
		if len(deviceCtx.Metadata) == 0 {
			deviceCtx.Metadata = cloneAnyMap(existing.Context.Metadata)
		}
	}

	deviceInsights := DeriveDeviceInsights(DeriveInput{
		Now:        now,
		Identity:   identity,
		Device:     deviceCtx,
		RawMetrics: mergedMetrics,
	})
	deviceInsights.RefreshedAt = now

	readModel := &ReadModel{
		Identity:        identity,
		Cursor:          Cursor{Seq: uint64(seq), TsUnixMs: cursorTs},
		UpdatedAtUnixMs: nowUnixMs,
		EnvelopeID:      strings.TrimSpace(env.GetEnvelopeId()),
		RawMetrics:      mergedMetrics,
		Context:         cloneDeviceContext(deviceCtx),
		DeviceInsights:  deviceInsights,
	}
	encoded, err := json.Marshal(readModel)
	if err != nil {
		return nil, fmt.Errorf("marshal inference read model: %w", err)
	}
	if err := s.client.Do(ctx, s.client.B().Set().Key(keys.readModel).Value(valkey.BinaryString(encoded)).Build()).Error(); err != nil {
		return nil, fmt.Errorf("persist inference read model: %w", err)
	}
	return readModel, nil
}

func (s *ValkeyStore) GetReadModel(ctx context.Context, identity Identity) (*ReadModel, error) {
	keys, err := s.keysForIdentity(identity.normalized())
	if err != nil {
		return nil, err
	}
	return s.getByKey(ctx, keys.readModel)
}

func (s *ValkeyStore) GetDeviceInsights(ctx context.Context, deviceID string, filter Filter) (DeviceInsights, error) {
	readModel, err := s.GetReadModel(ctx, Identity{DeviceID: deviceID})
	if err != nil {
		return DeviceInsights{}, err
	}
	if readModel == nil {
		return DeviceInsights{}, nil
	}
	return filteredDeviceInsights(readModel.DeviceInsights, filter), nil
}

func (s *ValkeyStore) ListFleetInsights(ctx context.Context, deviceIDs []string, filter Filter) ([]DeviceInsights, error) {
	out := make([]DeviceInsights, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		insights, err := s.GetDeviceInsights(ctx, deviceID, filter)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(insights.DeviceID) == "" {
			insights.DeviceID = strings.TrimSpace(deviceID)
		}
		out = append(out, insights)
	}
	return out, nil
}

func filteredDeviceInsights(in DeviceInsights, filter Filter) DeviceInsights {
	out := in
	out.Insights = nil
	if len(in.Insights) == 0 {
		return out
	}
	var allowed map[Kind]struct{}
	if len(filter.Kinds) > 0 {
		allowed = make(map[Kind]struct{}, len(filter.Kinds))
		for _, kind := range filter.Kinds {
			allowed[kind] = struct{}{}
		}
	}
	limit := filter.MaxItems
	if limit <= 0 {
		limit = len(in.Insights)
	}
	out.Insights = make([]Insight, 0, min(limit, len(in.Insights)))
	for _, insight := range in.Insights {
		if allowed != nil {
			if _, ok := allowed[insight.Kind]; !ok {
				continue
			}
		}
		out.Insights = append(out.Insights, cloneInsight(insight))
		if len(out.Insights) >= limit {
			break
		}
	}
	return out
}

type readModelKeys struct {
	seq       string
	readModel string
}

func (s *ValkeyStore) keysForIdentity(identity Identity) (readModelKeys, error) {
	identity = identity.normalized()
	tag, err := insightTag(identity)
	if err != nil {
		return readModelKeys{}, err
	}
	base := fmt.Sprintf("%s:{%s}", s.keyPrefix, tag)
	return readModelKeys{
		seq:       fmt.Sprintf("%s:seq", base),
		readModel: fmt.Sprintf("%s:read_model", base),
	}, nil
}

func insightTag(identity Identity) (string, error) {
	if identity.DeviceID == "" && identity.EcoflowSN == "" {
		return "", errors.New("inference identity requires device_id or ecoflow_sn")
	}
	if identity.DeviceID != "" {
		return fmt.Sprintf("did:%s", sanitizeKeySegment(identity.DeviceID)), nil
	}
	return fmt.Sprintf("sn:%s", sanitizeKeySegment(identity.EcoflowSN)), nil
}

func (s *ValkeyStore) getByKey(ctx context.Context, key string) (*ReadModel, error) {
	raw, err := s.client.Do(ctx, s.client.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("read inference key %q: %w", key, err)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var readModel ReadModel
	if err := json.Unmarshal([]byte(raw), &readModel); err != nil {
		return nil, fmt.Errorf("decode inference read model: %w", err)
	}
	if readModel.RawMetrics == nil {
		readModel.RawMetrics = map[string]float64{}
	}
	readModel.Context = cloneDeviceContext(readModel.Context)
	readModel.DeviceInsights = cloneDeviceInsights(readModel.DeviceInsights)
	return &readModel, nil
}

func cloneDeviceContext(in DeviceContext) DeviceContext {
	return DeviceContext{
		DeviceID:     in.DeviceID,
		EcoflowSN:    in.EcoflowSN,
		ProductName:  in.ProductName,
		Model:        in.Model,
		Capabilities: cloneAnyMap(in.Capabilities),
		Metadata:     cloneAnyMap(in.Metadata),
	}
}

func cloneDeviceInsights(in DeviceInsights) DeviceInsights {
	out := in
	out.Insights = make([]Insight, 0, len(in.Insights))
	for _, insight := range in.Insights {
		out.Insights = append(out.Insights, cloneInsight(insight))
	}
	return out
}

func cloneInsight(in Insight) Insight {
	out := in
	out.Tags = append([]string(nil), in.Tags...)
	out.Evidence = make([]Evidence, 0, len(in.Evidence))
	for _, evidence := range in.Evidence {
		out.Evidence = append(out.Evidence, Evidence{
			Source:  evidence.Source,
			Summary: evidence.Summary,
			Metrics: cloneAnyMap(evidence.Metrics),
		})
	}
	out.Actions = make([]Action, 0, len(in.Actions))
	for _, action := range in.Actions {
		out.Actions = append(out.Actions, Action{
			Kind:   action.Kind,
			Label:  action.Label,
			Target: action.Target,
			Params: cloneAnyMap(action.Params),
		})
	}
	out.Attributes = cloneAnyMap(in.Attributes)
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneAnyMap(v)
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = cloneAny(v[i])
		}
		return out
	default:
		return v
	}
}

func sanitizeKeySegment(in string) string {
	clean := strings.TrimSpace(in)
	clean = strings.ReplaceAll(clean, "{", "_")
	clean = strings.ReplaceAll(clean, "}", "_")
	clean = strings.ReplaceAll(clean, " ", "_")
	return clean
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
