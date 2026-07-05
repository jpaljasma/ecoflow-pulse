//go:build unix

package edgefiles

import (
	"os"
	"syscall"
)

func ownedByCurrentUserOrRoot(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}
	uid := stat.Uid
	return uid == 0 || uid == uint32(os.Geteuid())
}
