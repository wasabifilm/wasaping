package main

import (
	"os/exec"
	"runtime"
	"strings"
)

// notify shows a system notification (macOS and Linux; on Windows the page
// shows the news itself).
func notify(title, body string) {
	switch runtime.GOOS {
	case "darwin":
		script := "display notification " + appleString(body) + " with title " + appleString(title)
		exec.Command("/usr/bin/osascript", "-e", script).Start()
	case "linux":
		if p, err := exec.LookPath("notify-send"); err == nil {
			exec.Command(p, "-a", "wasaping", title, body).Start()
		}
	}
}

// appleString quotes s as an AppleScript string literal.
func appleString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
