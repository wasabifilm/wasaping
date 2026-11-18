package scanner

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"sort"
	"strings"
	"time"
)

// localNames asks every IP for its own name, in parallel:
//   - mDNS/Bonjour (Apple devices, printers, Linux with Avahi, many IoT):
//     a reverse PTR query sent straight to the device's port 5353 and also
//     as one-shot multicast queries (RFC 6762 §5.5, §6.7);
//   - NetBIOS node status (Windows PCs, NAS, Samba) on port 137.
//
// It also collects the Bonjour service types each device announces
// (AirPlay, printers, file sharing…). It returns IP -> name and
// IP -> services; it never needs admin rights.
func localNames(ctx context.Context, ips []string, wait time.Duration) (map[string]string, map[string][]string) {
	out := map[string]string{}
	services := map[string][]string{}
	if len(ips) == 0 {
		return out, services
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	type found struct {
		ip, name, src string
		svcs          []string
	}
	ch := make(chan found, 64)
	done := make(chan struct{})
	go func() {
		for f := range ch {
			if f.src == "svc" {
				services[f.ip] = mergeUnique(services[f.ip], f.svcs)
				continue
			}
			// mDNS wins over NetBIOS: it is the name users see in Finder.
			if cur, ok := out[f.ip]; !ok || (f.src == "mdns" && !strings.HasSuffix(cur, ".local")) {
				out[f.ip] = f.name
			}
		}
		close(done)
	}()

	want := make(map[string]bool, len(ips))
	for _, ip := range ips {
		want[ip] = true
	}
	var running int
	finish := make(chan struct{}, 2)

	if c, err := net.ListenUDP("udp4", nil); err == nil {
		running++
		go func() {
			defer func() { c.Close(); finish <- struct{}{} }()
			questions := make([]string, 0, len(ips))
			for _, ip := range ips {
				q := reverseName(ip)
				questions = append(questions, q)
				c.WriteToUDP(dnsQuery([]string{q}, 12), &net.UDPAddr{IP: net.ParseIP(ip), Port: 5353})
			}
			mcast := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
			for i := 0; i < len(questions); i += 20 {
				end := min(i+20, len(questions))
				c.WriteToUDP(dnsQuery(questions[i:end], 12), mcast)
			}
			c.WriteToUDP(dnsQuery([]string{serviceEnum}, 12), mcast)
			readUntil(ctx, c, func(b []byte, from *net.UDPAddr) {
				for ip, name := range parseMDNSPTR(b) {
					if want[ip] {
						ch <- found{ip: ip, name: name, src: "mdns"}
					}
				}
				if ip := from.IP.String(); want[ip] {
					if svcs := parseServiceTypes(b); len(svcs) > 0 {
						ch <- found{ip: ip, src: "svc", svcs: svcs}
					}
				}
			})
		}()
	}

	if c, err := net.ListenUDP("udp4", nil); err == nil {
		running++
		go func() {
			defer func() { c.Close(); finish <- struct{}{} }()
			q := netbiosStatusQuery()
			for _, ip := range ips {
				c.WriteToUDP(q, &net.UDPAddr{IP: net.ParseIP(ip), Port: 137})
			}
			readUntil(ctx, c, func(b []byte, from *net.UDPAddr) {
				if ip := from.IP.String(); want[ip] {
					if name := parseNetbiosStatus(b); name != "" {
						ch <- found{ip: ip, name: name, src: "netbios"}
					}
				}
			})
		}()
	}

	for ; running > 0; running-- {
		<-finish
	}
	close(ch)
	<-done
	for ip := range services {
		sort.Strings(services[ip])
	}
	return out, services
}

// serviceEnum is the DNS-SD meta-query listing every service type a host offers.
const serviceEnum = "_services._dns-sd._udp.local"

// parseServiceTypes returns the service types ("airplay", "ipp", "smb"…)
// from the answers to the DNS-SD meta-query.
func parseServiceTypes(msg []byte) []string {
	var out []string
	for _, rr := range parseRecords(msg) {
		if rr.typ != 12 || !strings.EqualFold(rr.name, serviceEnum) {
			continue
		}
		t, _, err := readName(msg, rr.data)
		if err != nil {
			continue
		}
		// "_airplay._tcp.local" -> "airplay"
		label, _, _ := strings.Cut(t, ".")
		if label = strings.TrimPrefix(label, "_"); label != "" {
			out = mergeUnique(out, []string{strings.ToLower(label)})
		}
	}
	return out
}

func mergeUnique(a, b []string) []string {
	for _, x := range b {
		if !slices.Contains(a, x) {
			a = append(a, x)
		}
	}
	return a
}

func readUntil(ctx context.Context, c *net.UDPConn, handle func([]byte, *net.UDPAddr)) {
	dl, _ := ctx.Deadline()
	c.SetReadDeadline(dl)
	buf := make([]byte, 9000)
	for ctx.Err() == nil {
		n, from, err := c.ReadFromUDP(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return
			}
			continue
		}
		handle(buf[:n], from)
	}
}

// reverseName turns 192.168.1.20 into 20.1.168.192.in-addr.arpa.
func reverseName(ip string) string {
	p := strings.Split(ip, ".")
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
	return strings.Join(p, ".") + ".in-addr.arpa"
}

