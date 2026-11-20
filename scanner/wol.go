package scanner

import (
	"context"
	"errors"
	"net"
	"sort"
	"sync"
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

// WellKnownPorts are ports 1–1024 plus the usual higher ones (databases,
// remote desktops, media, cameras, show and AV gear): about 1200 ports, the
// "common ports" of a deep scan of one device.
var WellKnownPorts = func() []int {
	extra := []int{1080, 1194, 1433, 1521, 1723, 1883, 1900, 2000, 2049, 2082, 2083, 2086, 2087, 2202, 2375, 2376, 3000, 3001, 3039, 3040,
		3128, 3260, 3306, 3389, 3478, 3689, 3702, 4000, 4040, 4352, 4443, 4444, 4455, 4567, 4848, 5000, 5001, 5004, 5005, 5060, 5061,
		5222, 5353, 5357, 5432, 5555, 5601, 5672, 5900, 5901, 5938, 5960, 5961, 5985, 5986, 6000, 6379, 6443, 6454, 6600, 6667,
		7000, 7001, 7070, 7100, 7547, 7681, 8000, 8001, 8008, 8009, 8010, 8060, 8080, 8081, 8088, 8090, 8096, 8123, 8181, 8200,
		8291, 8333, 8443, 8554, 8728, 8729, 8800, 8843, 8880, 8888, 8889, 9000, 9001, 9009, 9080, 9090, 9091, 9100, 9200, 9300,
		9443, 9900, 9910, 9990, 9993, 9999, 10000, 10001, 10443, 11211, 15672, 16992, 17500, 20000, 27017, 30000, 30001, 32400,
		37777, 40000, 47443, 47474, 49152, 49153, 49154, 49280, 51325, 51326, 53000, 53001, 55443, 62078}
	out := make([]int, 0, 1024+len(extra))
	for p := 1; p <= 1024; p++ {
		out = append(out, p)
	}
	for _, p := range extra {
		if p > 1024 {
			out = append(out, p)
		}
	}
	return out
}()

// ScanPortsProgress checks ports on one device with `workers` connections
// at a time, calling progress now and then with how many were checked and
// the open ones so far.
func ScanPortsProgress(ctx context.Context, ip string, ports []int, timeout time.Duration, workers int, progress func(done int, open []int)) []int {
	var (
		mu   sync.Mutex
		open []int
		done int
		wg   sync.WaitGroup
	)
	jobs := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				ok := portOpen(ctx, ip, p, timeout)
				mu.Lock()
				done++
				if ok {
					open = append(open, p)
				}
				mu.Unlock()
			}
		}()
	}
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(300 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				mu.Lock()
				d, o := done, append([]int(nil), open...)
				mu.Unlock()
				sort.Ints(o)
				progress(d, o)
			}
		}
	}()
feed:
	for _, p := range ports {
		select {
		case jobs <- p:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	close(stop)
	sort.Ints(open)
	return open
}
