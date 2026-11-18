package scanner

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

var (
	reIPv4 = regexp.MustCompile(`\b(\d{1,3}(?:\.\d{1,3}){3})\b`)
	reMAC  = regexp.MustCompile(`(?i)\b([0-9a-f]{1,2}(?:[:-][0-9a-f]{1,2}){5})\b`)
)

// ARPTable returns the OS neighbour cache as IP -> MAC.
// Linux reads /proc/net/arp; macOS, BSD and Windows parse `arp -a`.
func ARPTable(ctx context.Context) map[string]string {
	table := map[string]string{}
	if runtime.GOOS == "linux" {
		if f, err := os.Open("/proc/net/arp"); err == nil {
			defer f.Close()
			sc := bufio.NewScanner(f)
			sc.Scan() // header
			for sc.Scan() {
				fl := strings.Fields(sc.Text())
				// IP HWtype Flags HWaddress Mask Device ; flags 0x0 = incomplete
				if len(fl) >= 4 && fl[2] != "0x0" {
					if m := normalizeMAC(fl[3]); m != "" {
						table[fl[0]] = m
					}
				}
			}
			return table
		}
	}
	args := []string{"-a"}
	if runtime.GOOS != "windows" {
		args = []string{"-an"}
	}
	out, err := exec.CommandContext(ctx, "arp", args...).Output()
	if err != nil {
		return table
	}
	parseARPOutput(string(out), table)
	return table
}

func parseARPOutput(out string, table map[string]string) {
	for _, line := range strings.Split(out, "\n") {
		ip := reIPv4.FindString(line)
		mac := reMAC.FindString(line)
		if ip == "" || mac == "" {
			continue
		}
		if m := normalizeMAC(mac); m != "" && !strings.HasPrefix(m, "01:00:5e") {
			table[ip] = m
		}
	}
}
