package scanner

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// devInfo is what devices tell about themselves on the local network.
type devInfo struct {
	name       string // host name (reverse mDNS or NetBIOS)
	src        string // "mdns" or "netbios"
	friendly   string // name chosen by the owner, e.g. "iPhone di Anna", "CL5-FOH"
	model      string // e.g. "MacBookPro18,3", "Chromecast", "HP LaserJet M110"
	services   []string
	ndiSources []string
	danteChans map[string]bool // Dante transmit channel names
	danteRate  int
	danteLatNs int
	fprio      int // priority of the service that gave friendly (lower wins)
	mprio      int
}

// browseTypes are the DNS-SD service types asked by name: their instance
// names and TXT records carry friendly names and models. Show gear first.
var browseTypes = []string{
	"_netaudio-arc._udp", "_netaudio-cmc._udp", "_netaudio-chan._udp", "_ndi._tcp", "_blackmagic._tcp", "_videohub._tcp",
	"_qlab._tcp", "_qlab._udp", "_osc._udp", "_osc._tcp",
	"_googlecast._tcp", "_airplay._tcp", "_companion-link._tcp", "_raop._tcp",
	"_device-info._tcp", "_smb._tcp", "_afpovertcp._tcp", "_ipp._tcp", "_printer._tcp",
	"_hap._tcp", "_spotify-connect._tcp", "_sonos._tcp", "_workstation._tcp", "_http._tcp",
}

// Lower number = better source for the friendly name.
var friendlyPrio = map[string]int{
	"netaudio-arc": 1, "netaudio-cmc": 1, "blackmagic": 1, "videohub": 1, "qlab": 2,
	"googlecast": 2, "airplay": 3, "companion-link": 4, "raop": 5, "device-info": 6,
	"smb": 7, "afpovertcp": 8, "ipp": 9, "printer": 9, "hap": 10, "spotify-connect": 10, "sonos": 10,
	"ndi": 11, "osc": 12, "workstation": 13, "http": 14,
}

// localNames asks every IP about itself, in parallel:
//   - mDNS/Bonjour (Apple devices, printers, Dante, NDI, Blackmagic, QLab…):
//     reverse PTR queries (to each device and multicast), the DNS-SD
//     service list, and the common service types, whose answers carry the
//     device's friendly name and model (RFC 6762, RFC 6763);
//   - NetBIOS node status (Windows PCs, NAS, Samba) on port 137.
//
// It never needs admin rights.
func localNames(ctx context.Context, ips []string, wait time.Duration) map[string]*devInfo {
	out := map[string]*devInfo{}
	if len(ips) == 0 {
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	for _, ip := range ips {
		out[ip] = &devInfo{fprio: 99, mprio: 99}
	}
	type msg struct {
		from string
		b    []byte
		nb   bool
	}
	ch := make(chan msg, 128)
	done := make(chan struct{})
	go func() {
		for m := range ch {
			if m.nb {
				if d := out[m.from]; d != nil && d.name == "" {
					if name := parseNetbiosStatus(m.b); name != "" {
						d.name, d.src = name, "netbios"
					}
				}
				continue
			}
			absorbMDNS(m.b, m.from, out)
		}
		close(done)
	}()

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
				c.WriteToUDP(dnsQuery(questions[i:min(i+20, len(questions))], 12), mcast)
			}
			types := []string{serviceEnum}
			for _, t := range browseTypes {
				types = append(types, t+".local")
			}
			for i := 0; i < len(types); i += 12 {
				c.WriteToUDP(dnsQuery(types[i:min(i+12, len(types))], 12), mcast)
			}
			readUntil(ctx, c, func(b []byte, from *net.UDPAddr) {
				ch <- msg{from: from.IP.String(), b: append([]byte(nil), b...)}
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
				ch <- msg{from: from.IP.String(), b: append([]byte(nil), b...), nb: true}
			})
		}()
	}
	for ; running > 0; running-- {
		<-finish
	}
	close(ch)
	<-done
	for _, d := range out {
		sort.Strings(d.services)
		sort.Strings(d.ndiSources)
	}
	return out
}

