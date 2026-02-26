package ecoflowserver

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type compressionCodec struct {
	name      string
	pool      *sync.Pool
	newReader func(io.Reader) (io.ReadCloser, error)
}

var extraCompressionCodecsProvider = func(_ CompressionConfig) ([]compressionCodec, error) {
	return nil, nil
}

func buildCodecs(cfg CompressionConfig) ([]compressionCodec, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	gzipLevel := cfg.GzipLevel
	if gzipLevel == 0 {
		gzipLevel = 5
	}
	deflateLevel := cfg.DeflateLevel
	if deflateLevel == 0 {
		deflateLevel = 5
	}

	gzipPool := &sync.Pool{
		New: func() any {
			writer, err := gzip.NewWriterLevel(io.Discard, gzipLevel)
			if err != nil {
				panic(err)
			}
			return writer
		},
	}
	deflatePool := &sync.Pool{
		New: func() any {
			writer, err := flate.NewWriter(io.Discard, deflateLevel)
			if err != nil {
				panic(err)
			}
			return writer
		},
	}

	codecs := []compressionCodec{
		{
			name: "gzip",
			pool: gzipPool,
			newReader: func(r io.Reader) (io.ReadCloser, error) {
				return gzip.NewReader(r)
			},
		},
		{
			name: "deflate",
			pool: deflatePool,
			newReader: func(r io.Reader) (io.ReadCloser, error) {
				return flate.NewReader(r), nil
			},
		},
	}

	extra, err := extraCompressionCodecsProvider(cfg)
	if err != nil {
		return nil, err
	}
	codecs = append(codecs, extra...)

	return codecs, nil
}

func compressionMiddleware(cfg CompressionConfig, next http.Handler) (http.Handler, error) {
	if !cfg.Enabled {
		return next, nil
	}

	codecs, err := buildCodecs(cfg)
	if err != nil {
		return nil, err
	}
	preferred := make([]string, 0, len(codecs))
	for _, codec := range codecs {
		preferred = append(preferred, codec.name)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decodeCloser, decodeErr := decodeRequestBody(r, codecs)
		if decodeErr != nil {
			http.Error(w, decodeErr.Error(), http.StatusUnsupportedMediaType)
			return
		}
		if decodeCloser != nil {
			defer func() { _ = decodeCloser.Close() }()
		}

		responseEncoding := negotiateEncoding(r.Header.Get("Accept-Encoding"), preferred)
		if responseEncoding == "" || !shouldCompress(r) {
			next.ServeHTTP(w, r)
			return
		}

		writer := newCompressResponseWriter(w, cfg.MinResponseBytes, responseEncoding, codecs)
		defer writer.close()
		next.ServeHTTP(writer, r)
	}), nil
}

func decodeRequestBody(r *http.Request, codecs []compressionCodec) (io.Closer, error) {
	raw := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding")))
	if raw == "" || raw == "identity" {
		return nil, nil
	}

	for _, codec := range codecs {
		if raw != codec.name {
			continue
		}

		reader, err := codec.newReader(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed decoding %s request body: %w", raw, err)
		}

		original := r.Body
		r.Body = &compositeReadCloser{
			Reader: reader,
			closeFn: func() error {
				_ = reader.Close()
				return original.Close()
			},
		}
		r.ContentLength = -1
		r.Header.Del("Content-Encoding")
		r.Header.Del("Content-Length")
		return r.Body, nil
	}

	return nil, fmt.Errorf("unsupported content-encoding %q", raw)
}

func shouldCompress(r *http.Request) bool {
	if r.Method == http.MethodHead {
		return false
	}
	upgrade := strings.ToLower(strings.TrimSpace(r.Header.Get("Upgrade")))
	return upgrade == ""
}

type compressResponseWriter struct {
	http.ResponseWriter

	minBytes int
	encoding string
	codec    compressionCodec

	status      int
	wroteHeader bool
	compressed  bool
	compressor  io.WriteCloser
	buffer      []byte
}

func newCompressResponseWriter(
	w http.ResponseWriter,
	minBytes int,
	encoding string,
	codecs []compressionCodec,
) *compressResponseWriter {
	var selected compressionCodec
	for _, codec := range codecs {
		if codec.name == encoding {
			selected = codec
			break
		}
	}
	return &compressResponseWriter{
		ResponseWriter: w,
		minBytes:       minBytes,
		encoding:       encoding,
		codec:          selected,
		status:         http.StatusOK,
		buffer:         make([]byte, 0, minBytes),
	}
}

func (w *compressResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *compressResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.status = statusCode
	w.wroteHeader = true
}

