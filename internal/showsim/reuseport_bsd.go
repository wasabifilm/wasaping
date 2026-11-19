//go:build darwin || freebsd || netbsd || openbsd

package showsim

import "syscall"

func setReusePort(fd int) { syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1) }
