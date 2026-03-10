package ingestworker

import (
	"context"
	"fmt"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

type ProviderSessionRunner struct {
	runners map[string]SessionRunner
}

func NewProviderSessionRunner() *ProviderSessionRunner {
	return &ProviderSessionRunner{
		runners: map[string]SessionRunner{},
	}
}

func (r *ProviderSessionRunner) Register(provider string, runner SessionRunner) {
	if r == nil || runner == nil {
		return
	}
	provider = sanitizeProvider(provider)
	if provider == "" {
		return
	}
	if r.runners == nil {
		r.runners = map[string]SessionRunner{}
	}
	r.runners[provider] = runner
}

func (r *ProviderSessionRunner) Run(ctx context.Context, assignment controlplane.IngestAssignment) error {
	if r == nil {
		return fmt.Errorf("provider session runner is required")
	}
	provider := sanitizeProvider(assignment.Provider)
	runner, ok := r.runners[provider]
	if !ok {
		return fmt.Errorf("unsupported provider session runner: %s", provider)
	}
	assignment.Provider = provider
	return runner.Run(ctx, assignment)
}
