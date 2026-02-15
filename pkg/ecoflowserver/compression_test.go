package ecoflowserver

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressionMiddleware_CompressesGzipResponse(t *testing.T) {
	t.Parallel()

	handler, err := compressionMiddleware(
		CompressionConfig{
			Enabled:          true,
			MinResponseBytes: 10,
			GzipLevel:        1,
			DeflateLevel:     1,
		},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(strings.Repeat("a", 4096)))
		}),
	)
	if err != nil {
		t.Fatalf("compressionMiddleware() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/payload", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding mismatch: got %q", got)
	}

	reader, err := gzip.NewReader(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	if len(decoded) != 4096 {
		t.Fatalf("decoded length mismatch: got %d", len(decoded))
	}
}

func TestCompressionMiddleware_LeavesSmallResponseUncompressed(t *testing.T) {
	t.Parallel()

	handler, err := compressionMiddleware(
		CompressionConfig{
			Enabled:          true,
			MinResponseBytes: 1024,
			GzipLevel:        1,
			DeflateLevel:     1,
		},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("tiny"))
		}),
	)
	if err != nil {
		t.Fatalf("compressionMiddleware() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/tiny", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("expected no content encoding, got %q", got)
	}
	if body := recorder.Body.String(); body != "tiny" {
		t.Fatalf("body mismatch: got %q", body)
	}
}

func TestCompressionMiddleware_DecodesGzipRequestBody(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"hello":"world"}`)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write(payload)
	_ = writer.Close()

	handler, err := compressionMiddleware(
		CompressionConfig{
			Enabled:          true,
			MinResponseBytes: 0,
			GzipLevel:        1,
			DeflateLevel:     1,
		},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			defer r.Body.Close()
			if !bytes.Equal(body, payload) {
				t.Fatalf("decoded body mismatch: got %q", string(body))
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	if err != nil {
		t.Fatalf("compressionMiddleware() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(compressed.Bytes()))
	request.Header.Set("Content-Encoding", "gzip")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status mismatch: got %d", recorder.Code)
	}
}

func TestNegotiateEncoding_QValues(t *testing.T) {
	t.Parallel()

	got := negotiateEncoding("gzip;q=0.4, deflate;q=0.9", []string{"gzip", "deflate"})
	if got != "deflate" {
		t.Fatalf("encoding mismatch: got %q", got)
	}
}
