package gaprepair

import (
	"testing"
	"time"

	replayv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/replay/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

func TestNormalizePublisherConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := normalizePublisherConfig(NATSPublisherConfig{})
	if cfg.SubjectConfig.Prefix != telemetrybus.DefaultSubjectPrefix {
		t.Fatalf("unexpected default subject prefix: %s", cfg.SubjectConfig.Prefix)
	}
	if cfg.SubjectConfig.ShardCount != telemetrybus.DefaultShardCount {
		t.Fatalf("unexpected default shard count: %d", cfg.SubjectConfig.ShardCount)
	}
	if cfg.MsgIDBucket != time.Minute {
		t.Fatalf("unexpected msg id bucket default: %s", cfg.MsgIDBucket)
	}
}

func TestPublisherMsgIDStableWithinBucket(t *testing.T) {
	t.Parallel()

	p := &NATSPublisher{bucketMS: int64(time.Minute / time.Millisecond)}
	first := p.msgID(&replayv1.GapRepairRequest{
		Provider:         "ecoflow",
		ProviderDeviceId: "DEMOD2M00001057",
		FromUnixMs:       120_000,
		ToUnixMs:         179_999,
	})
	second := p.msgID(&replayv1.GapRepairRequest{
		Provider:         "ecoflow",
		ProviderDeviceId: "DEMOD2M00001057",
		FromUnixMs:       120_111,
		ToUnixMs:         179_500,
	})
	if first != second {
		t.Fatalf("expected stable msg id in same bucket, got %s and %s", first, second)
	}
}

func TestNormalizeRequestValidatesAndFillsShard(t *testing.T) {
	t.Parallel()

	p := &NATSPublisher{cfg: DefaultNATSPublisherConfig(telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 128}), bucketMS: int64(time.Minute / time.Millisecond)}
	req, err := p.normalizeRequest(&replayv1.GapRepairRequest{
		Provider:         " ecoflow ",
		ProviderDeviceId: " demod2m00001057 ",
		FromUnixMs:       1000,
		ToUnixMs:         4000,
	})
	if err != nil {
		t.Fatalf("normalizeRequest returned error: %v", err)
	}
	if req.GetProvider() != "ecoflow" {
		t.Fatalf("provider not normalized: %s", req.GetProvider())
	}
	if req.GetProviderDeviceId() != "DEMOD2M00001057" {
		t.Fatalf("provider device id not normalized: %s", req.GetProviderDeviceId())
	}
	if req.GetShardCount() != 128 {
		t.Fatalf("expected shard_count=128, got=%d", req.GetShardCount())
	}
	if req.GetRequestId() == "" {
		t.Fatalf("expected request id to be filled")
	}
}
