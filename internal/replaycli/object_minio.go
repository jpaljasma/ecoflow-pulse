package replaycli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOObjectReaderConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Secure          bool
}

func DefaultMinIOObjectReaderConfig() MinIOObjectReaderConfig {
	return MinIOObjectReaderConfig{
		Endpoint:        "127.0.0.1:9000",
		AccessKeyID:     "minio",
		SecretAccessKey: "minio123",
		Region:          "us-east-1",
		Secure:          false,
	}
}

type MinIOObjectReader struct {
	client *minio.Client
}

func NewMinIOObjectReader(cfg MinIOObjectReaderConfig) (*MinIOObjectReader, error) {
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
		return nil, fmt.Errorf("init minio object reader: %w", err)
	}
	return &MinIOObjectReader{client: client}, nil
}

func (r *MinIOObjectReader) ReadObject(ctx context.Context, bucket string, key string) ([]byte, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("minio object reader is not initialized")
	}
	bucket = strings.TrimSpace(bucket)
	key = strings.Trim(strings.TrimSpace(key), "/")
	if bucket == "" {
		return nil, errors.New("object bucket is required")
	}
	if key == "" {
		return nil, errors.New("object key is required")
	}
	object, err := r.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object %s/%s: %w", bucket, key, err)
	}
	defer object.Close()
	body, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("read object %s/%s: %w", bucket, key, err)
	}
	return body, nil
}

func (r *MinIOObjectReader) Close() error {
	return nil
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
