//go:build !windows && !darwin && !freebsd && !netbsd && !openbsd && !linux

package showsim

func setReusePort(int) {}
