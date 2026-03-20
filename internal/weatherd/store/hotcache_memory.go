package store

import (
	"context"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

type MemoryHotCache struct {
	mu    sync.RWMutex
	nowFn func() time.Time
	rows  map[string]weatherd.CachedBundle
}

func NewMemoryHotCache(nowFn func() time.Time) *MemoryHotCache {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &MemoryHotCache{
		nowFn: nowFn,
		rows:  map[string]weatherd.CachedBundle{},
	}
}

func (c *MemoryHotCache) GetForecast(_ context.Context, key string) (*weatherd.CachedBundle, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	row, ok := c.rows[key]
	if !ok {
		return nil, nil
	}
	out := row
	return &out, nil
}

func (c *MemoryHotCache) PutForecast(_ context.Context, key string, bundle weatherd.Bundle, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.nowFn().UTC()
	c.rows[key] = weatherd.CachedBundle{
		Bundle:     bundle,
		CachedAt:   now,
		StaleAfter: now.Add(ttl),
	}
	return nil
}
