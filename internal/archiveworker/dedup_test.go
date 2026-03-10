package archiveworker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecentEnvelopeDeduperConcurrentDuplicateSingleWinner(t *testing.T) {
	t.Parallel()

	deduper := newRecentEnvelopeDeduper(30*time.Minute, 1024)
	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)

	const workers = 64
	var accepted atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if deduper.Add(now, "env:dup-1") {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := accepted.Load(); got != 1 {
		t.Fatalf("expected exactly one accepted duplicate, got=%d", got)
	}
}

func TestRecentEnvelopeDeduperStressRemainsBounded(t *testing.T) {
	t.Parallel()

	const maxEntries = 1024
	deduper := newRecentEnvelopeDeduper(30*time.Minute, maxEntries)
	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 20000; i++ {
		if !deduper.Add(now, fmt.Sprintf("env:%d", i)) {
			t.Fatalf("unique key %d should have been accepted", i)
		}
	}

	deduper.mu.Lock()
	defer deduper.mu.Unlock()
	if got := len(deduper.entries); got > maxEntries {
		t.Fatalf("entries grew past maxEntries: got=%d max=%d", got, maxEntries)
	}
	if got := len(deduper.order) - deduper.head; got > maxEntries {
		t.Fatalf("live order window grew past maxEntries: got=%d max=%d", got, maxEntries)
	}
}

func BenchmarkRecentEnvelopeDeduperAddUnique(b *testing.B) {
	deduper := newRecentEnvelopeDeduper(30*time.Minute, 250000)
	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !deduper.Add(now, fmt.Sprintf("env:%d", i)) {
			b.Fatalf("unique key %d should have been accepted", i)
		}
	}
}

func BenchmarkRecentEnvelopeDeduperAddDuplicate(b *testing.B) {
	deduper := newRecentEnvelopeDeduper(30*time.Minute, 250000)
	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)
	if !deduper.Add(now, "env:dup-bench") {
		b.Fatalf("seed duplicate key should have been accepted")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if deduper.Add(now, "env:dup-bench") {
			b.Fatalf("duplicate key should have been rejected")
		}
	}
}
