package archiveworker

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestNewPostgresManifestStoreRejectsBlankDSN(t *testing.T) {
	t.Parallel()

	store, err := NewPostgresManifestStore("   ")
	if err == nil {
		t.Fatalf("expected error for blank dsn")
	}
	if store != nil {
		t.Fatalf("expected nil store for blank dsn")
	}
}

func TestNormalizeManifestRecordAppliesDefaults(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 25, 12, 34, 0, 0, time.UTC)
	record := normalizeManifestRecord(ManifestRecord{
		Provider:          "  ",
		PartitionHour:     now.Add(17 * time.Minute),
		ObjectBucket:      " pulse-telemetry-raw ",
		ObjectKey:         " /raw/path/object.pb.zst/ ",
		Compression:       "",
		ContentType:       "",
		ChecksumCRC32:     " AABBCCDD ",
		WriterID:          " writer-a ",
		DeviceIDs:         []string{" dev-2 ", "dev-1", "dev-2"},
		ProviderDeviceIDs: []string{" r351zabaph331057 ", "R351ZABAPH331057"},
	}, now)

	if record.Provider != defaultManifestProvider {
		t.Fatalf("provider default mismatch: got=%q", record.Provider)
	}
	if got, want := record.PartitionHour.Minute(), 0; got != want {
		t.Fatalf("partition hour not truncated: got minute=%d", got)
	}
	if record.ObjectBucket != "pulse-telemetry-raw" {
		t.Fatalf("object bucket mismatch: got=%q", record.ObjectBucket)
	}
	if record.ObjectKey != "raw/path/object.pb.zst" {
		t.Fatalf("object key mismatch: got=%q", record.ObjectKey)
	}
	if record.Compression != defaultManifestCompression {
		t.Fatalf("compression default mismatch: got=%q", record.Compression)
	}
	if record.ContentType != defaultObjectContentType {
		t.Fatalf("content type default mismatch: got=%q", record.ContentType)
	}
	if record.ChecksumCRC32 != "aabbccdd" {
		t.Fatalf("checksum normalization mismatch: got=%q", record.ChecksumCRC32)
	}
	if record.WriterID != "writer-a" {
		t.Fatalf("writer id normalization mismatch: got=%q", record.WriterID)
	}
	if strings.Join(record.DeviceIDs, ",") != "dev-1,dev-2" {
		t.Fatalf("device ids mismatch: got=%v", record.DeviceIDs)
	}
	if strings.Join(record.ProviderDeviceIDs, ",") != "R351ZABAPH331057" {
		t.Fatalf("provider device ids mismatch: got=%v", record.ProviderDeviceIDs)
	}
	if !record.CreatedAt.Equal(now) || !record.UpdatedAt.Equal(now) {
		t.Fatalf("created/updated defaults mismatch: created=%s updated=%s", record.CreatedAt, record.UpdatedAt)
	}
}

func TestValidateManifestRecord(t *testing.T) {
	t.Parallel()

	valid := ManifestRecord{
		Provider:        "ecoflow",
		Shard:           1,
		ShardCount:      128,
		PartitionHour:   time.Date(2026, time.February, 25, 12, 0, 0, 0, time.UTC),
		TSMinUnixMS:     1000,
		TSMaxUnixMS:     2000,
		RecordCount:     3,
		ObjectBucket:    "pulse-telemetry-raw",
		ObjectKey:       "raw/a.pb.zst",
		ObjectSizeBytes: 42,
		ContentType:     defaultObjectContentType,
		Compression:     defaultManifestCompression,
		ChecksumCRC32:   "aabbccdd",
		WriterID:        "archive-worker-1",
	}
	if err := validateManifestRecord(valid); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}

	invalid := valid
	invalid.RecordCount = 0
	if err := validateManifestRecord(invalid); err == nil {
		t.Fatalf("expected error for non-positive record_count")
	}

	invalid = valid
	invalid.Shard = invalid.ShardCount
	if err := validateManifestRecord(invalid); err == nil {
		t.Fatalf("expected error for shard >= shard_count")
	}
}

func TestUint32ToInt32(t *testing.T) {
	t.Parallel()

	converted, err := uint32ToInt32(uint32(math.MaxInt32), "manifest shard")
	if err != nil {
		t.Fatalf("expected max int32 uint32 conversion to succeed, got error: %v", err)
	}
	if converted != math.MaxInt32 {
		t.Fatalf("converted value mismatch: got=%d want=%d", converted, math.MaxInt32)
	}

	_, err = uint32ToInt32(uint32(math.MaxInt32)+1, "manifest shard")
	if err == nil {
		t.Fatalf("expected conversion overflow error")
	}
	if !strings.Contains(err.Error(), "manifest shard exceeds int32 bounds") {
		t.Fatalf("expected overflow error to mention field and bounds, got: %v", err)
	}
}

func TestIntToInt32(t *testing.T) {
	t.Parallel()

	converted, err := intToInt32(math.MaxInt32, "manifest record_count")
	if err != nil {
		t.Fatalf("expected max int32 conversion to succeed, got error: %v", err)
	}
	if converted != math.MaxInt32 {
		t.Fatalf("converted value mismatch: got=%d want=%d", converted, math.MaxInt32)
	}

	_, err = intToInt32(math.MaxInt32+1, "manifest record_count")
	if err == nil {
		t.Fatalf("expected conversion overflow error")
	}
	if !strings.Contains(err.Error(), "manifest record_count exceeds int32 bounds") {
		t.Fatalf("expected overflow error to mention field and bounds, got: %v", err)
	}
}
