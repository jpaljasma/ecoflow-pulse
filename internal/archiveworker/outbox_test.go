package archiveworker

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveUploadOutboxAcksAfterLocalFsyncAndDefersManifestUntilUpload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outbox, err := NewFileArchiveUploadOutbox(FileArchiveUploadOutboxConfig{
		Dir:      dir,
		MaxBytes: 64 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewFileArchiveUploadOutbox failed: %v", err)
	}
	store := &fakeObjectStore{err: errors.New("gcs offline")}
	manifest := &fakeManifestStore{}
	worker := newTestWorker(store, time.Unix(10, 0).UTC())
	worker.manifestStore = manifest
	worker.archiveOutbox = outbox
	worker.cfg.MaxRecordsPerPart = 1
	d := newFakeDelivery(t, envelope(2, "env-outbox", 1500))

	if err := worker.processDelivery(context.Background(), d); err != nil {
		t.Fatalf("process delivery with remote outage should ACK after local outbox fsync: %v", err)
	}
	if d.acked != 1 {
		t.Fatalf("delivery ack count=%d want 1", d.acked)
	}
	if d.nacked != 0 {
		t.Fatalf("delivery nack count=%d want 0", d.nacked)
	}
	pending, err := outbox.PendingCount(context.Background())
	if err != nil {
		t.Fatalf("PendingCount failed: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending outbox entries=%d want 1", pending)
	}
	if len(manifest.records) != 0 {
		t.Fatalf("manifest should not be written before remote upload succeeds, got %d records", len(manifest.records))
	}

	store.err = nil
	if err := worker.flushArchiveOutbox(context.Background()); err != nil {
		t.Fatalf("flushArchiveOutbox after recovery failed: %v", err)
	}
	pending, err = outbox.PendingCount(context.Background())
	if err != nil {
		t.Fatalf("PendingCount after flush failed: %v", err)
	}
	if pending != 0 {
		t.Fatalf("pending outbox entries after flush=%d want 0", pending)
	}
	if len(store.requests) != 1 {
		t.Fatalf("remote object writes=%d want 1", len(store.requests))
	}
	if ids := decodeEnvelopeIDs(t, store.requests[0].Body); len(ids) != 1 || ids[0] != "env-outbox" {
		t.Fatalf("remote archive envelope ids mismatch: got=%v", ids)
	}
	if len(manifest.records) != 1 {
		t.Fatalf("manifest records after upload=%d want 1", len(manifest.records))
	}
	if manifest.records[0].ObjectKey != store.requests[0].Key {
		t.Fatalf("manifest object key=%q want %q", manifest.records[0].ObjectKey, store.requests[0].Key)
	}
}

func TestArchiveUploadOutboxReplaysAfterRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	firstOutbox, err := NewFileArchiveUploadOutbox(FileArchiveUploadOutboxConfig{
		Dir:      filepath.Join(dir, "archive-outbox"),
		MaxBytes: 64 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewFileArchiveUploadOutbox first failed: %v", err)
	}
	firstStore := &fakeObjectStore{err: errors.New("gcs offline")}
	firstWorker := newTestWorker(firstStore, time.Unix(10, 0).UTC())
	firstWorker.log = slog.Default()
	firstWorker.manifestStore = &fakeManifestStore{}
	firstWorker.archiveOutbox = firstOutbox
	firstWorker.cfg.MaxRecordsPerPart = 1
	d := newFakeDelivery(t, envelope(2, "env-restart", 1500))

	if err := firstWorker.processDelivery(context.Background(), d); err != nil {
		t.Fatalf("process delivery before restart failed: %v", err)
	}
	if d.acked != 1 {
		t.Fatalf("delivery ack count=%d want 1", d.acked)
	}

	secondOutbox, err := NewFileArchiveUploadOutbox(FileArchiveUploadOutboxConfig{
		Dir:      filepath.Join(dir, "archive-outbox"),
		MaxBytes: 64 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewFileArchiveUploadOutbox second failed: %v", err)
	}
	secondStore := &fakeObjectStore{}
	secondManifest := &fakeManifestStore{}
	secondWorker := newTestWorker(secondStore, time.Unix(11, 0).UTC())
	secondWorker.manifestStore = secondManifest
	secondWorker.archiveOutbox = secondOutbox

	if err := secondWorker.flushArchiveOutbox(context.Background()); err != nil {
		t.Fatalf("flushArchiveOutbox after restart failed: %v", err)
	}
	pending, err := secondOutbox.PendingCount(context.Background())
	if err != nil {
		t.Fatalf("PendingCount after restart flush failed: %v", err)
	}
	if pending != 0 {
		t.Fatalf("pending outbox entries after restart flush=%d want 0", pending)
	}
	if len(secondStore.requests) != 1 {
		t.Fatalf("remote object writes after restart=%d want 1", len(secondStore.requests))
	}
	if ids := decodeEnvelopeIDs(t, secondStore.requests[0].Body); len(ids) != 1 || ids[0] != "env-restart" {
		t.Fatalf("remote archive envelope ids after restart mismatch: got=%v", ids)
	}
	if len(secondManifest.records) != 1 {
		t.Fatalf("manifest records after restart=%d want 1", len(secondManifest.records))
	}
}

func TestArchiveUploadOutboxDoesNotOverwritePendingEntryAfterRestart(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "archive-outbox")
	firstOutbox, err := NewFileArchiveUploadOutbox(FileArchiveUploadOutboxConfig{
		Dir:      dir,
		MaxBytes: 64 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewFileArchiveUploadOutbox first failed: %v", err)
	}
	firstWorker := newTestWorker(&fakeObjectStore{err: errors.New("gcs offline")}, time.Unix(10, 0).UTC())
	firstWorker.manifestStore = &fakeManifestStore{}
	firstWorker.archiveOutbox = firstOutbox
	firstWorker.cfg.MaxRecordsPerPart = 1
	firstDelivery := newFakeDelivery(t, envelope(2, "env-before-restart", 1500))

	if err := firstWorker.processDelivery(context.Background(), firstDelivery); err != nil {
		t.Fatalf("process delivery before restart failed: %v", err)
	}
	if firstDelivery.acked != 1 {
		t.Fatalf("first delivery ack count=%d want 1", firstDelivery.acked)
	}

	secondOutbox, err := NewFileArchiveUploadOutbox(FileArchiveUploadOutboxConfig{
		Dir:      dir,
		MaxBytes: 64 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewFileArchiveUploadOutbox second failed: %v", err)
	}
	secondWorker := newTestWorker(&fakeObjectStore{err: errors.New("gcs still offline")}, time.Unix(11, 0).UTC())
	secondWorker.manifestStore = &fakeManifestStore{}
	secondWorker.archiveOutbox = secondOutbox
	secondWorker.cfg.MaxRecordsPerPart = 1
	secondDelivery := newFakeDelivery(t, envelope(2, "env-after-restart", 1600))

	if err := secondWorker.processDelivery(context.Background(), secondDelivery); err == nil {
		t.Fatalf("process delivery after restart should fail instead of overwriting pending outbox entry")
	}
	if secondDelivery.acked != 0 {
		t.Fatalf("second delivery ack count=%d want 0", secondDelivery.acked)
	}
	if secondDelivery.nacked != 1 {
		t.Fatalf("second delivery nack count=%d want 1", secondDelivery.nacked)
	}
	paths, err := secondOutbox.pendingPaths()
	if err != nil {
		t.Fatalf("pendingPaths failed: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("pending outbox files=%d want 1", len(paths))
	}
	entry, err := readArchiveOutboxEntry(paths[0])
	if err != nil {
		t.Fatalf("readArchiveOutboxEntry failed: %v", err)
	}
	if ids := decodeEnvelopeIDs(t, entry.Object.Body); len(ids) != 1 || ids[0] != "env-before-restart" {
		t.Fatalf("pending outbox entry was overwritten, ids=%v", ids)
	}
}

func TestArchiveUploadOutboxRequiresManifestStoreBeforeUpload(t *testing.T) {
	t.Parallel()

	outbox, err := NewFileArchiveUploadOutbox(FileArchiveUploadOutboxConfig{
		Dir:      t.TempDir(),
		MaxBytes: 64 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewFileArchiveUploadOutbox failed: %v", err)
	}
	store := &fakeObjectStore{}
	entry := ArchiveUploadOutboxEntry{
		Object: PutObjectRequest{
			Bucket:      "archive-bucket",
			Key:         "raw/2026/06/04/part-000.zst",
			Body:        []byte("compressed payload"),
			ContentType: defaultObjectContentType,
		},
		Manifest: &ManifestRecord{
			Provider:        "ecoflow",
			Shard:           1,
			ShardCount:      8,
			PartitionHour:   time.Unix(100, 0).UTC(),
			TSMinUnixMS:     1000,
			TSMaxUnixMS:     1500,
			RecordCount:     1,
			ObjectBucket:    "archive-bucket",
			ObjectKey:       "raw/2026/06/04/part-000.zst",
			ObjectSizeBytes: 18,
			ContentType:     defaultObjectContentType,
			Compression:     defaultManifestCompression,
			ChecksumCRC32:   "12345678",
			WriterID:        "writer-a",
		},
	}

	if err := outbox.Enqueue(context.Background(), entry); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if err := outbox.Flush(context.Background(), store, nil); err == nil {
		t.Fatalf("Flush without manifest store should fail")
	}
	pending, err := outbox.PendingCount(context.Background())
	if err != nil {
		t.Fatalf("PendingCount failed: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending entries=%d want 1", pending)
	}
	if len(store.requests) != 0 {
		t.Fatalf("remote object writes without manifest store=%d want 0", len(store.requests))
	}
}
