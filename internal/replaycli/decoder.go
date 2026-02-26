package replaycli

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

var zstdDecoder = mustNewDecoder()

func mustNewDecoder() *zstd.Decoder {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		panic(fmt.Sprintf("init replay zstd decoder: %v", err))
	}
	return decoder
}

func DecodeEnvelopeFrames(payload []byte) ([][]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	decoded, err := zstdDecoder.DecodeAll(payload, nil)
	if err != nil {
		return nil, fmt.Errorf("zstd decode archive object: %w", err)
	}
	reader := bytes.NewReader(decoded)
	out := make([][]byte, 0, 64)
	for {
		size, err := binary.ReadUvarint(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read envelope frame size: %w", err)
		}
		if size == 0 {
			return nil, fmt.Errorf("invalid envelope frame size: %d", size)
		}
		if uint64(reader.Len()) < size {
			return nil, fmt.Errorf("truncated envelope frame: declared=%d remaining=%d", size, reader.Len())
		}
		frame := make([]byte, int(size))
		if _, err := io.ReadFull(reader, frame); err != nil {
			return nil, fmt.Errorf("read envelope frame body: %w", err)
		}
		out = append(out, frame)
	}
	return out, nil
}
