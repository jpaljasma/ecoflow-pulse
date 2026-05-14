package valkeycache

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/klauspost/compress/s2"
)

const (
	envelopeVersion             = 1
	DefaultCompressionThreshold = 4 * 1024

	CodecIdentity Codec = "identity"
	CodecS2       Codec = "s2"
)

var (
	ErrEncryptionNotConfigured = errors.New("cache encryption is not configured")
	ErrSessionExpired          = errors.New("cache session hard cap has expired")
)

// Codec records how the payload bytes are encoded inside the envelope.
type Codec string

// EnvelopeMeta describes the encoded cache payload.
type EnvelopeMeta struct {
	Version         int
	ContentType     string
	Codec           Codec
	EncryptionKeyID string
	OriginalSize    int
	StoredSize      int
	CreatedAt       time.Time
}

// EncodeOptions controls cache payload encoding.
type EncodeOptions struct {
	ContentType          string
	CompressionThreshold int
	Keyring              *Keyring
	Encrypt              bool
	Now                  func() time.Time
}

type payloadEnvelope struct {
	Version             int    `json:"v"`
	ContentType         string `json:"ct,omitempty"`
	Codec               Codec  `json:"codec"`
	EncryptionKeyID     string `json:"kid,omitempty"`
	OriginalSize        int    `json:"os"`
	StoredSize          int    `json:"ss"`
	CreatedAtUnixMillis int64  `json:"ts"`
	Payload             []byte `json:"p"`
}

// EncodePayload wraps bytes in a typed envelope with optional S2 compression
// and AES-GCM encryption.
func EncodePayload(raw []byte, opts EncodeOptions) ([]byte, EnvelopeMeta, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	threshold := opts.CompressionThreshold
	if threshold <= 0 {
		threshold = DefaultCompressionThreshold
	}

	stored := append([]byte(nil), raw...)
	codec := CodecIdentity
	if len(raw) >= threshold {
		compressed := s2.Encode(nil, raw)
		if len(compressed) < len(raw) {
			stored = compressed
			codec = CodecS2
		}
	}
	storedSize := len(stored)

	keyID := ""
	if opts.Encrypt {
		if opts.Keyring == nil || !opts.Keyring.Enabled() {
			return nil, EnvelopeMeta{}, ErrEncryptionNotConfigured
		}
		sealed, id, err := opts.Keyring.Seal(stored)
		if err != nil {
			return nil, EnvelopeMeta{}, err
		}
		stored = sealed
		keyID = id
	}

	now := opts.Now().UTC()
	env := payloadEnvelope{
		Version:             envelopeVersion,
		ContentType:         strings.TrimSpace(opts.ContentType),
		Codec:               codec,
		EncryptionKeyID:     keyID,
		OriginalSize:        len(raw),
		StoredSize:          storedSize,
		CreatedAtUnixMillis: now.UnixMilli(),
		Payload:             stored,
	}
	encoded, err := json.Marshal(env)
	if err != nil {
		return nil, EnvelopeMeta{}, fmt.Errorf("marshal cache envelope: %w", err)
	}
	return encoded, envelopeMeta(env), nil
}

// DecodePayload unwraps a cache payload envelope.
func DecodePayload(encoded []byte, keyring *Keyring) ([]byte, EnvelopeMeta, error) {
	var env payloadEnvelope
	if err := json.Unmarshal(encoded, &env); err != nil {
		return nil, EnvelopeMeta{}, fmt.Errorf("unmarshal cache envelope: %w", err)
	}
	if env.Version != envelopeVersion {
		return nil, EnvelopeMeta{}, fmt.Errorf("unsupported cache envelope version %d", env.Version)
	}
	stored := append([]byte(nil), env.Payload...)
	if env.EncryptionKeyID != "" {
		if keyring == nil || !keyring.Enabled() {
			return nil, EnvelopeMeta{}, ErrEncryptionNotConfigured
		}
		opened, err := keyring.Open(env.EncryptionKeyID, stored)
		if err != nil {
			return nil, EnvelopeMeta{}, err
		}
		stored = opened
	}

	var raw []byte
	switch env.Codec {
	case "", CodecIdentity:
		raw = stored
	case CodecS2:
		decoded, err := s2.Decode(nil, stored)
		if err != nil {
			return nil, EnvelopeMeta{}, fmt.Errorf("decode s2 cache payload: %w", err)
		}
		raw = decoded
	default:
		return nil, EnvelopeMeta{}, fmt.Errorf("unsupported cache payload codec %q", env.Codec)
	}
	return raw, envelopeMeta(env), nil
}

