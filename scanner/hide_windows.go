//go:build windows

package scanner

import (
	"os/exec"
	"syscall"
)

// hideWindow prevents a console window flashing for each ping when running the GUI.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
