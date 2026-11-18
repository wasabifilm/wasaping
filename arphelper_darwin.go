//go:build darwin

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Recent macOS versions hide the ARP table (and so every MAC address) from
// apps that are not signed by a registered developer, but not from root.
// The "ARP helper" is a tiny launchd daemon, installed once with the
// administrator password, that copies `arp -an` into a root-owned file
// every second — only while a wasaping trigger file exists and is fresh.
// It runs nothing but Apple binaries and takes no input from wasaping, so
// it cannot be used to run anything else as root.

const (
	helperLabel   = "eu.wasabi.wasaping.arp"
	helperScript  = "/Library/PrivilegedHelperTools/" + helperLabel + ".sh"
	helperPlist   = "/Library/LaunchDaemons/" + helperLabel + ".plist"
	helperTrigger = "/private/var/tmp/wasaping.active"
	helperOutput  = "/var/run/wasaping/arp"
)

const helperScriptBody = `#!/bin/sh
# wasaping ARP helper (v1): while wasaping is open, copy the ARP table where
# wasaping can read it. Remove with: wasaping helper uninstall
T=` + helperTrigger + `
D=/var/run/wasaping
/bin/mkdir -p "$D" && /bin/chmod 755 "$D"
while [ -f "$T" ] && [ ! -L "$T" ]; do
  now=$(/bin/date +%s)
  seen=$(/usr/bin/stat -f %m "$T" 2>/dev/null || echo 0)
  if [ $((now - seen)) -gt 30 ]; then /bin/rm -f "$T"; break; fi
  /usr/sbin/arp -an > "$D/arp.tmp" 2>/dev/null && /bin/mv -f "$D/arp.tmp" "$D/arp"
  /bin/sleep 1
done
exit 0
`

const helperPlistBody = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>` + helperLabel + `</string>
  <key>ProgramArguments</key><array><string>/bin/sh</string><string>` + helperScript + `</string></array>
  <key>KeepAlive</key><dict><key>PathState</key><dict><key>` + helperTrigger + `</key><true/></dict></dict>
  <key>ThrottleInterval</key><integer>2</integer>
  <key>StandardOutPath</key><string>/dev/null</string>
  <key>StandardErrorPath</key><string>/dev/null</string>
</dict>
</plist>
`

const helperSupported = true

// helperInstalled reports whether the current version of the helper is in place.
func helperInstalled() bool {
	s, err1 := os.ReadFile(helperScript)
	p, err2 := os.ReadFile(helperPlist)
	return err1 == nil && err2 == nil && bytes.Equal(s, []byte(helperScriptBody)) && bytes.Equal(p, []byte(helperPlistBody))
}

// asAdmin runs a shell command as root through the standard macOS password
// dialog (Touch ID is not offered there by macOS; wasaping never sees the
// password).
func asAdmin(cmd, why string) error {
	apple := fmt.Sprintf(`do shell script %s with prompt %s with administrator privileges`, appleString(cmd), appleString(why))
	out, err := exec.Command("/usr/bin/osascript", "-e", apple).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "-128") {
			return errors.New("annullato")
		}
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// installHelper asks for the password once and installs the daemon. The
// file contents travel inside the command itself (base64), so nothing on
// disk can be swapped in between.
func installHelper() error {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	cmd := strings.Join([]string{
		"set -e",
		"/bin/mkdir -p /Library/PrivilegedHelperTools",
		"echo " + b64(helperScriptBody) + " | /usr/bin/base64 -D > " + helperScript,
		"echo " + b64(helperPlistBody) + " | /usr/bin/base64 -D > " + helperPlist,
		"/usr/sbin/chown root:wheel " + helperScript + " " + helperPlist,
		"/bin/chmod 755 " + helperScript,
		"/bin/chmod 644 " + helperPlist,
		"/bin/launchctl bootout system/" + helperLabel + " 2>/dev/null || true",
		"/bin/launchctl bootstrap system " + helperPlist,
	}, "\n")
	return asAdmin(cmd, "wasaping vuole installare un piccolo aiuto per leggere gli indirizzi MAC dei dispositivi. Serve la password una sola volta.")
}

func uninstallHelper() error {
	os.Remove(helperTrigger)
	cmd := strings.Join([]string{
		"/bin/launchctl bootout system/" + helperLabel + " 2>/dev/null || true",
		"/bin/rm -f " + helperScript + " " + helperPlist + " " + helperTrigger,
		"/bin/rm -rf /var/run/wasaping",
	}, "\n")
	return asAdmin(cmd, "wasaping vuole rimuovere il suo aiuto per gli indirizzi MAC.")
}

// activateHelper switches the helper on for as long as this process runs
// (the trigger file is refreshed every 10 s; the helper stops by itself 30 s
// after the last refresh) and makes the scanner read its copy.
func activateHelper() {
	touch := func() {
		now := time.Now()
		if err := os.Chtimes(helperTrigger, now, now); err != nil {
			if f, err := os.OpenFile(helperTrigger, os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
				f.Close()
			}
		}
	}
	touch()
	go func() {
		for range time.Tick(10 * time.Second) {
			touch()
		}
	}()
	scanner.UseARPFile(helperOutput)
}

// setupARP is called at startup. interactive: the app may ask for the
// password to install the helper; the command line only gives a hint.
func setupARP(interactive bool) string {
	if !scanner.ARPHidden() {
		return "notneeded"
	}
	if helperInstalled() {
		activateHelper()
		return "installed"
	}
	if !interactive {
		return "missing"
	}
	if err := installHelper(); err != nil {
		fmt.Fprintln(os.Stderr, "Aiuto per i MAC non installato:", err)
		return "missing"
	}
	activateHelper()
	return "installed"
}
