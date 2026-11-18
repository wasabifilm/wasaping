//go:build !windows

package main

// On macOS the double-click case is handled by the wasaping.app bundle,
// whose launcher runs `wasaping app`.
func launchedByDoubleClick() bool { return false }
