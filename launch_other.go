//go:build !windows && !darwin

package main

func launchedByDoubleClick() bool { return false }
