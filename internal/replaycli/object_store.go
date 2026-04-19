package replaycli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/api/iterator"
)

type ObjectProvider string

const (
	ObjectProviderMinIO ObjectProvider = "minio"
	ObjectProviderGCS   ObjectProvider = "gcs"
)

type ObjectReaderConfig struct {
	Provider        ObjectProvider
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Secure          bool
	GCSProjectID    string
}

func DefaultObjectReaderConfig() ObjectReaderConfig {
	return ObjectReaderConfig{
		Provider:        ObjectProviderMinIO,
		Endpoint:        "127.0.0.1:9000",
		AccessKeyID:     "minio",
		SecretAccessKey: "minio123",
		Region:          "us-east-1",
		Secure:          false,
	}
}

type MinIOObjectReaderConfig = ObjectReaderConfig

func DefaultMinIOObjectReaderConfig() MinIOObjectReaderConfig {
	return DefaultObjectReaderConfig()
}

type directObjectInfo struct {
	Key       string
	SizeBytes int64
}

type objectReaderAdapter struct {
	readObject func(ctx context.Context, bucket string, key string) ([]byte, error)
	close      func() error
}

func (r *objectReaderAdapter) ReadObject(ctx context.Context, bucket string, key string) ([]byte, error) {
	return r.readObject(ctx, bucket, key)
}

func (r *objectReaderAdapter) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}

type MinIOObjectReader struct {
	*objectReaderAdapter
}

func NewMinIOObjectReader(cfg MinIOObjectReaderConfig) (*MinIOObjectReader, error) {
	cfg.Provider = ObjectProviderMinIO
	reader, err := NewObjectReader(cfg)
	if err != nil {
		return nil, err
	}
	return &MinIOObjectReader{
		objectReaderAdapter: reader.(*objectReaderAdapter),
	}, nil
}

func NewObjectReader(cfg ObjectReaderConfig) (ObjectReader, error) {
	cfg = normalizeObjectReaderConfig(cfg)
	switch cfg.Provider {
	case ObjectProviderGCS:
		client, err := newGCSClient(context.Background(), cfg)
		if err != nil {
			return nil, err
		}
		return &objectReaderAdapter{
			readObject: func(ctx context.Context, bucket string, key string) ([]byte, error) {
				bucket = strings.TrimSpace(bucket)
				key = strings.Trim(strings.TrimSpace(key), "/")
				if bucket == "" {
					return nil, errors.New("object bucket is required")
				}
				if key == "" {
					return nil, errors.New("object key is required")
				}
				reader, err := client.Bucket(bucket).Object(key).NewReader(ctx)
				if err != nil {
					return nil, fmt.Errorf("get gcs object %s/%s: %w", bucket, key, err)
				}
				defer func() { _ = reader.Close() }()
				body, err := io.ReadAll(reader)
				if err != nil {
					return nil, fmt.Errorf("read gcs object %s/%s: %w", bucket, key, err)
				}
				return body, nil
			},
			close: client.Close,
		}, nil
	case ObjectProviderMinIO:
		client, region, err := newMinIOClient(cfg)
		if err != nil {
			return nil, err
		}
		_ = region
		return &objectReaderAdapter{
			readObject: func(ctx context.Context, bucket string, key string) ([]byte, error) {
				bucket = strings.TrimSpace(bucket)
				key = strings.Trim(strings.TrimSpace(key), "/")
				if bucket == "" {
					return nil, errors.New("object bucket is required")
				}
				if key == "" {
					return nil, errors.New("object key is required")
				}
				object, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
				if err != nil {
					return nil, fmt.Errorf("get object %s/%s: %w", bucket, key, err)
				}
				defer func() { _ = object.Close() }()
				body, err := io.ReadAll(object)
				if err != nil {
					return nil, fmt.Errorf("read object %s/%s: %w", bucket, key, err)
				}
				return body, nil
			},
			close: func() error { return nil },
		}, nil
	default:
		return nil, fmt.Errorf("unsupported object provider %q", cfg.Provider)
	}
}

var archiveKeyShardPattern = regexp.MustCompile(`^([^/]+)/yyyy=(\d{4})/mm=(\d{2})/dd=(\d{2})/hh=(\d{2})/shard=(\d{3})/`)

