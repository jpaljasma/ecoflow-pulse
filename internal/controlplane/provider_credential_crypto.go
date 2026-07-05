package controlplane

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

const providerCredentialEnvelopePrefix = "pulse-provider-credential:v1:"

type providerCredentialCipher struct {
	keyring *valkeycache.Keyring
}

func newProviderCredentialCipherFromEnv() (*providerCredentialCipher, error) {
	keyID := runtimecfg.EnvOrDefault(
		"PROVIDER_CREDENTIAL_ENCRYPTION_KEY_ID",
		runtimecfg.EnvOrDefault("VALKEY_CACHE_ENCRYPTION_KEY_ID", ""),
	)
	keySpec := runtimecfg.EnvOrDefault(
		"PROVIDER_CREDENTIAL_ENCRYPTION_KEYS",
		runtimecfg.EnvOrDefault("VALKEY_CACHE_ENCRYPTION_KEYS", ""),
	)
	if strings.TrimSpace(keyID) == "" && strings.TrimSpace(keySpec) == "" {
		return nil, nil
	}
	keyring, err := valkeycache.NewKeyringFromSpec(keyID, keySpec)
	if err != nil {
		return nil, fmt.Errorf("configure provider credential encryption: %w", err)
	}
	return &providerCredentialCipher{keyring: keyring}, nil
}

func (c *providerCredentialCipher) enabled() bool {
	return c != nil && c.keyring.Enabled()
}

func (c *providerCredentialCipher) sealString(value string) ([]byte, error) {
	if !c.enabled() {
		return []byte(value), nil
	}
	sealed, keyID, err := c.keyring.Seal([]byte(value))
	if err != nil {
		return nil, err
	}
	envelope := providerCredentialEnvelopePrefix + keyID + ":" + base64.RawURLEncoding.EncodeToString(sealed)
	return []byte(envelope), nil
}

func (c *providerCredentialCipher) openString(data []byte) (string, error) {
	text := string(data)
	if !strings.HasPrefix(text, providerCredentialEnvelopePrefix) {
		return text, nil
	}
	if !c.enabled() {
		return "", valkeycache.ErrEncryptionNotConfigured
	}
	rest := strings.TrimPrefix(text, providerCredentialEnvelopePrefix)
	keyID, encoded, ok := strings.Cut(rest, ":")
	if !ok || strings.TrimSpace(keyID) == "" || strings.TrimSpace(encoded) == "" {
		return "", errors.New("provider credential encryption envelope is malformed")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode provider credential envelope: %w", err)
	}
	plaintext, err := c.keyring.Open(keyID, sealed)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
