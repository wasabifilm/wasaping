package scanner

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A small SNMP v2c client (read-only) to ask managed switches which port
// each MAC address is on: BRIDGE-MIB / Q-BRIDGE-MIB forwarding tables,
// bridge port -> interface, interface names and descriptions.

type oid []uint32

func parseOID(s string) oid {
	var o oid
	for _, p := range strings.Split(strings.TrimPrefix(s, "."), ".") {
		n, _ := strconv.ParseUint(p, 10, 32)
		o = append(o, uint32(n))
	}
	return o
}

func (o oid) String() string {
	p := make([]string, len(o))
	for i, n := range o {
		p[i] = strconv.FormatUint(uint64(n), 10)
	}
	return strings.Join(p, ".")
}

func (o oid) hasPrefix(p oid) bool {
	if len(o) < len(p) {
		return false
	}
	for i := range p {
		if o[i] != p[i] {
			return false
		}
	}
	return true
}

// ---- BER ----

func berLen(n int) []byte {
	if n < 128 {
		return []byte{byte(n)}
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte(n)}, b...)
		n >>= 8
	}
	return append([]byte{0x80 | byte(len(b))}, b...)
}

func tlv(tag byte, v []byte) []byte { return append(append([]byte{tag}, berLen(len(v))...), v...) }

func berInt(n int64) []byte {
	b := []byte{byte(n)}
	for n > 127 || n < -128 {
		n >>= 8
		b = append([]byte{byte(n)}, b...)
	}
	return tlv(0x02, b)
}

func berOID(o oid) []byte {
	if len(o) < 2 {
		return tlv(0x06, nil)
	}
	b := []byte{byte(o[0]*40 + o[1])}
	for _, n := range o[2:] {
		var enc []byte
		enc = append(enc, byte(n&0x7F))
		for n >>= 7; n > 0; n >>= 7 {
			enc = append([]byte{byte(n&0x7F | 0x80)}, enc...)
		}
		b = append(b, enc...)
	}
	return tlv(0x06, b)
}

// snmpRequest builds a GetRequest (0xA0) or GetBulkRequest (0xA5).
func snmpRequest(community string, pdu byte, reqID int32, oids []oid, maxRep int) []byte {
	var vbs []byte
	for _, o := range oids {
		vbs = append(vbs, tlv(0x30, append(berOID(o), 0x05, 0x00))...)
	}
	a, b := int64(0), int64(0)
	if pdu == 0xA5 {
		a, b = 0, int64(maxRep) // non-repeaters, max-repetitions
	}
	body := append(append(append(berInt(int64(reqID)), berInt(a)...), berInt(b)...), tlv(0x30, vbs)...)
	msg := append(append(berInt(1), tlv(0x04, []byte(community))...), tlv(pdu, body)...)
	return tlv(0x30, msg)
}

type berReader struct {
	b   []byte
	off int
}

func (r *berReader) next() (tag byte, val []byte, err error) {
	if r.off+2 > len(r.b) {
		return 0, nil, errors.New("short")
	}
	tag = r.b[r.off]
	l := int(r.b[r.off+1])
	r.off += 2
	if l&0x80 != 0 {
		n := l & 0x7F
		if n == 0 || n > 4 || r.off+n > len(r.b) {
			return 0, nil, errors.New("bad length")
		}
		l = 0
		for i := 0; i < n; i++ {
			l = l<<8 | int(r.b[r.off+i])
		}
		r.off += n
	}
	if l < 0 || r.off+l > len(r.b) {
		return 0, nil, errors.New("short value")
	}
	val = r.b[r.off : r.off+l]
	r.off += l
	return tag, val, nil
}

func decodeOID(b []byte) oid {
	if len(b) == 0 {
		return nil
	}
	o := oid{uint32(b[0]) / 40, uint32(b[0]) % 40}
	var n uint32
	for _, c := range b[1:] {
		n = n<<7 | uint32(c&0x7F)
		if c&0x80 == 0 {
			o = append(o, n)
			n = 0
		}
	}
	return o
}

func decodeInt(b []byte) int64 {
	var n int64
	if len(b) > 0 && b[0]&0x80 != 0 {
		n = -1
	}
	for _, c := range b {
		n = n<<8 | int64(c)
	}
	return n
}

type varbind struct {
	oid   oid
	tag   byte
	value []byte
}

