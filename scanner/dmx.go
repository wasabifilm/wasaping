package scanner

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

// Live DMX view: who sends which universes, how often, and the levels.
// Art-Net (ArtDmx, broadcast or to this computer) and sACN (multicast).

// DMXSource is one sender of a universe.
type DMXSource struct {
	IP       string  `json:"ip"`
	Name     string  `json:"name,omitempty"`
	Priority int     `json:"priority,omitempty"`
	FPS      float64 `json:"fps"`
	Lost     int     `json:"lost,omitempty"`    // frames missing (sequence gaps) in the last second
	MaxGapMs int     `json:"max_gap,omitempty"` // longest pause between two frames in the last second
	Data     string  `json:"data,omitempty"`    // base64 levels of this source (the universe looked at, with more sources)
}

// DMXWant says what the viewer is looking at: the levels of one universe
// (and the history of one channel of it), or the levels of all.
type DMXWant struct {
	Uni     string // "sacn:1"
	Channel int    // 1-512, 0 = none
	All     bool
	Nodes   bool // poll Art-Net nodes every few seconds
	// HistSecs is how much history of the channel to send: 10 (default),
	// 60 or up to 300 seconds, thinned to at most histPoints points.
	HistSecs int
}

// MaxHistSecs is the longest channel history kept.
const MaxHistSecs = 300

const histPoints = 600

// thinHistory keeps the points of the last secs seconds (before end, unix
// ms) and, when there are too many, the lowest and the highest value of
// each slice of time, so that a short flash or drop still shows.
func thinHistory(h [][2]int64, secs int, end int64) [][2]int64 {
	if secs <= 0 {
		secs = 10
	}
	from := end - int64(min(secs, MaxHistSecs))*1000
	i := 0
	for i < len(h) && h[i][0] < from {
		i++
	}
	if i > 0 { // the value at the start of the window
		h = append([][2]int64{{from, h[i-1][1]}}, h[i:]...)
	} else {
		h = append([][2]int64(nil), h...)
	}
	if len(h) <= histPoints {
		return h
	}
	slots := histPoints / 2
	step := (end - from) / int64(slots)
	var out [][2]int64
	j := 0
	for k := 0; k < slots && j < len(h); k++ {
		lim := from + step*int64(k+1)
		if k == slots-1 {
			lim = end + 1
		}
		lo, hi := h[j], h[j]
		for ; j < len(h) && h[j][0] < lim; j++ {
			if h[j][1] < lo[1] {
				lo = h[j]
			}
			if h[j][1] > hi[1] {
				hi = h[j]
			}
		}
		if lo[0] > hi[0] {
			lo, hi = hi, lo
		}
		out = append(out, lo)
		if hi != lo {
			out = append(out, hi)
		}
	}
	return out
}

// ArtNode is an Art-Net node heard in an ArtPollReply, with its address.
type ArtNode struct {
	IP string `json:"ip"`
	ArtNetNode
}

// DMXSnap is everything heard: universes, Art-Net nodes (when asked) and
// timecode streams.
type DMXSnap struct {
	Universes []Universe       `json:"universes"`
	Nodes     []ArtNode        `json:"nodes,omitempty"`
	Timecode  []TimecodeSource `json:"timecode,omitempty"`
}

// Universe is the live state of one DMX universe.
type Universe struct {
	ID       string      `json:"id"` // "art:0", "sacn:1"
	Protocol string      `json:"protocol"`
	Number   int         `json:"number"`
	Sources  []DMXSource `json:"sources"`
	Active   int         `json:"active"` // channels above zero
	Levels   []int       `json:"levels,omitempty"`
	Data     string      `json:"data,omitempty"`    // base64 of the 512 levels (overview)
	History  [][2]int64  `json:"history,omitempty"` // unix ms, value of the watched channel
}

type dmxSrc struct {
	ip     string
	name   string
	prio   int
	times  []time.Time
	levels [512]byte
	seq    int // last sequence number, -1 = none yet
	lost   []time.Time
}

type dmxUni struct {
	proto   string
	number  int
	levels  [512]byte
	sources map[string]*dmxSrc
	last    time.Time
	watch   int // channel whose history is kept
	hist    [][2]int64
}

