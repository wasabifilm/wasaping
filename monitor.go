package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// The show monitor runs in the program, not in the page: it keeps checking
// the critical devices with the window closed, keeps a log of every drop,
// and feeds the page, the menu bar and the remote view (iPad, phone).

// MonDevice is the live state of one critical device.
type MonDevice struct {
	Key    string    `json:"key"`
	Name   string    `json:"name"`
	Role   string    `json:"role,omitempty"`
	IP     string    `json:"ip"`
	Kind   string    `json:"kind,omitempty"`
	Known  bool      `json:"known"` // checked at least once
	Alive  bool      `json:"alive"`
	RTTms  int64     `json:"rtt_ms,omitempty"`
	Method string    `json:"method,omitempty"`
	Since  time.Time `json:"since,omitempty"` // when the current state began
	RTTs   []int64   `json:"rtts,omitempty"`  // latest response times, -1 = no answer
}

// MonEvent is a change of state: a device stops answering or comes back.
type MonEvent struct {
	ID      int       `json:"id"`
	At      time.Time `json:"at"`
	Name    string    `json:"name"`
	IP      string    `json:"ip"`
	Up      bool      `json:"up"`
	DownFor float64   `json:"down_for,omitempty"` // seconds, on "up" events
	Kind    string    `json:"kind,omitempty"`     // "", "universe", "ptp"
	Text    string    `json:"text,omitempty"`     // for warnings that are not up/down
}

// MonState is what the page and the remote view read.
type MonState struct {
	On        bool          `json:"on"`
	Every     int           `json:"every"`
	Devices   []MonDevice   `json:"devices"`
	Events    []MonEvent    `json:"events"`
	Down      int           `json:"down"`
	Network   string        `json:"network,omitempty"`
	Checked   time.Time     `json:"checked,omitempty"`
	Universes []MonUniverse `json:"universes,omitempty"`
	PTP       []MonPTP      `json:"ptp,omitempty"`
}

type monitor struct {
	showMonitor
	mu      sync.Mutex
	on      bool
	every   int
	devs    map[string]*MonDevice
	order   []string
	events  []MonEvent
	nextID  int
	checked time.Time
	kick    chan struct{}
}

var mon = &monitor{every: 5, devs: map[string]*MonDevice{}, kick: make(chan struct{}, 1)}

// onMonitorChange lets the macOS menu bar follow the monitor.
var onMonitorChange func(down int, names []string)

func (m *monitor) state(maxEvents int) MonState {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := MonState{On: m.on, Every: m.every, Checked: m.checked}
	for _, k := range m.order {
		d := *m.devs[k]
		st.Devices = append(st.Devices, d)
		if d.Known && !d.Alive {
			st.Down++
		}
	}
	for _, id := range m.uniOrder {
		u := *m.unis[id]
		st.Universes = append(st.Universes, u)
		if u.Known && !u.Alive {
			st.Down++
		}
	}
	st.PTP = m.ptpStateLocked()
	ev := m.events
	if len(ev) > maxEvents {
		ev = ev[len(ev)-maxEvents:]
	}
	st.Events = append([]MonEvent(nil), ev...)
	if n, ok := devices().Current(); ok {
		st.Network = firstNonEmpty(n.Name, n.Auto)
	}
	return st
}

func (m *monitor) set(on bool, every int) {
	m.mu.Lock()
	m.on = on
	if every >= 2 && every <= 600 {
		m.every = every
	}
	m.mu.Unlock()
	s := loadSettings()
	s.MonitorOn, s.MonitorEvery = on, m.every
	saveSettings(s)
	select {
	case m.kick <- struct{}{}:
	default:
	}
}

// loop checks the critical devices every `every` seconds while on.
func (m *monitor) loop(ctx context.Context) {
	s := loadSettings()
	m.mu.Lock()
	m.on = s.MonitorOn
	if s.MonitorEvery >= 2 {
		m.every = s.MonitorEvery
	}
	m.mu.Unlock()
	for {
		m.mu.Lock()
		on, every := m.on, m.every
		m.mu.Unlock()
		if on {
			m.startListeners(ctx)
			m.check(ctx)
		} else {
			m.stopListeners()
		}
		wait := time.Duration(every) * time.Second
		if !on {
			wait = time.Hour
		}
		select {
		case <-ctx.Done():
			return
		case <-m.kick:
		case <-time.After(wait):
		}
	}
}

