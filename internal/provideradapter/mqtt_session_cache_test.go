package provideradapter

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
	valkey "github.com/valkey-io/valkey-go"
)

func TestMQTTSessionCacheEncryptsPecronSessionAndRefreshesClientID(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{server.Addr()},
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("new valkey client: %v", err)
	}
	defer client.Close()
	keyring, err := valkeycache.NewKeyring("v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x42}, 32)})
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}
	clock := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	cacheClient, err := valkeycache.New(client, valkeycache.Options{
		Prefix:    "pulse",
		Namespace: "provider-mqtt-session",
		Keyring:   keyring,
		Now: func() time.Time {
			return clock
		},
	})
	if err != nil {
		t.Fatalf("new cache client: %v", err)
	}
	cache := NewMQTTSessionCache(cacheClient, MQTTSessionCacheConfig{
		IdleTTL:  time.Minute,
		MaxAge:   time.Hour,
		LocalTTL: time.Second,
		Now: func() time.Time {
			return clock
		},
	})
	credential := controlplane.ProviderCredential{
		ID:       "credential-a",
		Provider: controlplane.ProviderPecron,
		IsActive: true,
	}
	cache.PutPecron(ctx, credential, "pk:dk", pecron.MQTTSession{
		Address:  "mqtt.example:8443",
		Path:     "/ws",
		Token:    "super-secret-token",
		ClientID: "qu_user-1_1",
		Topics:   []string{"q/topic"},
	}, "user-1", clock.Add(30*time.Minute))

	session, ok := cache.GetPecron(ctx, credential, "pk:dk")
	if !ok {
		t.Fatal("expected pecron cache hit")
	}
	if session.Token != "super-secret-token" {
		t.Fatalf("token = %q", session.Token)
	}
	if session.ClientID != "qu_user-1_1778760000000" {
		t.Fatalf("client id = %q", session.ClientID)
	}
	clock = clock.Add(time.Second)
	session, ok = cache.GetPecron(ctx, credential, "pk:dk")
	if !ok {
		t.Fatal("expected second pecron cache hit")
	}
	if session.ClientID != "qu_user-1_1778760001000" {
		t.Fatalf("client id was not refreshed: %q", session.ClientID)
	}

	for _, key := range server.Keys() {
		raw, err := server.Get(key)
		if err != nil {
			t.Fatalf("miniredis get %q: %v", key, err)
		}
		if strings.Contains(raw, "super-secret-token") {
			t.Fatalf("cache key %q stored plaintext provider session: %q", key, raw)
		}
	}
}
