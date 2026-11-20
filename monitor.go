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
	Slow   bool      `json:"slow,omitempty"`  // over the threshold for 3 checks in a row
	Group  string    `json:"group"`           // department: lights, audio, video, network, other
	slowN  int
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
	Group   string    `json:"group,omitempty"`    // department of the device or universe
	Muted   bool      `json:"muted,omitempty"`    // happened while silenced: no sound
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
	Timecode  []MonTimecode `json:"timecode,omitempty"`
	SlowMs    int           `json:"slow_ms,omitempty"`     // threshold, 0 = off
	Slow      int           `json:"slow,omitempty"`        // devices over it
	MutedTill time.Time     `json:"muted_until,omitempty"` // no sound, notifications or alerts
	ShowSince time.Time     `json:"show_since,omitempty"`  // "show started" marker, zero = no show running
	Groups    []MonGroup    `json:"groups,omitempty"`      // departments with something to watch
	Sound     string        `json:"sound,omitempty"`       // alarm sound: "beep" or "moo"
}

// MonGroup is a department in the Monitor: how many devices, how many down.
type MonGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Total     int       `json:"total"`
	Down      int       `json:"down"`
	Slow      int       `json:"slow,omitempty"`
	MutedTill time.Time `json:"muted_until,omitempty"`
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
	muted   time.Time
	gMuted  map[string]time.Time // departments silenced on their own
	show    time.Time
	showSet bool // show read back from the log
}

// muted reports whether alarms are silenced now (m.mu held or not needed).
func (m *monitor) isMuted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Now().Before(m.muted)
}

// mute silences sound, notifications and alerts for d (0 = turn back on).
func (m *monitor) mute(d time.Duration) {
	m.mu.Lock()
	m.muted = time.Now().Add(d)
	m.addEventRaw(MonEvent{At: time.Now(), Name: "Monitor", Kind: "note", Text: map[bool]string{true: fmt.Sprintf(T("allarmi silenziati per %s", "alarms muted for %s"), shortDur(d.Seconds())), false: T("allarmi di nuovo attivi", "alarms back on")}[d > 0]})
	m.mu.Unlock()
}

// muteGroup silences one department for d (0 = turn back on).
func (m *monitor) muteGroup(g string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gMuted == nil {
		m.gMuted = map[string]time.Time{}
	}
	if d > 0 {
		m.gMuted[g] = time.Now().Add(d)
	} else {
		delete(m.gMuted, g)
	}
	text := fmt.Sprintf(T("%s: allarmi silenziati per %s", "%s: alarms muted for %s"), groupName(g), shortDur(d.Seconds()))
	if d <= 0 {
		text = fmt.Sprintf(T("%s: allarmi di nuovo attivi", "%s: alarms back on"), groupName(g))
	}
	m.addEventRaw(MonEvent{At: time.Now(), Name: "Monitor", Kind: "note", Group: g, Text: text})
}

// silencedLocked reports whether alarms of a department are off now:
// everything muted, or that department (m.mu held).
func (m *monitor) silencedLocked(g string, now time.Time) bool {
	return now.Before(m.muted) || (g != "" && now.Before(m.gMuted[g]))
}

// eventGroupLocked finds the department of an event (m.mu held).
func (m *monitor) eventGroupLocked(e MonEvent) string {
	switch e.Kind {
	case "universe":
		return "lights"
	case "note", "show", "timecode":
		return ""
	}
	for _, d := range m.devs {
		if e.IP != "" && d.IP == e.IP {
			return d.Group
		}
	}
	if e.Kind == "ptp" {
		return "audio" // the clock of Dante and AES67
	}
	return ""
}

// markShow writes "show started" / "show ended" in the log: the end-of-show
// report then covers exactly the show.
func (m *monitor) markShow(start bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if start {
		m.show = now
		m.addEventRaw(MonEvent{At: now, Name: "Show", Kind: "show", Text: showStartText})
	} else if !m.show.IsZero() {
		m.show = time.Time{}
		m.addEventRaw(MonEvent{At: now, Name: "Show", Kind: "show", Text: showEndText})
	}
	m.showSet = true
}

// The markers in monitor-log.csv (always in Italian, like the other events).
const (
	showStartText = "inizio show"
	showEndText   = "fine show"
)

var mon = &monitor{every: 5, devs: map[string]*MonDevice{}, kick: make(chan struct{}, 1)}

// onMonitorChange lets the macOS menu bar follow the monitor.
var onMonitorChange func(down int, names []string)

