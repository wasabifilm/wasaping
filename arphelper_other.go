//go:build !darwin

package main

import "errors"

// The ARP helper is only needed on macOS; elsewhere the table is readable.
const (
	helperSupported = false
	helperPlist     = ""
)

func setupARP(bool) string   { return "notneeded" }
func helperInstalled() bool  { return false }
func installHelper() error   { return errors.New("serve solo su macOS") }
func uninstallHelper() error { return errors.New("serve solo su macOS") }
