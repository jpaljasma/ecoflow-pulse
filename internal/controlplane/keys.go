package controlplane

import (
	"crypto/sha256"
	"strings"
)

func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func IsSupportedProvider(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderEcoFlow, ProviderPulseMQTT:
		return true
	default:
		return false
	}
}

func MaskAccessKey(accessKey string) string {
	key := strings.TrimSpace(accessKey)
	n := len(key)
	if n == 0 {
		return ""
	}
	if n <= 4 {
		return key[:1] + "***"
	}
	if n <= 8 {
		return key[:2] + "..." + key[n-2:]
	}
	return key[:4] + "..." + key[n-4:]
}

func HashAccessKey(accessKey string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(accessKey)))
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
}
