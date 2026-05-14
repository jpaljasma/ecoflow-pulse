package valkeycache

import "time"

// SlidingTTL returns the idle TTL bounded by an optional hard expiration.
func SlidingTTL(now time.Time, idleTTL time.Duration, hardExpiresAt time.Time) (time.Duration, bool) {
	if idleTTL <= 0 {
		return 0, false
	}
	if hardExpiresAt.IsZero() {
		return idleTTL, true
	}
	remaining := hardExpiresAt.Sub(now)
	if remaining <= 0 {
		return 0, false
	}
	if remaining < idleTTL {
		return remaining, true
	}
	return idleTTL, true
}
