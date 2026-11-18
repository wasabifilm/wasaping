//go:build linux

package scanner

import "syscall"

// soReusePort is SO_REUSEPORT on Linux; syscall only defines it on some
// architectures, the kernel value is the same everywhere.
const soReusePort = 0xf

func setReusePort(fd int) { syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, soReusePort, 1) }
