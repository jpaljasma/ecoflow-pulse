package replaycli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
)

type ManifestObject struct {
	Provider          string
	Shard             uint32
	ShardCount        uint32
	PartitionHour     time.Time
	TSMinUnixMS       int64
	TSMaxUnixMS       int64
	ObjectBucket      string
	ObjectKey         string
	DeviceIDs         []string
	ProviderDeviceIDs []string
}

type DeviceQuery struct {
	FromUnixMS         int64
	ToUnixMS           int64
	DeviceIDs          []string
	ProviderDeviceIDs  []string
	MaxObjectsReturned int
}

type FleetQuery struct {
	FromUnixMS         int64
	ToUnixMS           int64
	Shards             []uint32
	MaxObjectsReturned int
}

type ManifestStore interface {
	ListByDevices(ctx context.Context, query DeviceQuery) ([]ManifestObject, error)
	ListByFleetRange(ctx context.Context, query FleetQuery) ([]ManifestObject, error)
	Close() error
}

type ObjectReader interface {
	ReadObject(ctx context.Context, bucket string, key string) ([]byte, error)
	Close() error
}

type ReplayPublisher interface {
	Publish(ctx context.Context, shard uint32, payload []byte) error
	Close() error
}

type ReplayRequest struct {
	FromUnixMS        int64
	ToUnixMS          int64
	DeviceIDs         []string
	ProviderDeviceIDs []string
	Shards            []uint32
	MaxObjects        int
}

type ReplayReport struct {
	Mode               string
	ObjectsMatched     int
	ObjectsProcessed   int
	MessagesDecoded    int
	MessagesPublished  int
	MessagesFiltered   int
	MessagesFailed     int
	PublishedByShard   map[uint32]int
	FirstMessageUnixMS int64
	LastMessageUnixMS  int64
	StartedAt          time.Time
	FinishedAt         time.Time
}

func (r ReplayRequest) Validate(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "device" && mode != "fleet" {
		return errors.New("mode must be device or fleet")
	}
	if r.FromUnixMS <= 0 || r.ToUnixMS <= 0 {
		return errors.New("from and to must be positive unix-millisecond values")
	}
	if r.FromUnixMS > r.ToUnixMS {
		return errors.New("from must be <= to")
	}
	if r.MaxObjects < 0 {
		return errors.New("max objects must be >= 0")
	}
	if mode == "device" && len(r.DeviceIDs) == 0 && len(r.ProviderDeviceIDs) == 0 {
		return errors.New("device mode requires at least one device-id or provider-device-id")
	}
	return nil
}

func normalizeStrings(values []string, upper bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if upper {
			normalized = strings.ToUpper(normalized)
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}
