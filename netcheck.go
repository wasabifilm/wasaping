package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Network checks: DHCP servers (with every scan, at most every 5 minutes,
// and on demand) and the traffic arriving at this computer.

var dhcpCache struct {
	sync.Mutex
	at      time.Time
	network string // interface network the result belongs to
	servers []scanner.DHCPServer
	err     string
}

// dhcpServers asks the network of the default interface for DHCP offers,
// or returns the result of the last 5 minutes unless force.
func dhcpServers(ctx context.Context, force bool) (scanner.Interface, []scanner.DHCPServer, error) {
	iface, err := scanner.DefaultInterface()
	if err != nil {
		return iface, nil, err
	}
	dhcpCache.Lock()
	if !force && dhcpCache.network == iface.Network && time.Since(dhcpCache.at) < 5*time.Minute {
		s, e := dhcpCache.servers, dhcpCache.err
		dhcpCache.Unlock()
		if e != "" {
			return iface, s, fmt.Errorf("%s", e)
		}
		return iface, s, nil
	}
	dhcpCache.Unlock()
	servers, err := scanner.DHCPProbe(ctx, iface, 3*time.Second)
	if ctx.Err() != nil {
		return iface, nil, ctx.Err()
	}
	dhcpCache.Lock()
	dhcpCache.at, dhcpCache.network, dhcpCache.servers, dhcpCache.err = time.Now(), iface.Network, servers, ""
	if err != nil {
		dhcpCache.err = err.Error()
	}
	dhcpCache.Unlock()
	return iface, servers, err
}

// dhcpForScan runs the DHCP check alongside a scan of ips, when they are on
// the default interface's network; the result is read after the scan.
func dhcpForScan(ctx context.Context, ips []net.IP) func() []scanner.DHCPServer {
	iface, err := scanner.DefaultInterface()
	if err != nil || len(ips) == 0 {
		return func() []scanner.DHCPServer { return nil }
	}
	_, n, err := net.ParseCIDR(iface.Network)
	if err != nil || !n.Contains(ips[0]) {
		return func() []scanner.DHCPServer { return nil }
	}
	ch := make(chan []scanner.DHCPServer, 1)
	go func() {
		_, s, _ := dhcpServers(ctx, false)
		ch <- s
	}()
	return func() []scanner.DHCPServer {
		select {
		case s := <-ch:
			return s
		case <-time.After(4 * time.Second):
			return nil
		}
	}
}

// serverName describes a DHCP server with what the scan knows about it.
func serverName(ip string, hosts []HostView) string {
	for _, h := range hosts {
		if h.IP == ip {
			if n := firstNonEmpty(h.Name, h.Friendly, h.Hostname, h.Model, h.Vendor); n != "" {
				return fmt.Sprintf("%s (%s)", ip, n)
			}
		}
	}
	return ip
}

// dhcpIssues turns the servers heard into network problems (in Italian,
// translated like the others).
func dhcpIssues(servers []scanner.DHCPServer, hosts []HostView, gateway string) []string {
	var out []string
	if len(servers) > 1 {
		var names []string
		for _, s := range servers {
			names = append(names, serverName(s.IP, hosts))
		}
		out = append(out, fmt.Sprintf("Più server DHCP: %s. I dispositivi possono prendere indirizzi e router sbagliati: lascia acceso il DHCP su uno solo.", strings.Join(names, ", ")))
	}
	for _, s := range servers {
		if s.Router != "" && gateway != "" && s.Router != gateway {
			out = append(out, fmt.Sprintf("Il server DHCP %s dà come router %s, ma il router di questa rete è %s.", s.IP, s.Router, gateway))
		}
	}
	return out
}

// handleDHCP: GET asks the network now.
func handleDHCP(w http.ResponseWriter, r *http.Request) {
	iface, servers, err := dhcpServers(r.Context(), true)
	out := map[string]any{"interface": iface.Name, "network": iface.Network, "gateway": scanner.DefaultGateway(), "servers": servers}
	if servers == nil {
		out["servers"] = []scanner.DHCPServer{}
	}
	if err != nil {
		out["error"] = err.Error()
	}
	var hosts []HostView
	lastMu.Lock()
	if lastResult != nil {
		hosts = lastResult.Hosts
	}
	lastMu.Unlock()
	names := map[string]string{}
	for _, s := range servers {
		names[s.IP] = serverName(s.IP, hosts)
	}
	out["names"] = names
	out["issues"] = dhcpIssues(servers, hosts, scanner.DefaultGateway())
	writeJSON(w, out)
}

// TrafficNote is a finding of the traffic check.
type TrafficNote struct {
	Level string `json:"level"` // ok, info, warn
	Text  string `json:"text"`
}

// trafficNotes reads a measurement in plain words.
func trafficNotes(r scanner.TrafficReport) []TrafficNote {
	var out []TrafficNote
	var dmxMbps, artMbps float64
	for _, p := range r.Protocols {
		if p.Name == "Art-Net" || p.Name == "sACN" {
			dmxMbps += p.Mbps
		}
		if p.Name == "Art-Net" {
			artMbps = p.Mbps
		}
	}
	if n := len(r.ArtBroadcast); n > 0 {
		lvl := "info"
		if n >= 10 || artMbps > 5 {
			lvl = "warn"
		}
		what := fmt.Sprintf(T("%d universi Art-Net arrivano", "%d Art-Net universes reach"), n)
		if n == 1 {
			what = T("1 universo Art-Net arriva", "1 Art-Net universe reaches")
		}
		out = append(out, TrafficNote{lvl, what + T(" a questo computer senza che li abbia chiesti: sono in broadcast, quindi arrivano a tutti i dispositivi della rete (anche Dante, video e Wi-Fi). Meglio l'unicast verso i nodi.",
			" this computer without asking: it is broadcast, so every device on the network gets it (Dante, video and Wi-Fi too). Unicast to the nodes is better.")})
	}
	if dmxMbps > 50 {
		out = append(out, TrafficNote{"warn", fmt.Sprintf(T("Il DMX in rete occupa %.0f Mbit/s: troppo per i collegamenti a 100 Mbit (switch vecchi, molti nodi).", "DMX on the network takes %.0f Mbit/s: too much for 100 Mbit links (old switches, many nodes)."), dmxMbps)})
	}
	if r.NonUnicastPS > 1000 {
		out = append(out, TrafficNote{"warn", fmt.Sprintf(T("Arrivano %.0f pacchetti al secondo di broadcast e multicast: possibile tempesta di broadcast, o multicast senza IGMP snooping sugli switch.", "%.0f broadcast and multicast packets per second arrive: maybe a broadcast storm, or multicast without IGMP snooping on the switches."), r.NonUnicastPS)})
	}
	if r.RxPPS > 20000 {
		out = append(out, TrafficNote{"warn", fmt.Sprintf(T("Arrivano %.0f pacchetti al secondo a questo computer: tanti per una rete show. Controlla loop di rete e flussi multicast (Dante, NDI, sACN).", "%.0f packets per second reach this computer: a lot for a show network. Check for network loops and multicast streams (Dante, NDI, sACN)."), r.RxPPS)})
	}
	if len(out) == 0 {
		out = append(out, TrafficNote{"ok", T("Traffico tranquillo: niente broadcast DMX e nessun flusso esagerato.", "Calm traffic: no broadcast DMX and no excessive stream.")})
	}
	return out
}

// handleTraffic: GET measures for ?s= seconds (3–20, default 6).
func handleTraffic(w http.ResponseWriter, r *http.Request) {
	sec := 6
	fmt.Sscan(r.URL.Query().Get("s"), &sec)
	sec = max(3, min(sec, 20))
	iface, err := scanner.DefaultInterface()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	rep := scanner.MeasureTraffic(r.Context(), iface, time.Duration(sec)*time.Second)
	writeJSON(w, map[string]any{"report": rep, "notes": trafficNotes(rep)})
}

// runDHCPCLI: wasaping dhcp.
func runDHCPCLI(ctx context.Context) error {
	iface, servers, err := dhcpServers(ctx, true)
	if err != nil {
		return fmt.Errorf("%s", msgCLI(err.Error()))
	}
	fmt.Printf(TC("Server DHCP su %s (%s):\n", "DHCP servers on %s (%s):\n"), iface.Name, iface.Network)
	if len(servers) == 0 {
		fmt.Println(TC("  nessuno ha risposto (normale con indirizzi fissi)", "  none answered (normal with fixed addresses)"))
	}
	for _, s := range servers {
		fmt.Printf("  %-15s %s %-15s router %-15s dns %s\n", s.IP, TC("offre", "offers"), s.Offered, s.Router, strings.Join(s.DNS, ","))
	}
	for _, is := range dhcpIssues(servers, nil, scanner.DefaultGateway()) {
		fmt.Println("⚠ " + msgCLI(is))
	}
	return nil
}

// runTrafficCLI: wasaping traffic [-t 6s].
func runTrafficCLI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("traffic", flag.ContinueOnError)
	d := fs.Duration("t", 6*time.Second, TC("quanto ascoltare", "how long to listen"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	iface, err := scanner.DefaultInterface()
	if err != nil {
		return err
	}
	r := scanner.MeasureTraffic(ctx, iface, *d)
	num := func(v float64, f string) string {
		if v < 0 {
			return "—"
		}
		return fmt.Sprintf(f, v)
	}
	fmt.Printf(TC("%s, %.0f s: %s pacchetti/s, %s Mbit/s, broadcast e multicast %s/s\n", "%s, %.0f s: %s packets/s, %s Mbit/s, broadcast and multicast %s/s\n"),
		r.Interface, r.Seconds, num(r.RxPPS, "%.0f"), num(r.RxMbps, "%.1f"), num(r.NonUnicastPS, "%.0f"))
	for _, p := range r.Protocols {
		fmt.Printf("  %-17s %3d %s  %6.0f pkt/s  %6.2f Mbit/s  %s\n", p.Name, p.Universes, TC("universi", "universes"), p.PPS, p.Mbps, strings.Join(p.Sources, ", "))
	}
	for _, n := range trafficNotes(r) {
		mark := map[string]string{"ok": "✓", "info": "ℹ", "warn": "⚠"}[n.Level]
		fmt.Println(mark + " " + n.Text)
	}
	return nil
}
