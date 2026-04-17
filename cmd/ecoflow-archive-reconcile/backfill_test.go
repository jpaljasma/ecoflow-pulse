package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/proto"
)

func TestBuildManifestRecordFromBody(t *testing.T) {
	t.Parallel()

	partitionHour := time.Date(2026, time.April, 15, 13, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.April, 17, 12, 34, 56, 0, time.UTC)
	body := encodeArchiveObject(t,
		&envelopev1.TelemetryEnvelope{
			DeviceId:           "device-b",
			IngestedTimeUnixMs: 1713187205000,
			Shard:              7,
			ShardCount:         128,
			Labels: map[string]string{
				"provider":           "ecoflow",
				"provider_device_id": "demo0002",
			},
		},
		&envelopev1.TelemetryEnvelope{
			DeviceId:           "device-a",
			ObservedTimeUnixMs: 1713187201000,
			Shard:              7,
			ShardCount:         128,
			Labels: map[string]string{
				"provider": "ecoflow",
			},
			EcoflowSn: "demo0001",
		},
	)

	record, err := buildManifestRecordFromBody(replaycli.ManifestObject{
		ObjectBucket:    "pulse-telemetry-raw",
		ObjectKey:       "raw/yyyy=2026/mm=04/dd=15/hh=13/shard=007/part-00012-node-1.pb.zst",
		ObjectSizeBytes: int64(len(body)),
		PartitionHour:   partitionHour,
		Shard:           7,
	}, body, now)
	if err != nil {
		t.Fatalf("buildManifestRecordFromBody() error = %v", err)
	}

	if record.Provider != "ecoflow" {
		t.Fatalf("provider = %q, want ecoflow", record.Provider)
	}
	if record.Shard != 7 || record.ShardCount != 128 {
		t.Fatalf("shard values = (%d,%d), want (7,128)", record.Shard, record.ShardCount)
	}
	if !record.PartitionHour.Equal(partitionHour) {
		t.Fatalf("partition_hour = %s, want %s", record.PartitionHour, partitionHour)
	}
	if record.TSMinUnixMS != 1713187201000 || record.TSMaxUnixMS != 1713187205000 {
		t.Fatalf("ts range = [%d,%d], want [1713187201000,1713187205000]", record.TSMinUnixMS, record.TSMaxUnixMS)
	}
	if record.RecordCount != 2 {
		t.Fatalf("record_count = %d, want 2", record.RecordCount)
	}
	if record.WriterID != "node-1" {
		t.Fatalf("writer_id = %q, want node-1", record.WriterID)
	}
	if got, want := fmt.Sprintf("%08x", crc32.ChecksumIEEE(body)), record.ChecksumCRC32; want != got {
		t.Fatalf("checksum = %q, want %q", want, got)
	}
	if len(record.DeviceIDs) != 2 || record.DeviceIDs[0] != "device-a" || record.DeviceIDs[1] != "device-b" {
		t.Fatalf("device_ids = %#v", record.DeviceIDs)
	}
	if len(record.ProviderDeviceIDs) != 2 || record.ProviderDeviceIDs[0] != "DEMO0001" || record.ProviderDeviceIDs[1] != "DEMO0002" {
		t.Fatalf("provider_device_ids = %#v", record.ProviderDeviceIDs)
	}
	if !record.CreatedAt.Equal(now) || !record.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = (%s,%s), want %s", record.CreatedAt, record.UpdatedAt, now)
	}
}

func TestBuildManifestRecordFromBodyFallsBackToPartitionHourAndMixedProvider(t *testing.T) {
	t.Parallel()

	partitionHour := time.Date(2026, time.April, 16, 8, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.April, 17, 12, 34, 56, 0, time.UTC)
	body := encodeArchiveObject(t,
		&envelopev1.TelemetryEnvelope{
			DeviceId: "device-a",
			Labels: map[string]string{
				"provider":           "ecoflow",
				"provider_device_id": "demo0001",
			},
		},
		&envelopev1.TelemetryEnvelope{
			DeviceId: "device-b",
			Labels: map[string]string{
				"provider":           "other",
				"provider_device_id": "demo0002",
			},
		},
	)

	record, err := buildManifestRecordFromBody(replaycli.ManifestObject{
		ObjectBucket:  "pulse-telemetry-raw",
		ObjectKey:     "raw/yyyy=2026/mm=04/dd=16/hh=08/shard=099/part-00001.pb.zst",
		PartitionHour: partitionHour,
		Shard:         99,
	}, body, now)
	if err != nil {
		t.Fatalf("buildManifestRecordFromBody() error = %v", err)
	}

	if record.Provider != "mixed" {
		t.Fatalf("provider = %q, want mixed", record.Provider)
	}
	if record.ShardCount != 128 {
		t.Fatalf("shard_count = %d, want 128 fallback", record.ShardCount)
	}
	if record.WriterID != backfillWriterID {
		t.Fatalf("writer_id = %q, want %q", record.WriterID, backfillWriterID)
	}
	if record.TSMinUnixMS != partitionHour.UnixMilli() || record.TSMaxUnixMS != partitionHour.UnixMilli() {
		t.Fatalf("ts range = [%d,%d], want fallback partition hour %d", record.TSMinUnixMS, record.TSMaxUnixMS, partitionHour.UnixMilli())
	}
}

func encodeArchiveObject(t *testing.T, envs ...*envelopev1.TelemetryEnvelope) []byte {
	t.Helper()

	var raw bytes.Buffer
	for _, env := range envs {
		payload, err := proto.Marshal(env)
		if err != nil {
			t.Fatalf("marshal envelope: %v", err)
		}
		var sizePrefix [binary.MaxVarintLen64]byte
		n := binary.PutUvarint(sizePrefix[:], uint64(len(payload)))
		if _, err := raw.Write(sizePrefix[:n]); err != nil {
			t.Fatalf("write size prefix: %v", err)
		}
		if _, err := raw.Write(payload); err != nil {
			t.Fatalf("write payload: %v", err)
		}
	}

	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("new zstd writer: %v", err)
	}
	defer func() {
		if closeErr := encoder.Close(); closeErr != nil {
			t.Fatalf("close zstd writer: %v", closeErr)
		}
	}()
	return encoder.EncodeAll(raw.Bytes(), nil)
}
