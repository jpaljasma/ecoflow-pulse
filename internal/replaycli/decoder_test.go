package replaycli

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestDecodeEnvelopeFrames(t *testing.T) {
	t.Parallel()

	payload := encodeFramedZstdPayload(t, [][]byte{
		[]byte("env-a"),
		[]byte("env-b"),
	})
	frames, err := DecodeEnvelopeFrames(payload)
	if err != nil {
		t.Fatalf("decode envelope frames: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("frame count mismatch: got=%d want=2", len(frames))
	}
	if string(frames[0]) != "env-a" || string(frames[1]) != "env-b" {
		t.Fatalf("decoded frames mismatch: got=%q,%q", frames[0], frames[1])
	}
}

func TestDecodeEnvelopeFramesRejectsTruncatedFrame(t *testing.T) {
	t.Parallel()

	payload := encodeFramedZstdPayload(t, [][]byte{
		[]byte{0x01, 0x02, 0x03},
	})
	decoded, err := zstdDecoder.DecodeAll(payload, nil)
	if err != nil {
		t.Fatalf("decode payload for tamper: %v", err)
	}
	if len(decoded) < 2 {
		t.Fatalf("decoded payload too short: %d", len(decoded))
	}
	decoded = decoded[:len(decoded)-1]

	reencoded := reencodeZstd(t, decoded)
	_, err = DecodeEnvelopeFrames(reencoded)
	if err == nil {
		t.Fatalf("expected truncated frame error")
	}
}

func encodeFramedZstdPayload(t *testing.T, frames [][]byte) []byte {
	t.Helper()
	var raw bytes.Buffer
	for _, frame := range frames {
		var sizePrefix [binary.MaxVarintLen64]byte
		n := binary.PutUvarint(sizePrefix[:], uint64(len(frame)))
		if _, err := raw.Write(sizePrefix[:n]); err != nil {
			t.Fatalf("write frame size: %v", err)
		}
		if _, err := raw.Write(frame); err != nil {
			t.Fatalf("write frame body: %v", err)
		}
	}
	return reencodeZstd(t, raw.Bytes())
}

func reencodeZstd(t *testing.T, raw []byte) []byte {
	t.Helper()
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("create zstd encoder: %v", err)
	}
	defer encoder.Close()
	return encoder.EncodeAll(raw, nil)
}