func (w *compressResponseWriter) Write(p []byte) (int, error) {
	originalLen := len(p)
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	// Delay compression decision until we have enough payload bytes.
	if w.compressor == nil && len(w.buffer) < w.minBytes {
		needed := w.minBytes - len(w.buffer)
		if needed > len(p) {
			needed = len(p)
		}
		w.buffer = append(w.buffer, p[:needed]...)
		p = p[needed:]
	}

	if w.compressor == nil && len(w.buffer) < w.minBytes {
		return originalLen, nil
	}

	if w.compressor == nil {
		w.startCompressed()
		if len(w.buffer) > 0 {
			if _, err := w.compressor.Write(w.buffer); err != nil {
				return 0, err
			}
			w.buffer = w.buffer[:0]
		}
	}
	if len(p) == 0 {
		return originalLen, nil
	}
	_, err := w.compressor.Write(p)
	if err != nil {
		return 0, err
	}
	return originalLen, nil
}

func (w *compressResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		if w.compressor == nil && len(w.buffer) > 0 {
			w.writeUncompressedHeader()
			_, _ = w.ResponseWriter.Write(w.buffer)
			w.buffer = w.buffer[:0]
		}
		if w.compressor != nil {
			type flusherWriter interface{ Flush() error }
			if fw, flushOK := w.compressor.(flusherWriter); flushOK {
				_ = fw.Flush()
			}
		}
		flusher.Flush()
	}
}

func (w *compressResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hj.Hijack()
}

func (w *compressResponseWriter) close() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.compressor != nil {
		_ = w.compressor.Close()
		w.compressor = nil
	}
	if len(w.buffer) > 0 {
		// Small payloads stay uncompressed.
		w.writeUncompressedHeader()
		_, _ = w.ResponseWriter.Write(w.buffer)
		w.buffer = w.buffer[:0]
		return
	}
	if !w.compressed && w.compressor == nil {
		w.writeUncompressedHeader()
	}
}

func (w *compressResponseWriter) startCompressed() {
	header := w.Header()
	header.Del("Content-Length")
	header.Set("Content-Encoding", w.encoding)
	setVaryAcceptEncoding(header)
	w.ResponseWriter.WriteHeader(w.status)

	pooled := w.codec.pool.Get()
	writer, ok := pooled.(resettableWriteCloser)
	if !ok {
		panic("codec writer type does not implement resettableWriteCloser: " + w.encoding)
	}
	writer.Reset(w.ResponseWriter)
	w.compressed = true
	w.compressor = &pooledWriter{
		writer: writer,
		pool:   w.codec.pool,
	}
}

func (w *compressResponseWriter) writeUncompressedHeader() {
	setVaryAcceptEncoding(w.Header())
	w.ResponseWriter.WriteHeader(w.status)
}

type pooledWriter struct {
	writer resettableWriteCloser
	pool   *sync.Pool
}

func (w *pooledWriter) Write(p []byte) (int, error) {
	return w.writer.Write(p)
}

func (w *pooledWriter) Close() error {
	err := w.writer.Close()
	w.writer.Reset(io.Discard)
	w.pool.Put(w.writer)
	return err
}

func (w *pooledWriter) Flush() error {
	type flusher interface {
		Flush() error
	}
	if fw, ok := w.writer.(flusher); ok {
		return fw.Flush()
	}
	return nil
}

type resettableWriteCloser interface {
	io.WriteCloser
	Reset(io.Writer)
}

type compositeReadCloser struct {
	io.Reader
	closeFn func() error
}

func (r *compositeReadCloser) Close() error {
	if r.closeFn == nil {
		return nil
	}
	return r.closeFn()
}

func setVaryAcceptEncoding(h http.Header) {
	values := strings.Split(h.Get("Vary"), ",")
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	for _, value := range values {
		if strings.EqualFold(value, "Accept-Encoding") {
			return
		}
	}
	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		h.Set("Vary", "Accept-Encoding")
		return
	}
	h.Set("Vary", h.Get("Vary")+", Accept-Encoding")
}

func negotiateEncoding(acceptEncoding string, preferred []string) string {
	acceptEncoding = strings.ToLower(strings.TrimSpace(acceptEncoding))
	if acceptEncoding == "" {
		return ""
	}

	type candidate struct {
		name string
		q    float64
	}
	best := candidate{}
	for _, token := range strings.Split(acceptEncoding, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		name, q := parseEncodingToken(token)
		if q <= 0 {
			continue
		}
		if name == "*" {
			for _, pref := range preferred {
				if q > best.q {
					best = candidate{name: pref, q: q}
				}
			}
			continue
		}
		if !slices.Contains(preferred, name) {
			continue
		}
		if q > best.q {
			best = candidate{name: name, q: q}
		}
	}
	return best.name
}

func parseEncodingToken(token string) (name string, q float64) {
	parts := strings.Split(token, ";")
	name = strings.TrimSpace(parts[0])
	q = 1.0
	for i := 1; i < len(parts); i++ {
		part := strings.TrimSpace(parts[i])
		if !strings.HasPrefix(part, "q=") {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimPrefix(part, "q="), 64)
		if err != nil {
			continue
		}
		q = parsed
	}
	return name, q
}
