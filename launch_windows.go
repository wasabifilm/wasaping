//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// launchedByDoubleClick reports whether the program owns its console alone,
// which happens when it was started from Explorer rather than from a terminal.
func launchedByDoubleClick() bool {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	if proc.Find() != nil {
		return false
	}
	var pids [4]uint32
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}
