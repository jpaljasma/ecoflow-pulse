package ingestworker

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

func BenchmarkPublishPathLatency(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name      string
		queueSize int
		workers   int
		labels    bool
	}{
		{name: "sync_labels", queueSize: 0, workers: 0, labels: true},
		{name: "sync_nolabels", queueSize: 0, workers: 0, labels: false},
		{name: "async_q256_w1_labels", queueSize: 256, workers: 1, labels: true},
		{name: "async_q256_w1_nolabels", queueSize: 256, workers: 1, labels: false},
		{name: "async_q512_w4_labels", queueSize: 512, workers: 4, labels: true},
		{name: "async_q512_w4_nolabels", queueSize: 512, workers: 4, labels: false},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			latencies, throughput := runPublishLatencyScenario(b, tc.queueSize, tc.workers, tc.labels, b.N)
			if len(latencies) == 0 {
				b.Fatal("no latency samples collected")
			}
			p95 := percentileDuration(latencies, 95)
			p99 := percentileDuration(latencies, 99)
			b.ReportMetric(float64(p95.Microseconds()), "p95_us")
			b.ReportMetric(float64(p99.Microseconds()), "p99_us")
			b.ReportMetric(throughput, "msg_per_s")
		})
	}
}

func runPublishLatencyScenario(
	b *testing.B,
	queueSize int,
	workers int,
	labels bool,
	messages int,
) ([]time.Duration, float64) {
	b.Helper()

	if messages <= 0 {
		return nil, 0
	}

	pub := &syntheticDelayedPublisher{}
	ctx := context.Background()
	latencies := make([]time.Duration, 0, messages)

	start := time.Now()
	if queueSize <= 0 {
		for i := 0; i < messages; i++ {
			env := benchmarkEnvelope(i, labels)
			t0 := time.Now()
			if err := telemetrybus.PublishEnvelope(ctx, pub, env); err != nil {
				b.Fatalf("sync publish failed: %v", err)
			}
			latencies = append(latencies, time.Since(t0))
		}
	} else {
		asyncPub := newAsyncEnvelopePublisher(ctx, pub, queueSize, workers, 5*time.Second)
		defer func() {
			if err := asyncPub.Close(); err != nil {
				b.Fatalf("async close failed: %v", err)
			}
		}()
		for i := 0; i < messages; i++ {
			env := benchmarkEnvelope(i, labels)
			t0 := time.Now()
			if err := asyncPub.Publish(ctx, env); err != nil {
				b.Fatalf("async publish failed: %v", err)
			}
			latencies = append(latencies, time.Since(t0))
		}
		// Ensure all queued work is drained before measuring throughput.
		if err := asyncPub.Close(); err != nil {
			b.Fatalf("async close failed: %v", err)
		}
	}
	elapsed := time.Since(start)
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}
	throughput := float64(messages) / elapsed.Seconds()
	return latencies, throughput
}

type syntheticDelayedPublisher struct {
	seq atomic.Int64
}

func (p *syntheticDelayedPublisher) PublishEnvelope(_ context.Context, _ *envelopev1.TelemetryEnvelope) error {
	n := p.seq.Add(1)
	delay := 150 * time.Microsecond
	if n%50 == 0 {
		delay += 2 * time.Millisecond
	}
	if n%200 == 0 {
		delay += 4 * time.Millisecond
	}
	time.Sleep(delay)
	return nil
}

func (p *syntheticDelayedPublisher) Close() error { return nil }

func benchmarkEnvelope(i int, includeLabels bool) *envelopev1.TelemetryEnvelope {
	env := &envelopev1.TelemetryEnvelope{
		EnvelopeId: fmt.Sprintf("bench-%d", i),
		DeviceId:   "bench-device",
		Shard:      uint32(i % 128),
		Payload:    []byte(`{"typeCode":"pdStatus","params":{"wattsInSum":100}}`),
	}
	if includeLabels {
		env.Labels = map[string]string{
			"provider":      "ecoflow",
			"credential_id": "bench-cred",
			"addr":          "hs_yj751_pd_appshow_addr",
		}
	}
	return env
}

func percentileDuration(samples []time.Duration, percentile int) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	if percentile <= 0 {
		percentile = 1
	}
	if percentile > 100 {
		percentile = 100
	}
	tmp := make([]time.Duration, len(samples))
	copy(tmp, samples)
	sort.Slice(tmp, func(i, j int) bool {
		return tmp[i] < tmp[j]
	})
	idx := int(float64(len(tmp)-1) * (float64(percentile) / 100.0))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(tmp) {
		idx = len(tmp) - 1
	}
	return tmp[idx]
}
