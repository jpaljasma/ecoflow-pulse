package ingestlease

import (
	"fmt"
	"strings"
)

const (
	keyPrefixIngest = "pulse:v1:ingest"
)

// LeaseRef uniquely identifies a provider device for ingest lease ownership.
type LeaseRef struct {
	Provider         string
	ProviderDeviceID string
}

// Validate ensures lease reference fields are set and safe to embed in hash-tag keys.
func (r LeaseRef) Validate() error {
	provider := strings.TrimSpace(r.Provider)
	if provider == "" {
		return fmt.Errorf("provider is required")
	}
	deviceID := strings.TrimSpace(r.ProviderDeviceID)
	if deviceID == "" {
		return fmt.Errorf("provider_device_id is required")
	}
	if strings.ContainsAny(provider, "{}|") {
		return fmt.Errorf("provider contains reserved hash-tag characters")
	}
	if strings.ContainsAny(deviceID, "{}|") {
		return fmt.Errorf("provider_device_id contains reserved hash-tag characters")
	}
	return nil
}

// LeaseKeys is the ADR-0014 key set for one provider-device hash slot.
type LeaseKeys struct {
	HashTag string
	Lease   string
	Session string
	Fence   string
}

// KeysForRef builds cluster-aware keys for one provider-device lease group.
func KeysForRef(ref LeaseRef) (LeaseKeys, error) {
	if err := ref.Validate(); err != nil {
		return LeaseKeys{}, err
	}
	provider := strings.TrimSpace(ref.Provider)
	deviceID := strings.TrimSpace(ref.ProviderDeviceID)
	tag := "{" + provider + "|" + deviceID + "}"
	return LeaseKeys{
		HashTag: tag,
		Lease:   keyPrefixIngest + ":lease:" + tag,
		Session: keyPrefixIngest + ":session:" + tag,
		Fence:   keyPrefixIngest + ":fence:" + tag,
	}, nil
}
