package hashutil

import (
	"encoding/hex"
	"strings"

	"github.com/zeebo/xxh3"
)

// XXH3Hex128 returns the canonical lowercase hex encoding of an XXH3_128 digest.
func XXH3Hex128(parts ...string) string {
	var joined string
	switch len(parts) {
	case 0:
		joined = ""
	case 1:
		joined = parts[0]
	default:
		var builder strings.Builder
		total := 0
		for _, part := range parts {
			total += len(part)
		}
		builder.Grow(total)
		for _, part := range parts {
			builder.WriteString(part)
		}
		joined = builder.String()
	}

	sum := xxh3.HashString128(joined).Bytes()
	return hex.EncodeToString(sum[:])
}
