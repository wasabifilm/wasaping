package scanner

import (
	"context"
	"encoding/binary"
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
}

// Universe is the live state of one DMX universe.
type Universe struct {
	ID       string      `json:"id"` // "art:0", "sacn:1"
	Protocol string      `json:"protocol"`
	Number   int         `json:"number"`
	Sources  []DMXSource `json:"sources"`
	Active   int         `json:"active"` // channels above zero
	Levels   []int       `json:"levels,omitempty"`
}

type dmxSrc struct {
	name  string
	prio  int
	times []time.Time
}

type dmxUni struct {
	proto   string
	number  int
	levels  [512]byte
	sources map[string]*dmxSrc
	last    time.Time
}

// ListenDMX listens until ctx ends and calls snapshot every `every` with all
// universes heard (levels only for `detail`, e.g. "sacn:1").
func ListenDMX(ctx context.Context, iface *net.Interface, sacnUniverses int, every time.Duration, detail func() string, snapshot func([]Universe)) {
	var mu sync.Mutex
	unis := map[string]*dmxUni{}
	feed := func(proto string, number int, ip, name string, prio int, data []byte) {
		id := fmt.Sprintf("%s:%d", map[string]string{"Art-Net": "art", "sACN": "sacn"}[proto], number)
		mu.Lock()
		defer mu.Unlock()
		u := unis[id]
		if u == nil {
			u = &dmxUni{proto: proto, number: number, sources: map[string]*dmxSrc{}}
			unis[id] = u
		}
		copy(u.levels[:], data)
		u.last = time.Now()
		s := u.sources[ip]
		if s == nil {
			s = &dmxSrc{}
			u.sources[ip] = s
		}
		if name != "" {
			s.name = name
		}
		s.prio = prio
		s.times = append(s.times, u.last)
	}
	var wg sync.WaitGroup
	if c, err := listenShared("0.0.0.0:6454"); err == nil {
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
				if uni, data, ok := parseArtDmx(buf[:n]); ok {
					feed("Art-Net", uni, from.IP.String(), "", 0, data)
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
					feed("sACN", uni, from.IP.String(), name, prio, data)
				}
			}
		}()
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-t.C:
		}
		want := detail()
		now := time.Now()
		mu.Lock()
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
			for ip, s := range u.sources {
				var recent []time.Time
				for _, x := range s.times {
					if now.Sub(x) <= time.Second {
						recent = append(recent, x)
					}
				}
				s.times = recent
				if len(recent) == 0 && now.Sub(u.last) > 2*time.Second {
					continue
				}
				v.Sources = append(v.Sources, DMXSource{IP: ip, Name: s.name, Priority: s.prio, FPS: float64(len(recent))})
			}
			sort.Slice(v.Sources, func(i, j int) bool { return v.Sources[i].IP < v.Sources[j].IP })
			if id == want {
				v.Levels = make([]int, 512)
				for i, b := range u.levels {
					v.Levels[i] = int(b)
				}
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
		snapshot(out)
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
