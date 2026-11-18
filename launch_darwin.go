//go:build darwin

package main

import (
	"os"
	"strings"
)

// launchedByDoubleClick reports whether we run as the main executable of
// wasaping.app (Finder, Dock or `open`), where there is no terminal.
func launchedByDoubleClick() bool {
	exe, err := os.Executable()
	return err == nil && strings.Contains(exe, ".app/Contents/MacOS/")
}
