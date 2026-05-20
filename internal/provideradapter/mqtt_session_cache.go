package provideradapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ankersolix"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
)

const (
	defaultProviderSessionIdleTTL  = 30 * time.Minute
	defaultProviderSessionMaxAge   = 6 * time.Hour
	defaultProviderSessionLocalTTL = 5 * time.Second
)

type MQTTSessionCacheConfig struct {
	IdleTTL  time.Duration
	MaxAge   time.Duration
	LocalTTL time.Duration
	Now      func() time.Time
}

type MQTTSessionCache struct {
	cache    *valkeycache.Client
	idleTTL  time.Duration
	maxAge   time.Duration
	localTTL time.Duration
	now      func() time.Time
}

func NewMQTTSessionCache(cache *valkeycache.Client, cfg MQTTSessionCacheConfig) *MQTTSessionCache {
	if cache == nil {
		return nil
	}
	if cfg.IdleTTL <= 0 {
		cfg.IdleTTL = defaultProviderSessionIdleTTL
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = defaultProviderSessionMaxAge
	}
	if cfg.LocalTTL <= 0 {
		cfg.LocalTTL = defaultProviderSessionLocalTTL
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &MQTTSessionCache{
		cache:    cache,
		idleTTL:  cfg.IdleTTL,
		maxAge:   cfg.MaxAge,
		localTTL: cfg.LocalTTL,
		now:      cfg.Now,
	}
}

func (c *MQTTSessionCache) GetPecron(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (pecron.MQTTSession, bool) {
	if c == nil || c.cache == nil {
		return pecron.MQTTSession{}, false
	}
	var payload pecronMQTTSessionPayload
	if !c.getJSON(ctx, c.key("pecron", credential.ID, providerDeviceID), &payload) {
		return pecron.MQTTSession{}, false
	}
	clientIDSeed := strings.TrimSpace(payload.ClientIDSeed)
	if clientIDSeed == "" {
		clientIDSeed = "pecron"
	}
	return pecron.MQTTSession{
		Address:   payload.Address,
		Addresses: append([]string(nil), payload.Addresses...),
		Path:      payload.Path,
		Token:     payload.Token,
		ClientID:  fmt.Sprintf("qu_%s_%d", clientIDSeed, c.now().UTC().UnixMilli()),
		Topics:    append([]string(nil), payload.Topics...),
		Ref:       payload.Ref,
	}, true
}

func (c *MQTTSessionCache) PutPecron(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string, session pecron.MQTTSession, clientIDSeed string, providerExpiresAt time.Time) {
	if c == nil || c.cache == nil {
		return
	}
	hardExpiresAt := c.hardExpiresAt(providerExpiresAt)
	_ = c.setJSON(ctx, c.key("pecron", credential.ID, providerDeviceID), pecronMQTTSessionPayload{
		Address:       session.Address,
		Addresses:     append([]string(nil), session.Addresses...),
		Path:          session.Path,
		Token:         session.Token,
		ClientIDSeed:  strings.TrimSpace(clientIDSeed),
		Topics:        append([]string(nil), session.Topics...),
		Ref:           session.Ref,
		HardExpiresAt: hardExpiresAt,
	}, hardExpiresAt)
}

func (c *MQTTSessionCache) GetAnkerSolix(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (AnkerSolixMQTTSession, bool) {
	if c == nil || c.cache == nil {
		return AnkerSolixMQTTSession{}, false
	}
	var payload ankerSolixMQTTSessionPayload
	if !c.getJSON(ctx, c.key("anker-solix", credential.ID, providerDeviceID), &payload) {
		return AnkerSolixMQTTSession{}, false
	}
	tlsConfig, err := payload.Info.TLSConfig()
	if err != nil {
		return AnkerSolixMQTTSession{}, false
	}
	return AnkerSolixMQTTSession{
		Info:         payload.Info,
		DeviceRef:    payload.DeviceRef,
		Address:      payload.Address,
		Addresses:    append([]string(nil), payload.Addresses...),
		ClientID:     payload.ClientID,
		Topics:       append([]string(nil), payload.Topics...),
		PublishTopic: payload.PublishTopic,
		TLSConfig:    tlsConfig,
	}, true
}

func (c *MQTTSessionCache) PutAnkerSolix(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string, session AnkerSolixMQTTSession) {
	if c == nil || c.cache == nil {
		return
	}
	hardExpiresAt := c.hardExpiresAt(time.Time{})
	_ = c.setJSON(ctx, c.key("anker-solix", credential.ID, providerDeviceID), ankerSolixMQTTSessionPayload{
		Info:          session.Info,
		DeviceRef:     session.DeviceRef,
		Address:       session.Address,
		Addresses:     append([]string(nil), session.Addresses...),
		ClientID:      session.ClientID,
		Topics:        append([]string(nil), session.Topics...),
		PublishTopic:  session.PublishTopic,
		HardExpiresAt: hardExpiresAt,
	}, hardExpiresAt)
}

func (c *MQTTSessionCache) GetEcoFlowCertification(ctx context.Context, provider string, credential controlplane.ProviderCredential, providerDeviceID string) (ecoflow.GeneralInfoMQTTCertification, bool) {
	if c == nil || c.cache == nil {
		return ecoflow.GeneralInfoMQTTCertification{}, false
	}
	var payload ecoFlowMQTTCertificationPayload
	if !c.getJSON(ctx, c.key(provider, credential.ID, providerDeviceID), &payload) {
		return ecoflow.GeneralInfoMQTTCertification{}, false
	}
	return payload.Certification, true
}

func (c *MQTTSessionCache) PutEcoFlowCertification(ctx context.Context, provider string, credential controlplane.ProviderCredential, providerDeviceID string, cert ecoflow.GeneralInfoMQTTCertification) {
	if c == nil || c.cache == nil {
		return
	}
	hardExpiresAt := c.hardExpiresAt(time.Time{})
	_ = c.setJSON(ctx, c.key(provider, credential.ID, providerDeviceID), ecoFlowMQTTCertificationPayload{
		Certification: cert,
		HardExpiresAt: hardExpiresAt,
	}, hardExpiresAt)
}

func (c *MQTTSessionCache) getJSON(ctx context.Context, key string, out cacheSessionPayload) bool {
	ok, err := c.cache.GetJSON(ctx, key, out, valkeycache.ReadOptions{LocalTTL: c.localTTL})
	if err != nil || !ok {
		return false
	}
	if !out.HardExpiresAtValue().IsZero() && !c.now().UTC().Before(out.HardExpiresAtValue()) {
		return false
	}
	ttl, ok := valkeycache.SlidingTTL(c.now().UTC(), c.idleTTL, out.HardExpiresAtValue())
	if !ok {
		return false
	}
	_ = c.cache.Touch(ctx, key, ttl)
	return true
}

func (c *MQTTSessionCache) setJSON(ctx context.Context, key string, payload cacheSessionPayload, hardExpiresAt time.Time) error {
	ttl, ok := valkeycache.SlidingTTL(c.now().UTC(), c.idleTTL, hardExpiresAt)
	if !ok {
		return valkeycache.ErrSessionExpired
	}
	return c.cache.SetJSON(ctx, key, payload, valkeycache.SetOptions{
		TTL:     ttl,
		Encrypt: true,
	})
}

func (c *MQTTSessionCache) key(provider, credentialID, providerDeviceID string) string {
	provider = controlplane.NormalizeProvider(provider)
	if provider == "" {
		provider = "provider"
	}
	partition := provider + "-" + strings.TrimSpace(credentialID)
	return c.cache.Key(partition, "provider="+provider, "credential_id="+credentialID, "provider_device_id="+providerDeviceID)
}

func (c *MQTTSessionCache) hardExpiresAt(providerExpiresAt time.Time) time.Time {
	maxExpiresAt := c.now().UTC().Add(c.maxAge)
	if providerExpiresAt.IsZero() {
		return maxExpiresAt
	}
	providerExpiresAt = providerExpiresAt.UTC()
	if providerExpiresAt.Before(maxExpiresAt) {
		return providerExpiresAt
	}
	return maxExpiresAt
}

type cacheSessionPayload interface {
	HardExpiresAtValue() time.Time
}

type pecronMQTTSessionPayload struct {
	Address       string           `json:"address"`
	Addresses     []string         `json:"addresses,omitempty"`
	Path          string           `json:"path,omitempty"`
	Token         string           `json:"token"`
	ClientIDSeed  string           `json:"client_id_seed"`
	Topics        []string         `json:"topics,omitempty"`
	Ref           pecron.DeviceRef `json:"ref"`
	HardExpiresAt time.Time        `json:"hard_expires_at"`
}

func (p pecronMQTTSessionPayload) HardExpiresAtValue() time.Time { return p.HardExpiresAt }

type ankerSolixMQTTSessionPayload struct {
	Info          ankersolix.MQTTSessionInfo `json:"info"`
	DeviceRef     ankersolix.DeviceRef       `json:"device_ref"`
	Address       string                     `json:"address"`
	Addresses     []string                   `json:"addresses,omitempty"`
	ClientID      string                     `json:"client_id"`
	Topics        []string                   `json:"topics,omitempty"`
	PublishTopic  string                     `json:"publish_topic"`
	HardExpiresAt time.Time                  `json:"hard_expires_at"`
}

func (p ankerSolixMQTTSessionPayload) HardExpiresAtValue() time.Time { return p.HardExpiresAt }

type ecoFlowMQTTCertificationPayload struct {
	Certification ecoflow.GeneralInfoMQTTCertification `json:"certification"`
	HardExpiresAt time.Time                            `json:"hard_expires_at"`
}

func (p ecoFlowMQTTCertificationPayload) HardExpiresAtValue() time.Time { return p.HardExpiresAt }
