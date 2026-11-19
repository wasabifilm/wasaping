//go:build linux

package showsim

import "syscall"

func setReusePort(fd int) { syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, 0xf, 1) }