// seqGap is how many frames are missing between two sequence numbers (0 =
// none or out of order; a sequence of 0 means "not used" in Art-Net).
func seqGap(last, now int) int {
	if last < 0 || now < 0 {
		return 0
	}
	d := (now - last + 256) % 256
	if d >= 2 && d < 128 {
		return d - 1
	}
	return 0
}

// ListenDMX listens until ctx ends and calls snapshot every `every` with all
// universes heard (levels only for `detail`, e.g. "sacn:1").
func ListenDMX(ctx context.Context, iface *net.Interface, sacnUniverses int, every time.Duration, detail func() DMXWant, snapshot func([]Universe)) {
	ListenDMXFull(ctx, iface, sacnUniverses, every, detail, func(s DMXSnap) { snapshot(s.Universes) })
}

// ListenDMXFull is ListenDMX with Art-Net nodes and timecode as well.
func ListenDMXFull(ctx context.Context, iface *net.Interface, sacnUniverses int, every time.Duration, detail func() DMXWant, snapshot func(DMXSnap)) {
	var mu sync.Mutex
	var tc timecodes
	nodes := map[string]ArtNode{}
	nodeSeen := map[string]time.Time{}
	var artConn *net.UDPConn
	unis := map[string]*dmxUni{}
	var want DMXWant
	// key tells sources apart: the IP, plus the CID in sACN (two consoles
	// can run on the same computer).
	feed := func(proto string, number int, key, ip, name string, prio, seq int, data []byte) {
		id := fmt.Sprintf("%s:%d", map[string]string{"Art-Net": "art", "sACN": "sacn"}[proto], number)
		mu.Lock()
		defer mu.Unlock()
		u := unis[id]
		if u == nil {
			u = &dmxUni{proto: proto, number: number, sources: map[string]*dmxSrc{}}
			unis[id] = u
		}
		u.last = time.Now()
		s := u.sources[key]
		if s == nil {
			s = &dmxSrc{seq: -1, ip: ip}
			u.sources[key] = s
		}
		if name != "" {
			s.name = name
		}
		s.prio = prio
		s.times = append(s.times, u.last)
		if n := seqGap(s.seq, seq); n > 0 {
			for i := 0; i < n && i < 256; i++ {
				s.lost = append(s.lost, u.last)
			}
		}
		s.seq = seq
		copy(s.levels[:], data)
		// What the fixtures get: the highest sACN priority wins; among equal
		// priorities (and in Art-Net) the highest level (HTP), as most
		// receivers merge.
		top := -1
		for _, x := range u.sources {
			if u.last.Sub(x.times[len(x.times)-1]) <= 2*time.Second {
				top = max(top, x.prio)
			}
		}
		u.levels = [512]byte{}
		for _, x := range u.sources {
			if x.prio == top && u.last.Sub(x.times[len(x.times)-1]) <= 2*time.Second {
				for i, v := range x.levels {
					u.levels[i] = max(u.levels[i], v)
				}
			}
		}
		if id == want.Uni && want.Channel >= 1 && want.Channel <= 512 {
			if u.watch != want.Channel {
				u.watch, u.hist = want.Channel, nil
			}
			v := int64(u.levels[want.Channel-1])
			if len(u.hist) == 0 || u.hist[len(u.hist)-1][1] != v || u.last.UnixMilli()-u.hist[len(u.hist)-1][0] > 200 {
				u.hist = append(u.hist, [2]int64{u.last.UnixMilli(), v})
			}
			for len(u.hist) > 0 && u.last.UnixMilli()-u.hist[0][0] > MaxHistSecs*1000 {
				u.hist = u.hist[1:]
			}
		}
	}
	var wg sync.WaitGroup
	if c, err := listenShared("0.0.0.0:6454"); err == nil {
		artConn = c
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.Close()
			go func() { <-ctx.Done(); c.SetReadDeadline(time.Now()) }()
			buf := make([]byte, 1024)
			for ctx.Err() == nil {
				n, from, err := c.ReadFromUDP(buf)
				if err != nil {
					continue
				}
				if st, h, m, s, f, typ, ok := parseArtTimeCode(buf[:n]); ok {
					mu.Lock()
					tc.feed(time.Now(), from.IP.String(), st, h, m, s, f, typ)
					mu.Unlock()
					continue
				}
				if nd := parseArtPollReply(buf[:n]); nd != nil {
					ip := from.IP.String()
					if rep := net.IP(buf[10:14]).String(); rep != "0.0.0.0" {
						ip = rep
					}
					mu.Lock()
					if old, ok := nodes[ip]; ok && time.Since(nodeSeen[ip]) < 2*time.Second { // more binds of one node
						nd.Out, nd.In = mergeInts(old.Out, nd.Out), mergeInts(old.In, nd.In)
					}
					nodes[ip], nodeSeen[ip] = ArtNode{IP: ip, ArtNetNode: *nd}, time.Now()
					mu.Unlock()
					continue
				}
				if uni, data, ok := parseArtDmx(buf[:n]); ok {
					seq := int(buf[12])
					if seq == 0 {
						seq = -1 // sequence not used by this sender
					}
					feed("Art-Net", uni, from.IP.String(), from.IP.String(), "", 0, seq, data)
				}
			}
		}()
	}
	for u := 1; u <= sacnUniverses; u++ {
		c, err := net.ListenMulticastUDP("udp4", iface, &net.UDPAddr{IP: net.IPv4(239, 255, byte(u>>8), byte(u)), Port: 5568})
		if err != nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.Close()
			go func() { <-ctx.Done(); c.SetReadDeadline(time.Now()) }()
			buf := make([]byte, 1200)
			for ctx.Err() == nil {
				n, from, err := c.ReadFromUDP(buf)
				if err != nil {
					continue
				}
				if uni, name, prio, data, ok := parseSACNData(buf[:n]); ok {
					feed("sACN", uni, from.IP.String()+"/"+hex.EncodeToString(buf[22:38]), from.IP.String(), name, prio, int(buf[111]), data)
				}
			}
		}()
	}
	t := time.NewTicker(every)
	defer t.Stop()
	var polled time.Time
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-t.C:
		}
		w := detail()
		now := time.Now()
		if w.Nodes && artConn != nil && now.Sub(polled) >= 3*time.Second {
			polled = now
			poll := append([]byte("Art-Net\x00"), 0x00, 0x20, 0, 14, 0, 0)
			for _, b := range broadcastAddrs() {
				artConn.WriteToUDP(poll, &net.UDPAddr{IP: b, Port: ArtNetNodePort})
			}
		}
		mu.Lock()
		want = w
		snap := DMXSnap{Timecode: tc.snapshot(now)}
		for ip, nd := range nodes {
			if now.Sub(nodeSeen[ip]) > 10*time.Second {
				delete(nodes, ip)
				delete(nodeSeen, ip)
				continue
			}
			snap.Nodes = append(snap.Nodes, nd)
		}
		var out []Universe
		for id, u := range unis {
			if now.Sub(u.last) > 5*time.Second {
				delete(unis, id) // stopped sending
				continue
			}
			v := Universe{ID: id, Protocol: u.proto, Number: u.number}
			for _, b := range u.levels {
				if b > 0 {
					v.Active++
				}
			}
			for key, s := range u.sources {
				var recent []time.Time
				gap := time.Duration(0)
				for i, x := range s.times {
					if now.Sub(x) <= time.Second {
						if len(recent) > 0 {
							gap = max(gap, x.Sub(recent[len(recent)-1]))
						} else if i > 0 {
							gap = max(gap, x.Sub(s.times[i-1]))
						}
						recent = append(recent, x)
					}
				}
				if len(recent) > 0 {
					s.times = recent
				} else if len(s.times) > 1 {
					s.times = s.times[len(s.times)-1:] // keep the last one: when it stopped
				}
				var lost []time.Time
				for _, x := range s.lost {
					if now.Sub(x) <= time.Second {
						lost = append(lost, x)
					}
				}
				s.lost = lost
				if len(recent) == 0 {
					if now.Sub(s.times[len(s.times)-1]) > 2*time.Second {
						delete(u.sources, key) // this source stopped
					}
					continue
				}
				src := DMXSource{IP: s.ip, Name: s.name, Priority: s.prio, FPS: float64(len(recent)), Lost: len(lost), MaxGapMs: int(gap / time.Millisecond)}
				if id == w.Uni && len(u.sources) > 1 {
					src.Data = base64.StdEncoding.EncodeToString(s.levels[:])
				}
				v.Sources = append(v.Sources, src)
			}
			sort.Slice(v.Sources, func(i, j int) bool {
				if v.Sources[i].IP != v.Sources[j].IP {
					return v.Sources[i].IP < v.Sources[j].IP
				}
				return v.Sources[i].Name < v.Sources[j].Name
			})
			if id == w.Uni {
				v.Levels = make([]int, 512)
				for i, b := range u.levels {
					v.Levels[i] = int(b)
				}
				if w.Channel > 0 && u.watch == w.Channel {
					v.History = thinHistory(u.hist, w.HistSecs, time.Now().UnixMilli())
				}
			}
			if w.All {
				v.Data = base64.StdEncoding.EncodeToString(u.levels[:])
			}
			out = append(out, v)
		}
		mu.Unlock()
		sort.Slice(out, func(i, j int) bool {
			if out[i].Protocol != out[j].Protocol {
				return out[i].Protocol < out[j].Protocol
			}
			return out[i].Number < out[j].Number
		})
		sort.Slice(snap.Nodes, func(i, j int) bool {
			return ipToU32(net.ParseIP(snap.Nodes[i].IP)) < ipToU32(net.ParseIP(snap.Nodes[j].IP))
		})
		snap.Universes = out
		snapshot(snap)
	}
}

