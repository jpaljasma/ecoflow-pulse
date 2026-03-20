package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	valkey "github.com/valkey-io/valkey-go"
)

const defaultHotCacheKeyPrefix = "pulse:weather"

type ValkeyHotCache struct {
	client    valkey.Client
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
	return &ValkeyHotCache{
		client:    client,
		keyPrefix: strings.TrimSpace(keyPrefix),
		nowFn:     nowFn,
	}, nil
}

func (c *ValkeyHotCache) GetForecast(ctx context.Context, key string) (*weatherd.CachedBundle, error) {
	raw, err := c.client.Do(ctx, c.client.B().Get().Key(c.hotKey(key)).Build()).ToString()
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("read weather hot cache: %w", err)
	}
	var out weatherd.CachedBundle
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode weather hot cache: %w", err)
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
	encoded, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshal weather hot cache: %w", err)
	}
	if err := c.client.Do(
		ctx,
		c.client.B().Set().Key(c.hotKey(key)).Value(valkey.BinaryString(encoded)).ExSeconds(int64(ttl.Seconds())).Build(),
	).Error(); err != nil {
		return fmt.Errorf("write weather hot cache: %w", err)
	}
	return nil
}

func (c *ValkeyHotCache) hotKey(key string) string {
	return fmt.Sprintf("%s:{%s}:forecast", c.keyPrefix, sanitizeKeySegment(key))
}

func sanitizeKeySegment(in string) string {
	clean := strings.TrimSpace(in)
	clean = strings.ReplaceAll(clean, "{", "_")
	clean = strings.ReplaceAll(clean, "}", "_")
	clean = strings.ReplaceAll(clean, " ", "_")
	return clean
}
