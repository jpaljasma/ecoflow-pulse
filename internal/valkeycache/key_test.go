package valkeycache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jpaljasma/ecoflow-pulse/internal/hashutil"
	valkey "github.com/valkey-io/valkey-go"
)

func TestKeyBuilderStableAndHashTagged(t *testing.T) {
	builder := NewKeyBuilder("pulse", "weather-forecast")

	got := builder.Key("site {home}", "lat=60.1708", "lon=24.9375", "model=open-meteo")
	wantDigest := hashutil.XXH3Hex128("lat=60.1708", "lon=24.9375", "model=open-meteo")
	want := fmt.Sprintf("pulse:weather-forecast:{site__home_}:xxh3-128:%s", wantDigest)
	if got != want {
		t.Fatalf("key mismatch\nwant: %s\n got: %s", want, got)
	}

	if got != builder.Key("site {home}", "lat=60.1708", "lon=24.9375", "model=open-meteo") {
		t.Fatal("key generation is not stable")
	}
}

func TestTagInvalidationUsesVersionedKeys(t *testing.T) {
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

	cache, err := New(client, Options{
		Prefix:    "pulse",
		Namespace: "unit",
		Now:       fixedNow,
	})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}

	tag := Tag{Namespace: "device-context", Partition: "device-a", Name: "owner"}
	keyBefore, err := cache.TaggedKey(ctx, "device-a", []string{"view=summary"}, []Tag{tag})
	if err != nil {
		t.Fatalf("tagged key before invalidation: %v", err)
	}
	if err := cache.SetBytes(ctx, keyBefore, []byte("cached"), SetOptions{TTL: time.Minute}); err != nil {
		t.Fatalf("set tagged payload: %v", err)
	}
	if got, ok, err := cache.GetBytes(ctx, keyBefore, ReadOptions{}); err != nil || !ok || string(got) != "cached" {
		t.Fatalf("read tagged payload = %q, %v, %v", got, ok, err)
	}

	if err := cache.InvalidateTags(ctx, tag); err != nil {
		t.Fatalf("invalidate tag: %v", err)
	}

	keyAfter, err := cache.TaggedKey(ctx, "device-a", []string{"view=summary"}, []Tag{tag})
	if err != nil {
		t.Fatalf("tagged key after invalidation: %v", err)
	}
	if keyAfter == keyBefore {
		t.Fatalf("tagged key did not change after invalidation: %s", keyAfter)
	}
	if _, ok, err := cache.GetBytes(ctx, keyAfter, ReadOptions{}); err != nil || ok {
		t.Fatalf("new tag version unexpectedly hit cache: ok=%v err=%v", ok, err)
	}
}

func TestValkeyGetSetAndTouch(t *testing.T) {
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

	cache, err := New(client, Options{Prefix: "pulse", Namespace: "unit", Now: fixedNow})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	key := cache.Key("partition-a", "thing=1")

	if err := cache.SetBytes(ctx, key, []byte("value"), SetOptions{TTL: 2 * time.Second}); err != nil {
		t.Fatalf("set bytes: %v", err)
	}
	got, ok, err := cache.GetBytes(ctx, key, ReadOptions{})
	if err != nil {
		t.Fatalf("get bytes: %v", err)
	}
	if !ok || string(got) != "value" {
		t.Fatalf("get bytes = %q, %v", got, ok)
	}
	if err := cache.Touch(ctx, key, 5*time.Second); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if ttl := server.TTL(key); ttl < 4*time.Second {
		t.Fatalf("touch did not extend ttl, got %s", ttl)
	}
}
