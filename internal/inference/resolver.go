package inference

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

type DeviceContextResolver interface {
	ResolveDeviceContext(ctx context.Context, deviceID string) (DeviceContext, error)
}

type ControlPlaneResolverConfig struct {
	CacheTTL time.Duration
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
	r.mu.Lock()
	r.cache[deviceID] = resolverCacheEntry{
		device:    cloneDeviceContext(device),
		expiresAt: now.Add(r.cacheTTL),
	}
	r.mu.Unlock()
	return device, nil
}
