package scanner

import (
	"bufio"
	"context"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Traffic on the show network: what arrives at this computer's interface
// (packets and bits per second, and the broadcast/multicast share where
// the system counts it), plus the show protocols in it. A switch delivers
// to every port only broadcast and multicast (without IGMP snooping), so
// what arrives here unasked is what every device on the network gets too.

// IfaceCounters are the interface counters at one moment (-1 = unknown).
type IfaceCounters struct {
	RxPkts, RxBytes, RxNonUnicast int64
}

// ReadIfaceCounters reads the received packets and bytes of an interface
// (on Windows, of all interfaces together).
func ReadIfaceCounters(name string) (IfaceCounters, bool) {
	c := IfaceCounters{-1, -1, -1}
	switch runtime.GOOS {
	case "linux":
		rd := func(f string) int64 {
			b, err := os.ReadFile("/sys/class/net/" + name + "/statistics/" + f)
			if err != nil {
				return -1
			}
			v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
			if err != nil {
				return -1
			}
			return v
		}
		c.RxPkts, c.RxBytes, c.RxNonUnicast = rd("rx_packets"), rd("rx_bytes"), rd("multicast")
	case "windows":
		out, err := exec.Command("netstat", "-e").Output()
		if err != nil {
			return c, false
		}
		c = parseNetstatE(string(out))
	default: // macOS, BSD
		out, err := exec.Command("netstat", "-ibn", "-I", name).Output()
		if err != nil {
			return c, false
		}
		c = parseNetstatIbn(string(out))
	}
	return c, c.RxPkts >= 0
}

// parseNetstatIbn reads the <Link#…> line of `netstat -ibn -I en0`: the
// columns end with Ipkts Ierrs Ibytes Opkts Oerrs Obytes Coll.
func parseNetstatIbn(out string) IfaceCounters {
	c := IfaceCounters{-1, -1, -1}
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) < 9 || !strings.HasPrefix(f[2], "<Link") {
			continue
		}
		p, err1 := strconv.ParseInt(f[len(f)-7], 10, 64)
		b, err2 := strconv.ParseInt(f[len(f)-5], 10, 64)
		if err1 == nil && err2 == nil {
			c.RxPkts, c.RxBytes = p, b
		}
		break
	}
	return c
}

var reTwoNums = regexp.MustCompile(`(\d+)\s+(\d+)\s*$`)

// parseNetstatE reads `netstat -e` (Windows), whatever the language: the
// lines with two numbers are bytes, unicast packets, non-unicast packets…
func parseNetstatE(out string) IfaceCounters {
	c := IfaceCounters{-1, -1, -1}
	var rows []int64
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if m := reTwoNums.FindStringSubmatch(sc.Text()); m != nil {
			v, _ := strconv.ParseInt(m[1], 10, 64)
			rows = append(rows, v)
		}
	}
	if len(rows) >= 3 {
		c.RxBytes, c.RxPkts, c.RxNonUnicast = rows[0], rows[1]+rows[2], rows[2]
	}
	return c
}

// TrafficProto is one show protocol heard during the measurement.
type TrafficProto struct {
	Name      string   `json:"name"`
	Universes int      `json:"universes,omitempty"`
	PPS       float64  `json:"pps"`
	Mbps      float64  `json:"mbps"`
	Sources   []string `json:"sources,omitempty"`
}

// TrafficReport is what MeasureTraffic found.
type TrafficReport struct {
	Interface    string         `json:"interface"`
	Seconds      float64        `json:"seconds"`
	RxPPS        float64        `json:"rx_pps"`               // -1 = unknown
	RxMbps       float64        `json:"rx_mbps"`              // -1 = unknown
	NonUnicastPS float64        `json:"nonunicast_pps"`       // -1 = unknown
	AllIfaces    bool           `json:"all_ifaces,omitempty"` // Windows: the counters cover every interface
	Protocols    []TrafficProto `json:"protocols"`
	ArtBroadcast []int          `json:"art_broadcast,omitempty"` // Art-Net universes heard (so sent to everyone)
}

// Wire sizes of one DMX frame with the Ethernet, IP and UDP headers.
const (
	artDmxWire = 530 + 42
	sacnWire   = 638 + 42
)

// MeasureTraffic listens for d and reports the traffic arriving on iface.
func MeasureTraffic(ctx context.Context, iface Interface, d time.Duration) TrafficReport {
	rep := TrafficReport{Interface: iface.Name, RxPPS: -1, RxMbps: -1, NonUnicastPS: -1, AllIfaces: runtime.GOOS == "windows"}
	before, okB := ReadIfaceCounters(iface.Name)
	start := time.Now()
	var last DMXSnap
	var tcPkts float64
	lctx, cancel := context.WithTimeout(ctx, d)
	ListenDMXFull(lctx, InterfaceFor(net.ParseIP(iface.IP)), 64, time.Second, func() DMXWant { return DMXWant{} }, func(s DMXSnap) {
		if len(s.Universes) > 0 || len(last.Universes) == 0 {
			last = s
		}
		for _, t := range s.Timecode {
			tcPkts = max(tcPkts, float64(t.PPS))
		}
	})
	cancel()
	rep.Seconds = time.Since(start).Seconds()
	if after, okA := ReadIfaceCounters(iface.Name); okA && okB && rep.Seconds > 0 {
		rep.RxPPS = float64(after.RxPkts-before.RxPkts) / rep.Seconds
		rep.RxMbps = float64(after.RxBytes-before.RxBytes) * 8 / 1e6 / rep.Seconds
		if after.RxNonUnicast >= 0 && before.RxNonUnicast >= 0 {
			rep.NonUnicastPS = float64(after.RxNonUnicast-before.RxNonUnicast) / rep.Seconds
		}
	}
	protos := map[string]*TrafficProto{}
	srcs := map[string]map[string]bool{}
	for _, u := range last.Universes {
		p := protos[u.Protocol]
		if p == nil {
			p = &TrafficProto{Name: u.Protocol}
			protos[u.Protocol] = p
			srcs[u.Protocol] = map[string]bool{}
		}
		p.Universes++
		wire := float64(sacnWire)
		if u.Protocol == "Art-Net" {
			wire = artDmxWire
			rep.ArtBroadcast = append(rep.ArtBroadcast, u.Number)
		}
		for _, s := range u.Sources {
			p.PPS += s.FPS
			p.Mbps += s.FPS * wire * 8 / 1e6
			srcs[u.Protocol][firstNonEmpty(s.Name, s.IP)] = true
		}
	}
	for _, name := range []string{"Art-Net", "sACN"} {
		if p := protos[name]; p != nil {
			for s := range srcs[name] {
				p.Sources = append(p.Sources, s)
			}
			rep.Protocols = append(rep.Protocols, *p)
		}
	}
	if tcPkts > 0 {
		rep.Protocols = append(rep.Protocols, TrafficProto{Name: "Timecode Art-Net", PPS: tcPkts, Mbps: tcPkts * (19 + 42) * 8 / 1e6})
	}
	if rep.Protocols == nil {
		rep.Protocols = []TrafficProto{}
	}
	return rep
}
