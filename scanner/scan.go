// Package scanner discovers devices on a local network without needing
// administrator rights: it combines TCP probes, the system ping command,
// the operating system ARP cache, Bonjour/NetBIOS names and the show
// protocols (Art-Net, sACN, PTP, PJLink).
package scanner

import (
	"context"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Host is a device found on the network.
type Host struct {
	IP        string      `json:"ip"`
	MAC       string      `json:"mac,omitempty"`
	Vendor    string      `json:"vendor,omitempty"`
	Hostname  string      `json:"hostname,omitempty"`
	Friendly  string      `json:"friendly,omitempty"` // name the owner gave it (Bonjour, Dante, Art-Net…)
	Model     string      `json:"model,omitempty"`
	Method    string      `json:"method"` // tcp, ping, arp, artnet, sacn, ptp, self
	RTTms     int64       `json:"rtt_ms,omitempty"`
	Ports     []int       `json:"ports,omitempty"`
	Services  []string    `json:"services,omitempty"`  // Bonjour service types
	Protocols []string    `json:"protocols,omitempty"` // Art-Net, sACN, Dante, NDI, PTP master…
	ArtNet    *ArtNetNode `json:"artnet,omitempty"`
	SACN      *SACNSource `json:"sacn,omitempty"`
	PTP       string      `json:"ptp,omitempty"` // set when this device is a PTP clock master
	PJLink    *PJLink     `json:"pjlink,omitempty"`
	UPnP      string      `json:"upnp,omitempty"`      // UPnP device type, e.g. MediaRenderer
	WebTitle  string      `json:"web_title,omitempty"` // <title> of its web page
	NDI       []string    `json:"ndi,omitempty"`       // NDI sources it publishes
	Kind      string      `json:"kind,omitempty"`      // see GuessKind
	Warnings  []string    `json:"warnings,omitempty"`
	OffSubnet bool        `json:"off_subnet,omitempty"` // not in any network of this computer
	Gateway   bool        `json:"gateway,omitempty"`
	Self      bool        `json:"self,omitempty"`
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
	NoShow     bool // skip Art-Net/sACN/PTP discovery
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
		o.Workers = 256
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
	localNets := localNetworks()

	// Show protocols are heard while the probes run.
	var show map[string]*showInfo
	showReady := make(chan struct{})
	if opt.NoShow {
		close(showReady)
	} else {
		go func() {
			show = showDiscover(ctx, interfaceFor(opt.Targets, localNets), 2*time.Second)
			close(showReady)
		}()
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
	<-showReady
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Phase 2: the probes above filled the ARP cache, even for hosts
	// whose ports are all filtered. Any target with a MAC is alive, and so
	// is anyone who spoke a show protocol.
	arp := ARPTable(ctx)
	for ip := range arp {
		if _, ok := alive[ip]; !ok && inTargets[ip] {
			alive[ip] = hit{method: "arp"}
		}
	}
	for ip, s := range show {
		if _, ok := alive[ip]; ok {
			continue
		}
		// Outside the scanned range: keep it only when it is outside every
		// local network too — the classic "fixture on the wrong subnet".
		if !inTargets[ip] && inNets(ip, localNets) {
			continue
		}
		m := "ptp"
		if s.artnet != nil {
			m = "artnet"
		} else if s.sacn != nil {
			m = "sacn"
		}
		alive[ip] = hit{method: m}
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

	// Phase 3: enrich each alive host (DNS, names, vendor, ports, show gear).
	var (
		results []Host
		emitMu  sync.Mutex
		sem     = make(chan struct{}, 32)
	)
	var info map[string]*devInfo
	var upnp map[string]*upnpInfo
	infoReady := make(chan struct{})
	upnpReady := make(chan struct{})
	if opt.ResolveDNS {
		go func() { upnp = upnpDiscover(ctx, 1200*time.Millisecond); close(upnpReady) }()
	} else {
		close(upnpReady)
	}
	if opt.ResolveDNS {
		ips := make([]string, 0, len(alive))
		for ip := range alive {
			if _, self := selfIPs[ip]; !self {
				ips = append(ips, ip)
			}
		}
		go func() { info = localNames(ctx, ips, 1200*time.Millisecond); close(infoReady) }()
	} else {
		close(infoReady)
	}
	selfName, _ := os.Hostname()
	gateway := DefaultGateway()

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
			s := show[ip]
			if host.MAC == "" && s != nil && s.artnet != nil {
				host.MAC = s.artnet.MAC
			}
			host.Vendor = Vendor(host.MAC)
			if opt.ResolveDNS && !host.Self {
				host.Hostname = reverseDNS(ctx, ip, 1500*time.Millisecond)
			}
			if len(opt.ScanPorts) > 0 {
				host.Ports = scanPorts(ctx, ip, opt.ScanPorts, opt.Timeout)
			}
			if containsInt(host.Ports, 4352) {
				host.PJLink = queryPJLink(ctx, ip, opt.Timeout+time.Second)
			}
			if opt.ResolveDNS && !host.Self {
				var web []int
				for _, p := range []int{80, 8080, 443, 8443, 8000} {
					if containsInt(host.Ports, p) {
						web = append(web, p)
					}
				}
				host.WebTitle = webTitle(ctx, ip, web)
			}
			<-infoReady
			if d := info[ip]; d != nil {
				if host.Hostname == "" {
					host.Hostname = d.name
				}
				host.Friendly, host.Model, host.Services, host.NDI = d.friendly, d.model, d.services, d.ndiSources
			}
			if host.Self && opt.ResolveDNS {
				host.Hostname = selfName
			}
			if s != nil {
				host.ArtNet, host.SACN, host.PTP = s.artnet, s.sacn, s.ptp
				if len(s.artMACs) > 1 {
					host.Warnings = append(host.Warnings, "Conflitto di IP: su Art-Net rispondono più dispositivi con questo indirizzo ("+joinStrings(s.artMACs)+").")
				}
			}
			<-upnpReady
			if u := upnp[ip]; u != nil && !upnpSoftware(u) {
				host.UPnP = u.devType
				if host.Model == "" {
					m := u.model
					if u.number != "" && !strings.Contains(m, u.number) && !(strings.Contains(u.number, ".") && strings.IndexFunc(u.number, isLetter) >= 0) {
						m = joinNonEmpty(m, u.number)
					}
					if f := strings.Fields(u.maker); len(f) > 0 && !strings.Contains(strings.ToLower(m), strings.ToLower(f[0])) {
						m = joinNonEmpty(u.maker, m)
					}
					host.Model = m
				}
				if host.Friendly == "" {
					host.Friendly = u.friendly
				}
			}
			if host.PJLink != nil && host.Model == "" {
				host.Model = joinNonEmpty(host.PJLink.Maker, host.PJLink.Model)
			}
			if host.Friendly == "" {
				host.Friendly = showName(host)
			}
			host.Gateway = ip == gateway
			host.OffSubnet = !inNets(ip, localNets)
			host.Protocols = protocolsOf(host)
			if host.Self {
				host.Kind = KindComputer
			} else {
				host.Kind = GuessKind(host)
			}
			host.Warnings = append(host.Warnings, Exposure(host)...)
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

func showName(h Host) string {
	switch {
	case h.ArtNet != nil && h.ArtNet.LongName != "":
		return h.ArtNet.LongName
	case h.ArtNet != nil:
		return h.ArtNet.ShortName
	case h.SACN != nil:
		return h.SACN.Name
	case h.PJLink != nil:
		return h.PJLink.Name
	}
	return ""
}

func localNetworks() []*net.IPNet {
	var nets []*net.IPNet
	if ifs, err := LocalInterfaces(); err == nil {
		for _, i := range ifs {
			if _, n, err := net.ParseCIDR(i.Network); err == nil {
				nets = append(nets, n)
			}
		}
	}
	return nets
}

func inNets(ip string, nets []*net.IPNet) bool {
	p := net.ParseIP(ip)
	for _, n := range nets {
		if n.Contains(p) {
			return true
		}
	}
	return false
}

// interfaceFor returns the local interface on the targets' network, used to
// join the sACN and PTP multicast groups (nil = let the system choose).
func interfaceFor(targets []net.IP, nets []*net.IPNet) *net.Interface {
	if len(targets) == 0 {
		return nil
	}
	ifs, err := LocalInterfaces()
	if err != nil {
		return nil
	}
	for _, i := range ifs {
		if _, n, err := net.ParseCIDR(i.Network); err == nil && n.Contains(targets[0]) {
			if ifi, err := net.InterfaceByName(i.Name); err == nil {
				return ifi
			}
		}
	}
	if d, err := DefaultInterface(); err == nil {
		if ifi, err := net.InterfaceByName(d.Name); err == nil {
			return ifi
		}
	}
	return nil
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

func isLetter(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' }

// upnpSoftware recognizes descriptions of a software service (a DLNA media
// server running on a router or NAS), which say nothing about the device.
func upnpSoftware(u *upnpInfo) bool {
	s := strings.ToLower(u.maker + " " + u.model)
	return strings.Contains(s, "minidlna") || strings.Contains(s, "justin maggard") || strings.Contains(s, "windows media connect")
}
