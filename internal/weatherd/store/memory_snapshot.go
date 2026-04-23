package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

type MemorySnapshotStore struct {
	mu                  sync.RWMutex
	nowFn               func() time.Time
	bundlesByKey        map[string][]weatherd.Bundle
	verificationAnchors map[string]weatherd.Bundle
	requestToKey        map[string]string
	verifications       map[string]weatherd.VerificationResult
	biasByLocation      map[string]map[string]weatherd.BiasState
	refreshCandidates   map[string]weatherd.RefreshCandidate
}

func NewMemorySnapshotStore(nowFn func() time.Time) *MemorySnapshotStore {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &MemorySnapshotStore{
		nowFn:               nowFn,
		bundlesByKey:        map[string][]weatherd.Bundle{},
		verificationAnchors: map[string]weatherd.Bundle{},
		requestToKey:        map[string]string{},
		verifications:       map[string]weatherd.VerificationResult{},
		biasByLocation:      map[string]map[string]weatherd.BiasState{},
		refreshCandidates:   map[string]weatherd.RefreshCandidate{},
	}
}

func (s *MemorySnapshotStore) SaveForecastBundle(_ context.Context, req weatherd.Request, bundle weatherd.Bundle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := bundle.Provenance.CanonicalLocationKey
	s.bundlesByKey[key] = upsertBundleByIssuedAt(s.bundlesByKey[key], bundle)
	anchorKey := verificationAnchorKey(key, nextLocalDayStartUTC(bundle))
	if existing, ok := s.verificationAnchors[anchorKey]; !ok || !existing.Provenance.IssuedAt.After(bundle.Provenance.IssuedAt.UTC()) {
		s.verificationAnchors[anchorKey] = cloneBundle(bundle)
	}
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

func (s *MemorySnapshotStore) LoadVerificationForecastAnchor(_ context.Context, canonicalLocationKey string, verificationDate time.Time) (*weatherd.Bundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row, ok := s.verificationAnchors[verificationAnchorKey(canonicalLocationKey, verificationDate)]
	if !ok {
		return nil, nil
	}
	out := cloneBundle(row)
	return &out, nil
}

func (s *MemorySnapshotStore) UpsertVerificationForecastAnchor(_ context.Context, canonicalLocationKey string, verificationDate time.Time, bundle weatherd.Bundle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := verificationAnchorKey(canonicalLocationKey, verificationDate)
	if existing, ok := s.verificationAnchors[key]; !ok || !existing.Provenance.IssuedAt.After(bundle.Provenance.IssuedAt.UTC()) {
		s.verificationAnchors[key] = cloneBundle(bundle)
	}
	return nil
}

func (s *MemorySnapshotStore) FindCanonicalLocationKeyByRequest(_ context.Context, req weatherd.Request) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.requestToKey[requestKey(req)], nil
}

func (s *MemorySnapshotStore) SaveVerification(_ context.Context, result weatherd.VerificationResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifications[verificationKey(result.Provenance.CanonicalLocationKey, result.VerificationDate)] = cloneVerification(result)
	return nil
}

func (s *MemorySnapshotStore) LoadVerification(_ context.Context, canonicalLocationKey string, verificationDate time.Time) (*weatherd.VerificationResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row, ok := s.verifications[verificationKey(canonicalLocationKey, verificationDate)]
	if !ok {
		return nil, nil
	}
	out := cloneVerification(row)
	return &out, nil
}

func (s *MemorySnapshotStore) LoadBiasStates(_ context.Context, canonicalLocationKey string) ([]weatherd.BiasState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.biasByLocation[canonicalLocationKey]
	out := make([]weatherd.BiasState, 0, len(rows))
	for _, row := range rows {
		out = append(out, cloneBiasState(row))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Metric == out[j].Metric {
			return out[i].HourOfDay < out[j].HourOfDay
		}
		return out[i].Metric < out[j].Metric
	})
	return out, nil
}

func (s *MemorySnapshotStore) UpsertBiasStates(_ context.Context, states []weatherd.BiasState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, state := range states {
		byMetric := s.biasByLocation[state.CanonicalLocationKey]
		if byMetric == nil {
			byMetric = map[string]weatherd.BiasState{}
			s.biasByLocation[state.CanonicalLocationKey] = byMetric
		}
		byMetric[biasKey(state)] = cloneBiasState(state)
	}
	return nil
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

func verificationKey(canonicalLocationKey string, verificationDate time.Time) string {
	return canonicalLocationKey + "|" + verificationDate.UTC().Format(time.RFC3339)
}

func biasKey(state weatherd.BiasState) string {
	return fmt.Sprintf("%s|%d", state.Metric, state.HourOfDay)
}

func cloneBundle(in weatherd.Bundle) weatherd.Bundle {
	out := in
	out.Hourly = append([]weatherd.HourlyForecastPoint(nil), in.Hourly...)
	out.Daily = append([]weatherd.DailyForecastPoint(nil), in.Daily...)
	return out
}

func cloneVerification(in weatherd.VerificationResult) weatherd.VerificationResult {
	out := in
	out.Hourly = append([]weatherd.VerificationHour(nil), in.Hourly...)
	return out
}

func cloneBiasState(in weatherd.BiasState) weatherd.BiasState {
	out := in
	if in.AdditiveBias != nil {
		v := *in.AdditiveBias
		out.AdditiveBias = &v
	}
	if in.MultiplicativeRatio != nil {
		v := *in.MultiplicativeRatio
		out.MultiplicativeRatio = &v
	}
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

func verificationAnchorKey(canonicalLocationKey string, verificationDate time.Time) string {
	return canonicalLocationKey + "|" + verificationDate.UTC().Format(time.RFC3339)
}

func nextLocalDayStartUTC(bundle weatherd.Bundle) time.Time {
	loc := time.UTC
	if bundle.Provenance.Timezone != "" {
		if loaded, err := time.LoadLocation(bundle.Provenance.Timezone); err == nil {
			loc = loaded
		}
	}
	issuedLocal := bundle.Provenance.IssuedAt.In(loc)
	nextDay := time.Date(issuedLocal.Year(), issuedLocal.Month(), issuedLocal.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	return nextDay.UTC()
}
