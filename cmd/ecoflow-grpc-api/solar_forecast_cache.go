package main

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	solarforecastv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/solarforecast/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"golang.org/x/sync/singleflight"
	"google.golang.org/protobuf/proto"
)

const defaultSolarForecastOutlookCacheTTL = 45 * time.Second

type solarForecastOutlookCache struct {
	ttl    time.Duration
	nowFn  func() time.Time
	group  singleflight.Group
	mu     sync.Mutex
	values map[string]solarForecastOutlookCacheEntry
}

type solarForecastOutlookCacheEntry struct {
	response  *solarforecastv1.GetSolarOutlookResponse
	expiresAt time.Time
}

func newSolarForecastOutlookCache(ttl time.Duration, nowFn func() time.Time) *solarForecastOutlookCache {
	if ttl <= 0 {
		ttl = defaultSolarForecastOutlookCacheTTL
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	return &solarForecastOutlookCache{
		ttl:    ttl,
		nowFn:  nowFn,
		values: map[string]solarForecastOutlookCacheEntry{},
	}
}

func (c *solarForecastOutlookCache) Get(ctx context.Context, key string, loader func(context.Context) (*solarforecastv1.GetSolarOutlookResponse, error)) (*solarforecastv1.GetSolarOutlookResponse, error) {
	if c == nil {
		return loader(ctx)
	}
	if cached := c.lookup(key); cached != nil {
		return cached, nil
	}
	value, err, _ := c.group.Do(key, func() (any, error) {
		if cached := c.lookup(key); cached != nil {
			return cached, nil
		}
		resp, err := loader(ctx)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, nil
		}
		c.store(key, resp)
		return proto.Clone(resp).(*solarforecastv1.GetSolarOutlookResponse), nil
	})
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	return value.(*solarforecastv1.GetSolarOutlookResponse), nil
}

func (c *solarForecastOutlookCache) lookup(key string) *solarforecastv1.GetSolarOutlookResponse {
	if c == nil {
		return nil
	}
	now := c.nowFn()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.values[key]
	if !ok {
		return nil
	}
	if !now.Before(entry.expiresAt) {
		delete(c.values, key)
		return nil
	}
	return proto.Clone(entry.response).(*solarforecastv1.GetSolarOutlookResponse)
}

func (c *solarForecastOutlookCache) store(key string, resp *solarforecastv1.GetSolarOutlookResponse) {
	if c == nil || resp == nil {
		return
	}
	now := c.nowFn()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = solarForecastOutlookCacheEntry{
		response:  proto.Clone(resp).(*solarforecastv1.GetSolarOutlookResponse),
		expiresAt: now.Add(c.ttl),
	}
}

func solarForecastOutlookCacheKey(req weatherd.Request, scopeMode, requestedDeviceID string, visibleDeviceIDs []string) string {
	req = req.Normalized()
	ids := normalizedSolarForecastDeviceIDs(visibleDeviceIDs)
	var b strings.Builder
	b.Grow(192 + len(ids)*12)
	b.WriteString("lat=")
	b.WriteString(strconv.FormatFloat(req.Latitude, 'g', -1, 64))
	b.WriteString("|lon=")
	b.WriteString(strconv.FormatFloat(req.Longitude, 'g', -1, 64))
	b.WriteString("|tz=")
	b.WriteString(normalizeCacheString(req.Timezone))
	b.WriteString("|tilt=")
	b.WriteString(formatFloatPointer(req.PanelTiltDegrees))
	b.WriteString("|az=")
	b.WriteString(formatFloatPointer(req.PanelAzimuthDegrees))
	b.WriteString("|scope=")
	b.WriteString(normalizeCacheString(scopeMode))
	b.WriteString("|device=")
	b.WriteString(normalizeCacheString(requestedDeviceID))
	b.WriteString("|visible=")
	for idx, id := range ids {
		if idx > 0 {
			b.WriteByte(',')
		}
		b.WriteString(id)
	}
	return b.String()
}

func normalizedSolarForecastDeviceIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func normalizeCacheString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func formatFloatPointer(value *float64) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatFloat(*value, 'g', -1, 64)
}
