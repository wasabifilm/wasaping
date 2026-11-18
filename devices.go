package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	Name      string      `json:"name,omitempty"` // set by the user
	Role      string      `json:"role,omitempty"` // set by the user, e.g. "Console FOH"
	Note      string      `json:"note,omitempty"` // set by the user
	Critical  bool        `json:"critical,omitempty"`
	MAC       string      `json:"mac,omitempty"`
	IP        string      `json:"ip"`
	Hostname  string      `json:"hostname,omitempty"`
	Friendly  string      `json:"friendly,omitempty"`
	Model     string      `json:"model,omitempty"`
	Vendor    string      `json:"vendor,omitempty"`
	Kind      string      `json:"kind,omitempty"`
	Ports     []int       `json:"ports,omitempty"`
	FirstSeen time.Time   `json:"first_seen"`
	LastSeen  time.Time   `json:"last_seen"`
	Seen      []time.Time `json:"seen,omitempty"` // scans that found it (latest maxHistory)
}

// Network is a place: home, office, a venue. Devices are remembered per network.
type Network struct {
	Name      string             `json:"name,omitempty"` // set by the user, e.g. "Arena Verona"
	Auto      string             `json:"auto"`           // description wasaping made
	Devices   map[string]*Device `json:"devices"`
	Scans     []time.Time        `json:"scans,omitempty"` // latest maxHistory scans
	FirstSeen time.Time          `json:"first_seen"`
	LastSeen  time.Time          `json:"last_seen"`
}

const maxHistory = 200

// HostView is a scan result together with what we remember about it.
type HostView struct {
	scanner.Host
	Key       string    `json:"key"`
	Name      string    `json:"name,omitempty"`
	Role      string    `json:"role,omitempty"`
	Note      string    `json:"note,omitempty"`
	Critical  bool      `json:"critical,omitempty"`
	New       bool      `json:"new,omitempty"`
	FirstSeen time.Time `json:"first_seen,omitempty"`
	History   string    `json:"history,omitempty"` // latest scans, oldest first: 1 = seen, 0 = not
}

// GoneView is a known device that did not answer this scan.
type GoneView struct {
	Device
	Key     string `json:"key"`
	History string `json:"history,omitempty"`
}

// NetView describes the current network to the interface.
type NetView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Auto      string    `json:"auto"`
	Known     int       `json:"known"`
	Scans     int       `json:"scans"`
	FirstSeen time.Time `json:"first_seen"`
	IsNew     bool      `json:"is_new,omitempty"`
}

// Store keeps the known devices in devices.json, in the user config folder.
type Store struct {
	mu       sync.Mutex
	path     string
	current  string
	Networks map[string]*Network `json:"networks"`
	Legacy   map[string]*Device  `json:"devices,omitempty"` // v0.2 format, moved on first use
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
		theStore = openStore(filepath.Join(dir, "wasaping", "devices.json"))
	})
	return theStore
}

func openStore(path string) *Store {
	s := &Store{path: path, Networks: map[string]*Network{}}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, s)
		if s.Networks == nil {
			s.Networks = map[string]*Network{}
		}
	}
	return s
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

// networkID names the network the targets are on: by the router's MAC when
// there is a router (the same venue keeps its id even if the subnet changes),
// else by the subnet (show networks often have no router).
func networkID(ctx context.Context, targets []net.IP) (id, auto string) {
	subnet := ""
	if ifs, err := scanner.LocalInterfaces(); err == nil && len(targets) > 0 {
		for _, i := range ifs {
			if _, n, err := net.ParseCIDR(i.Network); err == nil && n.Contains(targets[0]) {
				subnet = n.String()
			}
		}
	}
	if subnet == "" && len(targets) > 0 {
		subnet = targets[0].Mask(net.CIDRMask(24, 32)).String() + "/24"
	}
	if gw := scanner.DefaultGateway(); gw != "" {
		if _, n, err := net.ParseCIDR(subnet); err == nil && n.Contains(net.ParseIP(gw)) {
			if mac := scanner.ARPTable(ctx)[gw]; mac != "" {
				v := scanner.Vendor(mac)
				desc := "Rete " + subnet + ", router " + gw
				if v != "" && !strings.HasPrefix(v, "(") {
					desc += " (" + v + ")"
				}
				return "gw:" + mac, desc
			}
			return "gw:" + gw + "@" + subnet, "Rete " + subnet + ", router " + gw
		}
	}
	return "net:" + subnet, "Rete " + subnet + " senza router"
}

// Session tracks one scan against the store.
type Session struct {
	s        *Store
	net      *Network
	id       string
	started  time.Time
	first    bool // nothing known on this network yet: do not flag every device as new
	lastScan time.Time
	seen     map[string]bool
}

func (s *Store) Begin(ctx context.Context, targets []net.IP) *Session {
	id, auto := networkID(ctx, targets)
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[id]
	if n == nil {
		n = &Network{Devices: map[string]*Device{}, FirstSeen: time.Now()}
		if len(s.Legacy) > 0 { // devices from wasaping 0.2 belong to the first network seen
			n.Devices, s.Legacy = s.Legacy, nil
		}
		s.Networks[id] = n
	}
	n.Auto = auto
	s.current = id
	ss := &Session{s: s, net: n, id: id, started: time.Now(), first: len(n.Devices) == 0, seen: map[string]bool{}}
	if len(n.Scans) > 0 {
		ss.lastScan = n.Scans[len(n.Scans)-1]
	}
	return ss
}

