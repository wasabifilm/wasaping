//go:build !darwin

package main

import "errors"

func canSelfUpdate() bool { return false }

func selfUpdate() error {
	return errors.New(T("scarica la nuova versione dalla pagina delle release", "download the new version from the releases page"))
}
