package ecoflow

import (
	"runtime"
	"time"
)

type runtimeSnapshot struct {
	capturedAt time.Time

	cpuTotal time.Duration
	cpuKnown bool

	memAllocBytes uint64
	memSysBytes   uint64
	heapInuse     uint64
}

type runtimeLoad struct {
	cpuLoadPercent float64
	cpuKnown       bool

	memAllocBytes uint64
	memSysBytes   uint64
	heapInuse     uint64
	memLoadPct    float64

	goMaxProcs int
	goroutines int
}

func captureRuntimeSnapshot(now time.Time) runtimeSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	cpuTotal, cpuKnown := processCPUTime()
	return runtimeSnapshot{
		capturedAt:    now,
		cpuTotal:      cpuTotal,
		cpuKnown:      cpuKnown,
		memAllocBytes: mem.Alloc,
		memSysBytes:   mem.Sys,
		heapInuse:     mem.HeapInuse,
	}
}

func computeRuntimeLoad(start, end runtimeSnapshot, elapsed time.Duration) runtimeLoad {
	load := runtimeLoad{
		memAllocBytes: end.memAllocBytes,
		memSysBytes:   end.memSysBytes,
		heapInuse:     end.heapInuse,
		goMaxProcs:    runtime.GOMAXPROCS(0),
		goroutines:    runtime.NumGoroutine(),
	}
	if load.memSysBytes > 0 {
		load.memLoadPct = float64(load.memAllocBytes) * 100 / float64(load.memSysBytes)
	}

	if start.cpuKnown && end.cpuKnown && elapsed > 0 && load.goMaxProcs > 0 {
		cpuDelta := end.cpuTotal - start.cpuTotal
		if cpuDelta < 0 {
			cpuDelta = 0
		}
		load.cpuLoadPercent = float64(cpuDelta) * 100 / (float64(elapsed) * float64(load.goMaxProcs))
		if load.cpuLoadPercent < 0 {
			load.cpuLoadPercent = 0
		}
		load.cpuKnown = true
	}

	return load
}
