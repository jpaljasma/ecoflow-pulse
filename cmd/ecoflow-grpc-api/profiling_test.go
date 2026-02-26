package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func startProfilingServer(t *testing.T) (telemetryv1.TelemetryServiceClient, func()) {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := grpcserver.DefaultConfig("local")
	cfg.ListenAddr = "127.0.0.1:0"

	unary := []grpc.UnaryServerInterceptor{
		grpcmw.RequestIDUnary(),
		grpcmw.RecoveryUnary(),
		grpcmw.AuthUnary(grpcmw.NoopAuthorizer{}),
		grpcmw.LoggingUnary(log),
	}
	stream := []grpc.StreamServerInterceptor{
		grpcmw.RequestIDStream(),
		grpcmw.RecoveryStream(),
		grpcmw.AuthStream(grpcmw.NoopAuthorizer{}),
		grpcmw.LoggingStream(log),
	}

	s, lis, err := grpcserver.New(cfg, unary, stream)
	if err != nil {
		t.Fatalf("grpcserver.New failed: %v", err)
	}

	telemetryv1.RegisterTelemetryServiceServer(s, NewTelemetryService(log))
	healthpb.RegisterHealthServer(s, healthpb.UnimplementedHealthServer{})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = grpcserver.ServeWithSignal(ctx, s, lis, 3*time.Second)
	}()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		wg.Wait()
		t.Fatalf("grpc dial failed: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		cancel()
		wg.Wait()
	}
	return telemetryv1.NewTelemetryServiceClient(conn), cleanup
}

func TestTelemetryServerNoGoroutineLeakAfterStreamBursts(t *testing.T) {
	t.Parallel()

	client, cleanup := startProfilingServer(t)
	defer cleanup()

	runtime.GC()
	runtime.GC()
	baseline := runtime.NumGoroutine()

	const bursts = 200
	for i := 0; i < bursts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		stream, err := client.Subscribe(ctx, &telemetryv1.SubscribeRequest{
			DeviceId:               fmt.Sprintf("dev-%d", i),
			IncludeInitialSnapshot: true,
		})
		if err != nil {
			cancel()
			t.Fatalf("subscribe failed: %v", err)
		}

		// Snapshot should arrive first; then we cancel to close stream quickly.
		if _, err := stream.Recv(); err != nil {
			cancel()
			t.Fatalf("recv snapshot failed: %v", err)
		}
		cancel()
	}

	// Give gRPC stream teardown time to complete.
	time.Sleep(750 * time.Millisecond)
	runtime.GC()
	runtime.GC()
	after := runtime.NumGoroutine()
	t.Logf("goroutines baseline=%d after=%d", baseline, after)

	// Allow small background variance from grpc internals.
	const maxGrowth = 20
	if after > baseline+maxGrowth {
		t.Fatalf("possible goroutine leak detected: baseline=%d after=%d growth=%d", baseline, after, after-baseline)
	}
}

func TestTelemetryServerHeapStableUnderSnapshotLoad(t *testing.T) {
	t.Parallel()

	client, cleanup := startProfilingServer(t)
	defer cleanup()

	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	const requests = 5000
	for i := 0; i < requests; i++ {
		_, err := client.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{DeviceId: "dev-heap"})
		if err != nil {
			t.Fatalf("GetSnapshot failed: %v", err)
		}
	}

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	delta := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("heap baseline=%d after=%d delta=%d", before.HeapAlloc, after.HeapAlloc, delta)

	// Conservative guardrail for regression detection, not an absolute leak proof.
	const maxHeapGrowth = 16 << 20 // 16 MiB
	if delta > maxHeapGrowth {
		t.Fatalf("possible heap growth regression detected: delta=%d bytes", delta)
	}
}

func BenchmarkTelemetryGetSnapshot(b *testing.B) {
	client, cleanup := startProfilingServer(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	req := &telemetryv1.GetSnapshotRequest{DeviceId: "bench-dev"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.GetSnapshot(ctx, req); err != nil {
			b.Fatalf("GetSnapshot failed: %v", err)
		}
	}
}

func BenchmarkTelemetrySubscribeBurst(b *testing.B) {
	client, cleanup := startProfilingServer(&testing.T{})
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		stream, err := client.Subscribe(ctx, &telemetryv1.SubscribeRequest{
			DeviceId:               fmt.Sprintf("bench-dev-%d", i),
			IncludeInitialSnapshot: true,
		})
		if err != nil {
			cancel()
			b.Fatalf("Subscribe failed: %v", err)
		}
		if _, err := stream.Recv(); err != nil {
			cancel()
			b.Fatalf("Recv failed: %v", err)
		}
		cancel()
	}
}

