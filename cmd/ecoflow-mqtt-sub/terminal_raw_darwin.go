//go:build darwin

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func setupTerminalForSingleKeyInput(file *os.File) (func(), error) {
	fd := file.Fd()

	var original syscall.Termios
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCGETA),
		uintptr(unsafe.Pointer(&original)),
		0,
		0,
		0,
	); errno != 0 {
		return nil, errno
	}

	raw := original
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCSETA),
		uintptr(unsafe.Pointer(&raw)),
		0,
		0,
		0,
	); errno != 0 {
		return nil, errno
	}

	restore := func() {
		_, _, _ = syscall.Syscall6(
			syscall.SYS_IOCTL,
			fd,
			uintptr(syscall.TIOCSETA),
			uintptr(unsafe.Pointer(&original)),
			0,
			0,
			0,
		)
	}
	return restore, nil
}

