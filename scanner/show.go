package scanner

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Show-network discovery: Art-Net, sACN (E1.31), PTP and PJLink. Everything
// is passive or a standard poll, and needs no admin rights.

// ArtNetNode is what an Art-Net device says in its ArtPollReply.
type ArtNetNode struct {
	ShortName string `json:"short_name,omitempty"`
	LongName  string `json:"long_name,omitempty"`
	Report    string `json:"report,omitempty"`
	Style     string `json:"style,omitempty"` // node, console, media server…
	Firmware  int    `json:"firmware,omitempty"`
	MAC       string `json:"mac,omitempty"`
	Out       []int  `json:"out,omitempty"` // port-addresses (universes) sent to DMX
	In        []int  `json:"in,omitempty"`  // port-addresses received from DMX
}

// SACNSource is a device sending sACN (E1.31).
type SACNSource struct {
	Name      string `json:"name,omitempty"`
	Universes []int  `json:"universes,omitempty"`
	Priority  int    `json:"priority,omitempty"`
}

// showInfo collects the show protocols heard from one IP.
type showInfo struct {
	artnet   *ArtNetNode
	artMACs  []string // more than one: IP conflict
	sacn     *SACNSource
	ptp      string // "PTPv1" (Dante) / "PTPv2 dominio N" when this IP is the clock master
	ptpPrio1 int
}

var artStyles = map[byte]string{0: "nodo", 1: "console", 2: "media server", 3: "router", 4: "backup", 5: "configuratore", 6: "visualizzatore"}

// showDiscover polls Art-Net and listens to sACN and PTP for `wait`, on the
// network of iface (nil: all). It returns what was heard, by source IP.
func showDiscover(ctx context.Context, iface *net.Interface, wait time.Duration) map[string]*showInfo {
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	var mu sync.Mutex
	out := map[string]*showInfo{}
	get := func(ip string) *showInfo {
		if out[ip] == nil {
			out[ip] = &showInfo{}
		}
		return out[ip]
	}
	var wg sync.WaitGroup

	// Art-Net: ArtPoll to every broadcast address, replies come to port 6454.
	if c, err := listenShared("0.0.0.0:6454"); err == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.Close()
			poll := append([]byte("Art-Net\x00"), 0x00, 0x20, 0, 14, 0, 0)
			for _, b := range broadcastAddrs() {
				c.WriteToUDP(poll, &net.UDPAddr{IP: b, Port: 6454})
			}
			readUntil(ctx, c, func(b []byte, from *net.UDPAddr) {
				if n := parseArtPollReply(b); n != nil {
					ip := from.IP.String()
					if len(b) >= 14 {
						if rep := net.IP(b[10:14]).String(); rep != "0.0.0.0" {
							ip = rep // the node's own address (it may reply via broadcast)
						}
					}
					mu.Lock()
					s := get(ip)
					if n.MAC != "" && !slices.Contains(s.artMACs, n.MAC) {
						s.artMACs = append(s.artMACs, n.MAC)
					}
					if s.artnet == nil {
						s.artnet = n
					} else { // more binds of the same node: merge ports
						s.artnet.Out = mergeInts(s.artnet.Out, n.Out)
						s.artnet.In = mergeInts(s.artnet.In, n.In)
					}
					mu.Unlock()
				}
			})
		}()
	}

	// sACN: universe discovery plus the first universes.
	groups := []net.IP{net.IPv4(239, 255, 250, 214)}
	for u := 1; u <= 32; u++ {
		groups = append(groups, net.IPv4(239, 255, byte(u>>8), byte(u)))
	}
	for _, g := range groups {
		c, err := net.ListenMulticastUDP("udp4", iface, &net.UDPAddr{IP: g, Port: 5568})
		if err != nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.Close()
			readUntil(ctx, c, func(b []byte, from *net.UDPAddr) {
				name, unis, prio, ok := parseSACN(b)
				if !ok {
					return
				}
				mu.Lock()
				s := get(from.IP.String())
				if s.sacn == nil {
					s.sacn = &SACNSource{}
				}
				if name != "" {
					s.sacn.Name = name
				}
				if prio > 0 {
					s.sacn.Priority = prio
				}
				s.sacn.Universes = mergeInts(s.sacn.Universes, unis)
				mu.Unlock()
			})
		}()
	}

	// PTP: whoever sends Sync/Announce is a clock master. Dante uses PTPv1,
	// AES67 and AVB PTPv2. Ports below 1024 may need rights on Linux.
	for _, port := range []int{319, 320} {
		c, err := net.ListenMulticastUDP("udp4", iface, &net.UDPAddr{IP: net.IPv4(224, 0, 1, 129), Port: port})
		if err != nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.Close()
			readUntil(ctx, c, func(b []byte, from *net.UDPAddr) {
				if kind, prio := parsePTP(b); kind != "" {
					mu.Lock()
					s := get(from.IP.String())
					s.ptp, s.ptpPrio1 = kind, prio
					mu.Unlock()
				}
			})
		}()
	}
	wg.Wait()
	for _, s := range out {
		if s.artnet != nil {
			sort.Ints(s.artnet.Out)
			sort.Ints(s.artnet.In)
		}
		if s.sacn != nil {
			sort.Ints(s.sacn.Universes)
		}
	}
	return out
}

