package store

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

type MemorySnapshotStore struct {
	mu                sync.RWMutex
	nowFn             func() time.Time
	bundlesByKey      map[string][]weatherd.Bundle
	requestToKey      map[string]string
	refreshCandidates map[string]weatherd.RefreshCandidate
}

func NewMemorySnapshotStore(nowFn func() time.Time) *MemorySnapshotStore {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &MemorySnapshotStore{
		nowFn:             nowFn,
		bundlesByKey:      map[string][]weatherd.Bundle{},
		requestToKey:      map[string]string{},
		refreshCandidates: map[string]weatherd.RefreshCandidate{},
	}
}

func (s *MemorySnapshotStore) SaveForecastBundle(_ context.Context, req weatherd.Request, bundle weatherd.Bundle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := bundle.Provenance.CanonicalLocationKey
	s.bundlesByKey[key] = upsertBundleByIssuedAt(s.bundlesByKey[key], bundle)
	s.requestToKey[requestKey(req)] = key
	return nil
}

func (s *MemorySnapshotStore) LatestBundle(_ context.Context, canonicalLocationKey string) (*weatherd.Bundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.bundlesByKey[canonicalLocationKey]
	if len(rows) == 0 {
		return nil, nil
	}
	out := cloneBundle(rows[0])
	return &out, nil
}

func (s *MemorySnapshotStore) LatestBundleBefore(_ context.Context, canonicalLocationKey string, before time.Time) (*weatherd.Bundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, row := range s.bundlesByKey[canonicalLocationKey] {
		if row.Provenance.IssuedAt.Before(before.UTC()) {
			out := cloneBundle(row)
			return &out, nil
		}
	}
	return nil, nil
}

func (s *MemorySnapshotStore) FindCanonicalLocationKeyByRequest(_ context.Context, req weatherd.Request) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.requestToKey[requestKey(req)], nil
}

func (s *MemorySnapshotStore) TouchRefreshCandidate(_ context.Context, canonicalLocationKey string, req weatherd.Request, requestedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.refreshCandidates[canonicalLocationKey]
	row.CanonicalLocationKey = canonicalLocationKey
	row.Request = req
	row.LastRequestedAt = requestedAt.UTC()
	if row.NextRefreshAt == nil {
		v := requestedAt.UTC()
		row.NextRefreshAt = &v
	}
	s.refreshCandidates[canonicalLocationKey] = row
	s.requestToKey[requestKey(req)] = canonicalLocationKey
	return nil
}

func (s *MemorySnapshotStore) ListRecentRefreshCandidates(_ context.Context, since time.Time) ([]weatherd.RefreshCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]weatherd.RefreshCandidate, 0, len(s.refreshCandidates))
	for _, row := range s.refreshCandidates {
		if !row.LastRequestedAt.Before(since.UTC()) {
			out = append(out, cloneRefreshCandidate(row))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastRequestedAt.After(out[j].LastRequestedAt)
	})
	return out, nil
}

func (s *MemorySnapshotStore) ListDueRefreshCandidates(_ context.Context, since, dueBefore time.Time) ([]weatherd.RefreshCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]weatherd.RefreshCandidate, 0, len(s.refreshCandidates))
	for _, row := range s.refreshCandidates {
		if row.LastRequestedAt.Before(since.UTC()) {
			continue
		}
		if row.NextRefreshAt != nil && row.NextRefreshAt.After(dueBefore.UTC()) {
			continue
		}
		out = append(out, cloneRefreshCandidate(row))
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].LastRequestedAt
		if out[i].NextRefreshAt != nil {
			left = out[i].NextRefreshAt.UTC()
		}
		right := out[j].LastRequestedAt
		if out[j].NextRefreshAt != nil {
			right = out[j].NextRefreshAt.UTC()
		}
		return left.Before(right)
	})
	return out, nil
}

func (s *MemorySnapshotStore) MarkRefreshCandidateRefreshed(_ context.Context, canonicalLocationKey string, refreshedAt, nextRefreshAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.refreshCandidates[canonicalLocationKey]
	row.CanonicalLocationKey = canonicalLocationKey
	v := refreshedAt.UTC()
	row.LastRefreshedAt = &v
	next := nextRefreshAt.UTC()
	row.NextRefreshAt = &next
	s.refreshCandidates[canonicalLocationKey] = row
	return nil
}

func (s *MemorySnapshotStore) Close() error {
	return nil
}

func requestKey(req weatherd.Request) string {
	raw, _ := json.Marshal(req.Normalized())
	return string(raw)
}

func cloneBundle(in weatherd.Bundle) weatherd.Bundle {
	out := in
	out.Hourly = append([]weatherd.HourlyForecastPoint(nil), in.Hourly...)
	out.Daily = append([]weatherd.DailyForecastPoint(nil), in.Daily...)
	return out
}

func cloneRefreshCandidate(in weatherd.RefreshCandidate) weatherd.RefreshCandidate {
	out := in
	if in.LastRefreshedAt != nil {
		v := *in.LastRefreshedAt
		out.LastRefreshedAt = &v
	}
	if in.NextRefreshAt != nil {
		v := *in.NextRefreshAt
		out.NextRefreshAt = &v
	}
	return out
}

func upsertBundleByIssuedAt(existing []weatherd.Bundle, bundle weatherd.Bundle) []weatherd.Bundle {
	out := make([]weatherd.Bundle, 0, len(existing)+1)
	inserted := false
	issuedAt := bundle.Provenance.IssuedAt.UTC()
	for _, row := range existing {
		switch {
		case row.Provenance.IssuedAt.Equal(issuedAt):
			if !inserted {
				out = append(out, cloneBundle(bundle))
				inserted = true
			}
		case !inserted && row.Provenance.IssuedAt.Before(issuedAt):
			out = append(out, cloneBundle(bundle))
			out = append(out, cloneBundle(row))
			inserted = true
		default:
			out = append(out, cloneBundle(row))
		}
	}
	if !inserted {
		out = append(out, cloneBundle(bundle))
	}
	return out
}