func (m *monitor) check(ctx context.Context) {
	crit := danteSecondaries(devices().CriticalDevices())
	ips := make([]string, 0, len(crit))
	ports := map[string][]int{}
	for _, d := range crit {
		ips = append(ips, d.IP)
		if len(d.Ports) > 0 {
			ports[d.IP] = d.Ports[:min(8, len(d.Ports))]
		}
	}
	var res []scanner.CheckResult
	if len(ips) > 0 {
		res = scanner.Check(ctx, ips, ports)
	}
	byIP := map[string]scanner.CheckResult{}
	for _, r := range res {
		byIP[r.IP] = r
	}
	now := time.Now()
	var lost, back []string
	m.mu.Lock()
	seen := map[string]bool{}
	m.order = m.order[:0]
	for _, d := range crit {
		seen[d.Key] = true
		m.order = append(m.order, d.Key)
		cur := m.devs[d.Key]
		if cur == nil {
			cur = &MonDevice{Key: d.Key}
			m.devs[d.Key] = cur
		}
		cur.Name, cur.Role, cur.IP, cur.Kind = firstNonEmpty(d.Name, d.Friendly, d.Hostname, d.IP), d.Role, d.IP, d.Kind
		r := byIP[d.IP]
		switch {
		case !cur.Known:
			cur.Known, cur.Alive, cur.Since = true, r.Alive, now
			if !r.Alive {
				m.addEvent(now, cur, false, 0)
				lost = append(lost, cur.Name)
			}
		case cur.Alive != r.Alive:
			down := now.Sub(cur.Since).Seconds()
			cur.Alive, cur.Since = r.Alive, now
			if r.Alive {
				m.addEvent(now, cur, true, down)
				back = append(back, fmt.Sprintf(T("%s (dopo %s)", "%s (after %s)"), cur.Name, shortDur(down)))
			} else {
				m.addEvent(now, cur, false, 0)
				lost = append(lost, cur.Name)
			}
		}
		cur.RTTms, cur.Method = r.RTTms, r.Method
		rtt := int64(-1)
		if r.Alive {
			rtt = r.RTTms
		}
		cur.RTTs = append(cur.RTTs, rtt)
		if len(cur.RTTs) > 60 {
			cur.RTTs = cur.RTTs[len(cur.RTTs)-60:]
		}
	}
	for k := range m.devs {
		if !seen[k] {
			delete(m.devs, k) // no longer critical
		}
	}
	m.checked = now
	var samples []rttSample
	for _, k := range m.order {
		if d := m.devs[k]; d != nil && len(d.RTTs) > 0 {
			samples = append(samples, rttSample{now, d.Name, d.IP, d.RTTs[len(d.RTTs)-1]})
		}
	}
	go appendRTTLog(samples)
	m.checkClockLocked(now)
	lostU, backU := m.checkShowLocked(now)
	lost, back = append(lost, lostU...), append(back, backU...)
	var downNames []string
	for _, k := range m.order {
		if d := m.devs[k]; d.Known && !d.Alive {
			downNames = append(downNames, d.Name)
		}
	}
	for _, u := range m.unis {
		if u.Known && !u.Alive {
			downNames = append(downNames, u.Name)
		}
	}
	m.mu.Unlock()
	if len(lost) > 0 {
		notify(T("⚠️ wasaping: non risponde", "⚠️ wasaping: not responding"), strings.Join(lost, ", "))
	}
	if len(back) > 0 {
		notify(T("wasaping: di nuovo in linea", "wasaping: back online"), strings.Join(back, ", "))
	}
	if onMonitorChange != nil {
		onMonitorChange(len(downNames), downNames)
	}
}

// addEvent records a change and appends it to the log file (m.mu held).
func (m *monitor) addEvent(at time.Time, d *MonDevice, up bool, downFor float64) {
	m.addEventRaw(MonEvent{At: at, Name: d.Name, IP: d.IP, Up: up, DownFor: downFor})
}

func (m *monitor) addEventRaw(e MonEvent) {
	m.nextID++
	e.ID = m.nextID
	m.events = append(m.events, e)
	if len(m.events) > 2000 {
		m.events = m.events[len(m.events)-2000:]
	}
	go appendMonitorLog(e)
	go sendAlert(e)
}

var logMu sync.Mutex

// appendMonitorLog keeps every event in monitor-log.csv in the config
// folder, so a whole tour can be read back later.
func appendMonitorLog(e MonEvent) {
	logMu.Lock()
	defer logMu.Unlock()
	p := monitorLogPath()
	os.MkdirAll(filepath.Dir(p), 0o700)
	newFile := false
	if _, err := os.Stat(p); os.IsNotExist(err) {
		newFile = true
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	if newFile {
		fmt.Fprintln(f, "data,ora,rete,dispositivo,ip,evento,durata_s")
	}
	net := ""
	if n, ok := devices().Current(); ok {
		net = firstNonEmpty(n.Name, n.Auto)
	}
	ev, dur := "non risponde", ""
	if e.Up {
		ev, dur = "di nuovo in linea", fmt.Sprintf("%.0f", e.DownFor)
	}
	if e.Text != "" {
		ev = e.Text
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	fmt.Fprintf(f, "%s,%s,%s,%s,%s,%s,%s\n", e.At.Format("2006-01-02"), e.At.Format("15:04:05"), q(net), q(e.Name), e.IP, ev, dur)
}

func shortDur(s float64) string {
	d := time.Duration(s) * time.Second
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min %d s", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
}

// handleMonitor: GET the live state; POST {on, every} to start/stop.
func handleMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			On    bool `json:"on"`
			Every int  `json:"every"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		mon.set(req.On, req.Every)
	}
	writeJSON(w, mon.state(300))
}
