//go:build !darwin

package main

import "errors"

func loginAvailable() bool { return false }
func loginEnabled() bool   { return false }
func setLogin(bool) error  { return errors.New("disponibile solo su macOS") }
