//go:build moderncompress

package ecoflow

import (
	"bytes"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func init() {
	extraClientCompressionCodecsProvider = buildModernClientCompressionCodecs
}

func buildModernClientCompressionCodecs() ([]compressionCodec, error) {
	return []compressionCodec{
		{
			name:   CompressionBrotli,
			encode: compressBrotli,
			decode: func(r io.Reader) (io.ReadCloser, error) {
				return io.NopCloser(brotli.NewReader(r)), nil
			},
		},
		{
			name:   CompressionZstd,
			encode: compressZstd,
			decode: func(r io.Reader) (io.ReadCloser, error) {
				decoder, err := zstd.NewReader(r)
				if err != nil {
					return nil, err
				}
				return &zstdDecoderReadCloser{decoder: decoder}, nil
			},
		},
	}, nil
}

type zstdDecoderReadCloser struct {
	decoder *zstd.Decoder
}

func (z *zstdDecoderReadCloser) Read(p []byte) (int, error) {
	return z.decoder.Read(p)
}

func (z *zstdDecoderReadCloser) Close() error {
	z.decoder.Close()
	return nil
}

var brotliRequestWriterPool = sync.Pool{
	New: func() any {
		return brotli.NewWriterLevel(io.Discard, 3)
	},
}

var zstdRequestWriterPool = sync.Pool{
	New: func() any {
		writer, err := zstd.NewWriter(io.Discard, zstd.WithEncoderLevel(zstd.SpeedFastest))
		if err != nil {
			panic(err)
		}
		return writer
	},
}

func compressBrotli(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := brotliRequestWriterPool.Get().(*brotli.Writer)
	writer.Reset(&buffer)
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		writer.Reset(io.Discard)
		brotliRequestWriterPool.Put(writer)
		return nil, err
	}
	if err := writer.Close(); err != nil {
		writer.Reset(io.Discard)
		brotliRequestWriterPool.Put(writer)
		return nil, err
	}
	writer.Reset(io.Discard)
	brotliRequestWriterPool.Put(writer)
	return append([]byte(nil), buffer.Bytes()...), nil
}

func compressZstd(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zstdRequestWriterPool.Get().(*zstd.Encoder)
	writer.Reset(&buffer)
	if _, err := writer.Write(data); err != nil {
		writer.Reset(io.Discard)
		zstdRequestWriterPool.Put(writer)
		return nil, err
	}
	if err := writer.Close(); err != nil {
		writer.Reset(io.Discard)
		zstdRequestWriterPool.Put(writer)
		return nil, err
	}
	writer.Reset(io.Discard)
	zstdRequestWriterPool.Put(writer)
	return append([]byte(nil), buffer.Bytes()...), nil
}