// broadcastAddrs: the general broadcast, every local network's broadcast,
// and the classic Art-Net networks 2.x and 10.x.
func broadcastAddrs() []net.IP {
	out := []net.IP{net.IPv4bcast, net.IPv4(2, 255, 255, 255), net.IPv4(10, 255, 255, 255)}
	if ifs, err := LocalInterfaces(); err == nil {
		for _, i := range ifs {
			if _, n, err := net.ParseCIDR(i.Network); err == nil {
				b := make(net.IP, 4)
				for k := range b {
					b[k] = n.IP[k] | ^n.Mask[k]
				}
				out = append(out, b)
			}
		}
	}
	return out
}

func cString(b []byte) string {
	if i := slices.Index(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(strings.ToValidUTF8(string(b), ""))
}

// parseArtPollReply decodes an Art-Net ArtPollReply (OpCode 0x2100).
func parseArtPollReply(b []byte) *ArtNetNode {
	if len(b) < 207 || string(b[:8]) != "Art-Net\x00" || binary.LittleEndian.Uint16(b[8:]) != 0x2100 {
		return nil
	}
	n := &ArtNetNode{
		Firmware:  int(binary.BigEndian.Uint16(b[16:])),
		ShortName: cString(b[26:44]),
		LongName:  cString(b[44:108]),
		Report:    cString(b[108:172]),
		Style:     artStyles[b[200]],
	}
	if mac := net.HardwareAddr(b[201:207]).String(); mac != "00:00:00:00:00:00" {
		n.MAC = mac
	}
	net7, sub := int(b[18]&0x7F), int(b[19]&0x0F)
	ports := int(b[173])
	for i := 0; i < ports && i < 4; i++ {
		t := b[174+i]
		if t&0x80 != 0 {
			n.Out = append(n.Out, net7<<8|sub<<4|int(b[190+i]&0x0F))
		}
		if t&0x40 != 0 {
			n.In = append(n.In, net7<<8|sub<<4|int(b[186+i]&0x0F))
		}
	}
	return n
}

// parseSACN decodes an E1.31 data or universe-discovery packet.
func parseSACN(b []byte) (name string, universes []int, priority int, ok bool) {
	if len(b) < 115 || string(b[4:16]) != "ASC-E1.17\x00\x00\x00" {
		return
	}
	root := binary.BigEndian.Uint32(b[18:])
	name = cString(b[44:108])
	switch root {
	case 0x00000004: // data
		return name, []int{int(binary.BigEndian.Uint16(b[113:]))}, int(b[108]), true
	case 0x00000008: // extended: universe discovery
		if len(b) < 120 || binary.BigEndian.Uint32(b[114:]) != 1 {
			return name, nil, 0, true
		}
		for off := 120; off+1 < len(b); off += 2 {
			universes = append(universes, int(binary.BigEndian.Uint16(b[off:])))
		}
		return name, universes, 0, true
	}
	return
}

// parsePTP recognizes a clock master's Sync or Announce message.
func parsePTP(b []byte) (string, int) {
	if len(b) >= 34 && b[1]&0x0F == 2 { // PTPv2
		switch b[0] & 0x0F {
		case 0x0B: // Announce
			prio := 0
			if len(b) >= 48 {
				prio = int(b[47])
			}
			return fmt.Sprintf("PTPv2 dominio %d", b[4]), prio
		case 0x00:
			return fmt.Sprintf("PTPv2 dominio %d", b[4]), 0
		}
	}
	if len(b) >= 33 && binary.BigEndian.Uint16(b) == 1 && b[32] == 0 { // PTPv1 Sync
		return "PTPv1", 0
	}
	return "", 0
}

func mergeInts(a, b []int) []int {
	for _, x := range b {
		if !slices.Contains(a, x) {
			a = append(a, x)
		}
	}
	return a
}

// PJLink is what a projector says over PJLink (port 4352).
type PJLink struct {
	Name       string `json:"name,omitempty"`
	Maker      string `json:"maker,omitempty"`
	Model      string `json:"model,omitempty"`
	Power      string `json:"power,omitempty"`
	LampHours  []int  `json:"lamp_hours,omitempty"`
	Errors     string `json:"errors,omitempty"`
	NeedsLogin bool   `json:"needs_login,omitempty"`
}

var pjPower = map[string]string{"0": "spento", "1": "acceso", "2": "in raffreddamento", "3": "in accensione"}

// queryPJLink asks a projector for its name, model, lamp hours and state.
func queryPJLink(ctx context.Context, ip string, timeout time.Duration) *PJLink {
	return queryPJLinkAddr(ctx, net.JoinHostPort(ip, "4352"), timeout)
}

func queryPJLinkAddr(ctx context.Context, addr string, timeout time.Duration) *PJLink {
	d := net.Dialer{Timeout: timeout}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * timeout))
	r := bufio.NewReader(c)
	line := func() string { s, _ := r.ReadString('\r'); return strings.TrimSpace(s) }
	hello := line()
	if !strings.HasPrefix(strings.ToUpper(hello), "PJLINK") {
		return nil
	}
	p := &PJLink{}
	if !strings.HasPrefix(strings.ToUpper(hello), "PJLINK 0") {
		p.NeedsLogin = true
		return p
	}
	ask := func(cmd string) string {
		fmt.Fprintf(c, "%%1%s ?\r", cmd)
		ans := line()
		if _, v, ok := strings.Cut(ans, "="); ok && !strings.HasPrefix(v, "ERR") {
			return strings.TrimSpace(v)
		}
		return ""
	}
	p.Name, p.Maker, p.Model = ask("NAME"), ask("INF1"), ask("INF2")
	p.Power = pjPower[ask("POWR")]
	f := strings.Fields(ask("LAMP"))
	for i := 0; i+1 < len(f); i += 2 {
		if h, err := strconv.Atoi(f[i]); err == nil {
			p.LampHours = append(p.LampHours, h)
		}
	}
	if e := ask("ERST"); len(e) == 6 && e != "000000" {
		names := []string{"ventola", "lampada", "temperatura", "coperchio", "filtro", "altro"}
		var bad []string
		for i, ch := range e {
			switch ch {
			case '1':
				bad = append(bad, names[i]+" (avviso)")
			case '2':
				bad = append(bad, names[i]+" (errore)")
			}
		}
		p.Errors = strings.Join(bad, ", ")
	}
	return p
}
