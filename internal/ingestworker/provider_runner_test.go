package ingestworker

import (
	"context"
	"errors"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

type stubSessionRunner struct {
	run func(context.Context, controlplane.IngestAssignment) error
}

func (s stubSessionRunner) Run(ctx context.Context, assignment controlplane.IngestAssignment) error {
	if s.run != nil {
		return s.run(ctx, assignment)
	}
	return nil
}

func TestProviderSessionRunnerDispatchesByProvider(t *testing.T) {
	t.Parallel()

	runner := NewProviderSessionRunner()
	called := false
	runner.Register(controlplane.ProviderEcoFlow, stubSessionRunner{
		run: func(_ context.Context, assignment controlplane.IngestAssignment) error {
			called = true
			if assignment.Provider != controlplane.ProviderEcoFlow {
				t.Fatalf("unexpected provider %q", assignment.Provider)
			}
			return nil
		},
	})

	if err := runner.Run(context.Background(), controlplane.IngestAssignment{Provider: " EcoFlow "}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !called {
		t.Fatal("expected provider-specific runner to be called")
	}
}

func TestProviderSessionRunnerRejectsUnsupportedProvider(t *testing.T) {
	t.Parallel()

	runner := NewProviderSessionRunner()
	err := runner.Run(context.Background(), controlplane.IngestAssignment{Provider: "victron"})
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

func TestProviderSessionRunnerPropagatesRunnerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("runner failed")
	runner := NewProviderSessionRunner()
	runner.Register(controlplane.ProviderEcoFlow, stubSessionRunner{
		run: func(context.Context, controlplane.IngestAssignment) error {
			return wantErr
		},
	})

	if err := runner.Run(context.Background(), controlplane.IngestAssignment{Provider: controlplane.ProviderEcoFlow}); !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v want=%v", err, wantErr)
	}
}
