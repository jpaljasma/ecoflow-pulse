package archiveworker

import (
	"strings"
	"sync"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/envelopededup"
)

type recentEnvelopeDeduper struct {
	window     time.Duration
	maxEntries int
	mu         sync.Mutex
	entries    map[string]time.Time
	order      []dedupEntry
	head       int
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
	d.mu.Lock()
	defer d.mu.Unlock()
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
	if d == nil || d.head >= len(d.order) {
		return
	}
	for d.head < len(d.order) {
		entry := d.order[d.head]
		currentExpiresAt, ok := d.entries[entry.key]
		if !ok || currentExpiresAt != entry.expiresAt || !currentExpiresAt.After(now) {
			delete(d.entries, entry.key)
			d.head++
			continue
		}
		break
	}
	d.compact()
}

func (d *recentEnvelopeDeduper) evictOverflow() {
	if d == nil || len(d.entries) <= d.maxEntries {
		return
	}
	remove := len(d.entries) - d.maxEntries
	for d.head < len(d.order) && remove > 0 {
		entry := d.order[d.head]
		currentExpiresAt, ok := d.entries[entry.key]
		if !ok || currentExpiresAt != entry.expiresAt {
			d.head++
			continue
		}
		delete(d.entries, entry.key)
		d.head++
		remove--
	}
	d.compact()
}

func (d *recentEnvelopeDeduper) compact() {
	if d == nil || d.head == 0 {
		return
	}
	if d.head < cap(d.order)/2 && len(d.order)-d.head <= d.maxEntries {
		return
	}
	remaining := len(d.order) - d.head
	if remaining <= 0 {
		d.order = d.order[:0]
		d.head = 0
		return
	}
	copy(d.order, d.order[d.head:])
	d.order = d.order[:remaining]
	d.head = 0
}

func archiveDedupKey(env *envelopev1.TelemetryEnvelope) string {
	return strings.TrimSpace(envelopededup.Key(env))
}
