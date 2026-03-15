package rolluprebuild

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var archiveKeyShardPattern = regexp.MustCompile(`^([^/]+)/yyyy=(\d{4})/mm=(\d{2})/dd=(\d{2})/hh=(\d{2})/shard=(\d{3})/`)

func ListArchiveObjectsDirect(
	ctx context.Context,
	cfg replaycli.MinIOObjectReaderConfig,
	bucket string,
	prefix string,
	from time.Time,
	to time.Time,
	maxObjects int,
) ([]replaycli.ManifestObject, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, fmt.Errorf("archive bucket is required")
	}
	if !to.After(from) {
		return nil, fmt.Errorf("archive direct listing requires from < to")
	}
	client, err := minio.New(normalizeObjectEndpoint(cfg.Endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.Secure,
		Region: strings.TrimSpace(cfg.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("init minio archive lister: %w", err)
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = "raw"
	}

	seen := make(map[string]struct{})
	objects := make([]replaycli.ManifestObject, 0, 1024)
	for hour := from.UTC().Truncate(time.Hour); !hour.After(to.UTC()); hour = hour.Add(time.Hour) {
		hourPrefix := fmt.Sprintf(
			"%s/yyyy=%04d/mm=%02d/dd=%02d/hh=%02d/",
			prefix,
			hour.Year(),
			int(hour.Month()),
			hour.Day(),
			hour.Hour(),
		)
		for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
			Prefix:    hourPrefix,
			Recursive: true,
		}) {
			if object.Err != nil {
				return nil, fmt.Errorf("list archive objects for prefix %s: %w", hourPrefix, object.Err)
			}
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
			objects = append(objects, replaycli.ManifestObject{
				ObjectBucket:    bucket,
				ObjectKey:       key,
				ObjectSizeBytes: object.Size,
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

func sortManifestObjects(objects []replaycli.ManifestObject) {
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
