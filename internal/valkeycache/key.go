package valkeycache

import (
	"fmt"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/internal/hashutil"
)

const digestAlgorithm = "xxh3-128"

// KeyBuilder creates cluster-ready cache keys with a stable hash-tag partition.
type KeyBuilder struct {
	prefix    string
	namespace string
}

// NewKeyBuilder returns a cache key builder for one logical namespace.
func NewKeyBuilder(prefix, namespace string) KeyBuilder {
	return KeyBuilder{
		prefix:    sanitizeSegment(prefix),
		namespace: sanitizeSegment(namespace),
	}
}

// Key builds prefix:namespace:{partition}:xxh3-128:<digest>.
func (b KeyBuilder) Key(partition string, inputs ...string) string {
	digest := hashutil.XXH3Hex128(inputs...)
	return b.KeyWithDigest(partition, digest)
}

// KeyWithDigest builds a canonical key from a caller-supplied digest.
func (b KeyBuilder) KeyWithDigest(partition, digest string) string {
	return fmt.Sprintf(
		"%s:%s:{%s}:%s:%s",
		b.prefix,
		b.namespace,
		sanitizePartition(partition),
		digestAlgorithm,
		strings.ToLower(strings.TrimSpace(digest)),
	)
}

func sanitizeSegment(in string) string {
	clean := strings.TrimSpace(in)
	clean = strings.ReplaceAll(clean, "{", "_")
	clean = strings.ReplaceAll(clean, "}", "_")
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, "\t", "_")
	clean = strings.ReplaceAll(clean, "\n", "_")
	if clean == "" {
		return "default"
	}
	return clean
}

func sanitizePartition(in string) string {
	return sanitizeSegment(in)
}