func (v varbind) str() string {
	return strings.TrimSpace(strings.ToValidUTF8(strings.TrimRight(string(v.value), "\x00"), ""))
}
func (v varbind) num() int64 { return decodeInt(v.value) }
func (v varbind) end() bool  { return v.tag == 0x80 || v.tag == 0x81 || v.tag == 0x82 }

// parseSNMPResponse returns the request id and varbinds of a GetResponse.
func parseSNMPResponse(b []byte) (int32, []varbind, error) {
	r := &berReader{b: b}
	tag, msg, err := r.next()
	if err != nil || tag != 0x30 {
		return 0, nil, errors.New("not SNMP")
	}
	m := &berReader{b: msg}
	m.next() // version
	m.next() // community
	tag, pdu, err := m.next()
	if err != nil || tag != 0xA2 {
		return 0, nil, errors.New("not a response")
	}
	p := &berReader{b: pdu}
	_, id, _ := p.next()
	_, status, _ := p.next()
	p.next() // error index
	if decodeInt(status) != 0 {
		return int32(decodeInt(id)), nil, fmt.Errorf("SNMP error %d", decodeInt(status))
	}
	_, list, err := p.next()
	if err != nil {
		return 0, nil, err
	}
	l := &berReader{b: list}
	var out []varbind
	for l.off < len(l.b) {
		_, vb, err := l.next()
		if err != nil {
			break
		}
		v := &berReader{b: vb}
		_, o, err1 := v.next()
		t, val, err2 := v.next()
		if err1 != nil || err2 != nil {
			break
		}
		out = append(out, varbind{oid: decodeOID(o), tag: t, value: val})
	}
	return int32(decodeInt(id)), out, nil
}

var snmpPort = 161 // changed by tests

type snmpClient struct {
	c         *net.UDPConn
	addr      *net.UDPAddr
	community string
	timeout   time.Duration
}

func newSNMP(ip, community string, timeout time.Duration) (*snmpClient, error) {
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, err
	}
	return &snmpClient{c: c, addr: &net.UDPAddr{IP: net.ParseIP(ip), Port: snmpPort}, community: community, timeout: timeout}, nil
}

func (s *snmpClient) close() { s.c.Close() }

func (s *snmpClient) do(ctx context.Context, pdu byte, oids []oid, maxRep int) ([]varbind, error) {
	var idb [4]byte
	rand.Read(idb[:])
	id := int32(binary.BigEndian.Uint32(idb[:]) & 0x7FFFFFFF)
	req := snmpRequest(s.community, pdu, id, oids, maxRep)
	buf := make([]byte, 65535)
	for try := 0; try < 2 && ctx.Err() == nil; try++ {
		s.c.WriteToUDP(req, s.addr)
		s.c.SetReadDeadline(time.Now().Add(s.timeout))
		for {
			n, _, err := s.c.ReadFromUDP(buf)
			if err != nil {
				break // timeout: retry
			}
			rid, vbs, err := parseSNMPResponse(buf[:n])
			if rid != id {
				continue
			}
			return vbs, err
		}
	}
	return nil, errors.New("nessuna risposta SNMP")
}

// walk reads a subtree with GetBulk (at most limit rows).
func (s *snmpClient) walk(ctx context.Context, root oid, limit int) []varbind {
	var out []varbind
	cur := root
	for len(out) < limit && ctx.Err() == nil {
		vbs, err := s.do(ctx, 0xA5, []oid{cur}, 30)
		if err != nil || len(vbs) == 0 {
			break
		}
		for _, v := range vbs {
			if v.end() || !v.oid.hasPrefix(root) {
				return out
			}
			out = append(out, v)
			cur = v.oid
		}
	}
	return out
}

var (
	oidSysDescr   = parseOID("1.3.6.1.2.1.1.1.0")
	oidSysName    = parseOID("1.3.6.1.2.1.1.5.0")
	oidQFdbPort   = parseOID("1.3.6.1.2.1.17.7.1.2.2.1.2") // .fdbId.mac -> bridge port
	oidFdbPort    = parseOID("1.3.6.1.2.1.17.4.3.1.2")     // .mac -> bridge port
	oidBasePortIf = parseOID("1.3.6.1.2.1.17.1.4.1.2")     // .bridgePort -> ifIndex
	oidIfName     = parseOID("1.3.6.1.2.1.31.1.1.1.1")
	oidIfAlias    = parseOID("1.3.6.1.2.1.31.1.1.1.18")
	oidIfDescr    = parseOID("1.3.6.1.2.1.2.2.1.2")
)