func BenchmarkTelemetryGetSnapshotParallel(b *testing.B) {
	client, cleanup := startProfilingServer(&testing.T{})
	defer cleanup()

	var seq uint64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := atomic.AddUint64(&seq, 1)
			if _, err := client.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{DeviceId: fmt.Sprintf("p-%d", id)}); err != nil {
				b.Fatalf("GetSnapshot failed: %v", err)
			}
		}
	})
}

func BenchmarkTelemetryGetSnapshotObservedFleetMix(b *testing.B) {
	client, cleanup := startProfilingServer(&testing.T{})
	defer cleanup()

	// Calibrated from logs/mqtt_payload_raw-2026-02-*.log:
	// - D2M (R351...): p95 ~= 3 msgs/s/device
	// - DPU (Y711...): p95 ~= 2 msgs/s/device
	// This models a 32-device visible fleet (16 D2M + 16 DPU) over 1s windows.
	const (
		d2mDevices = 16
		dpuDevices = 16
		d2mMPS     = 3
		dpuMPS     = 2
	)
	opsPerWindow := d2mDevices*d2mMPS + dpuDevices*dpuMPS // 80 RPCs/s window

	reqs := make([]*telemetryv1.GetSnapshotRequest, 0, opsPerWindow)
	for d := 0; d < d2mDevices; d++ {
		for i := 0; i < d2mMPS; i++ {
			reqs = append(reqs, &telemetryv1.GetSnapshotRequest{DeviceId: fmt.Sprintf("d2m-%02d-%d", d, i)})
		}
	}
	for d := 0; d < dpuDevices; d++ {
		for i := 0; i < dpuMPS; i++ {
			reqs = append(reqs, &telemetryv1.GetSnapshotRequest{DeviceId: fmt.Sprintf("dpu-%02d-%d", d, i)})
		}
	}

	rng := rand.New(rand.NewSource(42))
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Shuffle each window so we don't accidentally benchmark a fixed request order.
		rng.Shuffle(len(reqs), func(a, c int) { reqs[a], reqs[c] = reqs[c], reqs[a] })
		for _, req := range reqs {
			if _, err := client.GetSnapshot(ctx, req); err != nil {
				b.Fatalf("GetSnapshot failed: %v", err)
			}
		}
	}
}

func BenchmarkTelemetrySubscribeObservedBurst(b *testing.B) {
	client, cleanup := startProfilingServer(&testing.T{})
	defer cleanup()

	// Upper-end burst observed in logs for a single second.
	const burstStreams = 232

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		errCh := make(chan error, burstStreams)
		for s := 0; s < burstStreams; s++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				stream, err := client.Subscribe(ctx, &telemetryv1.SubscribeRequest{
					DeviceId:               fmt.Sprintf("burst-%d-%d", i, id),
					IncludeInitialSnapshot: true,
				})
				if err != nil {
					errCh <- err
					return
				}
				if _, err := stream.Recv(); err != nil {
					errCh <- err
					return
				}
			}(s)
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				b.Fatalf("Subscribe burst failed: %v", err)
			}
		}
	}
}

type syntheticDeviceProfile struct {
	id  string
	mps int
}

func buildSyntheticFleetProfiles(total int, seed int64) []syntheticDeviceProfile {
	rng := rand.New(rand.NewSource(seed))
	profiles := make([]syntheticDeviceProfile, 0, total)
	for i := 0; i < total; i++ {
		isD2M := rng.Float64() < 0.84 // approximated observed fleet skew.
		mps := sampleObservedMPS(rng, isD2M)
		deviceType := "dpu"
		if isD2M {
			deviceType = "d2m"
		}
		profiles = append(profiles, syntheticDeviceProfile{
			id:  fmt.Sprintf("%s-%05d", deviceType, i),
			mps: mps,
		})
	}
	return profiles
}

func sampleObservedMPS(rng *rand.Rand, isD2M bool) int {
	roll := rng.Float64()
	if isD2M {
		// D2M observed envelope: p50=1, p95=3, p99=5.
		switch {
		case roll < 0.66:
			return 1
		case roll < 0.90:
			return 2
		case roll < 0.99:
			return 3
		default:
			return 5
		}
	}
	// DPU observed envelope: p50=1, p95=2, p99=3.
	switch {
	case roll < 0.72:
		return 1
	case roll < 0.96:
		return 2
	default:
		return 3
	}
}

