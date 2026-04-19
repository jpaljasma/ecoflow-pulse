package hashutil

import "testing"

func TestXXH3Hex128Deterministic(t *testing.T) {
	t.Parallel()

	first := XXH3Hex128("scope=device|", "dev-123|", "last7d")
	second := XXH3Hex128("scope=device|", "dev-123|", "last7d")
	if first != second {
		t.Fatalf("expected deterministic digest, got %q and %q", first, second)
	}
	if len(first) != 32 {
		t.Fatalf("expected 32-char hex digest, got %d", len(first))
	}
}

func TestXXH3Hex128SeparatesJoinedInputs(t *testing.T) {
	t.Parallel()

	if XXH3Hex128("ab", "c") != XXH3Hex128("abc") {
		t.Fatal("expected helper to hash the concatenated input")
	}
}
