//go:build !darwin && !linux

package main

import "os"

func setupTerminalForSingleKeyInput(_ *os.File) (func(), error) {
	return func() {}, nil
}