// absorbMDNS reads one mDNS response sent by `from` into out.
func absorbMDNS(b []byte, from string, out map[string]*devInfo) {
	for ip, name := range parseMDNSPTR(b) {
		if d := out[ip]; d != nil && d.src != "mdns" {
			d.name, d.src = name, "mdns"
		}
	}
	d := out[from]
	if d == nil {
		return
	}
	for _, rr := range parseRecords(b) {
		nameLabels, _, err := readLabels(b, rr.off)
		if err != nil {
			continue
		}
		switch rr.typ {
		case 12: // PTR
			if strings.EqualFold(rr.name, serviceEnum) {
				if t, _, err := readLabels(b, rr.data); err == nil && len(t) > 0 {
					if svc := strings.TrimPrefix(strings.ToLower(t[0]), "_"); svc != "" {
						d.services = mergeUnique(d.services, []string{svc})
					}
				}
				continue
			}
			// "_ndi._tcp.local" -> instance "PC-GRAFICA (Resolume)._ndi._tcp.local"
			if len(nameLabels) >= 2 && strings.HasPrefix(nameLabels[0], "_") && !strings.HasPrefix(nameLabels[0], "_services") {
				svc := strings.TrimPrefix(strings.ToLower(nameLabels[0]), "_")
				d.services = mergeUnique(d.services, []string{svc})
				if inst, _, err := readLabels(b, rr.data); err == nil && len(inst) > 0 {
					d.addInstance(svc, inst[0])
				}
			}
		case 16: // TXT of "<instance>._svc._tcp.local"
			if len(nameLabels) >= 3 && strings.HasPrefix(nameLabels[1], "_") {
				svc := strings.TrimPrefix(strings.ToLower(nameLabels[1]), "_")
				txt := parseTXT(b[rr.data : rr.data+rr.rdlen])
				if svc != "device-info" {
					d.services = mergeUnique(d.services, []string{svc})
				}
				d.addInstance(svc, nameLabels[0])
				switch svc {
				case "googlecast":
					if fn := txt["fn"]; fn != "" {
						d.friendly, d.fprio = fn, 0
					}
					d.setModel(1, txt["md"])
				case "device-info":
					d.setModel(0, txt["model"])
				case "airplay", "raop":
					d.setModel(2, firstNonEmpty(txt["model"], txt["am"]))
				case "ipp", "printer":
					d.setModel(3, strings.Trim(firstNonEmpty(txt["ty"], txt["product"]), "()"))
				case "netaudio-arc", "netaudio-cmc":
					d.setModel(1, firstNonEmpty(txt["model"], txt["mf"]))
				case "netaudio-chan":
					if n, err := strconv.Atoi(txt["rate"]); err == nil {
						d.danteRate = n
					}
					if n, err := strconv.Atoi(txt["latency_ns"]); err == nil {
						d.danteLatNs = n
					}
				}
			}
		}
	}
}

func (d *devInfo) addInstance(svc, inst string) {
	if inst == "" {
		return
	}
	if svc == "netaudio-chan" { // "Mic 1@CL5-FOH": a channel, not a device name
		ch, dev, ok := strings.Cut(inst, "@")
		if d.danteChans == nil {
			d.danteChans = map[string]bool{}
		}
		d.danteChans[ch] = true
		if ok && dev != "" {
			inst = dev
			svc = "netaudio-arc"
		} else {
			return
		}
	}
	if svc == "ndi" {
		d.ndiSources = mergeUnique(d.ndiSources, []string{inst})
		if host, _, ok := strings.Cut(inst, " ("); ok {
			inst = host // "PC-GRAFICA (Resolume)" -> "PC-GRAFICA"
		}
	}
	if svc == "raop" { // "A1B2C3D4E5F6@Soggiorno"
		if _, after, ok := strings.Cut(inst, "@"); ok {
			inst = after
		}
	}
	if p, ok := friendlyPrio[svc]; ok && p < d.fprio {
		d.friendly, d.fprio = inst, p
	}
}

func (d *devInfo) setModel(prio int, m string) {
	m = strings.TrimSpace(m)
	if m == "" || strings.HasPrefix(m, "_") || prio >= d.mprio {
		return
	}
	d.model, d.mprio = m, prio
}

// parseTXT splits a TXT record into key=value pairs (keys lower-cased).
func parseTXT(b []byte) map[string]string {
	out := map[string]string{}
	for len(b) > 0 {
		l := int(b[0])
		if 1+l > len(b) {
			break
		}
		k, v, _ := strings.Cut(string(b[1:1+l]), "=")
		out[strings.ToLower(k)] = v
		b = b[1+l:]
	}
	return out
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
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

// readLabels decodes a (possibly compressed) DNS name starting at off,
// keeping label boundaries (instance names may contain dots).
func readLabels(msg []byte, off int) ([]string, int, error) {
	var labels []string
	next := -1
	for jumps := 0; ; {
		if off >= len(msg) {
			return nil, 0, errors.New("short name")
		}
		l := int(msg[off])
		switch {
		case l == 0:
			if next < 0 {
				next = off + 1
			}
			return labels, next, nil
		case l&0xC0 == 0xC0:
			if off+1 >= len(msg) || jumps > 16 {
				return nil, 0, errors.New("bad pointer")
			}
			if next < 0 {
				next = off + 2
			}
			off = int(binary.BigEndian.Uint16(msg[off:]) & 0x3FFF)
			jumps++
		default:
			if off+1+l > len(msg) {
				return nil, 0, errors.New("short label")
			}
			labels = append(labels, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
}

// readName decodes a DNS name as dotted text.
func readName(msg []byte, off int) (string, int, error) {
	l, n, err := readLabels(msg, off)
	return strings.Join(l, "."), n, err
}

type record struct {
	name  string
	typ   uint16
	off   int // offset of the record name
	data  int // offset of the RDATA in the message
	rdlen int
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
		_, next, err := readLabels(msg, off)
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
		out = append(out, record{name: name, typ: typ, off: off, data: data, rdlen: rdlen})
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

// serviceEnum is the DNS-SD meta-query listing every service type a host offers.
const serviceEnum = "_services._dns-sd._udp.local"

func mergeUnique(a, b []string) []string {
	for _, x := range b {
		if !slices.Contains(a, x) {
			a = append(a, x)
		}
	}
	return a
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
