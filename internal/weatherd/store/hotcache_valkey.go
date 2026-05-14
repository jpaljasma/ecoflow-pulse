package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	valkey "github.com/valkey-io/valkey-go"
)

const defaultHotCacheKeyPrefix = "pulse:weather"
const defaultHotCacheLocalTTL = 5 * time.Second

type ValkeyHotCache struct {
	cache     *valkeycache.Client
	keyPrefix string
	nowFn     func() time.Time
}

func NewValkeyHotCache(client valkey.Client, keyPrefix string, nowFn func() time.Time) (*ValkeyHotCache, error) {
	if client == nil {
		return nil, errors.New("valkey client is required")
	}
	if strings.TrimSpace(keyPrefix) == "" {
		keyPrefix = defaultHotCacheKeyPrefix
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	prefix, namespace := splitCacheKeyPrefix(keyPrefix)
	cache, err := valkeycache.New(client, valkeycache.Options{
		Prefix:          prefix,
		Namespace:       namespace,
		ContentType:     "application/json",
		DefaultLocalTTL: defaultHotCacheLocalTTL,
		Now:             nowFn,
	})
	if err != nil {
		return nil, err
	}
	return &ValkeyHotCache{
		cache:     cache,
		keyPrefix: strings.TrimSpace(keyPrefix),
		nowFn:     nowFn,
	}, nil
}

func (c *ValkeyHotCache) GetForecast(ctx context.Context, key string) (*weatherd.CachedBundle, error) {
	var out weatherd.CachedBundle
	ok, err := c.cache.GetJSON(ctx, c.hotKey(key), &out, valkeycache.ReadOptions{})
	if err != nil {
		return nil, fmt.Errorf("read weather hot cache: %w", err)
	}
	if !ok {
		return nil, nil
	}
	return &out, nil
}

func (c *ValkeyHotCache) PutForecast(ctx context.Context, key string, bundle weatherd.Bundle, ttl time.Duration) error {
	now := c.nowFn().UTC()
	row := weatherd.CachedBundle{
		Bundle:     bundle,
		CachedAt:   now,
		StaleAfter: now.Add(ttl),
	}
	if err := c.cache.SetJSON(ctx, c.hotKey(key), row, valkeycache.SetOptions{TTL: ttl}); err != nil {
		return fmt.Errorf("write weather hot cache: %w", err)
	}
	return nil
}

func (c *ValkeyHotCache) hotKey(key string) string {
	return c.cache.Key(sanitizeKeySegment(key), "forecast")
}

func sanitizeKeySegment(in string) string {
	clean := strings.TrimSpace(in)
	clean = strings.ReplaceAll(clean, "{", "_")
	clean = strings.ReplaceAll(clean, "}", "_")
	clean = strings.ReplaceAll(clean, " ", "_")
	return clean
}

func splitCacheKeyPrefix(keyPrefix string) (string, string) {
	keyPrefix = strings.Trim(strings.TrimSpace(keyPrefix), ":")
	if keyPrefix == "" {
		keyPrefix = defaultHotCacheKeyPrefix
	}
	prefix, namespace, ok := strings.Cut(keyPrefix, ":")
	if !ok {
		return prefix, "weather"
	}
	return prefix, namespace
}
