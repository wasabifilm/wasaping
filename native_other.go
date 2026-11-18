//go:build !darwin || !cgo

package main

const hasNativeWindow = false

func nativeWindow(string) {}
