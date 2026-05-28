package edgecollector

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
)

func GenerateSecret(prefix string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("read random secret: %w", err)
	}
	prefix = strings.TrimSpace(prefix)
	encoded := base64.RawURLEncoding.EncodeToString(raw[:])
	if prefix == "" {
		return encoded, nil
	}
	return prefix + "_" + encoded, nil
}

func HashCollectorSecret(secret string) string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func ValidateCollectorSecret(secret string, hash string) bool {
	expected := strings.TrimSpace(hash)
	actual := HashCollectorSecret(secret)
	if expected == "" || actual == "" {
		return false
	}
	if len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