func buildSnapshotWindowRequests(profiles []syntheticDeviceProfile) []*telemetryv1.GetSnapshotRequest {
	total := 0
	for _, p := range profiles {
		total += p.mps
	}
	reqs := make([]*telemetryv1.GetSnapshotRequest, 0, total)
	for _, p := range profiles {
		for i := 0; i < p.mps; i++ {
			reqs = append(reqs, &telemetryv1.GetSnapshotRequest{DeviceId: p.id})
		}
	}
	return reqs
}

func buildShuffledWindows(base []*telemetryv1.GetSnapshotRequest, windows int, seed int64) [][]*telemetryv1.GetSnapshotRequest {
	rng := rand.New(rand.NewSource(seed))
	out := make([][]*telemetryv1.GetSnapshotRequest, 0, windows)
	for i := 0; i < windows; i++ {
		window := make([]*telemetryv1.GetSnapshotRequest, len(base))
		copy(window, base)
		rng.Shuffle(len(window), func(a, b int) {
			window[a], window[b] = window[b], window[a]
		})
		out = append(out, window)
	}
	return out
}

func buildBurstSubscribeRequests(profiles []syntheticDeviceProfile, burstSize int, seed int64) []*telemetryv1.SubscribeRequest {
	rng := rand.New(rand.NewSource(seed))
	reqs := make([]*telemetryv1.SubscribeRequest, 0, burstSize)
	for i := 0; i < burstSize; i++ {
		p := profiles[rng.Intn(len(profiles))]
		reqs = append(reqs, &telemetryv1.SubscribeRequest{
			DeviceId:               fmt.Sprintf("%s-burst-%d", p.id, i),
			IncludeInitialSnapshot: true,
		})
	}
	return reqs
}

func percentileDuration(vals []time.Duration, p float64) time.Duration {
	if len(vals) == 0 {
		return 0
	}
	dup := append([]time.Duration(nil), vals...)
	sort.Slice(dup, func(i, j int) bool { return dup[i] < dup[j] })
	idx := int(float64(len(dup)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(dup) {
		idx = len(dup) - 1
	}
	return dup[idx]
}

func envDurationMs(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return time.Duration(v) * time.Millisecond
}

func envBytes(name string, fallback int64) int64 {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return int64(v) << 20 // MiB
}

func BenchmarkTelemetryGetSnapshotObservedFleetMix10k(b *testing.B) {
	client, cleanup := startProfilingServer(&testing.T{})
	defer cleanup()

	profiles := buildSyntheticFleetProfiles(10_000, 20260222)
	baseWindow := buildSnapshotWindowRequests(profiles)
	windows := buildShuffledWindows(baseWindow, 4, 77)

	ctx := context.Background()
	b.ReportAllocs()
	b.ReportMetric(float64(len(baseWindow)), "rpc/window")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		window := windows[i%len(windows)]
		for _, req := range window {
			if _, err := client.GetSnapshot(ctx, req); err != nil {
				b.Fatalf("GetSnapshot failed: %v", err)
			}
		}
	}
}

func BenchmarkTelemetrySubscribeObservedStartupSpike10k(b *testing.B) {
	client, cleanup := startProfilingServer(&testing.T{})
	defer cleanup()

	profiles := buildSyntheticFleetProfiles(10_000, 20260222)
	const burstStreams = 232 // observed upper-end startup spike.
	bursts := []*[]*telemetryv1.SubscribeRequest{
		ptrSlice(buildBurstSubscribeRequests(profiles, burstStreams, 101)),
		ptrSlice(buildBurstSubscribeRequests(profiles, burstStreams, 202)),
		ptrSlice(buildBurstSubscribeRequests(profiles, burstStreams, 303)),
	}

	b.ReportAllocs()
	b.ReportMetric(float64(burstStreams), "streams/burst")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		burst := *bursts[i%len(bursts)]
		var wg sync.WaitGroup
		errCh := make(chan error, len(burst))
		for _, req := range burst {
			wg.Add(1)
			go func(r *telemetryv1.SubscribeRequest) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				stream, err := client.Subscribe(ctx, r)
				if err != nil {
					errCh <- err
					return
				}
				if _, err := stream.Recv(); err != nil {
					errCh <- err
					return
				}
			}(req)
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				b.Fatalf("Subscribe burst failed: %v", err)
			}
		}
	}
}

func ptrSlice[T any](in []T) *[]T {
	return &in
}

