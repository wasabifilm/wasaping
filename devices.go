package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Device is what wasaping remembers about a device between scans.
type Device struct {
	Name      string    `json:"name,omitempty"` // set by the user
	Note      string    `json:"note,omitempty"` // set by the user
	MAC       string    `json:"mac,omitempty"`
	IP        string    `json:"ip"`
	Hostname  string    `json:"hostname,omitempty"`
	Vendor    string    `json:"vendor,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// HostView is a scan result together with what we remember about it.
type HostView struct {
	scanner.Host
	Key       string    `json:"key"`
	Name      string    `json:"name,omitempty"`
	Note      string    `json:"note,omitempty"`
	New       bool      `json:"new,omitempty"`
	FirstSeen time.Time `json:"first_seen,omitempty"`
}

// GoneView is a known device that did not answer this scan.
type GoneView struct {
	Device
	Key string `json:"key"`
}

// Store keeps the known devices in devices.json, in the user config folder.
type Store struct {
	mu      sync.Mutex
	path    string
	Devices map[string]*Device `json:"devices"`
}

var (
	storeOnce sync.Once
	theStore  *Store
)

func devices() *Store {
	storeOnce.Do(func() {
		dir, err := os.UserConfigDir()
		if err != nil {
			dir = os.TempDir()
		}
		theStore = &Store{path: filepath.Join(dir, "wasaping", "devices.json"), Devices: map[string]*Device{}}
		if b, err := os.ReadFile(theStore.path); err == nil {
			json.Unmarshal(b, theStore)
			if theStore.Devices == nil {
				theStore.Devices = map[string]*Device{}
			}
		}
	})
	return theStore
}

// deviceKey identifies a device: by MAC when known, else by IP.
func deviceKey(h scanner.Host) string {
	if h.MAC != "" {
		return h.MAC
	}
	return "ip:" + h.IP
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Session tracks one scan against the store.
type Session struct {
	s       *Store
	started time.Time
	first   bool // nothing known yet: do not flag every device as new
	seen    map[string]bool
}

func (s *Store) Begin() *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &Session{s: s, started: time.Now(), first: len(s.Devices) == 0, seen: map[string]bool{}}
}

// View annotates a host just found and records it.
func (ss *Session) View(h scanner.Host) HostView {
	s := ss.s
	s.mu.Lock()
	defer s.mu.Unlock()
	key := deviceKey(h)
	d, known := s.Devices[key]
	if !known && h.MAC != "" {
		// Seen before by IP only (e.g. before the MACs were readable):
		// keep the name the user gave it.
		if old, ok := s.Devices["ip:"+h.IP]; ok {
			d, known = old, true
			delete(s.Devices, "ip:"+h.IP)
		}
	}
	now := time.Now()
	if d == nil {
		d = &Device{FirstSeen: now}
	}
	d.MAC, d.IP, d.LastSeen = h.MAC, h.IP, now
	if h.Hostname != "" {
		d.Hostname = h.Hostname
	}
	if h.Vendor != "" {
		d.Vendor = h.Vendor
	}
	if h.Kind != "" {
		d.Kind = h.Kind
	}
	s.Devices[key] = d
	ss.seen[key] = true
	return HostView{Host: h, Key: key, Name: d.Name, Note: d.Note, New: !known && !ss.first, FirstSeen: d.FirstSeen}
}

// Finish saves the store and returns the known devices in the scanned range
// that did not answer.
func (ss *Session) Finish(targets []net.IP) []GoneView {
	s := ss.s
	s.mu.Lock()
	defer s.mu.Unlock()
	inRange := make(map[string]bool, len(targets))
	for _, ip := range targets {
		inRange[ip.String()] = true
	}
	var gone []GoneView
	for k, d := range s.Devices {
		if !ss.seen[k] && inRange[d.IP] && d.LastSeen.Before(ss.started) {
			gone = append(gone, GoneView{Device: *d, Key: k})
		}
	}
	sort.Slice(gone, func(i, j int) bool { return ipLess(gone[i].IP, gone[j].IP) })
	s.saveLocked()
	return gone
}

// Label sets the user's name and note for a device.
func (s *Store) Label(key, name, note string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.Devices[key]
	if !ok {
		return false
	}
	d.Name, d.Note = strings.TrimSpace(name), strings.TrimSpace(note)
	s.saveLocked()
	return true
}

// Forget removes a device from the list of known ones.
func (s *Store) Forget(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Devices, key)
	s.saveLocked()
}

func ipLess(a, b string) bool {
	x, y := net.ParseIP(a).To4(), net.ParseIP(b).To4()
	if x == nil || y == nil {
		return a < b
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return false
}
