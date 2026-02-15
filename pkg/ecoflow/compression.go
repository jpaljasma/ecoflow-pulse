package ecoflow

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
)

const (
	// CompressionIdentity indicates uncompressed payload transport.
	CompressionIdentity = "identity"
	// CompressionGzip indicates gzip-compressed payload transport.
	CompressionGzip = "gzip"
	// CompressionDeflate indicates deflate-compressed payload transport.
	CompressionDeflate = "deflate"
	// CompressionBrotli indicates brotli-compressed payload transport.
	CompressionBrotli = "br"
	// CompressionZstd indicates zstd-compressed payload transport.
	CompressionZstd = "zstd"
)

// CompressionOptions controls request-body compression and accepted response
// compression algorithms.
type CompressionOptions struct {
	EnableRequestCompression  bool
	EnableResponseCompression bool

	RequestCompressionMinBytes  int
	RequestCompressionAlgorithm string

	ResponseAcceptedEncodings []string
}

// DefaultCompressionOptions returns network-throughput-oriented compression
// defaults for EcoFlow client calls.
func DefaultCompressionOptions() CompressionOptions {
	return CompressionOptions{
		EnableRequestCompression:    true,
		EnableResponseCompression:   true,
		RequestCompressionMinBytes:  0,
		RequestCompressionAlgorithm: CompressionGzip,
		ResponseAcceptedEncodings: []string{
			CompressionGzip,
			CompressionDeflate,
		},
	}
}

type compressionCodec struct {
	name   string
	encode func([]byte) ([]byte, error)
	decode func(io.Reader) (io.ReadCloser, error)
}

var extraClientCompressionCodecsProvider = func() ([]compressionCodec, error) {
	return nil, nil
}

type clientCompressionRuntime struct {
	enableRequest  bool
	enableResponse bool

	requestMinBytes      int
	requestAlgorithm     string
	acceptEncodingHeader string

	codecs map[string]compressionCodec
}

func newClientCompressionRuntime(options CompressionOptions) (clientCompressionRuntime, error) {
	codecs, err := buildClientCompressionCodecs()
	if err != nil {
		return clientCompressionRuntime{}, err
	}

	requestAlgorithm := strings.ToLower(strings.TrimSpace(options.RequestCompressionAlgorithm))
	if requestAlgorithm == "" {
		requestAlgorithm = CompressionGzip
	}
	if options.EnableRequestCompression {
		if _, ok := codecs[requestAlgorithm]; !ok {
			return clientCompressionRuntime{}, fmt.Errorf("unsupported request compression algorithm %q", requestAlgorithm)
		}
	}

	accepted, err := normalizeAcceptedEncodings(options.ResponseAcceptedEncodings, codecs)
	if err != nil {
		return clientCompressionRuntime{}, err
	}
	return clientCompressionRuntime{
		enableRequest:        options.EnableRequestCompression,
		enableResponse:       options.EnableResponseCompression,
		requestMinBytes:      max(options.RequestCompressionMinBytes, 0),
		requestAlgorithm:     requestAlgorithm,
		acceptEncodingHeader: strings.Join(accepted, ", "),
		codecs:               codecs,
	}, nil
}

func (r clientCompressionRuntime) maybeCompressBody(body []byte) ([]byte, string, error) {
	if !r.enableRequest || len(body) == 0 || len(body) < r.requestMinBytes {
		return body, "", nil
	}

	codec, ok := r.codecs[r.requestAlgorithm]
	if !ok {
		return nil, "", fmt.Errorf("unsupported request compression algorithm %q", r.requestAlgorithm)
	}

	compressed, err := codec.encode(body)
	if err != nil {
		return nil, "", fmt.Errorf("compress request body with %s: %w", codec.name, err)
	}
	if len(compressed) >= len(body) {
		// Skip compression if it does not shrink payload.
		return body, "", nil
	}
	return compressed, codec.name, nil
}

func (r clientCompressionRuntime) responseAcceptEncodingHeader() string {
	if !r.enableResponse {
		return ""
	}
	return r.acceptEncodingHeader
}

