package provideradapter

import (
	"context"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

type staticDiscoverer struct{}

func (staticDiscoverer) DiscoverDevices(context.Context, controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error) {
	return nil, nil
}

func TestRegistryTracksSupportedProvidersAndDiscoverers(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterProvider(" EcoFlow ")
	registry.RegisterProvider("victron")
	registry.RegisterDiscoverer(controlplane.ProviderEcoFlow, staticDiscoverer{})

	if !registry.Supports(controlplane.ProviderEcoFlow) {
		t.Fatal("expected ecoflow to be supported")
	}
	if !registry.Supports("VICTRON") {
		t.Fatal("expected victron to be supported")
	}
	if registry.Supports("unknown") {
		t.Fatal("did not expect unknown provider to be supported")
	}
	if _, ok := registry.Discoverer("victron"); ok {
		t.Fatal("did not expect discoverer for victron")
	}
	if _, ok := registry.Discoverer(controlplane.ProviderEcoFlow); !ok {
		t.Fatal("expected ecoflow discoverer to be registered")
	}

	got := registry.SupportedProviders()
	want := []string{controlplane.ProviderEcoFlow, "victron"}
	if len(got) != len(want) {
		t.Fatalf("supported providers len=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("supported providers[%d]=%q want=%q", i, got[i], want[i])
		}
	}
}
