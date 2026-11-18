package scanner

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// DefaultGateway returns the IPv4 address of the default router ("" if unknown).
func DefaultGateway() string {
	switch runtime.GOOS {
	case "linux":
		f, err := os.Open("/proc/net/route")
		if err != nil {
			return ""
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			// Iface Destination Gateway Flags ... (hex, little endian)
			fl := strings.Fields(sc.Text())
			if len(fl) > 2 && fl[1] == "00000000" {
				if b, err := hex.DecodeString(fl[2]); err == nil && len(b) == 4 {
					ip := make(net.IP, 4)
					binary.LittleEndian.PutUint32(ip, binary.BigEndian.Uint32(b))
					if !ip.IsUnspecified() {
						return ip.String()
					}
				}
			}
		}
		return ""
	case "windows":
		cmd := exec.Command("route", "print", "-4", "0.0.0.0")
		hideWindow(cmd)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return parseWindowsRoute(string(out))
	default: // macOS, BSD
		out, err := exec.Command("route", "-n", "get", "default").Output()
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(out), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(line), ":"); ok && k == "gateway" {
				if ip := net.ParseIP(strings.TrimSpace(v)).To4(); ip != nil {
					return ip.String()
				}
			}
		}
		return ""
	}
}

// parseWindowsRoute finds "0.0.0.0  0.0.0.0  <gateway>  <iface>  <metric>".
func parseWindowsRoute(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fl := strings.Fields(line)
		if len(fl) >= 3 && fl[0] == "0.0.0.0" && fl[1] == "0.0.0.0" {
			if ip := net.ParseIP(fl[2]).To4(); ip != nil {
				return ip.String()
			}
		}
	}
	return ""
}
