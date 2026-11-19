package scanner

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// RDM through Art-Net, read-only: list the fixtures connected behind each
// node (ArtTodRequest/ArtTodData) and ask each one who it is (ArtRdm with
// RDM GET). Nothing is ever changed on the fixtures.

// Fixture is a DMX device found with RDM.
type Fixture struct {
	UID         string `json:"uid"` // "0000:12345678"
	Node        string `json:"node"`
	Universe    int    `json:"universe"` // Art-Net port-address
	Label       string `json:"label,omitempty"`
	Maker       string `json:"maker,omitempty"`
	Model       string `json:"model,omitempty"`
	Address     int    `json:"address,omitempty"` // DMX start address (1-512, 0 = none)
	Footprint   int    `json:"footprint,omitempty"`
	Personality string `json:"personality,omitempty"` // "2/5"
	Software    string `json:"software,omitempty"`
}

const (
	pidDeviceInfo   = 0x0060
	pidModelDescr   = 0x0080
	pidMakerLabel   = 0x0081
	pidDeviceLabel  = 0x0082
	pidSoftwareText = 0x00C0
)

// Our controller UID: manufacturer 0x7FF0 is reserved for prototypes and
// tests by ESTA; the device part is random.
var rdmSrc = func() [6]byte {
	var u [6]byte
	u[0], u[1] = 0x7F, 0xF0
	rand.Read(u[2:])
	return u
}()

func artHeader(op uint16, size int) []byte {
	b := make([]byte, size)
	copy(b, "Art-Net\x00")
	binary.LittleEndian.PutUint16(b[8:], op)
	b[10], b[11] = 0, 14
	return b
}

// artTodRequest asks a node for the table of devices on one universe.
func artTodRequest(universe int) []byte {
	b := artHeader(0x8000, 25)
	b[21] = byte(universe >> 8 & 0x7F) // net
	b[22] = 0                          // TodFull
	b[23] = 1                          // one address
	b[24] = byte(universe & 0xFF)      // sub-net + universe
	return b
}

// parseArtTodData returns the universe and UIDs of an ArtTodData packet.
func parseArtTodData(b []byte) (universe int, uids [][6]byte, ok bool) {
	if len(b) < 28 || string(b[:8]) != "Art-Net\x00" || binary.LittleEndian.Uint16(b[8:]) != 0x8100 {
		return
	}
	universe = int(b[21]&0x7F)<<8 | int(b[23])
	n := int(b[27])
	for i := 0; i < n && 28+i*6+6 <= len(b); i++ {
		var u [6]byte
		copy(u[:], b[28+i*6:])
		uids = append(uids, u)
	}
	return universe, uids, true
}

// rdmGet builds an RDM GET request (without the 0xCC start code, as ArtRdm carries it).
func rdmGet(dst [6]byte, tn byte, pid uint16) []byte { return rdmMessage(dst, tn, 0x20, pid, nil) }

func rdmMessage(dst [6]byte, tn, cc byte, pid uint16, pd []byte) []byte {
	m := []byte{0xCC, 0x01, byte(24 + len(pd))}
	m = append(m, dst[:]...)
	m = append(m, rdmSrc[:]...)
	m = append(m, tn, 1, 0, 0, 0, cc)
	m = binary.BigEndian.AppendUint16(m, pid)
	m = append(m, byte(len(pd)))
	m = append(m, pd...)
	var sum uint16
	for _, c := range m {
		sum += uint16(c)
	}
	m = binary.BigEndian.AppendUint16(m, sum)
	return m[1:]
}

func artRdm(universe int, rdm []byte) []byte {
	b := artHeader(0x8300, 24)
	b[12] = 1                          // RDM standard 1.0
	b[21] = byte(universe >> 8 & 0x7F) // net
	b[22] = 0                          // ArProcess
	b[23] = byte(universe & 0xFF)
	return append(b, rdm...)
}

type rdmReply struct {
	src [6]byte
	tn  byte
	cc  byte
	pid uint16
	pd  []byte
}

// parseArtRdmReply decodes an ArtRdm packet carrying an RDM GET response.
func parseArtRdmReply(b []byte) (rdmReply, bool) {
	var r rdmReply
	if len(b) < 24+24 || string(b[:8]) != "Art-Net\x00" || binary.LittleEndian.Uint16(b[8:]) != 0x8300 {
		return r, false
	}
	m := b[24:]
	if m[0] == 0xCC { // some nodes keep the start code
		m = m[1:]
	}
	if len(m) < 23 || m[0] != 0x01 {
		return r, false
	}
	copy(r.src[:], m[8:14])
	r.tn = m[14]
	respType, cc := m[15], m[19]
	if respType != 0x00 || (cc != 0x21 && cc != 0x31) { // only ACKs to GET and SET
		return r, false
	}
	r.cc = cc
	r.pid = binary.BigEndian.Uint16(m[20:])
	pdl := int(m[22])
	if 23+pdl > len(m) {
		return r, false
	}
	r.pd = m[23 : 23+pdl]
	return r, true
}

func uidString(u [6]byte) string {
	return fmt.Sprintf("%02X%02X:%02X%02X%02X%02X", u[0], u[1], u[2], u[3], u[4], u[5])
}

