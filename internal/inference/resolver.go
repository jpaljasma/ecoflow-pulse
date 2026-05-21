package inference

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
)

type DeviceContextResolver interface {
	ResolveDeviceContext(ctx context.Context, deviceID string) (DeviceContext, error)
}

type ControlPlaneResolverConfig struct {
	CacheTTL time.Duration
	Cache    *valkeycache.Client
	NowFn    func() time.Time
}

func DefaultControlPlaneResolverConfig() ControlPlaneResolverConfig {
	return ControlPlaneResolverConfig{
		CacheTTL: time.Minute,
		NowFn:    time.Now,
	}
}

type ControlPlaneResolver struct {
	store    controlplane.Store
	cacheTTL time.Duration
	valkey   *valkeycache.Client
	nowFn    func() time.Time

	mu    sync.RWMutex
	cache map[string]resolverCacheEntry
}

type resolverCacheEntry struct {
	device    DeviceContext
	expiresAt time.Time
}

func NewControlPlaneResolver(store controlplane.Store, cfg ControlPlaneResolverConfig) (*ControlPlaneResolver, error) {
	if store == nil {
		return nil, fmt.Errorf("control-plane store is required")
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = time.Minute
	}
	if cfg.NowFn == nil {
		cfg.NowFn = time.Now
	}
	return &ControlPlaneResolver{
		store:    store,
		cacheTTL: cfg.CacheTTL,
		valkey:   cfg.Cache,
		nowFn:    cfg.NowFn,
		cache:    map[string]resolverCacheEntry{},
	}, nil
}

func (r *ControlPlaneResolver) ResolveDeviceContext(ctx context.Context, deviceID string) (DeviceContext, error) {
	now := r.nowFn().UTC()
	r.mu.RLock()
	if entry, ok := r.cache[deviceID]; ok && now.Before(entry.expiresAt) {
		r.mu.RUnlock()
		return cloneDeviceContext(entry.device), nil
	}
	r.mu.RUnlock()
	if r.valkey != nil {
		var cached DeviceContext
		ok, err := r.valkey.GetJSON(ctx, r.valkey.Key("device-context", "device_id="+deviceID), &cached, valkeycache.ReadOptions{})
		if err == nil && ok {
			r.memoizeDeviceContext(deviceID, cached, now)
			return cloneDeviceContext(cached), nil
		}
	}

	row, err := r.store.GetProviderDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return DeviceContext{}, err
	}
	device := DeviceContext{
		DeviceID:     row.DeviceID,
		EcoflowSN:    row.CanonicalSN,
		ProductName:  row.ProductName,
		Model:        row.Model,
		Capabilities: cloneAnyMap(row.Capabilities),
		Metadata:     cloneAnyMap(row.Metadata),
	}
	r.memoizeDeviceContext(deviceID, device, now)
	if r.valkey != nil {
		_ = r.valkey.SetJSON(ctx, r.valkey.Key("device-context", "device_id="+deviceID), device, valkeycache.SetOptions{TTL: r.cacheTTL})
	}
	return device, nil
}

func (r *ControlPlaneResolver) memoizeDeviceContext(deviceID string, device DeviceContext, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[deviceID] = resolverCacheEntry{
		device:    cloneDeviceContext(device),
		expiresAt: now.Add(r.cacheTTL),
	}
}
