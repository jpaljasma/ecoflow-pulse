package ecoflow

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewClient_UnsupportedAcceptEncodingFails(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.Compression.ResponseAcceptedEncodings = []string{"x-unsupported"}

	_, err = NewClient(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported response encodings, got nil")
	}
}

func TestClient_Do_CompressesRequestBodyWithGzip(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.Compression = CompressionOptions{
		EnableRequestCompression:    true,
		EnableResponseCompression:   true,
		RequestCompressionMinBytes:  1,
		RequestCompressionAlgorithm: CompressionGzip,
		ResponseAcceptedEncodings:   []string{CompressionGzip},
	}
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Content-Encoding"); got != CompressionGzip {
				t.Fatalf("Content-Encoding mismatch: got %q", got)
			}
			if got := req.Header.Get("Accept-Encoding"); !strings.Contains(got, CompressionGzip) {
				t.Fatalf("Accept-Encoding missing gzip: got %q", got)
			}

			compressedBody, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body error: %v", err)
			}
			reader, err := gzip.NewReader(bytes.NewReader(compressedBody))
			if err != nil {
				t.Fatalf("gzip.NewReader() error = %v", err)
			}
			defer func() { _ = reader.Close() }()

			decodedBody, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("read decoded body error: %v", err)
			}

			var payload map[string]string
			if err := json.Unmarshal(decodedBody, &payload); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if payload["payload"] == "" {
				t.Fatalf("decoded payload missing")
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		}),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Do(context.Background(), Request{
		Method: http.MethodPost,
		Path:   "/test",
		Body: map[string]string{
			"payload": strings.Repeat("abc123", 1024),
		},
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestClient_Do_DecodesGzipResponseBody(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.Compression = CompressionOptions{
		EnableRequestCompression:    false,
		EnableResponseCompression:   true,
		RequestCompressionMinBytes:  1024,
		RequestCompressionAlgorithm: CompressionGzip,
		ResponseAcceptedEncodings:   []string{CompressionGzip},
	}

	originalBody := `{"result":"ok"}`
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write([]byte(originalBody))
	_ = writer.Close()

	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Accept-Encoding"); !strings.Contains(got, CompressionGzip) {
				t.Fatalf("Accept-Encoding missing gzip: got %q", got)
			}
			header := make(http.Header)
			header.Set("Content-Encoding", CompressionGzip)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(bytes.NewReader(compressed.Bytes())),
			}, nil
		}),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	response, err := client.Do(context.Background(), Request{
		Method: http.MethodGet,
		Path:   "/test",
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if got := string(response.Body); got != originalBody {
		t.Fatalf("response body mismatch: got %q", got)
	}
}