// history renders presence over the latest scans of the network (max 40).
func history(n *Network, d *Device, now time.Time, seenNow bool) string {
	scans := append([]time.Time(nil), n.Scans...)
	scans = append(scans, now)
	if len(scans) > 40 {
		scans = scans[len(scans)-40:]
	}
	seen := map[int64]bool{}
	for _, t := range d.Seen {
		seen[t.UnixNano()] = true
	}
	var b strings.Builder
	for i, t := range scans {
		switch {
		case i == len(scans)-1 && seenNow, seen[t.UnixNano()]:
			b.WriteByte('1')
		case t.Before(d.FirstSeen.Add(-time.Second)):
			b.WriteByte('-') // not known yet
		default:
			b.WriteByte('0')
		}
	}
	return b.String()
}

// View annotates a host just found and records it.
func (ss *Session) View(h scanner.Host) HostView {
	s, n := ss.s, ss.net
	s.mu.Lock()
	defer s.mu.Unlock()
	key := deviceKey(h)
	d, known := n.Devices[key]
	if !known && h.MAC != "" {
		// Seen before by IP only (e.g. before the MACs were readable):
		// keep what the user wrote about it.
		if old, ok := n.Devices["ip:"+h.IP]; ok {
			d, known = old, true
			delete(n.Devices, "ip:"+h.IP)
		}
	}
	if h.MAC != "" && !ss.lastScan.IsZero() {
		// Last time this IP belonged to another device: replaced, or an IP conflict.
		for k, o := range n.Devices {
			if k != key && o.IP == h.IP && o.MAC != "" && o.LastSeen.Equal(ss.lastScan) {
				h.Warnings = append(h.Warnings, fmt.Sprintf("Alla scansione precedente questo IP era di %s (%s): dispositivo sostituito o conflitto di IP.",
					firstNonEmpty(o.Name, o.Friendly, o.Hostname, "un altro dispositivo"), o.MAC))
			}
		}
	}
	now := ss.started
	if d == nil {
		d = &Device{FirstSeen: now}
	}
	d.MAC, d.IP, d.LastSeen = h.MAC, h.IP, now
	for _, kv := range [][2]*string{{&d.Hostname, &h.Hostname}, {&d.Friendly, &h.Friendly}, {&d.Model, &h.Model}, {&d.Vendor, &h.Vendor}, {&d.Kind, &h.Kind}} {
		if *kv[1] != "" {
			*kv[0] = *kv[1]
		}
	}
	if len(h.Ports) > 0 {
		d.Ports = h.Ports
	}
	hist := history(n, d, now, true)
	if len(d.Seen) == 0 || !d.Seen[len(d.Seen)-1].Equal(now) {
		d.Seen = append(d.Seen, now)
		if len(d.Seen) > maxHistory {
			d.Seen = d.Seen[len(d.Seen)-maxHistory:]
		}
	}
	n.Devices[key] = d
	ss.seen[key] = true
	return HostView{Host: h, Key: key, Name: d.Name, Role: d.Role, Note: d.Note, Critical: d.Critical,
		New: !known && !ss.first, FirstSeen: d.FirstSeen, History: hist}
}

// Finish saves the store and returns the known devices in the scanned range
// that did not answer (critical ones are always included).
func (ss *Session) Finish(targets []net.IP) []GoneView {
	s, n := ss.s, ss.net
	s.mu.Lock()
	defer s.mu.Unlock()
	inRange := make(map[string]bool, len(targets))
	for _, ip := range targets {
		inRange[ip.String()] = true
	}
	var gone []GoneView
	for k, d := range n.Devices {
		if !ss.seen[k] && (inRange[d.IP] || d.Critical) && d.LastSeen.Before(ss.started) {
			gone = append(gone, GoneView{Device: *d, Key: k, History: history(n, d, ss.started, false)})
		}
	}
	sort.Slice(gone, func(i, j int) bool { return ipLess(gone[i].IP, gone[j].IP) })
	n.Scans = append(n.Scans, ss.started)
	if len(n.Scans) > maxHistory {
		n.Scans = n.Scans[len(n.Scans)-maxHistory:]
	}
	n.LastSeen = ss.started
	s.saveLocked()
	return gone
}

// Network describes the network of the latest scan.
func (ss *Session) Network() NetView {
	ss.s.mu.Lock()
	defer ss.s.mu.Unlock()
	return netView(ss.id, ss.net, ss.first)
}

func netView(id string, n *Network, isNew bool) NetView {
	return NetView{ID: id, Name: n.Name, Auto: n.Auto, Known: len(n.Devices), Scans: len(n.Scans), FirstSeen: n.FirstSeen, IsNew: isNew}
}

// Current returns the network of the latest scan, if any.
func (s *Store) Current() (NetView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[s.current]
	if n == nil {
		return NetView{}, false
	}
	return netView(s.current, n, false), true
}

// RenameNetwork sets the user's name for the current network ("Arena Verona").
func (s *Store) RenameNetwork(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[s.current]
	if n == nil {
		return false
	}
	n.Name = strings.TrimSpace(name)
	s.saveLocked()
	return true
}

// Label sets the user's fields for a device of the current network.
func (s *Store) Label(key, name, role, note string, critical bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[s.current]
	if n == nil || n.Devices[key] == nil {
		return false
	}
	d := n.Devices[key]
	d.Name, d.Role, d.Note, d.Critical = strings.TrimSpace(name), strings.TrimSpace(role), strings.TrimSpace(note), critical
	s.saveLocked()
	return true
}

// Forget removes a device from the current network's known ones.
func (s *Store) Forget(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := s.Networks[s.current]; n != nil {
		delete(n.Devices, key)
		s.saveLocked()
	}
}

// CriticalDevices lists the devices marked critical on the current network.
func (s *Store) CriticalDevices() []GoneView {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []GoneView
	if n := s.Networks[s.current]; n != nil {
		for k, d := range n.Devices {
			if d.Critical {
				out = append(out, GoneView{Device: *d, Key: k})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return ipLess(out[i].IP, out[j].IP) })
	return out
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
