package scanner

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func dhcpOfferPacket(xid [4]byte, server, offered, router net.IP, typ byte) []byte {
	b := make([]byte, 240)
	b[0], b[1], b[2] = 2, 1, 6
	copy(b[4:8], xid[:])
	copy(b[16:20], offered.To4())
	copy(b[236:240], []byte{99, 130, 83, 99})
	b = append(b, 53, 1, typ)
	b = append(b, 54, 4)
	b = append(b, server.To4()...)
	b = append(b, 1, 4, 255, 255, 255, 0)
	b = append(b, 3, 4)
	b = append(b, router.To4()...)
	b = append(b, 6, 8, 1, 1, 1, 1, 8, 8, 8, 8)
	b = append(b, 15, 4, 's', 'h', 'o', 'w')
	lease := make([]byte, 4)
	binary.BigEndian.PutUint32(lease, 3600)
	b = append(b, 51, 4)
	b = append(b, lease...)
	return append(b, 255)
}

func TestDHCPPackets(t *testing.T) {
	xid := [4]byte{1, 2, 3, 4}
	d := dhcpDiscover(net.HardwareAddr{2, 0, 0, 0, 0, 1}, xid)
	if len(d) != 300 || d[0] != 1 || binary.BigEndian.Uint16(d[10:]) != 0x8000 || string(d[236:240]) != "\x63\x82\x53\x63" || d[240] != 53 || d[242] != 1 {
		t.Fatalf("discover: % x", d[:16])
	}
	p := dhcpOfferPacket(xid, net.IPv4(10, 0, 0, 1), net.IPv4(10, 0, 0, 50), net.IPv4(10, 0, 0, 254), 2)
	s, mine, ok := parseDHCPOffer(p, xid)
	if !ok || !mine || s.IP != "10.0.0.1" || s.Offered != "10.0.0.50" || s.Router != "10.0.0.254" || s.Mask != "255.255.255.0" ||
		len(s.DNS) != 2 || s.DNS[1] != "8.8.8.8" || s.Domain != "show" || s.Lease != 3600 {
		t.Fatalf("offer: %+v mine %v ok %v", s, mine, ok)
	}
	if _, mine, ok := parseDHCPOffer(p, [4]byte{9, 9, 9, 9}); !ok || mine {
		t.Error("offer to another client should be read as not ours")
	}
	if _, _, ok := parseDHCPOffer(dhcpOfferPacket(xid, net.IPv4(10, 0, 0, 1), net.IPv4(10, 0, 0, 50), net.IPv4(10, 0, 0, 254), 6), xid); ok {
		t.Error("DHCPNAK read as an offer")
	}
	if _, _, ok := parseDHCPOffer(p[:200], xid); ok {
		t.Error("short packet accepted")
	}
}

func TestIPConflicts(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	noteARPAt(t0, map[string]string{"10.9.9.5": "aa:aa:aa:aa:aa:01", "10.9.9.6": "aa:aa:aa:aa:aa:02"})
	noteARPAt(t0.Add(2*time.Second), map[string]string{"10.9.9.5": "aa:aa:aa:aa:aa:03", "10.9.9.6": "aa:aa:aa:aa:aa:02"})
	if got := conflictMACsAt(t0.Add(3*time.Second), "10.9.9.5", "aa:aa:aa:aa:aa:03"); len(got) != 1 || got[0] != "aa:aa:aa:aa:aa:01" {
		t.Fatalf("conflict not seen: %v", got)
	}
	if got := conflictMACsAt(t0.Add(3*time.Second), "10.9.9.6", "aa:aa:aa:aa:aa:02"); len(got) != 0 {
		t.Fatalf("false conflict: %v", got)
	}
	// After the window the old MAC is forgotten (e.g. a replaced device).
	noteARPAt(t0.Add(conflictWindow+5*time.Second), map[string]string{"10.9.9.5": "aa:aa:aa:aa:aa:03"})
	if got := conflictMACsAt(t0.Add(conflictWindow+6*time.Second), "10.9.9.5", "aa:aa:aa:aa:aa:03"); len(got) != 0 {
		t.Fatalf("old MAC still reported: %v", got)
	}
}

func TestTrafficCounters(t *testing.T) {
	mac := `Name       Mtu   Network       Address            Ipkts Ierrs     Ibytes    Opkts Oerrs     Obytes  Coll
en0        1500  <Link#15>   7a:ae:be:f8:91:8d 52808993     0 69606737807 22889652     0 14284456862     0
en0        1500  fe80::1cec: fe80:f::1cec:2780 52808993     - 69606737807 22889652     - 14284456862     -
`
	if c := parseNetstatIbn(mac); c.RxPkts != 52808993 || c.RxBytes != 69606737807 {
		t.Fatalf("netstat -ibn: %+v", c)
	}
	win := `Statistiche interfaccia

                           Ricevuti          Inviati

Byte                    1234567890       987654321
Pacchetti unicast           5000000          4000000
Pacchetti non unicast        250000            12000
Scartati                          0                0
Errori                            0                0
Protocolli sconosciuti            0
`
	if c := parseNetstatE(win); c.RxBytes != 1234567890 || c.RxPkts != 5250000 || c.RxNonUnicast != 250000 {
		t.Fatalf("netstat -e: %+v", c)
	}
}

func TestScanPortsProgress(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := l.Addr().(*net.TCPAddr).Port
	ports := []int{port}
	for p := 40000; p < 40300; p++ {
		if p != port {
			ports = append(ports, p)
		}
	}
	calls := 0
	open := ScanPortsProgress(context.Background(), "127.0.0.1", ports, 500*time.Millisecond, 64, func(done int, open []int) { calls++ })
	if len(open) != 1 || open[0] != port {
		t.Fatalf("open %v, want [%d]", open, port)
	}
	if n := len(WellKnownPorts); n < 1100 || n > 1300 || WellKnownPorts[0] != 1 {
		t.Fatalf("well-known ports: %d", n)
	}
}