// dnsQuery builds a query for names of the given type, asking for unicast
// replies (the "QU" bit), as mDNS one-shot queries do.
func dnsQuery(names []string, qtype uint16) []byte {
	b := make([]byte, 12, 512)
	binary.BigEndian.PutUint16(b[4:], uint16(len(names)))
	for _, n := range names {
		for _, label := range strings.Split(strings.TrimSuffix(n, "."), ".") {
			b = append(b, byte(len(label)))
			b = append(b, label...)
		}
		b = append(b, 0)
		b = binary.BigEndian.AppendUint16(b, qtype)
		b = binary.BigEndian.AppendUint16(b, 0x8001) // QU + class IN
	}
	return b
}

// readName decodes a (possibly compressed) DNS name starting at off.
func readName(msg []byte, off int) (string, int, error) {
	var labels []string
	next := -1
	for jumps := 0; ; {
		if off >= len(msg) {
			return "", 0, errors.New("short name")
		}
		l := int(msg[off])
		switch {
		case l == 0:
			if next < 0 {
				next = off + 1
			}
			return strings.Join(labels, "."), next, nil
		case l&0xC0 == 0xC0:
			if off+1 >= len(msg) || jumps > 16 {
				return "", 0, errors.New("bad pointer")
			}
			if next < 0 {
				next = off + 2
			}
			off = int(binary.BigEndian.Uint16(msg[off:]) & 0x3FFF)
			jumps++
		default:
			if off+1+l > len(msg) {
				return "", 0, errors.New("short label")
			}
			labels = append(labels, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
}

type record struct {
	name string
	typ  uint16
	data int // offset of the RDATA in the message
}

// parseRecords returns the resource records of every section of a DNS
// response (questions are skipped). It stops at the first malformed record.
func parseRecords(msg []byte) []record {
	if len(msg) < 12 || msg[2]&0x80 == 0 { // not a response
		return nil
	}
	qd := int(binary.BigEndian.Uint16(msg[4:]))
	n := int(binary.BigEndian.Uint16(msg[6:])) + int(binary.BigEndian.Uint16(msg[8:])) + int(binary.BigEndian.Uint16(msg[10:]))
	off := 12
	for i := 0; i < qd; i++ {
		_, next, err := readName(msg, off)
		if err != nil {
			return nil
		}
		off = next + 4
	}
	var out []record
	for i := 0; i < n; i++ {
		name, next, err := readName(msg, off)
		if err != nil || next+10 > len(msg) {
			return out
		}
		typ := binary.BigEndian.Uint16(msg[next:])
		rdlen := int(binary.BigEndian.Uint16(msg[next+8:]))
		data := next + 10
		if data+rdlen > len(msg) {
			return out
		}
		out = append(out, record{name, typ, data})
		off = data + rdlen
	}
	return out
}

// parseMDNSPTR returns IP -> host name from the reverse PTR records found
// in any section of a DNS/mDNS response.
func parseMDNSPTR(msg []byte) map[string]string {
	out := map[string]string{}
	for _, rr := range parseRecords(msg) {
		if rr.typ != 12 || !strings.HasSuffix(strings.ToLower(rr.name), ".in-addr.arpa") {
			continue
		}
		if target, _, err := readName(msg, rr.data); err == nil && target != "" {
			if ip := ipFromReverse(rr.name); ip != "" {
				out[ip] = target
			}
		}
	}
	return out
}

func ipFromReverse(name string) string {
	p := strings.Split(strings.TrimSuffix(strings.ToLower(name), ".in-addr.arpa"), ".")
	if len(p) != 4 {
		return ""
	}
	ip := net.ParseIP(p[3] + "." + p[2] + "." + p[1] + "." + p[0])
	if ip == nil {
		return ""
	}
	return ip.String()
}

// netbiosStatusQuery is a NetBIOS "node status" request for the name "*".
func netbiosStatusQuery() []byte {
	b := []byte{0x77, 0x61, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 32}
	name := make([]byte, 16)
	name[0] = '*'
	for _, c := range name {
		b = append(b, 'A'+c>>4, 'A'+c&0x0F)
	}
	return append(b, 0, 0x00, 0x21, 0x00, 0x01) // type NBSTAT, class IN
}

// parseNetbiosStatus returns the machine name (unique name, suffix 0x00).
func parseNetbiosStatus(msg []byte) string {
	if len(msg) < 12 || binary.BigEndian.Uint16(msg[6:]) == 0 {
		return ""
	}
	_, off, err := readName(msg, 12)
	if err != nil || off+11 > len(msg) {
		return ""
	}
	off += 10 // type, class, ttl, rdlength
	count := int(msg[off])
	off++
	for i := 0; i < count && off+18 <= len(msg); i, off = i+1, off+18 {
		suffix := msg[off+15]
		group := binary.BigEndian.Uint16(msg[off+16:])&0x8000 != 0
		if suffix == 0x00 && !group {
			return strings.TrimSpace(strings.TrimRight(string(msg[off:off+15]), "\x00"))
		}
	}
	return ""
}
