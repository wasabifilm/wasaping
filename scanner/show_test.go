package scanner

import (
	"bufio"
	"context"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

func TestArtPollReply(t *testing.T) {
	b := make([]byte, 239)
	copy(b, "Art-Net\x00")
	binary.LittleEndian.PutUint16(b[8:], 0x2100)
	copy(b[10:], []byte{2, 0, 0, 10})
	copy(b[26:], "Node-1")
	copy(b[44:], "Luminode 4 Palco SX")
	b[18], b[19] = 0, 1 // net 0, sub 1
	b[173] = 2
	b[174], b[175] = 0x80, 0x80 // two DMX outputs
	b[190], b[191] = 0, 1
	b[200] = 0
	copy(b[201:], []byte{0x00, 0x50, 0xc2, 0x11, 0x22, 0x33})
	n := parseArtPollReply(b)
	if n == nil || n.LongName != "Luminode 4 Palco SX" || n.Style != "nodo" || n.MAC != "00:50:c2:11:22:33" {
		t.Fatalf("unexpected %+v", n)
	}
	if len(n.Out) != 2 || n.Out[0] != 16 || n.Out[1] != 17 {
		t.Errorf("universes %v", n.Out)
	}
}

func TestSACN(t *testing.T) {
	b := make([]byte, 126)
	copy(b[4:], "ASC-E1.17\x00\x00\x00")
	binary.BigEndian.PutUint32(b[18:], 4)
	copy(b[44:], "grandMA3 FOH")
	b[108] = 100
	binary.BigEndian.PutUint16(b[113:], 7)
	name, unis, prio, ok := parseSACN(b)
	if !ok || name != "grandMA3 FOH" || len(unis) != 1 || unis[0] != 7 || prio != 100 {
		t.Errorf("got %q %v %d %v", name, unis, prio, ok)
	}
}

func TestPTP(t *testing.T) {
	v2 := make([]byte, 64)
	v2[0], v2[1], v2[4], v2[47] = 0x0B, 0x02, 0, 128
	if k, p := parsePTP(v2); k != "PTPv2 dominio 0" || p != 128 {
		t.Errorf("v2: %q %d", k, p)
	}
	v1 := make([]byte, 124)
	binary.BigEndian.PutUint16(v1, 1)
	if k, _ := parsePTP(v1); k != "PTPv1" {
		t.Errorf("v1: %q", k)
	}
}

func TestPJLink(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		c.Write([]byte("PJLINK 0\r"))
		r := bufio.NewReader(c)
		ans := map[string]string{"NAME": "Proiettore Sala", "INF1": "EPSON", "INF2": "EB-L1505U", "POWR": "1", "LAMP": "1234 1", "ERST": "002000"}
		for {
			line, err := r.ReadString('\r')
			if err != nil {
				return
			}
			cmd := strings.TrimPrefix(strings.Fields(line)[0], "%1")
			c.Write([]byte("%1" + cmd + "=" + ans[cmd] + "\r"))
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	if port == "4352" {
		t.Skip("port clash")
	}
	// queryPJLink dials 4352: test the dialogue through a small proxy-free hook.
	p := queryPJLinkAddr(context.Background(), ln.Addr().String(), time.Second)
	if p == nil || p.Name != "Proiettore Sala" || p.Model != "EB-L1505U" || p.Power != "acceso" || len(p.LampHours) != 1 || p.LampHours[0] != 1234 {
		t.Fatalf("unexpected %+v", p)
	}
	if p.Errors != "temperatura (errore)" {
		t.Errorf("errors %q", p.Errors)
	}
}

func TestDNSSDInstance(t *testing.T) {
	d := &devInfo{fprio: 99, mprio: 99}
	d.addInstance("ndi", "PC-GRAFICA (Resolume Arena)")
	d.addInstance("netaudio-arc", "CL5-FOH")
	if d.friendly != "CL5-FOH" || len(d.ndiSources) != 1 {
		t.Errorf("got %+v", d)
	}
	txt := parseTXT([]byte("\x0dmodel=Mac14,2\x05fn=TV"))
	if txt["model"] != "Mac14,2" || txt["fn"] != "TV" {
		t.Errorf("txt %v", txt)
	}
}

func TestUPnPFilters(t *testing.T) {
	if !upnpSoftware(&upnpInfo{maker: "Justin Maggard", model: "Windows Media Connect compatible (MiniDLNA)"}) {
		t.Error("MiniDLNA should be ignored")
	}
	for _, b := range []string{"404 - Not Found", "Opening...", "Login"} {
		if !boringTitle(b) {
			t.Errorf("%q should be boring", b)
		}
	}
	for _, g := range []string{"Glasshouse", "Archer A6", "grandMA3 Web Remote"} {
		if boringTitle(g) {
			t.Errorf("%q is a real title", g)
		}
	}
}

func TestDMXParsers(t *testing.T) {
	art := make([]byte, 18+512)
	copy(art, "Art-Net\x00")
	binary.LittleEndian.PutUint16(art[8:], 0x5000)
	art[14], art[15] = 0x12, 0x00 // sub-net 1, universe 2 -> port-address 18
	binary.BigEndian.PutUint16(art[16:], 512)
	art[18], art[19] = 255, 128
	u, data, ok := parseArtDmx(art)
	if !ok || u != 18 || len(data) != 512 || data[0] != 255 || data[1] != 128 {
		t.Errorf("artdmx: %v %d %d", ok, u, len(data))
	}
	s := make([]byte, 638)
	copy(s[4:], "ASC-E1.17\x00\x00\x00")
	binary.BigEndian.PutUint32(s[18:], 4)
	copy(s[44:], "FOH")
	s[108] = 100
	binary.BigEndian.PutUint16(s[113:], 3)
	binary.BigEndian.PutUint16(s[123:], 513)
	s[126], s[637] = 10, 20
	u, name, prio, data, ok := parseSACNData(s)
	if !ok || u != 3 || name != "FOH" || prio != 100 || len(data) != 512 || data[0] != 10 || data[511] != 20 {
		t.Errorf("sacn: %v %d %q %d %d", ok, u, name, prio, len(data))
	}
}

func TestDanteChannels(t *testing.T) {
	d := &devInfo{fprio: 99, mprio: 99}
	d.addInstance("netaudio-chan", "Mic 1@CL5-FOH")
	d.addInstance("netaudio-chan", "Mic 2@CL5-FOH")
	if d.friendly != "CL5-FOH" || len(d.danteChans) != 2 {
		t.Errorf("got %+v", d)
	}
}

func TestRDMPackets(t *testing.T) {
	uid := [6]byte{0x45, 0x4E, 0, 0, 0, 7}
	req := rdmGet(uid, 3, pidDeviceLabel)
	// sub start, length, dest, src, tn, port, count, subdev(2), cc, pid(2), pdl, checksum(2)
	if req[0] != 0x01 || req[1] != 24 || req[19] != 0x20 || binary.BigEndian.Uint16(req[20:]) != pidDeviceLabel || len(req) != 25 {
		t.Fatalf("request % x", req)
	}
	var sum uint16 = 0xCC
	for _, c := range req[:23] {
		sum += uint16(c)
	}
	if binary.BigEndian.Uint16(req[23:]) != sum {
		t.Error("bad checksum")
	}
	// A node's reply: ACK to GET DEVICE_LABEL "Spot 1".
	label := []byte("Spot 1")
	m := []byte{0x01, byte(24 + len(label))}
	m = append(m, rdmSrc[:]...)
	m = append(m, uid[:]...)
	m = append(m, 3, 0x00, 0, 0, 0, 0x21, 0x00, 0x82, byte(len(label)))
	m = append(m, label...)
	m = append(m, 0, 0)
	pkt := append(artRdm(5, nil), m...)
	r, ok := parseArtRdmReply(pkt)
	if !ok || r.src != uid || r.pid != pidDeviceLabel || string(r.pd) != "Spot 1" {
		t.Fatalf("reply %+v %v", r, ok)
	}
	tod := artHeader(0x8100, 28+12)
	tod[21], tod[23], tod[27] = 0, 0x12, 2
	copy(tod[28:], uid[:])
	u, uids, ok := parseArtTodData(tod)
	if !ok || u != 0x12 || len(uids) != 2 || uids[0] != uid {
		t.Errorf("tod %d %v %v", u, uids, ok)
	}
}
