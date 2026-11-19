package scanner

import (
	"context"
	"net"
	"sync"
	"time"
)

// CheckResult says whether a device answered a quick check.
type CheckResult struct {
	IP     string `json:"ip"`
	Alive  bool   `json:"alive"`
	RTTms  int64  `json:"rtt_ms,omitempty"`
	Method string `json:"method,omitempty"`
}

// Check quickly tells which devices are answering, for the show monitor:
// TCP on the ports each one is known to have open (plus the usual ones),
// the system ping, and an Art-Net poll for nodes that only speak Art-Net.
// It takes about a second.
func Check(ctx context.Context, ips []string, knownPorts map[string][]int) []CheckResult {
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	res := make([]CheckResult, len(ips))
	idx := map[string]int{}
	for i, ip := range ips {
		res[i].IP = ip
		idx[ip] = i
	}
	var mu sync.Mutex
	mark := func(ip, method string, rtt time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		if i, ok := idx[ip]; ok && !res[i].Alive {
			res[i].Alive, res[i].Method, res[i].RTTms = true, method, rtt.Milliseconds()
		}
	}
	var wg sync.WaitGroup
	if c, err := listenShared("0.0.0.0:6454"); err == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.Close()
			start := time.Now()
			poll := append([]byte("Art-Net\x00"), 0x00, 0x20, 0, 14, 0, 0)
			for _, b := range broadcastAddrs() {
				c.WriteToUDP(poll, &net.UDPAddr{IP: b, Port: ArtNetNodePort})
			}
			actx, acancel := context.WithTimeout(ctx, time.Second)
			defer acancel()
			readUntil(actx, c, "artnet", func(b []byte, from *net.UDPAddr) {
				if parseArtPollReply(b) != nil {
					ip := from.IP.String()
					if rep := net.IP(b[10:14]).String(); rep != "0.0.0.0" {
						ip = rep
					}
					mark(ip, "artnet", time.Since(start))
				}
			})
		}()
	}
	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			ports := append(append([]int(nil), knownPorts[ip]...), DefaultProbePorts...)
			if ok, rtt := tcpProbe(ctx, ip, ports, time.Second); ok {
				mark(ip, "tcp", rtt)
				return
			}
			if ok, rtt := systemPing(ctx, ip, time.Second); ok {
				mark(ip, "ping", rtt)
			}
		}(ip)
	}
	wg.Wait()
	return res
}
