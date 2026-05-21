package valkeycache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	valkey "github.com/valkey-io/valkey-go"
	"golang.org/x/sync/singleflight"
)

// Options configures a shared cache namespace.
type Options struct {
	Prefix               string
	Namespace            string
	ContentType          string
	CompressionThreshold int
	Keyring              *Keyring
	Encrypt              bool
	DefaultLocalTTL      time.Duration
	Metrics              *Metrics
	Now                  func() time.Time
}

// ReadOptions controls cache reads.
type ReadOptions struct {
	LocalTTL time.Duration
}

// SetOptions controls cache writes.
type SetOptions struct {
	TTL                  time.Duration
	ContentType          string
	CompressionThreshold int
	Encrypt              bool
}

// SessionOptions controls sliding-TTL session cache operations.
type SessionOptions struct {
	IdleTTL       time.Duration
	HardExpiresAt time.Time
	LocalTTL      time.Duration
	ContentType   string
	Encrypt       bool
}

// Tag describes a versioned invalidation domain.
type Tag struct {
	Namespace string
	Partition string
	Name      string
}

// Client wraps valkey-go with shared cache policy.
type Client struct {
	client    valkey.Client
	builder   KeyBuilder
	namespace string
	opts      Options
	group     singleflight.Group
}

// New creates a shared cache client around a persistent valkey-go client.
func New(client valkey.Client, opts Options) (*Client, error) {
	if client == nil {
		return nil, errors.New("valkey client is required")
	}
	if opts.Prefix == "" {
		opts.Prefix = "pulse"
	}
	if opts.Namespace == "" {
		return nil, errors.New("cache namespace is required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	builder := NewKeyBuilder(opts.Prefix, opts.Namespace)
	return &Client{
		client:    client,
		builder:   builder,
		namespace: sanitizeSegment(opts.Namespace),
		opts:      opts,
	}, nil
}

// Key builds a cache key in this client's namespace.
func (c *Client) Key(partition string, inputs ...string) string {
	return c.builder.Key(partition, inputs...)
}

// TaggedKey builds a cache key whose digest includes current tag versions.
func (c *Client) TaggedKey(ctx context.Context, partition string, inputs []string, tags []Tag) (string, error) {
	versionedInputs := append([]string(nil), inputs...)
	for _, tag := range tags {
		version, err := c.tagVersion(ctx, tag)
		if err != nil {
			return "", err
		}
		versionedInputs = append(
			versionedInputs,
			"tag.namespace="+sanitizeSegment(tag.Namespace),
			"tag.name="+tag.Name,
			"tag.version="+strconv.FormatInt(version, 10),
		)
	}
	return c.Key(partition, versionedInputs...), nil
}

// GetBytes reads and decodes a cache payload.
func (c *Client) GetBytes(ctx context.Context, key string, opts ReadOptions) ([]byte, bool, error) {
	started := time.Now()
	localTTL := opts.LocalTTL
	if localTTL <= 0 {
		localTTL = c.opts.DefaultLocalTTL
	}

	var raw []byte
	var err error
	if localTTL > 0 {
		resp := c.client.DoCache(ctx, c.client.B().Get().Key(key).Cache(), localTTL)
		if resp.IsCacheHit() {
			c.opts.Metrics.observeClientSideRead(c.namespace, "hit")
		} else {
			c.opts.Metrics.observeClientSideRead(c.namespace, "miss")
		}
		raw, err = resp.AsBytes()
	} else {
		raw, err = c.client.Do(ctx, c.client.B().Get().Key(key).Build()).AsBytes()
	}
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			c.opts.Metrics.observeOperation(c.namespace, "get", "miss", started)
			return nil, false, nil
		}
		c.opts.Metrics.observeOperation(c.namespace, "get", "error", started)
		return nil, false, err
	}

	decoded, meta, err := DecodePayload(raw, c.opts.Keyring)
	if err != nil {
		c.opts.Metrics.observeOperation(c.namespace, "get", "decode_error", started)
		return nil, false, err
	}
	c.opts.Metrics.observePayload(c.namespace, "stored", meta, meta.EncryptionKeyID != "")
	c.opts.Metrics.observeOperation(c.namespace, "get", "hit", started)
	return decoded, true, nil
}

// SetBytes encodes and writes a cache payload.
func (c *Client) SetBytes(ctx context.Context, key string, raw []byte, opts SetOptions) error {
	started := time.Now()
	encoded, meta, err := EncodePayload(raw, EncodeOptions{
		ContentType:          chooseContentType(opts.ContentType, c.opts.ContentType),
		CompressionThreshold: chooseCompressionThreshold(opts.CompressionThreshold, c.opts.CompressionThreshold),
		Keyring:              c.opts.Keyring,
		Encrypt:              opts.Encrypt || c.opts.Encrypt,
		Now:                  c.opts.Now,
	})
	if err != nil {
		c.opts.Metrics.observeOperation(c.namespace, "set", "encode_error", started)
		return err
	}

	value := valkey.BinaryString(encoded)
	var cmd valkey.Completed
	if opts.TTL > 0 {
		cmd = c.client.B().Set().Key(key).Value(value).Px(opts.TTL).Build()
	} else {
		cmd = c.client.B().Set().Key(key).Value(value).Build()
	}
	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		c.opts.Metrics.observeOperation(c.namespace, "set", "error", started)
		return err
	}
	c.opts.Metrics.observePayload(c.namespace, "raw", meta, meta.EncryptionKeyID != "")
	c.opts.Metrics.observePayload(c.namespace, "stored", meta, meta.EncryptionKeyID != "")
	c.opts.Metrics.observeOperation(c.namespace, "set", "ok", started)
	return nil
}