// parseArtDmx decodes an ArtDmx packet (OpCode 0x5000).
func parseArtDmx(b []byte) (universe int, data []byte, ok bool) {
	if len(b) < 18 || string(b[:8]) != "Art-Net\x00" || binary.LittleEndian.Uint16(b[8:]) != 0x5000 {
		return
	}
	universe = int(b[15]&0x7F)<<8 | int(b[14])
	n := int(binary.BigEndian.Uint16(b[16:]))
	if n > 512 || 18+n > len(b) {
		n = min(512, len(b)-18)
	}
	return universe, b[18 : 18+n], true
}

// parseSACNData decodes an E1.31 data packet with DMX levels (start code 0).
func parseSACNData(b []byte) (universe int, name string, prio int, data []byte, ok bool) {
	if len(b) < 126 || string(b[4:16]) != "ASC-E1.17\x00\x00\x00" || binary.BigEndian.Uint32(b[18:]) != 4 {
		return
	}
	if b[125] != 0 { // not DMX levels (e.g. per-address priority)
		return
	}
	count := int(binary.BigEndian.Uint16(b[123:])) - 1
	if count < 0 || 126+count > len(b) {
		count = len(b) - 126
	}
	if count > 512 {
		count = 512
	}
	return int(binary.BigEndian.Uint16(b[113:])), cString(b[44:108]), int(b[108]), b[126 : 126+count], true
}

