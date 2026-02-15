//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package ecoflow

import (
	"syscall"
	"time"
)

func processCPUTime() (time.Duration, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, false
	}
	return timevalDuration(usage.Utime) + timevalDuration(usage.Stime), true
}

func timevalDuration(v syscall.Timeval) time.Duration {
	return time.Duration(v.Sec)*time.Second + time.Duration(v.Usec)*time.Microsecond
}
