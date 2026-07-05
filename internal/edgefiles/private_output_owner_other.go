//go:build !unix

package edgefiles

import "os"

func ownedByCurrentUserOrRoot(os.FileInfo) bool {
	return true
}