func TestTelemetryServerP99LatencyAndHeapStable10k(t *testing.T) {
	if os.Getenv("ECOFLOW_GRPC_10K_SOAK") != "1" {
		t.Skip("set ECOFLOW_GRPC_10K_SOAK=1 to run 10k-device soak test")
	}
	if testing.Short() {
		t.Skip("skipping 10k-device soak test in -short mode")
	}

	client, cleanup := startProfilingServer(t)
	defer cleanup()

	profiles := buildSyntheticFleetProfiles(10_000, 20260222)
	baseWindow := buildSnapshotWindowRequests(profiles)
	windows := buildShuffledWindows(baseWindow, 2, 88)
	steadyRequests := append(append([]*telemetryv1.GetSnapshotRequest{}, windows[0]...), windows[1]...)
	burstReqs := buildBurstSubscribeRequests(profiles, 232, 99)

	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	const workers = 64
	jobs := make(chan *telemetryv1.GetSnapshotRequest, workers*2)
	latencies := make([]time.Duration, len(steadyRequests))
	var idx uint64
	var wg sync.WaitGroup
	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range jobs {
				reqStart := time.Now()
				_, err := client.GetSnapshot(context.Background(), req)
				elapsed := time.Since(reqStart)
				if err != nil {
					t.Errorf("GetSnapshot failed: %v", err)
					continue
				}
				i := atomic.AddUint64(&idx, 1) - 1
				if i < uint64(len(latencies)) {
					latencies[i] = elapsed
				}
			}
		}()
	}
	for _, req := range steadyRequests {
		jobs <- req
	}
	close(jobs)
	wg.Wait()
	steadyDuration := time.Since(start)

	var burstWG sync.WaitGroup
	burstLat := make([]time.Duration, len(burstReqs))
	var burstErr uint64
	for i, req := range burstReqs {
		burstWG.Add(1)
		go func(i int, req *telemetryv1.SubscribeRequest) {
			defer burstWG.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			reqStart := time.Now()
			stream, err := client.Subscribe(ctx, req)
			if err != nil {
				atomic.AddUint64(&burstErr, 1)
				return
			}
			if _, err := stream.Recv(); err != nil {
				atomic.AddUint64(&burstErr, 1)
				return
			}
			burstLat[i] = time.Since(reqStart)
		}(i, req)
	}
	burstWG.Wait()
	if burstErr > 0 {
		t.Fatalf("burst stream failures: %d", burstErr)
	}

	validLat := latencies[:min(int(idx), len(latencies))]
	p99Steady := percentileDuration(validLat, 0.99)
	p95Steady := percentileDuration(validLat, 0.95)
	p99Burst := percentileDuration(burstLat, 0.99)
	throughput := float64(len(validLat)) / steadyDuration.Seconds()

	maxP99Steady := envDurationMs("ECOFLOW_GRPC_P99_STEADY_MAX_MS", 50*time.Millisecond)
	maxP99Burst := envDurationMs("ECOFLOW_GRPC_P99_BURST_MAX_MS", 250*time.Millisecond)
	maxHeapDelta := envBytes("ECOFLOW_GRPC_MAX_HEAP_DELTA_MB", 64)

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	heapDelta := int64(after.HeapAlloc) - int64(before.HeapAlloc)

	t.Logf("10k soak steady_ops=%d throughput=%.0f req/s p95=%s p99=%s burst_p99=%s heap_delta=%dB",
		len(validLat), throughput, p95Steady, p99Steady, p99Burst, heapDelta)

	if p99Steady > maxP99Steady {
		t.Fatalf("steady p99 latency too high: got=%s max=%s", p99Steady, maxP99Steady)
	}
	if p99Burst > maxP99Burst {
		t.Fatalf("burst p99 latency too high: got=%s max=%s", p99Burst, maxP99Burst)
	}
	if heapDelta > maxHeapDelta {
		t.Fatalf("heap delta too high: got=%d max=%d", heapDelta, maxHeapDelta)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestTelemetryServerGCProfileSnapshotLoad(t *testing.T) {
	t.Parallel()

	client, cleanup := startProfilingServer(t)
	defer cleanup()

	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	const requests = 8000
	for i := 0; i < requests; i++ {
		_, err := client.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{DeviceId: "dev-gc"})
		if err != nil {
			t.Fatalf("GetSnapshot failed: %v", err)
		}
	}

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	t.Logf("gc_profile requests=%d num_gc_delta=%d pause_total_ms=%.3f heap_alloc_delta=%d total_alloc_delta=%d",
		requests,
		after.NumGC-before.NumGC,
		float64(after.PauseTotalNs-before.PauseTotalNs)/1e6,
		int64(after.HeapAlloc)-int64(before.HeapAlloc),
		int64(after.TotalAlloc)-int64(before.TotalAlloc),
	)
}
