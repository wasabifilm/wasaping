package scanner

import (
	"context"
	"net"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

// IPv6 neighbours: the system's table (ndp on macOS, ip -6 neigh on
// Linux, netsh on Windows), matched to the scanned hosts by MAC. A ping to
// all-nodes (ff02::1) on the interface fills it first.

// wakeV6 pings ff02::1 on the interface so the neighbours answer.
func wakeV6(ctx context.Context, ifi *net.Interface) {
	if ifi == nil || runtime.GOOS == "windows" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	dst := "ff02::1%" + ifi.Name
	if runtime.GOOS == "darwin" {
		exec.CommandContext(ctx, "ping6", "-c", "2", dst).Run()
	} else {
		exec.CommandContext(ctx, "ping", "-6", "-c", "2", "-w", "2", dst).Run()
	}
}

// NeighborsV6 returns MAC -> IPv6 addresses (global first, then link-local).
func NeighborsV6(ctx context.Context) map[string][]string {
	var out []byte
	switch runtime.GOOS {
	case "darwin":
		out, _ = exec.CommandContext(ctx, "ndp", "-an").Output()
	case "windows":
		out, _ = exec.CommandContext(ctx, "netsh", "interface", "ipv6", "show", "neighbors").Output()
	default:
		out, _ = exec.CommandContext(ctx, "ip", "-6", "neigh", "show").Output()
	}
	return parseNeighborsV6(string(out), runtime.GOOS)
}

func parseNeighborsV6(text, goos string) map[string][]string {
	res := map[string][]string{}
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		var ip, mac string
		switch goos {
		case "darwin": // Neighbor Linklayer Netif Expire St Flgs Prbs
			ip, mac = f[0], f[1]
		case "windows": // Internet Address  Physical Address  Type
			if len(f) >= 3 && strings.EqualFold(f[2], "Unreachable") {
				continue
			}
			ip, mac = f[0], f[1]
		default: // fe80::1 dev eth0 lladdr aa:bb:… router REACHABLE
			ip = f[0]
			for i := 1; i+1 < len(f); i++ {
				if f[i] == "lladdr" {
					mac = f[i+1]
				}
			}
			if st := f[len(f)-1]; st == "FAILED" || st == "INCOMPLETE" {
				continue
			}
		}
		ip, _, _ = strings.Cut(ip, "%")
		p := net.ParseIP(ip)
		if p == nil || p.To4() != nil || p.IsLoopback() || p.IsMulticast() {
			continue
		}
		m := normalizeMAC(mac)
		if m == "" || m == "00:00:00:00:00:00" {
			continue
		}
		if !contains(res[m], p.String()) {
			res[m] = append(res[m], p.String())
		}
	}
	for m, a := range res {
		sort.SliceStable(a, func(i, j int) bool {
			li, lj := net.ParseIP(a[i]).IsLinkLocalUnicast(), net.ParseIP(a[j]).IsLinkLocalUnicast()
			if li != lj {
				return !li
			}
			return a[i] < a[j]
		})
		res[m] = a
	}
	return res
}

func contains(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}
