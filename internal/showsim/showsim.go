// Package showsim simulates show gear on the network, to test wasaping
// without hardware: an Art-Net node with RDM fixtures, a console sending
// sACN and Art-Net DMX, and a PTP clock master (Dante style, PTPv1).
package showsim

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"syscall"
	"time"
)

// Config says what to simulate. Zero values give a sensible default.
type Config struct {
	NodeIP    net.IP // address the node announces (default 2.0.0.10, "wrong subnet")
	NodePort  int    // where the node listens for Art-Net (default 6454)
	ReplyPort int    // where replies go (default 6454)
	DMXTo     net.IP // where ArtDmx goes (default broadcast)
	NoPTP     bool
	Universes []int // sACN universes sent (default 1, 2)
	// Identify, when set, is called on RDM SET IDENTIFY_DEVICE.
	Identify func(uid [6]byte, on bool)
}

// Fixture is an RDM device behind the simulated node.
type Fixture struct {
	UID             [6]byte
	Label, Maker    string
	Model           string
	Address         int
	Footprint       int
	Pers, PersCount byte
}

// Fixtures are the simulated RDM devices: two of them overlap on purpose.
var Fixtures = []Fixture{
	{[6]byte{0x50, 0x52, 0, 0, 0, 1}, "Spot SX 1", "Robe", "MegaPointe", 1, 39, 2, 3},
	{[6]byte{0x50, 0x52, 0, 0, 0, 2}, "Spot SX 2", "Robe", "MegaPointe", 40, 39, 2, 3},
	{[6]byte{0x41, 0x4C, 0, 0, 0, 9}, "Wash 1", "Ayrton", "Perseo", 60, 41, 1, 4}, // overlaps Spot SX 2
}

// NodeMAC is the MAC address the simulated node announces.
var NodeMAC = net.HardwareAddr{0x00, 0x50, 0xc2, 0x11, 0x22, 0x33}

func shared(addr string) (*net.UDPConn, error) {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) { setShared(fd) })
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", addr)
	if err != nil {
		return nil, err
	}
	return pc.(*net.UDPConn), nil
}

func artHeader(op uint16, size int) []byte {
	b := make([]byte, size)
	copy(b, "Art-Net\x00")
	binary.LittleEndian.PutUint16(b[8:], op)
	b[10], b[11] = 0, 14
	return b
}

func (c Config) pollReply() []byte {
	b := artHeader(0x2100, 239)
	copy(b[10:], c.NodeIP.To4())
	binary.LittleEndian.PutUint16(b[14:], 6454)
	copy(b[26:], "Luminode SX")
	copy(b[44:], "Luminode 4 Palco SX")
	copy(b[108:], "#0001 [0042] Power On Tests successful")
	b[173] = 4
	for i := 0; i < 4; i++ {
		b[174+i] = 0x80
		b[190+i] = byte(i)
	}
	copy(b[201:], NodeMAC)
	return b
}

func todData() []byte {
	b := artHeader(0x8100, 28+6*len(Fixtures))
	b[25], b[27] = byte(len(Fixtures)), byte(len(Fixtures))
	for i, f := range Fixtures {
		copy(b[28+i*6:], f.UID[:])
	}
	return b
}

// rdmReply answers an ArtRdm GET (or SET IDENTIFY) for one fixture.
func (c Config) rdmReply(req []byte) []byte {
	m := req[24:]
	if len(m) < 23 {
		return nil
	}
	var dst [6]byte
	copy(dst[:], m[2:8])
	cc, pid := m[19], binary.BigEndian.Uint16(m[20:])
	for _, f := range Fixtures {
		if f.UID != dst {
			continue
		}
		var pd []byte
		respCC := byte(0x21)
		switch {
		case cc == 0x30 && pid == 0x1000: // SET IDENTIFY_DEVICE
			if c.Identify != nil && len(m) > 23 {
				c.Identify(f.UID, m[23] == 1)
			}
			respCC = 0x31
		case pid == 0x0060:
			pd = make([]byte, 19)
			binary.BigEndian.PutUint16(pd[10:], uint16(f.Footprint))
			pd[12], pd[13] = f.Pers, f.PersCount
			binary.BigEndian.PutUint16(pd[14:], uint16(f.Address))
		case pid == 0x0082:
			pd = []byte(f.Label)
		case pid == 0x0081:
			pd = []byte(f.Maker)
		case pid == 0x0080:
			pd = []byte(f.Model)
		case pid == 0x00C0:
			pd = []byte("v1.2.3")
		}
		r := []byte{0x01, byte(24 + len(pd))}
		r = append(r, m[8:14]...)
		r = append(r, f.UID[:]...)
		r = append(r, m[14], 0x00, 0, 0, 0, respCC)
		r = binary.BigEndian.AppendUint16(r, pid)
		r = append(r, byte(len(pd)))
		r = append(r, pd...)
		r = append(r, 0, 0)
		return append(append([]byte{}, req[:24]...), r...)
	}
	return nil
}

