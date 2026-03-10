package provideradapter

import (
	"context"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

type Discoverer interface {
	DiscoverDevices(ctx context.Context, credential controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error)
}

type Registry struct {
	supported   map[string]struct{}
	discoverers map[string]Discoverer
}

func NewRegistry() *Registry {
	return &Registry{
		supported:   map[string]struct{}{},
		discoverers: map[string]Discoverer{},
	}
}

func (r *Registry) RegisterProvider(provider string) {
	if r == nil {
		return
	}
	provider = controlplane.NormalizeProvider(provider)
	if provider == "" {
		return
	}
	if r.supported == nil {
		r.supported = map[string]struct{}{}
	}
	r.supported[provider] = struct{}{}
}

func (r *Registry) RegisterDiscoverer(provider string, discoverer Discoverer) {
	if r == nil || discoverer == nil {
		return
	}
	r.RegisterProvider(provider)
	if r.discoverers == nil {
		r.discoverers = map[string]Discoverer{}
	}
	r.discoverers[controlplane.NormalizeProvider(provider)] = discoverer
}

func (r *Registry) Supports(provider string) bool {
	if r == nil {
		return false
	}
	provider = controlplane.NormalizeProvider(provider)
	if provider == "" {
		return false
	}
	_, ok := r.supported[provider]
	return ok
}

func (r *Registry) Discoverer(provider string) (Discoverer, bool) {
	if r == nil {
		return nil, false
	}
	provider = controlplane.NormalizeProvider(provider)
	if provider == "" {
		return nil, false
	}
	discoverer, ok := r.discoverers[provider]
	return discoverer, ok
}

func (r *Registry) SupportedProviders() []string {
	if r == nil || len(r.supported) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.supported))
	for provider := range r.supported {
		out = append(out, provider)
	}
	// Small stable ordering for deterministic tests/docs.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if strings.Compare(out[j], out[i]) < 0 {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
