//go:build darwin

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// "Apri al login": a LaunchAgent of this user that starts wasaping.app in
// the menu bar (no window) at login. Turning it off deletes the file.

const loginLabel = "eu.wasabi.wasaping.login"

func loginPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", loginLabel+".plist")
}

// appBundlePath is the .app this program runs from ("" when not in one).
func appBundlePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return ""
	}
	if i := strings.Index(exe, ".app/Contents/MacOS/"); i >= 0 {
		return exe[:i+len(".app")]
	}
	return ""
}

func loginAvailable() bool { return appBundlePath() != "" }

func loginEnabled() bool {
	_, err := os.Stat(loginPlistPath())
	return err == nil
}

func setLogin(on bool) error {
	if !on {
		err := os.Remove(loginPlistPath())
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	app := appBundlePath()
	if app == "" {
		return errors.New("funziona solo con wasaping.app")
	}
	esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>` + loginLabel + `</string>
  <key>ProgramArguments</key><array>
    <string>/usr/bin/open</string><string>-g</string><string>-a</string><string>` + esc(app) + `</string>
    <string>--args</string><string>app</string><string>-hidden</string>
  </array>
  <key>RunAtLoad</key><true/>
</dict>
</plist>
`
	if err := os.MkdirAll(filepath.Dir(loginPlistPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(loginPlistPath(), []byte(plist), 0o644)
}