func envelopeMeta(env payloadEnvelope) EnvelopeMeta {
	return EnvelopeMeta{
		Version:         env.Version,
		ContentType:     env.ContentType,
		Codec:           env.Codec,
		EncryptionKeyID: env.EncryptionKeyID,
		OriginalSize:    env.OriginalSize,
		StoredSize:      env.StoredSize,
		CreatedAt:       time.UnixMilli(env.CreatedAtUnixMillis).UTC(),
	}
}

// Keyring stores AES-GCM keys by key id.
type Keyring struct {
	defaultID string
	keys      map[string][]byte
}

// NewKeyring creates an AES-GCM keyring. AES-128, AES-192, and AES-256 keys
// are accepted by the standard library; production config should use AES-256.
func NewKeyring(defaultID string, keys map[string][]byte) (*Keyring, error) {
	defaultID = strings.TrimSpace(defaultID)
	if defaultID == "" {
		return nil, errors.New("default cache encryption key id is required")
	}
	copied := make(map[string][]byte, len(keys))
	for id, key := range keys {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, errors.New("cache encryption key id is required")
		}
		if _, err := aes.NewCipher(key); err != nil {
			return nil, fmt.Errorf("invalid cache encryption key %q: %w", id, err)
		}
		copied[id] = append([]byte(nil), key...)
	}
	if _, ok := copied[defaultID]; !ok {
		return nil, fmt.Errorf("default cache encryption key %q is not present", defaultID)
	}
	return &Keyring{defaultID: defaultID, keys: copied}, nil
}

// NewKeyringFromSpec parses comma-separated key specs:
// key-id:base64-encoded-aes-key,key-id-2:base64-encoded-aes-key.
func NewKeyringFromSpec(defaultID, spec string) (*Keyring, error) {
	keys := map[string][]byte{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, encoded, ok := strings.Cut(part, ":")
		if !ok {
			id, encoded, ok = strings.Cut(part, "=")
		}
		if !ok {
			return nil, fmt.Errorf("cache encryption key %q must use id:base64", part)
		}
		decoded, err := decodeBase64Key(strings.TrimSpace(encoded))
		if err != nil {
			return nil, fmt.Errorf("decode cache encryption key %q: %w", strings.TrimSpace(id), err)
		}
		keys[strings.TrimSpace(id)] = decoded
	}
	return NewKeyring(defaultID, keys)
}

// Enabled reports whether the keyring can encrypt and decrypt payloads.
func (k *Keyring) Enabled() bool {
	return k != nil && k.defaultID != "" && len(k.keys) > 0
}

// DefaultKeyID returns the active write key id.
func (k *Keyring) DefaultKeyID() string {
	if k == nil {
		return ""
	}
	return k.defaultID
}

// Seal encrypts plaintext with the default key id.
func (k *Keyring) Seal(plaintext []byte) ([]byte, string, error) {
	if !k.Enabled() {
		return nil, "", ErrEncryptionNotConfigured
	}
	key := k.keys[k.defaultID]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, "", fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", fmt.Errorf("create AES-GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, "", fmt.Errorf("read AES-GCM nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), k.defaultID, nil
}

// Open decrypts ciphertext with the requested key id.
func (k *Keyring) Open(keyID string, sealed []byte) ([]byte, error) {
	if !k.Enabled() {
		return nil, ErrEncryptionNotConfigured
	}
	key, ok := k.keys[strings.TrimSpace(keyID)]
	if !ok {
		return nil, fmt.Errorf("cache encryption key %q is not configured", keyID)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("encrypted cache payload is shorter than nonce")
	}
	nonce := sealed[:gcm.NonceSize()]
	ciphertext := sealed[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt cache payload: %w", err)
	}
	return plaintext, nil
}

func decodeBase64Key(encoded string) ([]byte, error) {
	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, decoder := range decoders {
		decoded, err := decoder.DecodeString(encoded)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
