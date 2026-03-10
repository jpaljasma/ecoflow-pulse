package archiveworker

import (
	"fmt"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

type recentEnvelopeDeduper struct {
	window     time.Duration
	maxEntries int
	entries    map[string]time.Time
	order      []dedupEntry
}

type dedupEntry struct {
	key       string
	expiresAt time.Time
}

func newRecentEnvelopeDeduper(window time.Duration, maxEntries int) *recentEnvelopeDeduper {
	if window <= 0 {
		window = defaultDedupWindow
	}
	if maxEntries <= 0 {
		maxEntries = defaultDedupMaxEntries
	}
	return &recentEnvelopeDeduper{
		window:     window,
		maxEntries: maxEntries,
		entries:    make(map[string]time.Time, maxEntries),
		order:      make([]dedupEntry, 0, maxEntries),
	}
}

func (d *recentEnvelopeDeduper) Add(now time.Time, key string) bool {
	if d == nil {
		return true
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return true
	}
	now = now.UTC()
	d.evictExpired(now)
	if expiresAt, ok := d.entries[key]; ok && now.Before(expiresAt) {
		return false
	}
	expiresAt := now.Add(d.window)
	d.entries[key] = expiresAt
	d.order = append(d.order, dedupEntry{key: key, expiresAt: expiresAt})
	d.evictOverflow()
	return true
}

func (d *recentEnvelopeDeduper) evictExpired(now time.Time) {
	if d == nil || len(d.order) == 0 {
		return
	}
	cut := 0
	for _, entry := range d.order {
		currentExpiresAt, ok := d.entries[entry.key]
		if !ok || currentExpiresAt != entry.expiresAt || !currentExpiresAt.After(now) {
			delete(d.entries, entry.key)
			cut++
			continue
		}
		break
	}
	if cut > 0 {
		d.order = append([]dedupEntry(nil), d.order[cut:]...)
	}
}

func (d *recentEnvelopeDeduper) evictOverflow() {
	if d == nil || len(d.entries) <= d.maxEntries {
		return
	}
	remove := len(d.entries) - d.maxEntries
	cut := 0
	for _, entry := range d.order {
		currentExpiresAt, ok := d.entries[entry.key]
		if !ok || currentExpiresAt != entry.expiresAt {
			cut++
			continue
		}
		delete(d.entries, entry.key)
		cut++
		remove--
		if remove <= 0 {
			break
		}
	}
	if cut > 0 {
		d.order = append([]dedupEntry(nil), d.order[cut:]...)
	}
}

func archiveDedupKey(env *envelopev1.TelemetryEnvelope) string {
	if env == nil {
		return ""
	}
	if envelopeID := strings.TrimSpace(env.GetEnvelopeId()); envelopeID != "" {
		return "env:" + envelopeID
	}
	deviceID := strings.TrimSpace(env.GetDeviceId())
	messageID := strings.TrimSpace(env.GetMessageId())
	if deviceID != "" && messageID != "" {
		return fmt.Sprintf("msg:%s:%s", deviceID, messageID)
	}
	return ""
}
