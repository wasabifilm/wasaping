package scanner

import (
	"context"
	"errors"
	"net"
	"time"
)

// WakeOnLAN sends the "magic packet" that wakes a sleeping or switched-off
// device whose network card supports it. It goes to the general broadcast
// and to the broadcast address of every local network, on ports 9 and 7.
func WakeOnLAN(mac string) error {
	hw, err := net.ParseMAC(normalizeMAC(mac))
	if err != nil || len(hw) != 6 {
		return errors.New("indirizzo MAC non valido")
	}
	pkt := make([]byte, 0, 102)
	for i := 0; i < 6; i++ {
		pkt = append(pkt, 0xFF)
	}
	for i := 0; i < 16; i++ {
		pkt = append(pkt, hw...)
	}
	dests := []net.IP{net.IPv4bcast}
	if ifs, err := LocalInterfaces(); err == nil {
		for _, i := range ifs {
			if _, n, err := net.ParseCIDR(i.Network); err == nil {
				b := make(net.IP, 4)
				for k := range b {
					b[k] = n.IP[k] | ^n.Mask[k]
				}
				dests = append(dests, b)
			}
		}
	}
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return err
	}
	defer c.Close()
	sent := 0
	for _, d := range dests {
		for _, port := range []int{9, 7} {
			if _, err := c.WriteToUDP(pkt, &net.UDPAddr{IP: d, Port: port}); err == nil {
				sent++
			}
		}
	}
	if sent == 0 {
		return errors.New("impossibile inviare il pacchetto sulla rete")
	}
	return nil
}

// ScanPorts checks which of ports accept TCP connections on ip.
func ScanPorts(ctx context.Context, ip string, ports []int, timeout time.Duration) []int {
	if timeout <= 0 {
		timeout = 800 * time.Millisecond
	}
	// Large lists: limit concurrency so we do not exhaust file descriptors.
	var open []int
	const chunk = 256
	for i := 0; i < len(ports) && ctx.Err() == nil; i += chunk {
		open = append(open, scanPorts(ctx, ip, ports[i:min(i+chunk, len(ports))], timeout)...)
	}
	return open
}