func ListObjectsDirect(
	ctx context.Context,
	cfg ObjectReaderConfig,
	bucket string,
	prefix string,
	from time.Time,
	to time.Time,
	maxObjects int,
) ([]ManifestObject, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, fmt.Errorf("archive bucket is required")
	}
	if !to.After(from) {
		return nil, fmt.Errorf("archive direct listing requires from < to")
	}
	cfg = normalizeObjectReaderConfig(cfg)
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = "raw"
	}

	var (
		listObjects func(ctx context.Context, bucket string, hourPrefix string) ([]directObjectInfo, error)
		closeFunc   func() error
	)

	switch cfg.Provider {
	case ObjectProviderGCS:
		client, err := newGCSClient(ctx, cfg)
		if err != nil {
			return nil, err
		}
		closeFunc = client.Close
		listObjects = func(ctx context.Context, bucket string, hourPrefix string) ([]directObjectInfo, error) {
			objects := make([]directObjectInfo, 0, 64)
			it := client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: hourPrefix})
			for {
				attrs, err := it.Next()
				if errors.Is(err, iterator.Done) {
					break
				}
				if err != nil {
					return nil, fmt.Errorf("list gcs archive objects for prefix %s: %w", hourPrefix, err)
				}
				objects = append(objects, directObjectInfo{
					Key:       attrs.Name,
					SizeBytes: attrs.Size,
				})
			}
			return objects, nil
		}
	case ObjectProviderMinIO:
		client, _, err := newMinIOClient(cfg)
		if err != nil {
			return nil, err
		}
		closeFunc = func() error { return nil }
		listObjects = func(ctx context.Context, bucket string, hourPrefix string) ([]directObjectInfo, error) {
			objects := make([]directObjectInfo, 0, 64)
			for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
				Prefix:    hourPrefix,
				Recursive: true,
			}) {
				if object.Err != nil {
					return nil, fmt.Errorf("list archive objects for prefix %s: %w", hourPrefix, object.Err)
				}
				objects = append(objects, directObjectInfo{
					Key:       object.Key,
					SizeBytes: object.Size,
				})
			}
			return objects, nil
		}
	default:
		return nil, fmt.Errorf("unsupported object provider %q", cfg.Provider)
	}
	defer func() { _ = closeFunc() }()

	seen := make(map[string]struct{})
	objects := make([]ManifestObject, 0, 1024)
	for _, hour := range archiveListingHours(from, to) {
		hourPrefix := fmt.Sprintf(
			"%s/yyyy=%04d/mm=%02d/dd=%02d/hh=%02d/",
			prefix,
			hour.Year(),
			int(hour.Month()),
			hour.Day(),
			hour.Hour(),
		)
		listed, err := listObjects(ctx, bucket, hourPrefix)
		if err != nil {
			return nil, err
		}
		for _, object := range listed {
			key := strings.Trim(strings.TrimSpace(object.Key), "/")
			if key == "" {
				continue
			}
			seenKey := bucket + "|" + key
			if _, exists := seen[seenKey]; exists {
				continue
			}
			seen[seenKey] = struct{}{}
			parsedPrefix, partitionHour, shard, err := parseArchiveObjectKey(key)
			if err != nil {
				continue
			}
			if parsedPrefix != prefix {
				continue
			}
			objects = append(objects, ManifestObject{
				ObjectBucket:    bucket,
				ObjectKey:       key,
				ObjectSizeBytes: object.SizeBytes,
				PartitionHour:   partitionHour,
				Shard:           shard,
				ShardCount:      0,
			})
			if maxObjects > 0 && len(objects) >= maxObjects {
				sortManifestObjects(objects)
				return objects, nil
			}
		}
	}
	sortManifestObjects(objects)
	return objects, nil
}

func archiveListingHours(from time.Time, to time.Time) []time.Time {
	startHour := from.UTC().Truncate(time.Hour)
	endUTC := to.UTC()
	endHour := endUTC.Truncate(time.Hour)
	includeEndHour := !endUTC.Equal(endHour)

	hours := make([]time.Time, 0, int(endHour.Sub(startHour)/time.Hour)+1)
	for hour := startHour; hour.Before(endHour) || (includeEndHour && hour.Equal(endHour)); hour = hour.Add(time.Hour) {
		hours = append(hours, hour)
	}
	return hours
}

func parseArchiveObjectKey(key string) (string, time.Time, uint32, error) {
	matches := archiveKeyShardPattern.FindStringSubmatch(strings.Trim(strings.TrimSpace(key), "/"))
	if len(matches) != 7 {
		return "", time.Time{}, 0, fmt.Errorf("archive key does not match raw layout")
	}
	partitionHour, err := time.Parse("2006-01-02-15", matches[2]+"-"+matches[3]+"-"+matches[4]+"-"+matches[5])
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("parse archive partition hour: %w", err)
	}
	var shard uint32
	if _, err := fmt.Sscanf(matches[6], "%d", &shard); err != nil {
		return "", time.Time{}, 0, fmt.Errorf("parse archive shard: %w", err)
	}
	return matches[1], partitionHour.UTC(), shard, nil
}

func sortManifestObjects(objects []ManifestObject) {
	sort.Slice(objects, func(i, j int) bool {
		if !objects[i].PartitionHour.Equal(objects[j].PartitionHour) {
			return objects[i].PartitionHour.Before(objects[j].PartitionHour)
		}
		if objects[i].Shard != objects[j].Shard {
			return objects[i].Shard < objects[j].Shard
		}
		return objects[i].ObjectKey < objects[j].ObjectKey
	})
}

func normalizeObjectReaderConfig(cfg ObjectReaderConfig) ObjectReaderConfig {
	if cfg.Provider == "" {
		cfg.Provider = DefaultObjectReaderConfig().Provider
	}
	cfg.Provider = ObjectProvider(strings.ToLower(strings.TrimSpace(string(cfg.Provider))))
	if cfg.Provider == ObjectProviderMinIO {
		defaults := DefaultObjectReaderConfig()
		if strings.TrimSpace(cfg.Region) == "" {
			cfg.Region = defaults.Region
		}
	}
	return cfg
}

func newMinIOClient(cfg ObjectReaderConfig) (*minio.Client, string, error) {
	endpoint := normalizeObjectEndpoint(cfg.Endpoint)
	if endpoint == "" {
		return nil, "", errors.New("minio endpoint is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" {
		return nil, "", errors.New("minio access key is required")
	}
	if strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, "", errors.New("minio secret key is required")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.Secure,
		Region: region,
	})
	if err != nil {
		return nil, "", fmt.Errorf("init minio object reader: %w", err)
	}
	return client, region, nil
}

func newGCSClient(ctx context.Context, cfg ObjectReaderConfig) (*storage.Client, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("init gcs object client: %w", err)
	}
	return client, nil
}

func normalizeObjectEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil && strings.TrimSpace(parsed.Host) != "" {
			return strings.TrimSpace(parsed.Host)
		}
	}
	return strings.TrimSpace(raw)
}