func (m *monitor) state(maxEvents int) MonState {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := MonState{On: m.on, Every: m.every, Checked: m.checked, SlowMs: loadSettings().MonitorSlowMs, ShowSince: m.show}
	if time.Now().Before(m.muted) {
		st.MutedTill = m.muted
	}
	for _, k := range m.order {
		d := *m.devs[k]
		st.Devices = append(st.Devices, d)
		if d.Known && !d.Alive {
			st.Down++
		}
		if d.Slow {
			st.Slow++
		}
	}
	for _, id := range m.uniOrder {
		u := *m.unis[id]
		st.Universes = append(st.Universes, u)
		if u.Known && !u.Alive {
			st.Down++
		}
	}
	now := time.Now()
	byG := map[string]*MonGroup{}
	group := func(id string) *MonGroup {
		if byG[id] == nil {
			byG[id] = &MonGroup{ID: id, Name: groupName(id)}
			if t := m.gMuted[id]; now.Before(t) {
				byG[id].MutedTill = t
			}
		}
		return byG[id]
	}
	for _, d := range st.Devices {
		g := group(d.Group)
		g.Total++
		if d.Known && !d.Alive {
			g.Down++
		}
		if d.Slow {
			g.Slow++
		}
	}
	for _, u := range st.Universes {
		g := group("lights")
		g.Total++
		if u.Known && !u.Alive {
			g.Down++
		}
	}
	for _, id := range groupIDs {
		if g := byG[id]; g != nil {
			st.Groups = append(st.Groups, *g)
		}
	}
	st.Sound = firstNonEmpty(loadSettings().AlarmSound, "beep")
	st.PTP = m.ptpStateLocked()
	st.Timecode = m.timecodeStateLocked()
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
	// A show still running when wasaping was closed goes on.
	if start, end, ok := lastShow(); ok && end.IsZero() && time.Since(start) < 24*time.Hour {
		m.mu.Lock()
		if !m.showSet {
			m.show = start
		}
		m.mu.Unlock()
	}
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
	slowMs := int64(loadSettings().MonitorSlowMs)
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
		cur.Name, cur.Role, cur.IP, cur.Kind = firstNonEmpty(d.Name, d.Friendly, d.Hostname, d.IP), d.Role, d.IP, firstNonEmpty(d.KindManual, d.Kind)
		cur.Group = deviceGroup(d.Device)
		quiet := m.silencedLocked(cur.Group, now)
		who := cur.Name + " (" + groupName(cur.Group) + ")"
		r := byIP[d.IP]
		switch {
		case !cur.Known:
			cur.Known, cur.Alive, cur.Since = true, r.Alive, now
			if !r.Alive {
				m.addEvent(now, cur, false, 0)
				if !quiet {
					lost = append(lost, who)
				}
			}
		case cur.Alive != r.Alive:
			down := now.Sub(cur.Since).Seconds()
			cur.Alive, cur.Since = r.Alive, now
			if r.Alive {
				m.addEvent(now, cur, true, down)
				if !quiet {
					back = append(back, fmt.Sprintf(T("%s (dopo %s)", "%s (after %s)"), who, shortDur(down)))
				}
			} else {
				m.addEvent(now, cur, false, 0)
				if !quiet {
					lost = append(lost, who)
				}
			}
		}
		cur.RTTms, cur.Method = r.RTTms, r.Method
		rtt := int64(-1)
		if r.Alive {
			rtt = r.RTTms
		}
		// Slow: over the threshold three checks in a row (a cable or a
		// switch port going bad often slows down before it drops).
		if slowMs > 0 && r.Alive && r.RTTms > slowMs {
			cur.slowN++
		} else {
			cur.slowN = 0
		}
		if slow := cur.slowN >= 3; slow != cur.Slow && (slow || cur.slowN == 0) {
			cur.Slow = slow
			if slow {
				m.addEventRaw(MonEvent{At: now, Name: cur.Name, IP: cur.IP, Kind: "slow", Text: fmt.Sprintf("risponde lento (%d ms)", r.RTTms)})
			} else if r.Alive {
				m.addEventRaw(MonEvent{At: now, Name: cur.Name, IP: cur.IP, Kind: "slow", Text: "di nuovo veloce"})
			}
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
	if !m.silencedLocked("lights", now) {
		lost, back = append(lost, lostU...), append(back, backU...)
	}
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
	muted := now.Before(m.muted)
	m.mu.Unlock()
	if muted {
		lost, back = nil, nil
	}
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
	m.addEventRaw(MonEvent{At: at, Name: d.Name, IP: d.IP, Up: up, DownFor: downFor, Group: d.Group})
}

func (m *monitor) addEventRaw(e MonEvent) {
	m.nextID++
	e.ID = m.nextID
	if e.Group == "" {
		e.Group = m.eventGroupLocked(e)
	}
	e.Muted = m.silencedLocked(e.Group, time.Now())
	m.events = append(m.events, e)
	if len(m.events) > 2000 {
		m.events = m.events[len(m.events)-2000:]
	}
	go appendMonitorLog(e)
	if e.Muted || e.Kind == "note" || e.Kind == "show" || e.Kind == "timecode" {
		return // silenced, or not something to alert about
	}
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
		if int(d.Seconds())%60 == 0 {
			return fmt.Sprintf("%d min", int(d.Minutes()))
		}
		return fmt.Sprintf("%d min %d s", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
}

// handleMonitor: GET the live state; POST {on, every} to start/stop.
func handleMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			On     bool    `json:"on"`
			Every  int     `json:"every"`
			Mute   *int    `json:"mute"`    // seconds, 0 = sound back on
			Group  string  `json:"group"`   // with mute: only this department
			Show   *string `json:"show"`    // "start" or "end"
			SlowMs *int    `json:"slow_ms"` // threshold, 0 = off
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		switch {
		case req.Mute != nil && req.Group != "":
			if !validGroup(req.Group) {
				http.Error(w, "reparto sconosciuto", http.StatusBadRequest)
				return
			}
			mon.muteGroup(req.Group, time.Duration(min(max(*req.Mute, 0), 3600))*time.Second)
		case req.Mute != nil:
			mon.mute(time.Duration(min(max(*req.Mute, 0), 3600)) * time.Second)
		case req.Show != nil:
			mon.markShow(*req.Show == "start")
		case req.SlowMs != nil:
			s := loadSettings()
			s.MonitorSlowMs = min(max(*req.SlowMs, 0), 5000)
			saveSettings(s)
		default:
			mon.set(req.On, req.Every)
		}
	}
	writeJSON(w, mon.state(300))
}
