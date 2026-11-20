package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// The show part of the monitor: critical DMX universes and the PTP clock
// master, heard by listeners that run while the monitor is on.

// MonUniverse is the live state of a critical universe.
type MonUniverse struct {
	ID      string    `json:"id"` // "sacn:1", "art:0"
	Name    string    `json:"name"`
	Known   bool      `json:"known"`
	Alive   bool      `json:"alive"`
	FPS     float64   `json:"fps"`
	Sources []string  `json:"sources,omitempty"`
	Since   time.Time `json:"since,omitempty"`
}

// MonPTP is a clock master heard recently.
type MonPTP struct {
	Kind  string    `json:"kind"`
	IP    string    `json:"ip"`
	Since time.Time `json:"since"`
}

type showMonitor struct {
	unis      map[string]*MonUniverse
	uniOrder  []string
	snap      map[string]scanner.Universe
	snapAt    time.Time
	ptpSeen   map[string]map[string]time.Time // kind -> ip -> last heard
	ptpCur    map[string]string               // kind -> current master
	ptpSince  map[string]time.Time
	ptpLost   map[string]bool
	ptpFollow map[string]time.Time // ip -> last Delay_Req: follows the clock
	clockLost map[string]bool      // device key -> warned that it stopped following
	tc        []scanner.TimecodeSource
	tcSeen    time.Time // latest timecode event already in the log
	cancel    context.CancelFunc
	listenMu  sync.Mutex
}

func uniName(id string) string {
	proto, num, _ := strings.Cut(id, ":")
	return map[string]string{"art": "Art-Net", "sacn": "sACN"}[proto] + " " + T("universo ", "universe ") + num
}

// startListeners runs the DMX and PTP listeners while the monitor is on.
func (m *monitor) startListeners(ctx context.Context) {
	m.listenMu.Lock()
	defer m.listenMu.Unlock()
	if m.cancel != nil {
		return
	}
	lctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	iface := defaultIface()
	go scanner.ListenDMXFull(lctx, iface, 64, time.Second, func() scanner.DMXWant { return scanner.DMXWant{} }, func(ds scanner.DMXSnap) {
		m.mu.Lock()
		m.snap = map[string]scanner.Universe{}
		for _, u := range ds.Universes {
			m.snap[u.ID] = u
		}
		m.tc = ds.Timecode
		m.snapAt = time.Now()
		m.mu.Unlock()
	})
	go scanner.ListenPTPAll(lctx, iface, func(kind, ip string) {
		m.mu.Lock()
		if m.ptpSeen == nil {
			m.ptpSeen = map[string]map[string]time.Time{}
		}
		if m.ptpSeen[kind] == nil {
			m.ptpSeen[kind] = map[string]time.Time{}
		}
		m.ptpSeen[kind][ip] = time.Now()
		m.mu.Unlock()
	}, func(ip string) {
		m.mu.Lock()
		if m.ptpFollow == nil {
			m.ptpFollow = map[string]time.Time{}
		}
		m.ptpFollow[ip] = time.Now()
		m.mu.Unlock()
	})
}

