package grpcmw

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"os"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

var (
	requestIDPrefix  [8]byte
	requestIDCounter atomic.Uint64
)

func init() {
	if _, err := rand.Read(requestIDPrefix[:]); err != nil {
		seed := uint64(time.Now().UnixNano()) ^ uint64(os.Getpid())
		binary.BigEndian.PutUint64(requestIDPrefix[:], seed)
	}
	requestIDCounter.Store(uint64(time.Now().UnixNano()))
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(requestIDKey).(string)
	return v, ok
}

func requestIDFromMetadata(md metadata.MD) string {
	// common conventions: x-request-id
	vals := md.Get("x-request-id")
	if len(vals) > 0 && vals[0] != "" {
		return vals[0]
	}
	return ""
}

func newRequestID() string {
	seq := requestIDCounter.Add(1)

	var raw [16]byte
	copy(raw[:8], requestIDPrefix[:])
	binary.BigEndian.PutUint64(raw[8:], seq)

	var enc [32]byte
	hex.Encode(enc[:], raw[:])
	return string(enc[:])
}

func RequestIDUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		rid := requestIDFromMetadata(md)
		if rid == "" {
			rid = newRequestID()
		}
		ctx = context.WithValue(ctx, requestIDKey, rid)
		return handler(ctx, req)
	}
}

func RequestIDStream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		rid := requestIDFromMetadata(md)
		if rid == "" {
			rid = newRequestID()
		}
		wrapped := &serverStream{ServerStream: ss, ctx: context.WithValue(ctx, requestIDKey, rid)}
		return handler(srv, wrapped)
	}
}

type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStream) Context() context.Context { return s.ctx }