func sacnPacket(u int) []byte {
	b := make([]byte, 638)
	binary.BigEndian.PutUint16(b[0:], 0x0010)
	copy(b[4:], "ASC-E1.17\x00\x00\x00")
	binary.BigEndian.PutUint32(b[18:], 4)
	binary.BigEndian.PutUint32(b[40:], 2)
	copy(b[44:], "grandMA3 FOH (simulatore)")
	b[108] = 100
	binary.BigEndian.PutUint16(b[113:], uint16(u))
	binary.BigEndian.PutUint16(b[123:], 513)
	for i := 0; i < 24; i++ {
		b[126+i] = 200
	}
	return b
}

func artDmx(u int, phase byte) []byte {
	b := artHeader(0x5000, 18+512)
	b[14], b[15] = byte(u&0xFF), byte(u>>8)
	binary.BigEndian.PutUint16(b[16:], 512)
	for i := 0; i < 48; i++ {
		b[18+i] = byte(i*5) + phase
	}
	return b
}

// Run simulates until ctx ends.
func Run(ctx context.Context, cfg Config) error {
	if cfg.NodeIP == nil {
		cfg.NodeIP = net.IPv4(2, 0, 0, 10)
	}
	if cfg.NodePort == 0 {
		cfg.NodePort = 6454
	}
	if cfg.ReplyPort == 0 {
		cfg.ReplyPort = 6454
	}
	if cfg.DMXTo == nil {
		cfg.DMXTo = net.IPv4bcast
	}
	if len(cfg.Universes) == 0 {
		cfg.Universes = []int{1, 2}
	}
	node, err := shared("0.0.0.0:" + itoa(cfg.NodePort))
	if err != nil {
		return err
	}
	out, err := net.ListenUDP("udp4", nil)
	if err != nil {
		node.Close()
		return err
	}
	setBroadcast(out)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // the Art-Net node
		defer wg.Done()
		buf := make([]byte, 2048)
		for {
			n, from, err := node.ReadFromUDP(buf)
			if err != nil {
				return
			}
			b := buf[:n]
			if n < 10 || string(b[:8]) != "Art-Net\x00" {
				continue
			}
			to := &net.UDPAddr{IP: from.IP, Port: cfg.ReplyPort}
			if cfg.NodePort == cfg.ReplyPort {
				// Same port as wasaping on the same computer: the system would
				// hand a direct reply to the first program on the port (us), so
				// reply in broadcast, as many real nodes do.
				to.IP = net.IPv4bcast
			}
			switch binary.LittleEndian.Uint16(b[8:]) {
			case 0x2000: // ArtPoll
				out.WriteToUDP(cfg.pollReply(), to)
			case 0x8000: // ArtTodRequest
				out.WriteToUDP(todData(), to)
			case 0x8300: // ArtRdm
				if n >= 48 && (b[24+19] == 0x20 || b[24+19] == 0x30) {
					if r := cfg.rdmReply(append([]byte{}, b...)); r != nil {
						out.WriteToUDP(r, to)
					}
				}
			}
		}
	}()
	if !cfg.NoPTP {
		wg.Add(1)
		go func() { // a PTPv1 master, as Dante uses
			defer wg.Done()
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for {
				b := make([]byte, 124)
				binary.BigEndian.PutUint16(b, 1)
				out.WriteToUDP(b, &net.UDPAddr{IP: net.IPv4(224, 0, 1, 129), Port: 319})
				select {
				case <-ctx.Done():
					return
				case <-t.C:
				}
			}
		}()
	}
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	phase := byte(0)
	for {
		phase += 7
		out.WriteToUDP(artDmx(0, phase), &net.UDPAddr{IP: cfg.DMXTo, Port: 6454})
		for _, u := range cfg.Universes {
			out.WriteToUDP(sacnPacket(u), &net.UDPAddr{IP: net.IPv4(239, 255, byte(u>>8), byte(u)), Port: 5568})
		}
		select {
		case <-ctx.Done():
			node.Close()
			out.Close()
			wg.Wait()
			return nil
		case <-t.C:
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
