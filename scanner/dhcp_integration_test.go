package scanner

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

// TestDHCPIntegration runs two fake DHCP servers on spare ports (never the
// real 67/68) and checks that the probe hears both offers.
func TestDHCPIntegration(t *testing.T) {
	if os.Getenv("WASAPING_INTEGRATION") == "" {
		t.Skip("set WASAPING_INTEGRATION=1 to run the DHCP integration test")
	}
	DHCPPort = 16767
	defer func() { DHCPPort = 67 }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := func(server, router net.IP) {
		c, err := listenShared("0.0.0.0:16767")
		if err != nil {
			t.Error(err)
			return
		}
		go func() { <-ctx.Done(); c.Close() }()
		buf := make([]byte, 1500)
		for {
			n, _, err := c.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < 240 || buf[0] != 1 {
				continue
			}
			var xid [4]byte
			copy(xid[:], buf[4:8])
			c.WriteToUDP(dhcpOfferPacket(xid, server, net.IPv4(10, 0, 0, 99), router, 2), &net.UDPAddr{IP: net.IPv4bcast, Port: 16768})
		}
	}
	go fake(net.IPv4(10, 0, 0, 1), net.IPv4(10, 0, 0, 1))
	go fake(net.IPv4(10, 0, 0, 77), net.IPv4(192, 168, 0, 1))
	time.Sleep(200 * time.Millisecond)
	iface, err := DefaultInterface()
	if err != nil {
		t.Skip("no interface:", err)
	}
	servers, err := DHCPProbe(ctx, iface, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 || servers[0].IP != "10.0.0.1" || servers[1].IP != "10.0.0.77" || servers[1].Router != "192.168.0.1" || servers[0].Passive {
		t.Fatalf("servers: %+v", servers)
	}
}
