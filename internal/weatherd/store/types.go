package store

import (
	"context"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

type CachedBundle struct {
	Bundle     weatherd.Bundle `json:"bundle"`
	CachedAt   time.Time       `json:"cached_at"`
	StaleAfter time.Time       `json:"stale_after"`
}

type HotCache interface {
	GetForecast(ctx context.Context, key string) (*CachedBundle, error)
	PutForecast(ctx context.Context, key string, bundle weatherd.Bundle, ttl time.Duration) error
}

type SnapshotStore interface {
	SaveForecastBundle(ctx context.Context, req weatherd.Request, bundle weatherd.Bundle) error
	LatestBundle(ctx context.Context, canonicalLocationKey string) (*weatherd.Bundle, error)
	LatestBundleBefore(ctx context.Context, canonicalLocationKey string, before time.Time) (*weatherd.Bundle, error)
	FindCanonicalLocationKeyByRequest(ctx context.Context, req weatherd.Request) (string, error)
	SaveVerification(ctx context.Context, result weatherd.VerificationResult) error
	LoadBiasStates(ctx context.Context, canonicalLocationKey string) ([]weatherd.BiasState, error)
	UpsertBiasStates(ctx context.Context, states []weatherd.BiasState) error
	TouchRefreshCandidate(ctx context.Context, canonicalLocationKey string, req weatherd.Request, requestedAt time.Time) error
	ListRecentRefreshCandidates(ctx context.Context, since time.Time) ([]weatherd.RefreshCandidate, error)
	ListDueRefreshCandidates(ctx context.Context, since, dueBefore time.Time) ([]weatherd.RefreshCandidate, error)
	MarkRefreshCandidateRefreshed(ctx context.Context, canonicalLocationKey string, refreshedAt, nextRefreshAt time.Time) error
	Close() error
}
