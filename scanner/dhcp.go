package scanner

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

// DHCP servers on the network: a DHCPDISCOVER in broadcast, then every
// DHCPOFFER that comes back. No DHCPREQUEST follows, so no address is ever
// taken: the offers simply expire. Two servers answering is the classic
// "someone plugged in a home router" problem on a show network.

// DHCPServer is one server that answered (or was heard answering others).
type DHCPServer struct {
	IP      string   `json:"ip"`                // server identifier (option 54)
	From    string   `json:"from,omitempty"`    // source address of the packet, when different
	Offered string   `json:"offered,omitempty"` // address it would give us
	Mask    string   `json:"mask,omitempty"`
	Router  string   `json:"router,omitempty"`
	DNS     []string `json:"dns,omitempty"`
	Domain  string   `json:"domain,omitempty"`
	Lease   int      `json:"lease,omitempty"`   // seconds
	Passive bool     `json:"passive,omitempty"` // heard answering another client, not us
}

// probeMAC stands in for our MAC when the system does not tell it: the same
// one for the whole run, so repeated probes never ask for new addresses.
var probeMAC = func() net.HardwareAddr {
	m := make(net.HardwareAddr, 6)
	rand.Read(m)
	m[0] = m[0]&0xFC | 0x02 // locally administered
	return m
}()

// DHCPPort is where servers listen (67); clients get the answers on 68.
var DHCPPort = 67

// DHCPProbe asks for offers on the network of iface (its MAC goes in the
// request, so a server offers our own address again if it knows us).
func DHCPProbe(ctx context.Context, iface Interface, wait time.Duration) ([]DHCPServer, error) {
	c, err := listenShared(fmt.Sprintf("0.0.0.0:%d", DHCPPort+1))
	if err != nil {
		if runtime.GOOS == "linux" && errors.Is(err, os.ErrPermission) {
			return nil, errors.New("per sentire i server DHCP serve la porta 68: avvia wasaping come root, oppure dagli il permesso una volta con «sudo setcap cap_net_bind_service=+ep $(which wasaping)»")
		}
		return nil, fmt.Errorf("porta 68 non disponibile: %v", err)
	}
	defer c.Close()
	mac, _ := net.ParseMAC(iface.MAC)
	if len(mac) != 6 {
		mac = probeMAC // the system hides our MAC: always the same stand-in
	}
	var xid [4]byte
	rand.Read(xid[:])
	pkt := dhcpDiscover(mac, xid)
	dsts := []net.IP{net.IPv4bcast}
	if _, n, err := net.ParseCIDR(iface.Network); err == nil {
		b := make(net.IP, 4)
		for i := range b {
			b[i] = n.IP.To4()[i] | ^n.Mask[i]
		}
		dsts = append(dsts, b)
	}
	sent := false
	for _, d := range dsts {
		if _, err := c.WriteToUDP(pkt, &net.UDPAddr{IP: d, Port: DHCPPort}); err == nil {
			sent = true
		}
	}
	if !sent {
		return nil, errors.New("non riesco a mandare la richiesta DHCP")
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	found := map[string]*DHCPServer{}
	readUntil(ctx, c, "dhcp", func(b []byte, from *net.UDPAddr) {
		s, mine, ok := parseDHCPOffer(b, xid)
		if !ok {
			return
		}
		if s.IP == "" {
			s.IP = from.IP.String()
		}
		if from.IP.String() != s.IP && !from.IP.Equal(net.IPv4zero) {
			s.From = from.IP.String()
		}
		s.Passive = !mine
		if old, ok := found[s.IP]; ok && (!old.Passive || s.Passive) {
			return // keep our own offer over one heard for someone else
		}
		found[s.IP] = s
	})
	out := make([]DHCPServer, 0, len(found))
	for _, s := range found {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })
	return out, nil
}

// dhcpDiscover builds a DHCPDISCOVER with the broadcast flag set (we have no
// address to receive a unicast offer on).
func dhcpDiscover(mac net.HardwareAddr, xid [4]byte) []byte {
	b := make([]byte, 240, 300)
	b[0], b[1], b[2] = 1, 1, 6 // BOOTREQUEST, Ethernet, address length
	copy(b[4:8], xid[:])
	binary.BigEndian.PutUint16(b[10:], 0x8000) // broadcast flag
	copy(b[28:34], mac)
	copy(b[236:240], []byte{99, 130, 83, 99}) // magic cookie
	b = append(b, 53, 1, 1)                   // DHCPDISCOVER
	b = append(b, 55, 6, 1, 3, 6, 15, 51, 54) // ask for mask, router, DNS, domain, lease, server id
	b = append(b, 255)
	for len(b) < 300 { // some servers ignore requests shorter than BOOTP's 300 bytes
		b = append(b, 0)
	}
	return b
}

// parseDHCPOffer reads a DHCPOFFER or DHCPACK; mine says whether it answers
// our request (same transaction id).
func parseDHCPOffer(b []byte, xid [4]byte) (*DHCPServer, bool, bool) {
	if len(b) < 240 || b[0] != 2 || string(b[236:240]) != "\x63\x82\x53\x63" {
		return nil, false, false
	}
	s := &DHCPServer{}
	if y := net.IP(b[16:20]); !y.Equal(net.IPv4zero) {
		s.Offered = y.String()
	}
	typ := 0
	for i := 240; i < len(b); {
		code := b[i]
		if code == 255 {
			break
		}
		if code == 0 {
			i++
			continue
		}
		if i+1 >= len(b) {
			break
		}
		n := int(b[i+1])
		if i+2+n > len(b) {
			break
		}
		v := b[i+2 : i+2+n]
		switch code {
		case 53:
			if n == 1 {
				typ = int(v[0])
			}
		case 54:
			if n == 4 {
				s.IP = net.IP(v).String()
			}
		case 1:
			if n == 4 {
				s.Mask = net.IP(v).String()
			}
		case 3:
			if n >= 4 {
				s.Router = net.IP(v[:4]).String()
			}
		case 6:
			for k := 0; k+4 <= n; k += 4 {
				s.DNS = append(s.DNS, net.IP(v[k:k+4]).String())
			}
		case 15:
			s.Domain = strings.TrimRight(string(v), "\x00")
		case 51:
			if n == 4 {
				s.Lease = int(binary.BigEndian.Uint32(v))
			}
		}
		i += 2 + n
	}
	if typ != 2 && typ != 5 { // OFFER or ACK
		return nil, false, false
	}
	if s.IP == "" {
		if si := net.IP(b[20:24]); !si.Equal(net.IPv4zero) {
			s.IP = si.String()
		}
	}
	return s, string(b[4:8]) == string(xid[:]), true
}