func (r clientCompressionRuntime) decodeResponseBody(response *http.Response) (io.ReadCloser, string, error) {
	if response == nil || response.Body == nil {
		return nil, "unknown", nil
	}

	encoding := compressionOrIdentity(response.Header.Get("Content-Encoding"))
	if encoding == CompressionIdentity {
		return response.Body, encoding, nil
	}

	codec, ok := r.codecs[encoding]
	if !ok {
		return nil, encoding, fmt.Errorf("unsupported response content-encoding %q", encoding)
	}
	reader, err := codec.decode(response.Body)
	if err != nil {
		return nil, encoding, fmt.Errorf("decode %s response body: %w", encoding, err)
	}
	return &mergedReadCloser{
		Reader: reader,
		closeFn: func() error {
			_ = reader.Close()
			return response.Body.Close()
		},
	}, encoding, nil
}

func normalizeAcceptedEncodings(values []string, codecs map[string]compressionCodec) ([]string, error) {
	if len(values) == 0 {
		values = []string{CompressionGzip, CompressionDeflate}
	}

	out := make([]string, 0, len(values))
	unsupported := make([]string, 0, 2)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || value == CompressionIdentity {
			continue
		}
		if _, ok := codecs[value]; !ok {
			unsupported = append(unsupported, value)
			continue
		}
		if slices.Contains(out, value) {
			continue
		}
		out = append(out, value)
	}
	if len(unsupported) > 0 {
		return nil, fmt.Errorf(
			"unsupported accepted encodings %q; enable modern codecs with -tags moderncompress for br/zstd",
			strings.Join(unsupported, ","),
		)
	}
	return out, nil
}

func buildClientCompressionCodecs() (map[string]compressionCodec, error) {
	codecs := map[string]compressionCodec{
		CompressionGzip: {
			name:   CompressionGzip,
			encode: compressGzip,
			decode: func(r io.Reader) (io.ReadCloser, error) {
				return gzip.NewReader(r)
			},
		},
		CompressionDeflate: {
			name:   CompressionDeflate,
			encode: compressDeflate,
			decode: func(r io.Reader) (io.ReadCloser, error) {
				return flate.NewReader(r), nil
			},
		},
	}

	extras, err := extraClientCompressionCodecsProvider()
	if err != nil {
		return nil, err
	}
	for _, codec := range extras {
		codecs[strings.ToLower(codec.name)] = codec
	}
	return codecs, nil
}

var gzipRequestWriterPool = sync.Pool{
	New: func() any {
		writer, err := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		if err != nil {
			panic(err)
		}
		return writer
	},
}

var deflateRequestWriterPool = sync.Pool{
	New: func() any {
		writer, err := flate.NewWriter(io.Discard, flate.BestSpeed)
		if err != nil {
			panic(err)
		}
		return writer
	},
}

func compressGzip(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzipRequestWriterPool.Get().(*gzip.Writer)
	writer.Reset(&buffer)
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		writer.Reset(io.Discard)
		gzipRequestWriterPool.Put(writer)
		return nil, err
	}
	if err := writer.Close(); err != nil {
		writer.Reset(io.Discard)
		gzipRequestWriterPool.Put(writer)
		return nil, err
	}
	writer.Reset(io.Discard)
	gzipRequestWriterPool.Put(writer)
	return append([]byte(nil), buffer.Bytes()...), nil
}

func compressDeflate(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := deflateRequestWriterPool.Get().(*flate.Writer)
	writer.Reset(&buffer)
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		writer.Reset(io.Discard)
		deflateRequestWriterPool.Put(writer)
		return nil, err
	}
	if err := writer.Close(); err != nil {
		writer.Reset(io.Discard)
		deflateRequestWriterPool.Put(writer)
		return nil, err
	}
	writer.Reset(io.Discard)
	deflateRequestWriterPool.Put(writer)
	return append([]byte(nil), buffer.Bytes()...), nil
}

type mergedReadCloser struct {
	io.Reader
	closeFn func() error
}

func (r *mergedReadCloser) Close() error {
	if r.closeFn == nil {
		return nil
	}
	return r.closeFn()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
