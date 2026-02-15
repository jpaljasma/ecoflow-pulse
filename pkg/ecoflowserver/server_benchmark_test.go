package ecoflowserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkCompressionMiddleware_Gzip(b *testing.B) {
	handler, err := compressionMiddleware(
		CompressionConfig{
			Enabled:          true,
			MinResponseBytes: 256,
			GzipLevel:        1,
			DeflateLevel:     1,
		},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
		}),
	)
	if err != nil {
		b.Fatalf("compressionMiddleware() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	request.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
		}
	})
}
