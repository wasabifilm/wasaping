// Package scanner discovers devices on a local network without needing
// administrator rights: it combines TCP probes, the system ping command and
// the operating system ARP cache.
package scanner

import (
	"context"
	"net"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Host is a device found on the network.
type Host struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac,omitempty"`
	Vendor   string `json:"vendor,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Method   string `json:"method"` // tcp, ping, arp, self
	RTTms    int64  `json:"rtt_ms,omitempty"`
	Ports    []int  `json:"ports,omitempty"`
	Self     bool   `json:"self,omitempty"`
}

// Options controls a scan.
type Options struct {
	Targets    []net.IP
	Timeout    time.Duration // per probe
	Workers    int
	ProbePorts []int
	Ping       bool
	ScanPorts  []int // ports checked on alive hosts (empty = none)
	ResolveDNS bool
}

// Progress is reported while probing.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

func (o *Options) defaults() {
	if o.Timeout <= 0 {
		o.Timeout = 800 * time.Millisecond
	}
	if o.Workers <= 0 {
		o.Workers = 128
	}
	if len(o.ProbePorts) == 0 {
		o.ProbePorts = DefaultProbePorts
	}
}

// Scan probes every target and calls onHost for each device found
// (concurrently safe: calls are serialized). Returns all hosts sorted by IP.
func Scan(ctx context.Context, opt Options, onHost func(Host), onProgress func(Progress)) ([]Host, error) {
	opt.defaults()
	total := len(opt.Targets)

	inTargets := make(map[string]bool, total)
	for _, ip := range opt.Targets {
		inTargets[ip.String()] = true
	}

	// Phase 1: liveness probes.
	type hit struct {
		method string
		rtt    time.Duration
	}
	var (
		mu    sync.Mutex
		alive = map[string]hit{}
		done  int64
		wg    sync.WaitGroup
	)
	jobs := make(chan string)
	for w := 0; w < opt.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				if ok, rtt := tcpProbe(ctx, ip, opt.ProbePorts, opt.Timeout); ok {
					mu.Lock()
					alive[ip] = hit{"tcp", rtt}
					mu.Unlock()
				} else if opt.Ping && ctx.Err() == nil {
					if ok, rtt := systemPing(ctx, ip, opt.Timeout); ok {
						mu.Lock()
						alive[ip] = hit{"ping", rtt}
						mu.Unlock()
					}
				}
				n := atomic.AddInt64(&done, 1)
				if onProgress != nil {
					onProgress(Progress{int(n), total})
				}
			}
		}()
	}
	for _, ip := range opt.Targets {
		select {
		case jobs <- ip.String():
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Phase 2: the probes above filled the ARP cache, even for hosts
	// whose ports are all filtered. Any target with a MAC is alive.
	arp := ARPTable(ctx)
	for ip := range arp {
		if _, ok := alive[ip]; !ok && inTargets[ip] {
			alive[ip] = hit{method: "arp"}
		}
	}
	selfIPs := map[string]string{}
	if ifs, err := LocalInterfaces(); err == nil {
		for _, i := range ifs {
			selfIPs[i.IP] = i.MAC
			if inTargets[i.IP] {
				alive[i.IP] = hit{method: "self"}
			}
		}
	}

	// Phase 3: enrich each alive host (DNS, vendor, ports).
	var (
		results []Host
		emitMu  sync.Mutex
		sem     = make(chan struct{}, 32)
	)
	// Local names (mDNS, NetBIOS) are asked for all hosts at once, while
	// each host does its DNS lookup and port checks.
	var names map[string]string
	namesReady := make(chan struct{})
	if opt.ResolveDNS {
		ips := make([]string, 0, len(alive))
		for ip := range alive {
			if _, self := selfIPs[ip]; !self {
				ips = append(ips, ip)
			}
		}
		go func() { names = localNames(ctx, ips, 1500*time.Millisecond); close(namesReady) }()
	} else {
		close(namesReady)
	}
	selfName, _ := os.Hostname()

	var wg2 sync.WaitGroup
	for ip, h := range alive {
		wg2.Add(1)
		sem <- struct{}{}
		go func(ip string, h hit) {
			defer wg2.Done()
			defer func() { <-sem }()
			host := Host{IP: ip, Method: h.method, RTTms: h.rtt.Milliseconds()}
			if mac, ok := selfIPs[ip]; ok {
				host.Self, host.MAC = true, mac
			} else {
				host.MAC = arp[ip]
			}
			host.Vendor = Vendor(host.MAC)
			if opt.ResolveDNS && !host.Self {
				host.Hostname = reverseDNS(ctx, ip, 2*time.Second)
			}
			if len(opt.ScanPorts) > 0 {
				host.Ports = scanPorts(ctx, ip, opt.ScanPorts, opt.Timeout)
			}
			<-namesReady
			if host.Hostname == "" {
				host.Hostname = names[ip]
			}
			if host.Self && opt.ResolveDNS {
				host.Hostname = selfName
			}
			emitMu.Lock()
			results = append(results, host)
			if onHost != nil {
				onHost(host)
			}
			emitMu.Unlock()
		}(ip, h)
	}
	wg2.Wait()

	sort.Slice(results, func(i, j int) bool {
		return ipToU32(net.ParseIP(results[i].IP)) < ipToU32(net.ParseIP(results[j].IP))
	})
	return results, ctx.Err()
}

func scanPorts(ctx context.Context, ip string, ports []int, timeout time.Duration) []int {
	var (
		open []int
		mu   sync.Mutex
		wg   sync.WaitGroup
	)
	for _, p := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			if portOpen(ctx, ip, p, timeout) {
				mu.Lock()
				open = append(open, p)
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	sort.Ints(open)
	return open
}
