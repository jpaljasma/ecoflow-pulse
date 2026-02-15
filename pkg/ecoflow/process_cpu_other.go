//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package ecoflow

import "time"

func processCPUTime() (time.Duration, bool) {
	return 0, false
}
