package archiveworker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	defaultObjectContentType = "application/x-protobuf+zstd"
)

type ObjectStoreProvider string

const (
	ObjectStoreProviderMinIO ObjectStoreProvider = "minio"
	ObjectStoreProviderGCS   ObjectStoreProvider = "gcs"
)

type PutObjectRequest struct {
	Bucket      string
	Key         string
	Body        []byte
	Metadata    map[string]string
	ContentType string
}

type ObjectStore interface {
	PutObject(ctx context.Context, request PutObjectRequest) error
}

type ObjectStoreConfig struct {
	Provider         ObjectStoreProvider
	Endpoint         string
	AccessKeyID      string
	SecretAccessKey  string
	Region           string
	Secure           bool
	AutoCreateBucket bool
	GCSProjectID     string
}

func DefaultObjectStoreConfig() ObjectStoreConfig {
	return ObjectStoreConfig{
		Provider:         ObjectStoreProviderMinIO,
		Endpoint:         "127.0.0.1:9000",
		AccessKeyID:      "minio",
		SecretAccessKey:  "minio123",
		Region:           "us-east-1",
		Secure:           false,
		AutoCreateBucket: true,
	}
}

type MinIOObjectStoreConfig struct {
	Endpoint         string
	AccessKeyID      string
	SecretAccessKey  string
	Region           string
	Secure           bool
	AutoCreateBucket bool
}

func DefaultMinIOObjectStoreConfig() MinIOObjectStoreConfig {
	defaults := DefaultObjectStoreConfig()
	return MinIOObjectStoreConfig{
		Endpoint:         defaults.Endpoint,
		AccessKeyID:      defaults.AccessKeyID,
		SecretAccessKey:  defaults.SecretAccessKey,
		Region:           defaults.Region,
		Secure:           defaults.Secure,
		AutoCreateBucket: defaults.AutoCreateBucket,
	}
}

type MinIOObjectStore struct {
	client           *minio.Client
	defaultRegion    string
	autoCreateBucket bool
	ensuredBuckets   sync.Map
}

func NewMinIOObjectStore(cfg MinIOObjectStoreConfig) (*MinIOObjectStore, error) {
	endpoint := normalizeObjectEndpoint(cfg.Endpoint)
	if endpoint == "" {
		return nil, errors.New("minio endpoint is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" {
		return nil, errors.New("minio access key is required")
	}
	if strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, errors.New("minio secret key is required")
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
		return nil, fmt.Errorf("init minio client: %w", err)
	}
	return &MinIOObjectStore{
		client:           client,
		defaultRegion:    region,
		autoCreateBucket: cfg.AutoCreateBucket,
	}, nil
}

type GCSObjectStore struct {
	client           *storage.Client
	autoCreateBucket bool
	ensuredBuckets   sync.Map
	projectID        string
}

func NewObjectStore(ctx context.Context, cfg ObjectStoreConfig) (ObjectStore, error) {
	cfg = normalizeObjectStoreConfig(cfg)
	switch cfg.Provider {
	case ObjectStoreProviderGCS:
		client, err := storage.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("init gcs client: %w", err)
		}
		return &GCSObjectStore{
			client:           client,
			autoCreateBucket: cfg.AutoCreateBucket,
			projectID:        strings.TrimSpace(cfg.GCSProjectID),
		}, nil
	case ObjectStoreProviderMinIO:
		return NewMinIOObjectStore(MinIOObjectStoreConfig{
			Endpoint:         cfg.Endpoint,
			AccessKeyID:      cfg.AccessKeyID,
			SecretAccessKey:  cfg.SecretAccessKey,
			Region:           cfg.Region,
			Secure:           cfg.Secure,
			AutoCreateBucket: cfg.AutoCreateBucket,
		})
	default:
		return nil, fmt.Errorf("unsupported object store provider %q", cfg.Provider)
	}
}

