package valkeycache

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

func TestEnvelopeS2ThresholdAndRoundTrip(t *testing.T) {
	raw := bytes.Repeat([]byte("forecast-payload:"), 512)
	encoded, meta, err := EncodePayload(raw, EncodeOptions{
		ContentType:          "application/json",
		CompressionThreshold: 4 * 1024,
		Now:                  fixedNow,
	})
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	if meta.Codec != CodecS2 {
		t.Fatalf("codec = %q, want %q", meta.Codec, CodecS2)
	}
	if meta.OriginalSize != len(raw) || meta.StoredSize >= len(raw) {
		t.Fatalf("unexpected sizes: %+v raw=%d", meta, len(raw))
	}

	decoded, decodedMeta, err := DecodePayload(encoded, nil)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatal("decoded payload mismatch")
	}
	if decodedMeta.Codec != CodecS2 {
		t.Fatalf("decoded codec = %q", decodedMeta.Codec)
	}
}

func TestEnvelopeSkipsCompressionWhenNotSmaller(t *testing.T) {
	raw := make([]byte, 8*1024)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand read: %v", err)
	}

	encoded, meta, err := EncodePayload(raw, EncodeOptions{
		ContentType:          "application/octet-stream",
		CompressionThreshold: 4 * 1024,
		Now:                  fixedNow,
	})
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	if meta.Codec != CodecIdentity {
		t.Fatalf("codec = %q, want %q", meta.Codec, CodecIdentity)
	}
	decoded, _, err := DecodePayload(encoded, nil)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatal("decoded payload mismatch")
	}
}

func TestEnvelopeAESGCMRoundTripWrongKeyAndNoPlaintext(t *testing.T) {
	raw := []byte("provider-session-secret-material")
	keyring, err := NewKeyring("v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x42}, 32)})
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}
	wrongKeyring, err := NewKeyring("v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x24}, 32)})
	if err != nil {
		t.Fatalf("new wrong keyring: %v", err)
	}

	encoded, meta, err := EncodePayload(raw, EncodeOptions{
		ContentType: "application/json",
		Keyring:     keyring,
		Encrypt:     true,
		Now:         fixedNow,
	})
	if err != nil {
		t.Fatalf("encode encrypted payload: %v", err)
	}
	if meta.EncryptionKeyID != "v1" {
		t.Fatalf("encryption key id = %q", meta.EncryptionKeyID)
	}
	if bytes.Contains(encoded, raw) {
		t.Fatalf("encoded envelope contains plaintext secret: %q", encoded)
	}
	if _, _, err := DecodePayload(encoded, wrongKeyring); err == nil {
		t.Fatal("decode with wrong key unexpectedly succeeded")
	}
	decoded, decodedMeta, err := DecodePayload(encoded, keyring)
	if err != nil {
		t.Fatalf("decode encrypted payload: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatal("decoded encrypted payload mismatch")
	}
	if decodedMeta.EncryptionKeyID != "v1" {
		t.Fatalf("decoded key id = %q", decodedMeta.EncryptionKeyID)
	}
}

func TestSlidingTTLHonorsHardCap(t *testing.T) {
	now := fixedNow()
	ttl, ok := SlidingTTL(now, time.Hour, now.Add(30*time.Minute))
	if !ok {
		t.Fatal("sliding ttl unexpectedly expired")
	}
	if ttl != 30*time.Minute {
		t.Fatalf("ttl = %s, want 30m", ttl)
	}

	ttl, ok = SlidingTTL(now, time.Hour, time.Time{})
	if !ok || ttl != time.Hour {
		t.Fatalf("ttl without hard cap = %s, %v", ttl, ok)
	}

	if ttl, ok := SlidingTTL(now, time.Hour, now.Add(-time.Second)); ok || ttl != 0 {
		t.Fatalf("expired hard cap = %s, %v", ttl, ok)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
}