// SwitchInfo is a managed switch that answered.
type SwitchInfo struct {
	IP    string `json:"ip"`
	Name  string `json:"name,omitempty"`
	Descr string `json:"descr,omitempty"`
	MACs  int    `json:"macs"` // addresses in its forwarding table
}

// SwitchPort is where a MAC address is connected.
type SwitchPort struct {
	Switch   string `json:"switch"` // name or IP
	SwitchIP string `json:"switch_ip"`
	Port     string `json:"port"`
	Alias    string `json:"alias,omitempty"` // port description set on the switch
	Shared   int    `json:"shared"`          // MACs seen on that port (1 = only this device)
}

// SwitchPorts asks every candidate for its forwarding table and returns
// the switches found and, per MAC, the most likely access port (the one
// with the fewest MACs: uplinks between switches carry many).
func SwitchPorts(ctx context.Context, candidates []string, community string) ([]SwitchInfo, map[string]SwitchPort) {
	type loc struct {
		SwitchPort
		count int
	}
	var mu sync.Mutex
	var switches []SwitchInfo
	all := map[string][]loc{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)
	for _, ip := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			s, err := newSNMP(ip, community, 700*time.Millisecond)
			if err != nil {
				return
			}
			defer s.close()
			sys, err := s.do(ctx, 0xA0, []oid{oidSysName, oidSysDescr}, 0)
			if err != nil || len(sys) < 2 {
				return
			}
			name, descr := sys[0].str(), sys[1].str()
			// Forwarding table: Q-BRIDGE first (VLAN-aware switches), then BRIDGE.
			macPort := map[string]int64{}
			for _, v := range s.walk(ctx, oidQFdbPort, 20000) {
				if idx := v.oid[len(oidQFdbPort):]; len(idx) >= 7 {
					macPort[macFromOID(idx[len(idx)-6:])] = v.num()
				}
			}
			if len(macPort) == 0 {
				for _, v := range s.walk(ctx, oidFdbPort, 20000) {
					if idx := v.oid[len(oidFdbPort):]; len(idx) == 6 {
						macPort[macFromOID(idx)] = v.num()
					}
				}
			}
			if len(macPort) == 0 {
				return // not a switch, or no access to the tables
			}
			portIf := map[int64]int64{}
			for _, v := range s.walk(ctx, oidBasePortIf, 2000) {
				portIf[int64(v.oid[len(v.oid)-1])] = v.num()
			}
			ifName, ifAlias := map[int64]string{}, map[int64]string{}
			for _, v := range s.walk(ctx, oidIfName, 2000) {
				ifName[int64(v.oid[len(v.oid)-1])] = v.str()
			}
			if len(ifName) == 0 {
				for _, v := range s.walk(ctx, oidIfDescr, 2000) {
					ifName[int64(v.oid[len(v.oid)-1])] = v.str()
				}
			}
			for _, v := range s.walk(ctx, oidIfAlias, 2000) {
				ifAlias[int64(v.oid[len(v.oid)-1])] = v.str()
			}
			perPort := map[int64]int{}
			for _, p := range macPort {
				perPort[p]++
			}
			mu.Lock()
			defer mu.Unlock()
			switches = append(switches, SwitchInfo{IP: ip, Name: name, Descr: descr, MACs: len(macPort)})
			label := firstNonEmpty(name, ip)
			for mac, bp := range macPort {
				ifi, ok := portIf[bp]
				if !ok {
					ifi = bp
				}
				port := firstNonEmpty(ifName[ifi], "porta "+strconv.FormatInt(bp, 10))
				all[mac] = append(all[mac], loc{SwitchPort{Switch: label, SwitchIP: ip, Port: port, Alias: ifAlias[ifi], Shared: perPort[bp]}, perPort[bp]})
			}
		}(ip)
	}
	wg.Wait()
	out := map[string]SwitchPort{}
	for mac, locs := range all {
		sort.Slice(locs, func(i, j int) bool { return locs[i].count < locs[j].count })
		out[mac] = locs[0].SwitchPort
	}
	sort.Slice(switches, func(i, j int) bool { return switches[i].IP < switches[j].IP })
	return switches, out
}

func macFromOID(o oid) string {
	b := make(net.HardwareAddr, 6)
	for i := 0; i < 6 && i < len(o); i++ {
		b[i] = byte(o[i])
	}
	return b.String()
}
