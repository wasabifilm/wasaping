package scanner

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// arpFile, when set, is a copy of `arp -an` refreshed every second by a
// privileged helper (see startARPHelper in package main).
var arpFile atomic.Value

// UseARPFile makes ARPTable read the neighbour cache from path.
func UseARPFile(path string) { arpFile.Store(path) }

// ARPHidden reports whether the system gives us an empty ARP table, which
// recent macOS versions do for apps not signed by a registered developer.
// The router is practically always in the cache, so empty means hidden.
func ARPHidden() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	out, err := exec.Command("arp", "-an").Output()
	if err != nil {
		return false
	}
	t := map[string]string{}
	parseARPOutput(string(out), t)
	return len(t) == 0
}

// readARPFile waits for a copy written after since (the helper refreshes it
// every second), then parses it.
func readARPFile(ctx context.Context, path string, since time.Time, table map[string]string) bool {
	deadline := time.Now().Add(2500 * time.Millisecond)
	for {
		if st, err := os.Stat(path); err == nil && st.ModTime().After(since) {
			break
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			break // use what is there, even if a bit older
		}
		time.Sleep(100 * time.Millisecond)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	parseARPOutput(string(b), table)
	return len(table) > 0
}

var (
	reIPv4 = regexp.MustCompile(`\b(\d{1,3}(?:\.\d{1,3}){3})\b`)
	reMAC  = regexp.MustCompile(`(?i)\b([0-9a-f]{1,2}(?:[:-][0-9a-f]{1,2}){5})\b`)
)

// ARPSnapshot is ARPTable without waiting for the helper's next copy: for
// lookups that do not follow fresh probes (e.g. the router's MAC).
func ARPSnapshot(ctx context.Context) map[string]string {
	table := map[string]string{}
	if p, ok := arpFile.Load().(string); ok && p != "" && readARPFile(ctx, p, time.Time{}, table) {
		return table
	}
	return ARPTable(ctx)
}

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
	if p, ok := arpFile.Load().(string); ok && p != "" && readARPFile(ctx, p, time.Now(), table) {
		return table
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