// GetJSON reads JSON into out.
func (c *Client) GetJSON(ctx context.Context, key string, out any, opts ReadOptions) (bool, error) {
	raw, ok, err := c.GetBytes(ctx, key, opts)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("decode cache JSON: %w", err)
	}
	return true, nil
}

// SetJSON writes value as JSON.
func (c *Client) SetJSON(ctx context.Context, key string, value any, opts SetOptions) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode cache JSON: %w", err)
	}
	if opts.ContentType == "" {
		opts.ContentType = "application/json"
	}
	return c.SetBytes(ctx, key, raw, opts)
}

// GetOrLoadBytes returns a cache hit or loads, stores, and returns a value.
func (c *Client) GetOrLoadBytes(
	ctx context.Context,
	key string,
	readOpts ReadOptions,
	setOpts SetOptions,
	loader func(context.Context) ([]byte, error),
) ([]byte, bool, error) {
	if loader == nil {
		return nil, false, errors.New("cache loader is required")
	}
	if raw, ok, err := c.GetBytes(ctx, key, readOpts); err != nil || ok {
		return raw, ok, err
	}
	v, err, _ := c.group.Do(key, func() (any, error) {
		if raw, ok, err := c.GetBytes(ctx, key, readOpts); err != nil || ok {
			return loadResult{raw: raw, hit: ok}, err
		}
		raw, err := loader(ctx)
		if err != nil {
			return loadResult{}, err
		}
		if err := c.SetBytes(ctx, key, raw, setOpts); err != nil {
			return loadResult{}, err
		}
		return loadResult{raw: raw}, nil
	})
	if err != nil {
		return nil, false, err
	}
	result := v.(loadResult)
	return result.raw, result.hit, nil
}

// Touch extends a cache key TTL in milliseconds.
func (c *Client) Touch(ctx context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("touch ttl must be positive")
	}
	started := time.Now()
	err := c.client.Do(ctx, c.client.B().Pexpire().Key(key).Milliseconds(ttl.Milliseconds()).Xx().Build()).Error()
	if err != nil {
		c.opts.Metrics.observeOperation(c.namespace, "touch", "error", started)
		return err
	}
	c.opts.Metrics.observeOperation(c.namespace, "touch", "ok", started)
	return nil
}

// SetSessionBytes writes a sensitive or normal session payload with an idle TTL
// bounded by an optional hard expiration.
func (c *Client) SetSessionBytes(ctx context.Context, key string, raw []byte, opts SessionOptions) error {
	ttl, ok := SlidingTTL(c.opts.Now().UTC(), opts.IdleTTL, opts.HardExpiresAt)
	if !ok {
		return ErrSessionExpired
	}
	return c.SetBytes(ctx, key, raw, SetOptions{
		TTL:         ttl,
		ContentType: opts.ContentType,
		Encrypt:     opts.Encrypt,
	})
}

// GetSessionBytes reads a session payload and optionally extends its idle TTL.
func (c *Client) GetSessionBytes(ctx context.Context, key string, opts SessionOptions) ([]byte, bool, error) {
	raw, ok, err := c.GetBytes(ctx, key, ReadOptions{LocalTTL: opts.LocalTTL})
	if err != nil || !ok {
		return raw, ok, err
	}
	ttl, ttlOK := SlidingTTL(c.opts.Now().UTC(), opts.IdleTTL, opts.HardExpiresAt)
	if !ttlOK {
		return nil, false, ErrSessionExpired
	}
	if err := c.Touch(ctx, key, ttl); err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

// InvalidateTags increments version keys so future cache keys miss without scans.
func (c *Client) InvalidateTags(ctx context.Context, tags ...Tag) error {
	started := time.Now()
	for _, tag := range tags {
		if err := c.client.Do(ctx, c.client.B().Incr().Key(c.tagKey(tag)).Build()).Error(); err != nil {
			c.opts.Metrics.observeOperation(c.namespace, "invalidate_tags", "error", started)
			return err
		}
	}
	c.opts.Metrics.observeOperation(c.namespace, "invalidate_tags", "ok", started)
	return nil
}

func (c *Client) tagVersion(ctx context.Context, tag Tag) (int64, error) {
	raw, err := c.client.Do(ctx, c.client.B().Get().Key(c.tagKey(tag)).Build()).ToString()
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			return 0, nil
		}
		return 0, err
	}
	version, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse cache tag version: %w", err)
	}
	return version, nil
}

func (c *Client) tagKey(tag Tag) string {
	tagBuilder := NewKeyBuilder(c.opts.Prefix, "cache-tags:"+sanitizeSegment(tag.Namespace))
	return tagBuilder.Key(sanitizeSegment(tag.Partition), "tag="+tag.Name)
}

type loadResult struct {
	raw []byte
	hit bool
}

func chooseContentType(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func chooseCompressionThreshold(primary, fallback int) int {
	if primary > 0 {
		return primary
	}
	if fallback > 0 {
		return fallback
	}
	return DefaultCompressionThreshold
}
