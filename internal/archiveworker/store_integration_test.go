//go:build integration

package archiveworker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestMinIOObjectStoreContractIntegration(t *testing.T) {
	t.Parallel()

	cfg, inspector := requireMinIOIntegration(t)
	store, err := NewMinIOObjectStore(cfg)
	if err != nil {
		t.Fatalf("init minio store failed: %v", err)
	}

	bucket := integrationBucketName("archive-int-contract")
	if err := ensureIntegrationBucket(context.Background(), inspector, bucket, cfg.Region); err != nil {
		t.Fatalf("ensure bucket failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	key := "raw/yyyy=2026/mm=02/dd=26/hh=11/shard=007/part-000001.pb.zst"
	body := []byte("archive-worker-contract-payload")
	req := PutObjectRequest{
		Bucket:      bucket,
		Key:         key,
		Body:        body,
		ContentType: "application/x-protobuf+zstd",
		Metadata: map[string]string{
			"envelopes": "3",
			"shard":     "7",
			"ts_min":    "1700000000000",
			"ts_max":    "1700000009999",
		},
	}
	if err := store.PutObject(ctx, req); err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	got, info, err := fetchIntegrationObject(ctx, inspector, bucket, key)
	if err != nil {
		t.Fatalf("fetch object failed: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("stored body mismatch: got=%q want=%q", string(got), string(body))
	}
	if strings.TrimSpace(info.ContentType) != req.ContentType {
		t.Fatalf("content type mismatch: got=%q want=%q", info.ContentType, req.ContentType)
	}
	for k, v := range req.Metadata {
		if got := findUserMetadata(info.UserMetadata, k); got != v {
			t.Fatalf("metadata mismatch key=%q got=%q want=%q metadata=%v", k, got, v, info.UserMetadata)
		}
	}
}

func TestMinIOObjectStoreFailureInjectionIntegration(t *testing.T) {
	t.Parallel()

	cfg, inspector := requireMinIOIntegration(t)
	bucket := integrationBucketName("archive-int-failure")
	if err := ensureIntegrationBucket(context.Background(), inspector, bucket, cfg.Region); err != nil {
		t.Fatalf("ensure bucket failed: %v", err)
	}

	t.Run("bad credentials", func(t *testing.T) {
		t.Parallel()
		bad := cfg
		bad.AccessKeyID = "invalid-user"
		bad.SecretAccessKey = "invalid-secret"
		bad.AutoCreateBucket = false

		store, err := NewMinIOObjectStore(bad)
		if err != nil {
			t.Fatalf("init minio store with bad credentials failed unexpectedly: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err = store.PutObject(ctx, PutObjectRequest{
			Bucket: bucket,
			Key:    "raw/invalid-creds-test.pb.zst",
			Body:   []byte("x"),
		})
		if err == nil {
			t.Fatalf("expected PutObject failure for bad credentials")
		}
	})

	t.Run("bad endpoint", func(t *testing.T) {
		t.Parallel()
		bad := cfg
		bad.Endpoint = "127.0.0.1:1"
		bad.AutoCreateBucket = false

		store, err := NewMinIOObjectStore(bad)
		if err != nil {
			t.Fatalf("init minio store with bad endpoint failed unexpectedly: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = store.PutObject(ctx, PutObjectRequest{
			Bucket: bucket,
			Key:    "raw/invalid-endpoint-test.pb.zst",
			Body:   []byte("x"),
		})
		if err == nil {
			t.Fatalf("expected PutObject failure for bad endpoint")
		}
	})
}

func requireMinIOIntegration(t *testing.T) (MinIOObjectStoreConfig, *minio.Client) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("ARCHIVE_STORE_INTEGRATION")) != "1" {
		t.Skip("set ARCHIVE_STORE_INTEGRATION=1 to run archive object-store integration tests")
	}

	cfg := DefaultMinIOObjectStoreConfig()
	if v := strings.TrimSpace(os.Getenv("ARCHIVE_OBJECT_ENDPOINT")); v != "" {
		cfg.Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv("ARCHIVE_OBJECT_ACCESS_KEY")); v != "" {
		cfg.AccessKeyID = v
	}
	if v := strings.TrimSpace(os.Getenv("ARCHIVE_OBJECT_SECRET_KEY")); v != "" {
		cfg.SecretAccessKey = v
	}
	if v := strings.TrimSpace(os.Getenv("ARCHIVE_OBJECT_REGION")); v != "" {
		cfg.Region = v
	}
	cfg.AutoCreateBucket = true
	cfg.Secure = false
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		t.Skip("archive integration requires ARCHIVE_OBJECT_ENDPOINT/ACCESS_KEY/SECRET_KEY")
	}

	client, err := minio.New(normalizeObjectEndpoint(cfg.Endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.Secure,
		Region: cfg.Region,
	})
	if err != nil {
		t.Fatalf("init minio integration inspector failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ListBuckets(ctx); err != nil {
		t.Fatalf("list buckets failed: %v", err)
	}
	return cfg, client
}

func ensureIntegrationBucket(ctx context.Context, client *minio.Client, bucket string, region string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: strings.TrimSpace(region)})
}

func fetchIntegrationObject(ctx context.Context, client *minio.Client, bucket string, key string) ([]byte, minio.ObjectInfo, error) {
	reader, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}
	defer reader.Close()
	info, err := reader.Stat()
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}
	return body, info, nil
}

func findUserMetadata(metadata map[string]string, key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return ""
	}
	withPrefix := "x-amz-meta-" + key
	for k, v := range metadata {
		normalized := strings.ToLower(strings.TrimSpace(k))
		switch normalized {
		case key, withPrefix:
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func integrationBucketName(prefix string) string {
	return fmt.Sprintf("%s-%d", strings.Trim(prefix, "-"), time.Now().UTC().UnixNano())
}
