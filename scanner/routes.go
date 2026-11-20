package scanner

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Networks reached through a route rather than directly: the far side of
// a VPN (WireGuard, OpenVPN, Tailscale, a UniFi or FRITZ!Box VPN…) or of a
// router. They can be scanned like a local network, but MACs, makers and
// broadcast names (Bonjour, Art-Net) do not cross a router: IP, ports, ping
// and DNS or SMB names do.

// RoutedNet is a private network reached through a route.
type RoutedNet struct {
	Network string `json:"network"`           // CIDR
	Iface   string `json:"iface"`             // interface the route goes out of (on Windows its IP)
	Gateway string `json:"gateway,omitempty"` // next hop, when there is one
	Label   string `json:"label,omitempty"`   // "VPN (utun6)", "router 10.0.0.1"
}

// RoutedNetworks lists the private networks (/16 to /30) in the routing
// table that are not one of this computer's own networks.
func RoutedNetworks() []RoutedNet {
	var all []RoutedNet
	switch runtime.GOOS {
	case "linux":
		if b, err := os.ReadFile("/proc/net/route"); err == nil {
			all = parseProcRoute(string(b))
		}
	case "windows":
		cmd := exec.Command("route", "print", "-4")
		hideWindow(cmd)
		if out, err := cmd.Output(); err == nil {
			all = parseRoutePrint(string(out))
		}
	default:
		if out, err := exec.Command("netstat", "-rn", "-f", "inet").Output(); err == nil {
			all = parseNetstatRoutes(string(out))
		}
	}
	return keepRouted(all, localNetworks())
}

// keepRouted keeps private networks of a scannable size that are not local.
func keepRouted(all []RoutedNet, local []*net.IPNet) []RoutedNet {
	seen := map[string]bool{}
	var out []RoutedNet
	for _, r := range all {
		ip, n, err := net.ParseCIDR(r.Network)
		if err != nil || ip.To4() == nil {
			continue
		}
		ones, _ := n.Mask.Size()
		if ones < 16 || ones > 30 || !(n.IP.IsPrivate() || cgnat.Contains(n.IP)) {
			continue
		}
		dup := false
		for _, l := range local {
			lo, _ := l.Mask.Size()
			if l.Contains(n.IP) && lo <= ones {
				dup = true // the network itself, or a part of it, is local
			}
		}
		if dup || seen[n.String()] {
			continue
		}
		seen[n.String()] = true
		r.Network = n.String()
		r.Label = RouteLabel(r)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Network < out[j].Network })
	return out
}

// 100.64.0.0/10: carrier-grade NAT, used by Tailscale and similar VPNs.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0).To4(), Mask: net.CIDRMask(10, 32)}

// InRouted tells how ip is reached when it is in a routed network ("VPN
// (utun6)", "router 10.0.0.1"; "" = not routed).
func InRouted(ip string, routed []RoutedNet) string {
	p := net.ParseIP(ip)
	for _, r := range routed {
		if _, n, err := net.ParseCIDR(r.Network); err == nil && n.Contains(p) {
			return RouteLabel(r)
		}
	}
	return ""
}

// parseNetstatRoutes reads `netstat -rn -f inet` (macOS, BSD), where
// destinations may be shortened: "192.168.3" = 192.168.3.0/24,
// "10.10.10/24" = 10.10.10.0/24.
func parseNetstatRoutes(out string) []RoutedNet {
	var rs []RoutedNet
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 || f[0] == "default" || strings.Contains(f[2], "H") {
			continue
		}
		dst, bits, ok := strings.Cut(f[0], "/")
		parts := strings.Split(dst, ".")
		if len(parts) > 4 {
			continue
		}
		for _, p := range parts {
			if n, err := strconv.Atoi(p); err != nil || n < 0 || n > 255 {
				parts = nil
				break
			}
		}
		if len(parts) == 0 {
			continue
		}
		if !ok {
			bits = strconv.Itoa(8 * len(parts))
		}
		for len(parts) < 4 {
			parts = append(parts, "0")
		}
		r := RoutedNet{Network: strings.Join(parts, ".") + "/" + bits, Iface: f[3]}
		if gw := net.ParseIP(f[1]); gw != nil {
			r.Gateway = gw.String()
		}
		rs = append(rs, r)
	}
	return rs
}

// parseProcRoute reads Linux /proc/net/route (addresses in little-endian hex).
func parseProcRoute(out string) []RoutedNet {
	ip := func(h string) net.IP {
		b, err := hex.DecodeString(h)
		if err != nil || len(b) != 4 {
			return nil
		}
		v := make(net.IP, 4)
		binary.BigEndian.PutUint32(v, binary.LittleEndian.Uint32(b))
		return v
	}
	var rs []RoutedNet
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Scan() // header
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 8 {
			continue
		}
		flags, _ := strconv.ParseUint(f[3], 16, 32)
		dst, gw, mask := ip(f[1]), ip(f[2]), ip(f[7])
		if dst == nil || mask == nil || flags&0x4 != 0 || dst.Equal(net.IPv4zero.To4()) { // host route or default
			continue
		}
		ones, _ := net.IPMask(mask).Size()
		r := RoutedNet{Network: dst.String() + "/" + strconv.Itoa(ones), Iface: f[0]}
		if gw != nil && !gw.Equal(net.IPv4zero.To4()) {
			r.Gateway = gw.String()
		}
		rs = append(rs, r)
	}
	return rs
}

// parseRoutePrint reads the IPv4 table of `route print -4` (Windows):
// "Network Destination  Netmask  Gateway  Interface  Metric".
func parseRoutePrint(out string) []RoutedNet {
	var rs []RoutedNet
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) != 5 {
			continue
		}
		dst, mask := net.ParseIP(f[0]).To4(), net.ParseIP(f[1]).To4()
		if dst == nil || mask == nil || dst.Equal(net.IPv4zero.To4()) {
			continue
		}
		ones, _ := net.IPMask(mask).Size()
		r := RoutedNet{Network: dst.String() + "/" + strconv.Itoa(ones), Iface: f[3]}
		if gw := net.ParseIP(f[2]); gw != nil {
			r.Gateway = gw.String()
		}
		rs = append(rs, r)
	}
	return rs
}

// RouteLabel describes how a routed network is reached: "VPN (utun6)" for
// tunnel interfaces, else "router 10.0.0.1" or the interface.
func RouteLabel(r RoutedNet) string {
	n := strings.ToLower(r.Iface)
	for _, p := range []string{"utun", "tun", "tap", "wg", "ppp", "ipsec", "tailscale", "zt", "nordlynx", "proton"} {
		if strings.HasPrefix(n, p) {
			return "VPN (" + r.Iface + ")"
		}
	}
	if r.Gateway != "" {
		return "router " + r.Gateway
	}
	return r.Iface
}
