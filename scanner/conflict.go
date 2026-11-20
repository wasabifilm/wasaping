package scanner

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Duplicate IPs: the neighbour cache keeps one MAC per IP, the one that
// answered last. Two devices with the same address take turns in it, so
// wasaping reads it every second during a scan and remembers what it saw
// for a few minutes (continuous and background scans see the flapping).

const conflictWindow = 10 * time.Minute

var ipHist = struct {
	sync.Mutex
	m map[string]map[string]time.Time // ip -> mac -> last seen
}{m: map[string]map[string]time.Time{}}

// NoteARP records a reading of the neighbour cache.
func NoteARP(table map[string]string) { noteARPAt(time.Now(), table) }

func noteARPAt(now time.Time, table map[string]string) {
	ipHist.Lock()
	defer ipHist.Unlock()
	for ip, mac := range table {
		if mac == "" {
			continue
		}
		if ipHist.m[ip] == nil {
			ipHist.m[ip] = map[string]time.Time{}
		}
		ipHist.m[ip][mac] = now
	}
	for ip, macs := range ipHist.m {
		for mac, t := range macs {
			if now.Sub(t) > conflictWindow {
				delete(macs, mac)
			}
		}
		if len(macs) == 0 {
			delete(ipHist.m, ip)
		}
	}
}

// ConflictMACs returns the other MACs seen recently at ip besides mac.
func ConflictMACs(ip, mac string) []string { return conflictMACsAt(time.Now(), ip, mac) }

func conflictMACsAt(now time.Time, ip, mac string) []string {
	ipHist.Lock()
	defer ipHist.Unlock()
	var out []string
	for m, t := range ipHist.m[ip] {
		if m != mac && now.Sub(t) <= conflictWindow {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

// watchARP reads the neighbour cache every second until ctx ends.
func watchARP(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c, cancel := context.WithTimeout(ctx, 2*time.Second)
			NoteARP(ARPSnapshot(c))
			cancel()
		}
	}
}