// RDMDiscover asks every node (IP -> its output universes) for its fixtures.
func RDMDiscover(ctx context.Context, nodes map[string][]int) []Fixture {
	c, err := listenShared("0.0.0.0:6454")
	if err != nil {
		return nil
	}
	defer c.Close()
	type key struct {
		node string
		uni  int
	}
	var mu sync.Mutex
	tod := map[key][][6]byte{}
	replies := make(chan rdmReply, 256)
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := c.ReadFromUDP(buf)
			if err != nil {
				return
			}
			b := buf[:n]
			if u, uids, ok := parseArtTodData(b); ok {
				mu.Lock()
				k := key{from.IP.String(), u}
				for _, x := range uids {
					if !containsUID(tod[k], x) {
						tod[k] = append(tod[k], x)
					}
				}
				mu.Unlock()
			} else if r, ok := parseArtRdmReply(b); ok {
				select {
				case replies <- r:
				default:
				}
			}
		}
	}()
	for ip, unis := range nodes {
		for _, u := range unis {
			c.WriteToUDP(artTodRequest(u), &net.UDPAddr{IP: net.ParseIP(ip), Port: ArtNetNodePort})
		}
	}
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(2 * time.Second): // nodes may take a moment to run discovery
	}

	// Ask each fixture, one request at a time (RDM is half-duplex per line).
	mu.Lock()
	keys := make([]key, 0, len(tod))
	for k := range tod {
		keys = append(keys, k)
	}
	mu.Unlock()
	var out []Fixture
	var tn byte
	ask := func(k key, uid [6]byte, pid uint16) []byte {
		tn++
		req := artRdm(k.uni, rdmGet(uid, tn, pid))
		for try := 0; try < 2; try++ {
			c.WriteToUDP(req, &net.UDPAddr{IP: net.ParseIP(k.node), Port: ArtNetNodePort})
			deadline := time.After(400 * time.Millisecond)
			for {
				select {
				case r := <-replies:
					if r.src == uid && r.pid == pid {
						return r.pd
					}
					continue
				case <-deadline:
				case <-ctx.Done():
					return nil
				}
				break
			}
		}
		return nil
	}
	count := 0
	for _, k := range keys {
		for _, uid := range tod[k] {
			if count++; count > 512 || ctx.Err() != nil {
				break
			}
			f := Fixture{UID: uidString(uid), Node: k.node, Universe: k.uni}
			if pd := ask(k, uid, pidDeviceInfo); len(pd) >= 19 {
				f.Footprint = int(binary.BigEndian.Uint16(pd[10:]))
				if pd[13] > 0 {
					f.Personality = fmt.Sprintf("%d/%d", pd[12], pd[13])
				}
				if a := int(binary.BigEndian.Uint16(pd[14:])); a >= 1 && a <= 512 {
					f.Address = a
				}
			}
			f.Label = rdmText(ask(k, uid, pidDeviceLabel))
			f.Maker = rdmText(ask(k, uid, pidMakerLabel))
			f.Model = rdmText(ask(k, uid, pidModelDescr))
			f.Software = rdmText(ask(k, uid, pidSoftwareText))
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Universe != out[j].Universe {
			return out[i].Universe < out[j].Universe
		}
		return out[i].Address < out[j].Address
	})
	return out
}

func rdmText(b []byte) string {
	return strings.TrimSpace(strings.ToValidUTF8(strings.TrimRight(string(b), "\x00"), ""))
}

func containsUID(a [][6]byte, x [6]byte) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}

// RDMIdentify makes a fixture flash (on) or stop (off): RDM SET
// IDENTIFY_DEVICE, the only thing wasaping ever writes to a fixture.
func RDMIdentify(ctx context.Context, node string, universe int, uid string, on bool) error {
	var u [6]byte
	var hi uint16
	var lo uint32
	if _, err := fmt.Sscanf(uid, "%04X:%08X", &hi, &lo); err != nil {
		return fmt.Errorf("UID non valido %q", uid)
	}
	binary.BigEndian.PutUint16(u[:], hi)
	binary.BigEndian.PutUint32(u[2:], lo)
	ip := net.ParseIP(node).To4()
	if ip == nil {
		return fmt.Errorf("nodo non valido %q", node)
	}
	c, err := listenShared("0.0.0.0:6454")
	if err != nil {
		return err
	}
	defer c.Close()
	v := byte(0)
	if on {
		v = 1
	}
	req := artRdm(universe, rdmMessage(u, 1, 0x30, 0x1000, []byte{v}))
	buf := make([]byte, 2048)
	for try := 0; try < 3 && ctx.Err() == nil; try++ {
		c.WriteToUDP(req, &net.UDPAddr{IP: ip, Port: ArtNetNodePort})
		c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		for {
			n, _, err := c.ReadFromUDP(buf)
			if err != nil {
				break
			}
			if r, ok := parseArtRdmReply(buf[:n]); ok && r.src == u && r.cc == 0x31 && r.pid == 0x1000 {
				return nil
			}
		}
	}
	return fmt.Errorf("l'apparecchio %s non ha risposto", uid)
}
