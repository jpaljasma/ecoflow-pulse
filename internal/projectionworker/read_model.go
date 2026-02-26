package projectionworker

import (
	"context"
	"strings"
)

// SnapshotIdentity identifies one live snapshot read target.
// device_id is preferred when available; ecoflow_sn is a fallback identity.
type SnapshotIdentity struct {
	DeviceID  string
	EcoflowSN string
}

func (s SnapshotIdentity) normalized() SnapshotIdentity {
	return SnapshotIdentity{
		DeviceID:  strings.TrimSpace(s.DeviceID),
		EcoflowSN: strings.ToUpper(strings.TrimSpace(s.EcoflowSN)),
	}
}

// SnapshotCursor is the read-model cursor consumed by downstream query/realtime
// consumers. Seq is monotonic per projected device stream.
type SnapshotCursor struct {
	Seq      uint64
	TsUnixMs int64
}

// SnapshotReadModel is the downstream snapshot contract consumed by realtime
// and query paths.
type SnapshotReadModel struct {
	DeviceID  string
	EcoflowSN string
	Cursor    SnapshotCursor
	Metrics   map[string]float64
}

// SnapshotReader is consumed by downstream services (for example gRPC query
// APIs) that need latest live metrics without coupling to projection internals.
type SnapshotReader interface {
	ReadSnapshot(ctx context.Context, identity SnapshotIdentity) (*SnapshotReadModel, error)
}

// ReadSnapshot returns the downstream read-model contract from the persisted
// live snapshot state.
func (s *ValkeySnapshotStore) ReadSnapshot(ctx context.Context, identity SnapshotIdentity) (*SnapshotReadModel, error) {
	identity = identity.normalized()
	snapshot, err := s.GetSnapshot(ctx, identity.DeviceID, identity.EcoflowSN)
	if err != nil || snapshot == nil {
		return nil, err
	}
	return toSnapshotReadModel(snapshot), nil
}

func toSnapshotReadModel(snapshot *LiveSnapshot) *SnapshotReadModel {
	if snapshot == nil {
		return nil
	}
	out := &SnapshotReadModel{
		DeviceID:  snapshot.DeviceID,
		EcoflowSN: snapshot.EcoflowSN,
		Cursor: SnapshotCursor{
			Seq:      snapshot.CursorSeq,
			TsUnixMs: snapshot.CursorTsUnixMs,
		},
		Metrics: make(map[string]float64, len(snapshot.Metrics)),
	}
	for k, v := range snapshot.Metrics {
		out.Metrics[k] = v
	}
	return out
}
