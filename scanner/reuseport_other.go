//go:build !windows && !darwin && !freebsd && !netbsd && !openbsd && !linux

package scanner

func setReusePort(int) {}
