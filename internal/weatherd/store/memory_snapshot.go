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
	mu                sync.RWMutex
	nowFn             func() time.Time
	bundlesByKey      map[string][]weatherd.Bundle
	requestToKey      map[string]string
	verifications     map[string]weatherd.VerificationResult
	biasByLocation    map[string]map[string]weatherd.BiasState
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
		verifications:     map[string]weatherd.VerificationResult{},
		biasByLocation:    map[string]map[string]weatherd.BiasState{},
		refreshCandidates: map[string]weatherd.RefreshCandidate{},
	}
}

func (s *MemorySnapshotStore) SaveForecastBundle(_ context.Context, req weatherd.Request, bundle weatherd.Bundle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := bundle.Provenance.CanonicalLocationKey
	s.bundlesByKey[key] = append(s.bundlesByKey[key], cloneBundle(bundle))
	sort.Slice(s.bundlesByKey[key], func(i, j int) bool {
		return s.bundlesByKey[key][i].Provenance.IssuedAt.After(s.bundlesByKey[key][j].Provenance.IssuedAt)
	})
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

func (s *MemorySnapshotStore) MarkRefreshCandidateRefreshed(_ context.Context, canonicalLocationKey string, refreshedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.refreshCandidates[canonicalLocationKey]
	row.CanonicalLocationKey = canonicalLocationKey
	v := refreshedAt.UTC()
	row.LastRefreshedAt = &v
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
	return out
}
