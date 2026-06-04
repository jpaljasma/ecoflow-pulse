package archiveworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const archiveOutboxFileMode = 0o600

type ArchiveUploadOutbox interface {
	Enqueue(ctx context.Context, entry ArchiveUploadOutboxEntry) error
	Flush(ctx context.Context, store ObjectStore, manifest ManifestStore) error
	PendingCount(ctx context.Context) (int, error)
}

type ArchiveUploadOutboxEntry struct {
	Object    PutObjectRequest `json:"object"`
	Manifest  *ManifestRecord  `json:"manifest,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

type FileArchiveUploadOutboxConfig struct {
	Dir      string
	MaxBytes int64
}

type FileArchiveUploadOutbox struct {
	dir      string
	maxBytes int64
	now      func() time.Time
}

func NewFileArchiveUploadOutbox(cfg FileArchiveUploadOutboxConfig) (*FileArchiveUploadOutbox, error) {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		return nil, errors.New("archive upload outbox dir is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create archive upload outbox dir: %w", err)
	}
	return &FileArchiveUploadOutbox{
		dir:      dir,
		maxBytes: cfg.MaxBytes,
		now:      utcNow,
	}, nil
}

func (o *FileArchiveUploadOutbox) Enqueue(ctx context.Context, entry ArchiveUploadOutboxEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if o == nil {
		return errors.New("archive upload outbox is not initialized")
	}
	normalized, err := normalizeArchiveOutboxEntry(entry, o.now())
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal archive upload outbox entry: %w", err)
	}
	path := o.entryPath(normalized)
	if err := o.checkCapacity(path, int64(len(body))); err != nil {
		return err
	}
	if err := writeFileAtomicSyncNoReplace(path, body, archiveOutboxFileMode); err != nil {
		return fmt.Errorf("persist archive upload outbox entry: %w", err)
	}
	return nil
}

func (o *FileArchiveUploadOutbox) Flush(ctx context.Context, store ObjectStore, manifest ManifestStore) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if o == nil {
		return nil
	}
	if store == nil {
		return errors.New("archive upload outbox object store is required")
	}
	paths, err := o.pendingPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, err := readArchiveOutboxEntry(path)
		if err != nil {
			return err
		}
		if entry.Manifest != nil && manifest == nil {
			return errors.New("archive upload outbox manifest store is required")
		}
		if err := store.PutObject(ctx, entry.Object); err != nil {
			return fmt.Errorf("upload archive outbox object %s/%s: %w", entry.Object.Bucket, entry.Object.Key, err)
		}
		if manifest != nil && entry.Manifest != nil {
			if err := manifest.UpsertObjectManifest(ctx, *entry.Manifest); err != nil {
				return fmt.Errorf("persist archive outbox manifest %s/%s: %w", entry.Object.Bucket, entry.Object.Key, err)
			}
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove uploaded archive outbox entry: %w", err)
		}
		if err := syncDir(o.dir); err != nil {
			return fmt.Errorf("sync archive upload outbox dir after remove: %w", err)
		}
	}
	return nil
}

func (o *FileArchiveUploadOutbox) PendingCount(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if o == nil {
		return 0, nil
	}
	paths, err := o.pendingPaths()
	if err != nil {
		return 0, err
	}
	return len(paths), nil
}

func (o *FileArchiveUploadOutbox) pendingPaths() ([]string, error) {
	entries, err := os.ReadDir(o.dir)
	if err != nil {
		return nil, fmt.Errorf("read archive upload outbox dir: %w", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		paths = append(paths, filepath.Join(o.dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func (o *FileArchiveUploadOutbox) entryPath(entry ArchiveUploadOutboxEntry) string {
	sum := sha256.Sum256([]byte(entry.Object.Bucket + "\x00" + entry.Object.Key))
	return filepath.Join(o.dir, hex.EncodeToString(sum[:])+".json")
}

func (o *FileArchiveUploadOutbox) checkCapacity(path string, entryBytes int64) error {
	if o.maxBytes <= 0 {
		return nil
	}
	entries, err := os.ReadDir(o.dir)
	if err != nil {
		return fmt.Errorf("read archive upload outbox dir for capacity: %w", err)
	}
	var used int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat archive upload outbox entry: %w", err)
		}
		entryPath := filepath.Join(o.dir, entry.Name())
		if entryPath == path {
			continue
		}
		used += info.Size()
	}
	if used+entryBytes > o.maxBytes {
		return fmt.Errorf("archive upload outbox capacity exceeded: used=%d entry=%d max=%d", used, entryBytes, o.maxBytes)
	}
	return nil
}

func readArchiveOutboxEntry(path string) (ArchiveUploadOutboxEntry, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return ArchiveUploadOutboxEntry{}, fmt.Errorf("read archive upload outbox entry: %w", err)
	}
	var entry ArchiveUploadOutboxEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		return ArchiveUploadOutboxEntry{}, fmt.Errorf("decode archive upload outbox entry: %w", err)
	}
	return normalizeArchiveOutboxEntry(entry, utcNow())
}

func normalizeArchiveOutboxEntry(entry ArchiveUploadOutboxEntry, now time.Time) (ArchiveUploadOutboxEntry, error) {
	out := entry
	out.Object.Bucket = strings.TrimSpace(out.Object.Bucket)
	out.Object.Key = strings.Trim(strings.TrimSpace(out.Object.Key), "/")
	out.Object.ContentType = strings.TrimSpace(out.Object.ContentType)
	if out.Object.ContentType == "" {
		out.Object.ContentType = defaultObjectContentType
	}
	if out.Object.Bucket == "" {
		return ArchiveUploadOutboxEntry{}, errors.New("archive upload outbox object bucket is required")
	}
	if out.Object.Key == "" {
		return ArchiveUploadOutboxEntry{}, errors.New("archive upload outbox object key is required")
	}
	if len(out.Object.Body) == 0 {
		return ArchiveUploadOutboxEntry{}, errors.New("archive upload outbox object body is required")
	}
	out.Object.Metadata = copyMetadata(out.Object.Metadata)
	if out.Manifest != nil {
		record := normalizeManifestRecord(*out.Manifest, now)
		out.Manifest = &record
	}
	if out.CreatedAt.IsZero() {
		out.CreatedAt = now.UTC()
	} else {
		out.CreatedAt = out.CreatedAt.UTC()
	}
	return out, nil
}

func writeFileAtomicSyncNoReplace(path string, body []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("archive upload outbox entry already exists: %w", err)
		}
		return err
	}
	if err := os.Remove(tmpName); err != nil {
		return err
	}
	return syncDir(dir)
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()
	return handle.Sync()
}