func (m *monitor) stopListeners() {
	m.listenMu.Lock()
	defer m.listenMu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

// checkShowLocked evaluates universes and PTP (m.mu held); it returns the
// names that dropped and came back, for notifications.
func (m *monitor) checkShowLocked(now time.Time) (lost, back []string) {
	crit := devices().CriticalUniverses()
	if m.unis == nil {
		m.unis = map[string]*MonUniverse{}
	}
	keep := map[string]bool{}
	m.uniOrder = m.uniOrder[:0]
	listening := !m.snapAt.IsZero() && now.Sub(m.snapAt) < 5*time.Second
	for _, id := range crit {
		keep[id] = true
		m.uniOrder = append(m.uniOrder, id)
		u := m.unis[id]
		if u == nil {
			u = &MonUniverse{ID: id, Name: uniName(id)}
			m.unis[id] = u
		}
		if !listening {
			continue // the listeners have not reported yet
		}
		s, ok := m.snap[id]
		alive := ok && len(s.Sources) > 0
		var names []string
		fps := 0.0
		for _, x := range s.Sources {
			names = append(names, firstNonEmpty(x.Name, x.IP))
			fps = max(fps, x.FPS)
		}
		switch {
		case !u.Known:
			u.Known, u.Alive, u.Since = true, alive, now
			if !alive {
				m.addEventRaw(MonEvent{At: now, Name: u.Name, Kind: "universe"})
				lost = append(lost, u.Name)
			}
		case u.Alive != alive:
			down := now.Sub(u.Since).Seconds()
			u.Alive, u.Since = alive, now
			m.addEventRaw(MonEvent{At: now, Name: u.Name, Kind: "universe", Up: alive, DownFor: map[bool]float64{true: down}[alive]})
			if alive {
				back = append(back, u.Name)
			} else {
				lost = append(lost, u.Name)
			}
		}
		if alive && len(names) > 1 && len(u.Sources) <= 1 {
			m.addEventRaw(MonEvent{At: now, Name: u.Name, Kind: "universe", Up: true, Text: T("più sorgenti: ", "several sources: ") + strings.Join(names, ", ")})
		}
		u.FPS, u.Sources = fps, names
	}
	for id := range m.unis {
		if !keep[id] {
			delete(m.unis, id)
		}
	}
	// PTP: the current master per kind is the one heard most recently.
	if m.ptpCur == nil {
		m.ptpCur, m.ptpSince = map[string]string{}, map[string]time.Time{}
	}
	for kind, ips := range m.ptpSeen {
		cur, latest := "", time.Time{}
		for ip, t := range ips {
			if now.Sub(t) < 6*time.Second && t.After(latest) {
				cur, latest = ip, t
			}
		}
		prev := m.ptpCur[kind]
		switch {
		case cur != "" && prev == "":
			m.ptpCur[kind], m.ptpSince[kind] = cur, now
			if m.ptpLost[kind] {
				m.ptpLost[kind] = false
				m.addEventRaw(MonEvent{At: now, Name: "PTP " + kind, IP: cur, Kind: "ptp", Up: true, Text: fmt.Sprintf(T("master del clock di nuovo presente: %s", "clock master back: %s"), cur)})
				back = append(back, "PTP "+kind)
			}
		case cur != "" && cur != prev:
			m.ptpCur[kind], m.ptpSince[kind] = cur, now
			m.addEventRaw(MonEvent{At: now, Name: "PTP " + kind, IP: cur, Kind: "ptp", Up: true, Text: fmt.Sprintf(T("nuovo master %s (prima %s)", "new master %s (was %s)"), cur, prev)})
		case cur == "" && prev != "":
			delete(m.ptpCur, kind)
			if m.ptpLost == nil {
				m.ptpLost = map[string]bool{}
			}
			m.ptpLost[kind] = true
			m.addEventRaw(MonEvent{At: now, Name: "PTP " + kind, IP: prev, Kind: "ptp", Text: T("nessun master del clock", "no clock master")})
			lost = append(lost, "PTP "+kind)
		}
	}
	m.checkTimecodeLocked()
	return lost, back
}

// MonTimecode is a timecode stream as the monitor shows it.
type MonTimecode struct {
	IP    string `json:"ip"`
	Type  string `json:"type"`
	Time  string `json:"time"`
	State string `json:"state"`
}

// tcEventText describes a timecode event for the log.
func tcEventText(e scanner.TimecodeEvent) string {
	switch e.Kind {
	case "start":
		return fmt.Sprintf(T("partito da %s", "running from %s"), e.To)
	case "stop":
		return fmt.Sprintf(T("fermo a %s", "stopped at %s"), e.To)
	case "jump":
		return fmt.Sprintf(T("saltato da %s a %s", "jumped from %s to %s"), e.From, e.To)
	case "silent":
		return fmt.Sprintf(T("non arriva più (ultimo %s)", "no longer arriving (last %s)"), e.To)
	case "type":
		return fmt.Sprintf(T("formato cambiato da %s a %s", "format changed from %s to %s"), e.From, e.To)
	}
	return ""
}

// checkTimecodeLocked writes the new timecode events in the log (m.mu
// held). Timecode stops between cues, so these never raise an alarm.
func (m *monitor) checkTimecodeLocked() {
	latest := m.tcSeen
	for _, s := range m.tc {
		for i := len(s.Events) - 1; i >= 0; i-- { // oldest first
			e := s.Events[i]
			if !e.At.After(m.tcSeen) {
				continue
			}
			if m.tcSeen.IsZero() && e.Kind != "silent" {
				continue // monitor just started: only the state, no history
			}
			m.addEventRaw(MonEvent{At: e.At, Name: "Timecode", IP: s.IP, Kind: "timecode", Up: true, Text: tcEventText(e)})
		}
		if len(s.Events) > 0 && s.Events[0].At.After(latest) {
			latest = s.Events[0].At
		}
	}
	if latest.IsZero() && len(m.tc) > 0 {
		latest = time.Now()
	}
	m.tcSeen = latest
}

func (m *monitor) timecodeStateLocked() []MonTimecode {
	var out []MonTimecode
	for _, s := range m.tc {
		out = append(out, MonTimecode{IP: s.IP, Type: s.Type, Time: s.Time, State: s.State})
	}
	return out
}

func (m *monitor) ptpStateLocked() []MonPTP {
	var out []MonPTP
	for kind, ip := range m.ptpCur {
		out = append(out, MonPTP{Kind: kind, IP: ip, Since: m.ptpSince[kind]})
	}
	return out
}

// danteSecondaries adds, for each critical Dante device, its secondary
// interface (same Dante id, another IP in the latest scan) as a device to
// watch: losing the secondary means no redundancy, before the show stops.
func danteSecondaries(crit []GoneView) []GoneView {
	lastMu.Lock()
	var hosts []scanner.Host
	if lastResult != nil {
		for _, h := range lastResult.Hosts {
			hosts = append(hosts, h.Host)
		}
	}
	lastMu.Unlock()
	byIP := map[string]scanner.Host{}
	for _, h := range hosts {
		byIP[h.IP] = h
	}
	have := map[string]bool{}
	for _, d := range crit {
		have[d.IP] = true
	}
	out := crit
	for _, d := range crit {
		h, ok := byIP[d.IP]
		if !ok || h.Dante == nil || h.Dante.ID == "" {
			continue
		}
		for _, o := range hosts {
			if o.IP != d.IP && !have[o.IP] && o.Dante != nil && o.Dante.ID == h.Dante.ID {
				sec := GoneView{Device: d.Device, Key: d.Key + "#dante2"}
				sec.IP, sec.Ports = o.IP, nil
				sec.Name = firstNonEmpty(d.Name, d.Friendly, d.Hostname, d.IP) + T(" (Dante secondaria)", " (Dante secondary)")
				out = append(out, sec)
				have[o.IP] = true
			}
		}
	}
	return out
}

// checkClockLocked warns when a critical device that was following the PTP
// clock stops asking the master (m.mu held).
func (m *monitor) checkClockLocked(now time.Time) {
	if m.clockLost == nil {
		m.clockLost = map[string]bool{}
	}
	for _, k := range m.order {
		d := m.devs[k]
		last, ok := m.ptpFollow[d.IP]
		if !ok || !d.Alive {
			continue // never seen following, or already reported as down
		}
		lost := now.Sub(last) > 20*time.Second
		switch {
		case lost && !m.clockLost[k]:
			m.clockLost[k] = true
			m.addEventRaw(MonEvent{At: now, Name: d.Name, IP: d.IP, Kind: "ptp", Text: T("non segue più il clock PTP", "no longer follows the PTP clock")})
		case !lost && m.clockLost[k]:
			delete(m.clockLost, k)
			m.addEventRaw(MonEvent{At: now, Name: d.Name, IP: d.IP, Kind: "ptp", Text: T("di nuovo agganciato al clock PTP", "locked to the PTP clock again")})
		}
	}
}
