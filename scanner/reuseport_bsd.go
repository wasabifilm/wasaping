//go:build darwin || freebsd || netbsd || openbsd || linux

package scanner

import "syscall"

func setReusePort(fd int) { syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1) }
