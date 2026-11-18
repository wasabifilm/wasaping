package main

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Several networks at once: show computers are often on a house network,
// a lighting network (2.x or 10.x), Dante primary and secondary, video…

// localScanTargets lists the networks of this computer worth scanning:
// IPv4 with a real subnet (not VPN point-to-point links), clamped to /24.
func localScanTargets() []string {
	ifs, err := scanner.LocalInterfaces()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, i := range ifs {
		_, n, err := net.ParseCIDR(i.Network)
		if err != nil {
			continue
		}
		if ones, _ := n.Mask.Size(); ones >= 31 {
			continue
		}
		t := clampNetwork(i)
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// runCollectAll scans every target network in turn and merges the results.
func runCollectAll(ctx context.Context, targets []string, opt scanner.Options, auto bool,
	onHost func(HostView), onProgress func(scanner.Progress)) (*ScanResult, error) {
	start := time.Now()
	all := &ScanResult{Target: strings.Join(targets, ","), Auto: auto}
	var hostsAll []scanner.Host
	for i, t := range targets {
		ips, err := scanner.ParseTargets(t)
		if err != nil {
			continue
		}
		res, err := runCollect(ctx, t, ips, opt, auto, onHost, func(p scanner.Progress) {
			if onProgress != nil {
				// one bar for all networks
				onProgress(scanner.Progress{Done: i*1000 + p.Done*1000/max(p.Total, 1), Total: len(targets) * 1000})
			}
		})
		if err != nil {
			return nil, err
		}
		all.Hosts = append(all.Hosts, res.Hosts...)
		all.Gone = append(all.Gone, res.Gone...)
		all.Fresh = append(all.Fresh, res.Fresh...)
		all.Networks = append(all.Networks, res.Network)
		all.MacHidden = all.MacHidden || res.MacHidden
		for _, is := range res.Issues {
			all.Issues = append(all.Issues, fmt.Sprintf("%s: %s", firstNonEmpty(res.Network.Name, res.Network.Auto), is))
		}
		for _, h := range res.Hosts {
			hostsAll = append(hostsAll, h.Host)
		}
	}
	all.Issues = append(all.Issues, danteRedundancy(all)...)
	all.At, all.Count, all.Seconds, all.New = time.Now(), len(all.Hosts), time.Since(start).Seconds(), len(all.Fresh)
	if len(all.Networks) > 0 {
		all.Network = all.Networks[0]
	}
	lastMu.Lock()
	lastResult = all
	lastMu.Unlock()
	if onScanDone != nil {
		onScanDone(all)
	}
	return all, nil
}

// danteRedundancy: with Dante on two networks (primary and secondary),
// every Dante device should be on both.
func danteRedundancy(r *ScanResult) []string {
	byNet := map[string]map[string]bool{} // network -> Dante device names
	names := map[string]string{}
	for _, n := range r.Networks {
		names[n.ID] = firstNonEmpty(n.Name, n.Auto)
	}
	for _, h := range r.Hosts {
		dante := false
		for _, p := range h.Protocols {
			if p == "Dante" {
				dante = true
			}
		}
		if !dante || h.Friendly == "" {
			continue
		}
		if byNet[h.Net] == nil {
			byNet[h.Net] = map[string]bool{}
		}
		byNet[h.Net][h.Friendly] = true
	}
	if len(byNet) < 2 {
		return nil
	}
	every := map[string][]string{} // device -> networks it is on
	for netID, devs := range byNet {
		for d := range devs {
			every[d] = append(every[d], netID)
		}
	}
	var out []string
	for d, nets := range every {
		if len(nets) < len(byNet) {
			var missing []string
			for netID := range byNet {
				if !contains(nets, netID) {
					missing = append(missing, names[netID])
				}
			}
			sort.Strings(missing)
			out = append(out, fmt.Sprintf("Dante: %s non è sulla rete %s (ridondanza primaria/secondaria assente).", d, strings.Join(missing, ", ")))
		}
	}
	sort.Strings(out)
	return out
}

func contains(a []string, x string) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}
