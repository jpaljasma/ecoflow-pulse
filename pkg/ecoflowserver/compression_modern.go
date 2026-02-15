//go:build moderncompress

package ecoflowserver

import (
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func init() {
	extraCompressionCodecsProvider = buildModernCodecs
}

func buildModernCodecs(cfg CompressionConfig) ([]compressionCodec, error) {
	brotliLevel := cfg.BrotliLevel
	if brotliLevel == 0 {
		brotliLevel = 5
	}
	zstdLevel := cfg.ZstdLevel
	if zstdLevel == 0 {
		zstdLevel = 3
	}

	brotliPool := &sync.Pool{
		New: func() any {
			return brotli.NewWriterLevel(io.Discard, brotliLevel)
		},
	}

	zstdPool := &sync.Pool{
		New: func() any {
			writer, err := zstd.NewWriter(io.Discard, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(zstdLevel)))
			if err != nil {
				panic(err)
			}
			return writer
		},
	}

	return []compressionCodec{
		{
			name: "br",
			pool: brotliPool,
			newReader: func(r io.Reader) (io.ReadCloser, error) {
				return io.NopCloser(brotli.NewReader(r)), nil
			},
		},
		{
			name: "zstd",
			pool: zstdPool,
			newReader: func(r io.Reader) (io.ReadCloser, error) {
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
