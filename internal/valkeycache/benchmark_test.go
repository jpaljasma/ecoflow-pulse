package valkeycache

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	valkey "github.com/valkey-io/valkey-go"
)

func BenchmarkKeyGeneration(b *testing.B) {
	builder := NewKeyBuilder("pulse", "energy-history")
	for i := 0; i < b.N; i++ {
		_ = builder.Key("device-a", "period=month", "start=2026-05-01", "end=2026-06-01")
	}
}

func BenchmarkEnvelopeDecodeCompressed(b *testing.B) {
	raw := bytes.Repeat([]byte("calendar-history:"), 1024)
	encoded, _, err := EncodePayload(raw, EncodeOptions{CompressionThreshold: 4 * 1024, Now: fixedNow})
	if err != nil {
		b.Fatalf("encode payload: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := DecodePayload(encoded, nil); err != nil {
			b.Fatalf("decode payload: %v", err)
		}
	}
}

func BenchmarkEnvelopeDecodeEncrypted(b *testing.B) {
	keyring, err := NewKeyring("v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x42}, 32)})
	if err != nil {
		b.Fatalf("new keyring: %v", err)
	}
	encoded, _, err := EncodePayload(bytes.Repeat([]byte("secret:"), 512), EncodeOptions{
		Keyring: keyring,
		Encrypt: true,
		Now:     fixedNow,
	})
	if err != nil {
		b.Fatalf("encode payload: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := DecodePayload(encoded, keyring); err != nil {
			b.Fatalf("decode payload: %v", err)
		}
	}
}

func BenchmarkCacheHitPath(b *testing.B) {
	ctx := context.Background()
	server := miniredis.RunT(b)
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{server.Addr()}, DisableCache: true})
	if err != nil {
		b.Fatalf("new valkey client: %v", err)
	}
	defer client.Close()
	cache, err := New(client, Options{Prefix: "pulse", Namespace: "bench", Now: fixedNow})
	if err != nil {
		b.Fatalf("new cache: %v", err)
	}
	key := cache.Key("partition", "id=1")
	if err := cache.SetBytes(ctx, key, []byte("value"), SetOptions{TTL: time.Minute}); err != nil {
		b.Fatalf("set bytes: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok, err := cache.GetBytes(ctx, key, ReadOptions{}); err != nil || !ok {
			b.Fatalf("cache hit failed: ok=%v err=%v", ok, err)
		}
	}
}
