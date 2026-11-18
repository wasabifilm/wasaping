package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// ScanResult is a finished scan, as the interface and the menu bar see it.
type ScanResult struct {
	At        time.Time  `json:"at"`
	Target    string     `json:"target"`
	Auto      bool       `json:"auto,omitempty"`
	Count     int        `json:"count"`
	Seconds   float64    `json:"seconds"`
	MacHidden bool       `json:"macHidden"`
	Hosts     []HostView `json:"hosts"`
	Gone      []GoneView `json:"gone"`
	New       int        `json:"new"`
	Fresh     []string   `json:"fresh,omitempty"`
	Network   NetView    `json:"network"`
	Issues    []string   `json:"issues"`
}

var (
	lastMu     sync.Mutex
	lastResult *ScanResult
	// onScanDone lets the macOS menu bar show the latest result.
	onScanDone func(*ScanResult)
)

// runCollect scans, records the result in the device archive and keeps it
// as the latest one.
func runCollect(ctx context.Context, target string, ips []net.IP, opt scanner.Options, auto bool,
	onHost func(HostView), onProgress func(scanner.Progress)) (*ScanResult, error) {
	activeScan.Add(1)
	defer activeScan.Add(-1)
	start := time.Now()
	sess := devices().Begin(ctx, ips)
	res := &ScanResult{Target: target, Auto: auto}
	var mu sync.Mutex
	opt.Targets = ips
	hosts, err := scanner.Scan(ctx, opt, func(h scanner.Host) {
		v := sess.View(h)
		mu.Lock()
		res.Hosts = append(res.Hosts, v)
		if v.New {
			res.Fresh = append(res.Fresh, firstNonEmpty(v.Name, v.Friendly, v.Hostname, v.IP))
		}
		mu.Unlock()
		if onHost != nil {
			onHost(v)
		}
	}, onProgress)
	if err != nil {
		return nil, err
	}
	res.Gone = sess.Finish(ips)
	res.At, res.Count, res.Seconds = time.Now(), len(hosts), time.Since(start).Seconds()
	res.MacHidden, res.New = scanner.MACsHidden(hosts), len(res.Fresh)
	res.Network, res.Issues = sess.Network(), scanner.NetworkIssues(hosts)
	lastMu.Lock()
	lastResult = res
	lastMu.Unlock()
	if onScanDone != nil {
		onScanDone(res)
	}
	return res, nil
}

// ---- Settings ----

type settings struct {
	Repeat        int       `json:"repeat_minutes"` // 0 = no background scans
	MonitorOn     bool      `json:"monitor_on,omitempty"`
	MonitorEvery  int       `json:"monitor_every,omitempty"` // seconds
	RemoteOn      bool      `json:"remote_on,omitempty"`
	RemotePIN     string    `json:"remote_pin,omitempty"`
	RemotePort    int       `json:"remote_port,omitempty"`
	SNMPCommunity string    `json:"snmp_community,omitempty"`
	NoUpdateCheck bool      `json:"no_update_check,omitempty"`
	UpdateChecked time.Time `json:"update_checked,omitempty"`
	LatestVersion string    `json:"latest_version,omitempty"`
	LatestURL     string    `json:"latest_url,omitempty"`
	Lang          string    `json:"lang,omitempty"`
}

var (
	setMu    sync.Mutex
	setCache *settings
)

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "wasaping", "settings.json")
}

func loadSettings() settings {
	setMu.Lock()
	defer setMu.Unlock()
	if setCache == nil {
		setCache = &settings{}
		if b, err := os.ReadFile(settingsPath()); err == nil {
			json.Unmarshal(b, setCache)
		}
	}
	return *setCache
}

func saveSettings(s settings) {
	setMu.Lock()
	defer setMu.Unlock()
	setCache = &s
	os.MkdirAll(filepath.Dir(settingsPath()), 0o755)
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		os.WriteFile(settingsPath(), b, 0o600)
	}
}

// ---- Background scans ----

var (
	bgKick    = make(chan struct{}, 1)
	bgRunning atomic.Bool
)

// backgroundLoop scans the local network every `Repeat` minutes (and when
// kicked by "Scansiona ora" in the menu bar), with notifications for new
// devices and for critical devices that stop answering.
func backgroundLoop(ctx context.Context) {
	next := time.Now().Add(time.Duration(max(loadSettings().Repeat, 1)) * time.Minute)
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		kicked := false
		select {
		case <-ctx.Done():
			return
		case <-bgKick:
			kicked = true
		case <-t.C:
		}
		rep := loadSettings().Repeat
		if !kicked && (rep <= 0 || time.Now().Before(next)) {
			continue
		}
		if rep > 0 {
			next = time.Now().Add(time.Duration(rep) * time.Minute)
		}
		if activeScan.Load() > 0 {
			continue // someone is scanning from the page already
		}
		backgroundScan(ctx)
	}
}

func backgroundScan(ctx context.Context) {
	if !bgRunning.CompareAndSwap(false, true) {
		return
	}
	defer bgRunning.Store(false)
	i, err := scanner.DefaultInterface()
	if err != nil {
		return
	}
	target := clampNetwork(i)
	ips, err := scanner.ParseTargets(target)
	if err != nil {
		return
	}
	res, err := runCollect(ctx, target, ips, scanner.Options{Ping: true, ResolveDNS: true, ScanPorts: scanner.CommonPorts}, true, nil, nil)
	if err != nil {
		return
	}
	if len(res.Fresh) > 0 {
		notify(T("wasaping: nuovi dispositivi", "wasaping: new devices"), strings.Join(res.Fresh, ", "))
	}
	var lost []string
	for _, g := range res.Gone {
		if g.Critical {
			lost = append(lost, firstNonEmpty(g.Name, g.Friendly, g.Hostname, g.IP))
		}
	}
	if len(lost) > 0 {
		notify(T("⚠️ wasaping: non risponde", "⚠️ wasaping: not responding"), strings.Join(lost, ", "))
	}
}

// handleBackground reads or sets the repeat interval; POST ?now=1 scans now.
func handleBackground(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if r.URL.Query().Get("now") == "1" {
			select {
			case bgKick <- struct{}{}:
			default:
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var req settings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil || req.Repeat < 0 || req.Repeat > 24*60 {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		s := loadSettings()
		s.Repeat = req.Repeat
		saveSettings(s)
	}
	writeJSON(w, map[string]any{"repeat_minutes": loadSettings().Repeat, "running": bgRunning.Load()})
}

// handleLast returns the latest finished scan (page reopened, background scans).
func handleLast(w http.ResponseWriter, r *http.Request) {
	lastMu.Lock()
	res := lastResult
	lastMu.Unlock()
	writeJSON(w, res)
}
