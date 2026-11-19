package scanner

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
)

// Interface describes a local network interface usable for scanning.
type Interface struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	MAC     string `json:"mac"`
	Network string `json:"network"` // CIDR, e.g. 192.168.1.0/24
}

// MaxHosts is the largest number of addresses a single scan accepts.
const MaxHosts = 65536

// LocalInterfaces returns the active IPv4 interfaces (non-loopback).
func LocalInterfaces() ([]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []Interface
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipn.IP.To4()
			if ip4 == nil || ip4.IsLinkLocalUnicast() {
				continue
			}
			netw := &net.IPNet{IP: ip4.Mask(ipn.Mask), Mask: ipn.Mask}
			out = append(out, Interface{
				Name:    ifc.Name,
				IP:      ip4.String(),
				MAC:     normalizeMAC(ifc.HardwareAddr.String()),
				Network: netw.String(),
			})
		}
	}
	return out, nil
}

// DefaultInterface picks the most likely LAN interface (private address first).
func DefaultInterface() (Interface, error) {
	ifs, err := LocalInterfaces()
	if err != nil {
		return Interface{}, err
	}
	for _, i := range ifs {
		if ip := net.ParseIP(i.IP); ip != nil && ip.IsPrivate() && i.MAC != "" {
			return i, nil
		}
	}
	for _, i := range ifs {
		if ip := net.ParseIP(i.IP); ip != nil && ip.IsPrivate() {
			return i, nil
		}
	}
	if len(ifs) > 0 {
		return ifs[0], nil
	}
	return Interface{}, errors.New("nessuna interfaccia di rete IPv4 attiva trovata")
}

// ParseTargets accepts a CIDR ("192.168.1.0/24"), a range ("192.168.1.10-50"
// or "192.168.1.10-192.168.2.20"), a single IP, or a comma-separated list.
func ParseTargets(spec string) ([]net.IP, error) {
	var all []net.IP
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ips, err := parseOne(part)
		if err != nil {
			return nil, err
		}
		all = append(all, ips...)
		if len(all) > MaxHosts {
			return nil, fmt.Errorf("troppi indirizzi (massimo %d)", MaxHosts)
		}
	}
	if len(all) == 0 {
		return nil, errors.New("nessun indirizzo da scansionare")
	}
	return all, nil
}

func parseOne(s string) ([]net.IP, error) {
	if strings.Contains(s, "/") {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("CIDR non valido %q", s)
		}
		if n.IP.To4() == nil {
			return nil, fmt.Errorf("solo IPv4 supportato: %q", s)
		}
		ones, bits := n.Mask.Size()
		size := uint64(1) << uint(bits-ones)
		if size > MaxHosts+2 {
			return nil, fmt.Errorf("rete %s troppo grande (massimo /16)", s)
		}
		start := ipToU32(n.IP)
		end := start + uint32(size) - 1
		if size > 2 { // skip network and broadcast addresses
			start++
			end--
		}
		return rangeIPs(start, end), nil
	}
	if i := strings.Index(s, "-"); i > 0 {
		a := net.ParseIP(strings.TrimSpace(s[:i])).To4()
		if a == nil {
			return nil, fmt.Errorf("intervallo non valido %q", s)
		}
		rest := strings.TrimSpace(s[i+1:])
		var b net.IP
		if n, err := strconv.Atoi(rest); err == nil && n >= 0 && n <= 255 {
			b = net.IPv4(a[0], a[1], a[2], byte(n)).To4()
		} else {
			b = net.ParseIP(rest).To4()
		}
		if b == nil || ipToU32(b) < ipToU32(a) {
			return nil, fmt.Errorf("intervallo non valido %q", s)
		}
		if ipToU32(b)-ipToU32(a) >= MaxHosts {
			return nil, fmt.Errorf("intervallo %s troppo grande", s)
		}
		return rangeIPs(ipToU32(a), ipToU32(b)), nil
	}
	ip := net.ParseIP(s).To4()
	if ip == nil {
		return nil, fmt.Errorf("indirizzo non valido %q", s)
	}
	return []net.IP{ip}, nil
}

func rangeIPs(start, end uint32) []net.IP {
	out := make([]net.IP, 0, end-start+1)
	for v := start; ; v++ {
		out = append(out, u32ToIP(v))
		if v == end {
			break
		}
	}
	return out
}

func ipToU32(ip net.IP) uint32 { return binary.BigEndian.Uint32(ip.To4()) }

func u32ToIP(v uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, v)
	return ip
}

// NormalizeMAC turns any common MAC notation into "aa:bb:cc:dd:ee:ff" ("" if invalid).
func NormalizeMAC(s string) string { return normalizeMAC(s) }

// normalizeMAC turns "a4-2b-8-1-..." or "A4:2B:08:01:..." into "a4:2b:08:01:...".
func normalizeMAC(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "-", ":")
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return ""
	}
	for i, p := range parts {
		if len(p) == 1 {
			parts[i] = "0" + p
		} else if len(p) != 2 {
			return ""
		}
	}
	m := strings.Join(parts, ":")
	switch m {
	case "00:00:00:00:00:00", "ff:ff:ff:ff:ff:ff",
		"02:00:00:00:00:00": // placeholder macOS returns when it hides the real address
		return ""
	}
	return m
}

// MACsHidden reports the macOS privacy case: devices on our own LAN were
// found but the system gave us no MAC address for any of them. Recent macOS
// versions hide the ARP table from apps that are not signed by Apple or by a
// registered developer.
func MACsHidden(hosts []Host) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	ifs, _ := LocalInterfaces()
	var nets []*net.IPNet
	for _, i := range ifs {
		if _, n, err := net.ParseCIDR(i.Network); err == nil {
			nets = append(nets, n)
		}
	}
	onLAN := 0
	for _, h := range hosts {
		if h.Self {
			continue
		}
		ip := net.ParseIP(h.IP)
		for _, n := range nets {
			if n.Contains(ip) {
				if h.MAC != "" {
					return false
				}
				onLAN++
				break
			}
		}
	}
	return onLAN > 0
}
