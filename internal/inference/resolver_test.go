package inference

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
)

func TestControlPlaneResolverMemoizesValkeyHits(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server := miniredis.RunT(t)
	client, err := ingestlease.NewValkeyClient(ingestlease.DefaultValkeyClientConfig([]string{server.Addr()}))
	if err != nil {
		t.Fatalf("new valkey client: %v", err)
	}

	now := time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC)
	cache, err := valkeycache.New(client, valkeycache.Options{
		Prefix:      "pulse",
		Namespace:   "inference",
		ContentType: "application/json",
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	cached := DeviceContext{
		DeviceID:    "dev-1",
		EcoflowSN:   "SN-1",
		ProductName: "Garage",
		Model:       "DELTA 2 Max",
		Capabilities: map[string]any{
			"supports_extra_battery": true,
		},
	}
	if err := cache.SetJSON(ctx, cache.Key("device-context", "device_id=dev-1"), cached, valkeycache.SetOptions{TTL: time.Minute}); err != nil {
		t.Fatalf("seed valkey cache: %v", err)
	}

	resolver, err := NewControlPlaneResolver(controlplane.NewMemoryStore(), ControlPlaneResolverConfig{
		CacheTTL: time.Minute,
		Cache:    cache,
		NowFn:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	got, err := resolver.ResolveDeviceContext(ctx, "dev-1")
	if err != nil {
		t.Fatalf("first resolve from valkey: %v", err)
	}
	if got.ProductName != cached.ProductName {
		t.Fatalf("first product name = %q, want %q", got.ProductName, cached.ProductName)
	}
	got.Capabilities["supports_extra_battery"] = false

	client.Close()
	server.Close()

	got, err = resolver.ResolveDeviceContext(ctx, "dev-1")
	if err != nil {
		t.Fatalf("second resolve should use process memo after valkey hit: %v", err)
	}
	if got.ProductName != cached.ProductName {
		t.Fatalf("second product name = %q, want %q", got.ProductName, cached.ProductName)
	}
	if got.Capabilities["supports_extra_battery"] != true {
		t.Fatal("resolver returned caller-mutated memoized device context")
	}
}