func (s *MinIOObjectStore) PutObject(ctx context.Context, request PutObjectRequest) error {
	if s == nil || s.client == nil {
		return errors.New("minio object store is not initialized")
	}
	request.Bucket = strings.TrimSpace(request.Bucket)
	request.Key = strings.Trim(strings.TrimSpace(request.Key), "/")
	if request.Bucket == "" {
		return errors.New("object bucket is required")
	}
	if request.Key == "" {
		return errors.New("object key is required")
	}
	if len(request.Body) == 0 {
		return errors.New("object body is required")
	}

	if err := s.ensureBucket(ctx, request.Bucket); err != nil {
		return err
	}

	contentType := strings.TrimSpace(request.ContentType)
	if contentType == "" {
		contentType = defaultObjectContentType
	}
	_, err := s.client.PutObject(
		ctx,
		request.Bucket,
		request.Key,
		bytes.NewReader(request.Body),
		int64(len(request.Body)),
		minio.PutObjectOptions{
			ContentType:  contentType,
			UserMetadata: copyMetadata(request.Metadata),
		},
	)
	if err != nil {
		return fmt.Errorf("put object %s/%s: %w", request.Bucket, request.Key, err)
	}
	return nil
}

func (s *GCSObjectStore) PutObject(ctx context.Context, request PutObjectRequest) error {
	if s == nil || s.client == nil {
		return errors.New("gcs object store is not initialized")
	}
	request.Bucket = strings.TrimSpace(request.Bucket)
	request.Key = strings.Trim(strings.TrimSpace(request.Key), "/")
	if request.Bucket == "" {
		return errors.New("object bucket is required")
	}
	if request.Key == "" {
		return errors.New("object key is required")
	}
	if len(request.Body) == 0 {
		return errors.New("object body is required")
	}
	if err := s.ensureBucket(ctx, request.Bucket); err != nil {
		return err
	}

	contentType := strings.TrimSpace(request.ContentType)
	if contentType == "" {
		contentType = defaultObjectContentType
	}
	writer := s.client.Bucket(request.Bucket).Object(request.Key).NewWriter(ctx)
	writer.ContentType = contentType
	writer.Metadata = copyMetadata(request.Metadata)
	if _, err := writer.Write(request.Body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("put gcs object %s/%s: %w", request.Bucket, request.Key, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close gcs writer %s/%s: %w", request.Bucket, request.Key, err)
	}
	return nil
}

func (s *MinIOObjectStore) ensureBucket(ctx context.Context, bucket string) error {
	if !s.autoCreateBucket {
		return nil
	}
	if _, ok := s.ensuredBuckets.Load(bucket); ok {
		return nil
	}
	exists, err := s.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check bucket %q existence: %w", bucket, err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: s.defaultRegion}); err != nil {
			resp := minio.ToErrorResponse(err)
			switch strings.TrimSpace(resp.Code) {
			case "BucketAlreadyOwnedByYou", "BucketAlreadyExists":
			default:
				return fmt.Errorf("create bucket %q: %w", bucket, err)
			}
		}
	}
	s.ensuredBuckets.Store(bucket, struct{}{})
	return nil
}

func (s *GCSObjectStore) ensureBucket(ctx context.Context, bucket string) error {
	if !s.autoCreateBucket {
		return nil
	}
	if _, ok := s.ensuredBuckets.Load(bucket); ok {
		return nil
	}
	if _, err := s.client.Bucket(bucket).Attrs(ctx); err == nil {
		s.ensuredBuckets.Store(bucket, struct{}{})
		return nil
	}
	if strings.TrimSpace(s.projectID) == "" {
		return fmt.Errorf("gcs bucket %q does not exist and ARCHIVE_OBJECT_GCS_PROJECT_ID is required for auto-create", bucket)
	}
	if err := s.client.Bucket(bucket).Create(ctx, s.projectID, nil); err != nil {
		return fmt.Errorf("create gcs bucket %q: %w", bucket, err)
	}
	s.ensuredBuckets.Store(bucket, struct{}{})
	return nil
}

func copyMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

func normalizeObjectStoreConfig(cfg ObjectStoreConfig) ObjectStoreConfig {
	if cfg.Provider == "" {
		cfg.Provider = DefaultObjectStoreConfig().Provider
	}
	cfg.Provider = ObjectStoreProvider(strings.ToLower(strings.TrimSpace(string(cfg.Provider))))
	if cfg.Provider == ObjectStoreProviderMinIO {
		defaults := DefaultObjectStoreConfig()
		if strings.TrimSpace(cfg.Region) == "" {
			cfg.Region = defaults.Region
		}
	}
	return cfg
}