// InterfaceFor returns the local interface on the network of ip (nil if none).
func InterfaceFor(ip net.IP) *net.Interface {
	return interfaceFor([]net.IP{ip}, localNetworks())
}

// ListenPTP calls onMaster for every Sync/Announce heard (a clock master
// speaking) until ctx ends. Ports 319/320 may need rights on Linux.
func ListenPTP(ctx context.Context, iface *net.Interface, onMaster func(kind, ip string)) {
	ListenPTPAll(ctx, iface, onMaster, nil)
}

// ListenPTPAll also calls onFollower (if set) for every device asking the
// master for the delay: a device following the clock.
func ListenPTPAll(ctx context.Context, iface *net.Interface, onMaster func(kind, ip string), onFollower func(ip string)) {
	var wg sync.WaitGroup
	for _, port := range []int{319, 320} {
		c, err := net.ListenMulticastUDP("udp4", iface, &net.UDPAddr{IP: net.IPv4(224, 0, 1, 129), Port: port})
		if err != nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.Close()
			go func() { <-ctx.Done(); c.SetReadDeadline(time.Now()) }()
			buf := make([]byte, 1500)
			for ctx.Err() == nil {
				n, from, err := c.ReadFromUDP(buf)
				if err != nil {
					continue
				}
				if kind, _ := parsePTP(buf[:n]); kind != "" {
					onMaster(kind, from.IP.String())
				} else if onFollower != nil && ptpFollower(buf[:n]) {
					onFollower(from.IP.String())
				}
			}
		}()
	}
	wg.Wait()
}
