package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Device is what wasaping remembers about a device between scans.
type Device struct {
	Name       string      `json:"name,omitempty"` // set by the user
	Role       string      `json:"role,omitempty"` // set by the user, e.g. "Console FOH"
	Note       string      `json:"note,omitempty"` // set by the user
	Critical   bool        `json:"critical,omitempty"`
	MAC        string      `json:"mac,omitempty"`
	IP         string      `json:"ip"`
	Hostname   string      `json:"hostname,omitempty"`
	Friendly   string      `json:"friendly,omitempty"`
	Model      string      `json:"model,omitempty"`
	Vendor     string      `json:"vendor,omitempty"`
	Kind       string      `json:"kind,omitempty"`
	KindManual string      `json:"kind_manual,omitempty"` // type chosen by the user
	Ports      []int       `json:"ports,omitempty"`
	FirstSeen  time.Time   `json:"first_seen"`
	LastSeen   time.Time   `json:"last_seen"`
	Seen       []time.Time `json:"seen,omitempty"` // scans that found it (latest maxHistory)
}

// Network is a place: home, office, a venue. Devices are remembered per network.
type Network struct {
	Name      string             `json:"name,omitempty"` // set by the user, e.g. "Arena Verona"
	Auto      string             `json:"auto"`           // description wasaping made
	Subnet    string             `json:"subnet,omitempty"`
	Devices   map[string]*Device `json:"devices"`
	Scans     []time.Time        `json:"scans,omitempty"` // latest maxHistory scans
	Plan      []PlanItem         `json:"plan,omitempty"`  // imported IP plan
	Universes []string           `json:"critical_universes,omitempty"`
	Snapshots []Snapshot         `json:"snapshots,omitempty"` // the network, one per session (latest 30)
	RDM       []RDMSnapshot      `json:"rdm,omitempty"`       // RDM fixtures, latest searches
	Notes     string             `json:"notes,omitempty"`     // the show's notes, set by the user
	Routing   []RoutingSnap      `json:"routing,omitempty"`   // Dante / Videohub patch, latest sessions
	FirstSeen time.Time          `json:"first_seen"`
	LastSeen  time.Time          `json:"last_seen"`
}

const maxHistory = 200

// HostView is a scan result together with what we remember about it.
type HostView struct {
	scanner.Host
	Key       string    `json:"key"`
	Net       string    `json:"net,omitempty"` // network id (several networks scanned at once)
	Name      string    `json:"name,omitempty"`
	Role      string    `json:"role,omitempty"`
	Note      string    `json:"note,omitempty"`
	Critical  bool      `json:"critical,omitempty"`
	New       bool      `json:"new,omitempty"`
	FirstSeen time.Time `json:"first_seen,omitempty"`
	History   string    `json:"history,omitempty"`   // latest scans, oldest first: 1 = seen, 0 = not
	AutoKind  string    `json:"auto_kind,omitempty"` // what wasaping guessed, when the user corrected it
}

// GoneView is a known device that did not answer this scan.
type GoneView struct {
	Device
	Key     string `json:"key"`
	Net     string `json:"net,omitempty"`
	History string `json:"history,omitempty"`
}

// NetView describes the current network to the interface.
type NetView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Auto      string    `json:"auto"`
	Subnet    string    `json:"subnet,omitempty"`
	Notes     string    `json:"notes,omitempty"`
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
	// Notes may hold private details: readable only by this user.
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// networkID names the network the targets are on: by the router's MAC when
// there is a router (the same venue keeps its id even if the subnet changes),
// else by the subnet (show networks often have no router).
func networkID(ctx context.Context, targets []net.IP) (id, auto, subnetOut string) {
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
	defer func() { subnetOut = subnet }()
	if gw := scanner.DefaultGateway(); gw != "" {
		if _, n, err := net.ParseCIDR(subnet); err == nil && n.Contains(net.ParseIP(gw)) {
			if mac := scanner.ARPSnapshot(ctx)[gw]; mac != "" {
				v := scanner.Vendor(mac)
				desc := "Rete " + subnet + ", router " + gw
				if v != "" && !strings.HasPrefix(v, "(") {
					desc += " (" + v + ")"
				}
				return "gw:" + mac, desc, subnet
			}
			return "gw:" + gw + "@" + subnet, "Rete " + subnet + ", router " + gw, subnet
		}
	}
	return "net:" + subnet, "Rete " + subnet + " senza router", subnet
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
	id, auto, subnet := networkID(ctx, targets)
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
	n.Auto, n.Subnet = auto, subnet
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
	for _, kv := range [][2]*string{{&d.Hostname, &h.Hostname}, {&d.Friendly, &h.Friendly}, {&d.Model, &h.Model}, {&d.Vendor, &h.Vendor}} {
		if *kv[1] != "" {
			*kv[0] = *kv[1]
		}
	}
	if len(h.Ports) > 0 {
		d.Ports = h.Ports
	}
	if h.Kind != "" && d.KindManual == "" {
		d.Kind = h.Kind
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
	auto := ""
	if d.KindManual != "" && d.KindManual != h.Kind {
		auto, h.Kind = h.Kind, d.KindManual
	}
	return HostView{AutoKind: auto, Host: h, Key: key, Net: ss.id, Name: d.Name, Role: d.Role, Note: d.Note, Critical: d.Critical,
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
			gone = append(gone, GoneView{Device: *d, Key: k, Net: ss.id, History: history(n, d, ss.started, false)})
		}
	}
	sort.Slice(gone, func(i, j int) bool { return ipLess(gone[i].IP, gone[j].IP) })
	n.Scans = append(n.Scans, ss.started)
	if len(n.Scans) > maxHistory {
		n.Scans = n.Scans[len(n.Scans)-maxHistory:]
	}
	n.LastSeen = ss.started
	n.addSnapshotLocked(ss)
	s.saveLocked()
	return gone
}

// Snapshot is how a network looked: used to compare with an earlier show.
type Snapshot struct {
	At      time.Time `json:"at"`
	Devices []SnapDev `json:"devices"`
}

type SnapDev struct {
	Key   string `json:"key"`
	IP    string `json:"ip"`
	Name  string `json:"name,omitempty"`
	Model string `json:"model,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// RDMSnapshot is the result of one RDM search.
type RDMSnapshot struct {
	At       time.Time         `json:"at"`
	Fixtures []scanner.Fixture `json:"fixtures"`
}

// addSnapshotLocked keeps one snapshot per working session: scans less
// than an hour apart replace the latest one.
func (n *Network) addSnapshotLocked(ss *Session) {
	snap := Snapshot{At: ss.started}
	for k := range ss.seen {
		d := n.Devices[k]
		if d == nil {
			continue
		}
		snap.Devices = append(snap.Devices, SnapDev{Key: k, IP: d.IP, Name: firstNonEmpty(d.Name, d.Friendly, d.Hostname), Model: firstNonEmpty(d.Model, d.Vendor), Kind: firstNonEmpty(d.KindManual, d.Kind)})
	}
	sort.Slice(snap.Devices, func(i, j int) bool { return ipLess(snap.Devices[i].IP, snap.Devices[j].IP) })
	if l := len(n.Snapshots); l > 0 && ss.started.Sub(n.Snapshots[l-1].At) < time.Hour {
		n.Snapshots[l-1] = snap
	} else {
		n.Snapshots = append(n.Snapshots, snap)
	}
	if len(n.Snapshots) > 30 {
		n.Snapshots = n.Snapshots[len(n.Snapshots)-30:]
	}
}

// Snapshots returns the snapshots of the current network.
func (s *Store) Snapshots() []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := s.Networks[s.current]; n != nil {
		return append([]Snapshot(nil), n.Snapshots...)
	}
	return nil
}

// AddRDM records an RDM search and returns the previous one (if any).
// LastRDM is the latest RDM search on the current network (nil if none).
func (s *Store) LastRDM() *RDMSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[s.current]
	if n == nil || len(n.RDM) == 0 {
		return nil
	}
	r := n.RDM[len(n.RDM)-1]
	return &r
}

func (s *Store) AddRDM(f []scanner.Fixture) (prev *RDMSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[s.current]
	if n == nil || len(f) == 0 {
		return nil
	}
	if l := len(n.RDM); l > 0 {
		p := n.RDM[l-1]
		prev = &p
	}
	n.RDM = append(n.RDM, RDMSnapshot{At: time.Now(), Fixtures: f})
	if len(n.RDM) > 10 {
		n.RDM = n.RDM[len(n.RDM)-10:]
	}
	s.saveLocked()
	return prev
}

// Network describes the network of the latest scan.
func (ss *Session) Network() NetView {
	ss.s.mu.Lock()
	defer ss.s.mu.Unlock()
	return netView(ss.id, ss.net, ss.first)
}

func netView(id string, n *Network, isNew bool) NetView {
	return NetView{ID: id, Name: n.Name, Auto: n.Auto, Subnet: n.Subnet, Notes: n.Notes, Known: len(n.Devices), Scans: len(n.Scans), FirstSeen: n.FirstSeen, IsNew: isNew}
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

// Select makes a known network the current one (tabs with several networks).
func (s *Store) Select(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Networks[id] == nil {
		return false
	}
	s.current = id
	return true
}

// net returns the given network, or the current one when id is "" (s.mu held).
func (s *Store) netLocked(id string) *Network {
	if id == "" {
		id = s.current
	}
	return s.Networks[id]
}

// Label sets the user's fields for a device (of network id, "" = current).
func (s *Store) Label(id, key, name, role, note string, critical bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.netLocked(id)
	if n == nil || n.Devices[key] == nil {
		return false
	}
	d := n.Devices[key]
	d.Name, d.Role, d.Note, d.Critical = strings.TrimSpace(name), strings.TrimSpace(role), strings.TrimSpace(note), critical
	s.saveLocked()
	return true
}

// SetKind corrects the type of a device ("" = back to automatic).
func (s *Store) SetKind(id, key, kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.netLocked(id)
	if n == nil || n.Devices[key] == nil {
		return false
	}
	n.Devices[key].KindManual = kind
	s.saveLocked()
	return true
}

// Forget removes a device from a network's known ones ("" = current).
func (s *Store) Forget(id, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := s.netLocked(id); n != nil {
		delete(n.Devices, key)
		s.saveLocked()
	}
}

// ForgetOld removes the devices of a network ("" = current) not seen for
// more than days (0 = not found by the latest scan). Critical ones always
// stay; those the user named or wrote a note on stay unless named is true.
// With dry, it only counts.
func (s *Store) ForgetOld(id string, days int, named, dry bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.netLocked(id)
	if n == nil {
		return 0
	}
	limit := n.LastSeen.Add(-time.Minute) // started before the latest scan
	if days > 0 {
		limit = time.Now().AddDate(0, 0, -days)
	}
	removed := 0
	for k, d := range n.Devices {
		if d.Critical || !d.LastSeen.Before(limit) || (!named && (d.Name != "" || d.Role != "" || d.Note != "")) {
			continue
		}
		if !dry {
			delete(n.Devices, k)
		}
		removed++
	}
	if removed > 0 && !dry {
		s.saveLocked()
	}
	return removed
}

// SetPlan stores the IP plan of the current network (nil removes it).
func (s *Store) SetPlan(p []PlanItem) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[s.current]
	if n == nil {
		return false
	}
	n.Plan = p
	s.saveLocked()
	return true
}

// Plan returns the IP plan of the current network.
func (s *Store) Plan() []PlanItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := s.Networks[s.current]; n != nil && n.Plan != nil {
		return n.Plan
	}
	return []PlanItem{}
}

// Venue is a network's archive in a file to share with colleagues or keep
// as the show's project: names, roles, notes, critical devices and
// universes, the IP plan, the RDM patch and the history. Version 1 (0.6)
// had only devices, plan and name; it still opens.
type Venue struct {
	Format    string        `json:"format"` // "wasaping-venue"
	Version   int           `json:"version"`
	ID        string        `json:"id,omitempty"` // gw:<router MAC> or net:<subnet>
	Name      string        `json:"name,omitempty"`
	Auto      string        `json:"auto,omitempty"`
	Subnet    string        `json:"subnet,omitempty"`
	Notes     string        `json:"notes,omitempty"`
	Devices   []VenueDevice `json:"devices"`
	Plan      []PlanItem    `json:"plan,omitempty"`
	Universes []string      `json:"critical_universes,omitempty"`
	RDM       []RDMSnapshot `json:"rdm,omitempty"`
	Snapshots []Snapshot    `json:"snapshots,omitempty"`
	Routing   []RoutingSnap `json:"routing,omitempty"`
}

type VenueDevice struct {
	Key        string    `json:"key"`
	Name       string    `json:"name,omitempty"`
	Role       string    `json:"role,omitempty"`
	Note       string    `json:"note,omitempty"`
	Critical   bool      `json:"critical,omitempty"`
	MAC        string    `json:"mac,omitempty"`
	IP         string    `json:"ip"`
	Kind       string    `json:"kind,omitempty"`
	KindManual string    `json:"kind_manual,omitempty"`
	Model      string    `json:"model,omitempty"`
	Friendly   string    `json:"friendly,omitempty"`
	Hostname   string    `json:"hostname,omitempty"`
	LastSeen   time.Time `json:"last_seen,omitempty"`
}

// Project is the show's file: one or more networks (lights, Dante primary
// and secondary, video…), with a name and notes.
type Project struct {
	Format  string    `json:"format"` // "wasaping-project"
	Version int       `json:"version"`
	Name    string    `json:"name,omitempty"`
	Notes   string    `json:"notes,omitempty"`
	SavedAt time.Time `json:"saved_at"`
	App     string    `json:"app,omitempty"`
	Venues  []Venue   `json:"venues"`
}

// ExportNetwork returns a network ("" = current) as a Venue.
func (s *Store) ExportNetwork(id string) (Venue, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = s.current
	}
	n := s.Networks[id]
	if n == nil {
		return Venue{}, false
	}
	v := Venue{Format: "wasaping-venue", Version: 2, ID: id, Name: n.Name, Auto: n.Auto, Subnet: n.Subnet, Notes: n.Notes, Plan: n.Plan,
		Universes: n.Universes, Snapshots: n.Snapshots, Routing: n.Routing}
	if len(n.RDM) > 0 {
		v.RDM = n.RDM[len(n.RDM)-1:]
	}
	for k, d := range n.Devices {
		v.Devices = append(v.Devices, VenueDevice{Key: k, Name: d.Name, Role: d.Role, Note: d.Note, Critical: d.Critical,
			MAC: d.MAC, IP: d.IP, Kind: d.Kind, KindManual: d.KindManual, Model: d.Model, Friendly: d.Friendly, Hostname: d.Hostname, LastSeen: d.LastSeen})
	}
	sort.Slice(v.Devices, func(i, j int) bool { return ipLess(v.Devices[i].IP, v.Devices[j].IP) })
	return v, true
}

var reNetID = regexp.MustCompile(`^(gw:[0-9a-f:]{17}|gw:[0-9.]{7,15}@[0-9./]{9,18}|net:[0-9./]{9,18})$`)

// ImportVenueInto merges a venue into the current network (target "") or
// into its own network (target "own"), created when this computer has never
// been there: when you arrive and scan, everything is ready. What the file
// says about a device wins, devices it knows and we do not are added; the
// critical universes add up, the history and the RDM searches join.
func (s *Store) ImportVenueInto(v Venue, target string) (int, string, error) {
	if v.Format != "wasaping-venue" {
		return 0, "", fmt.Errorf("non è un file di locale di wasaping")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.current
	if target == "own" && reNetID.MatchString(v.ID) {
		id = v.ID
	}
	n := s.Networks[id]
	if n == nil && id == v.ID && id != "" {
		n = &Network{Devices: map[string]*Device{}, FirstSeen: time.Now(), Auto: v.Auto, Subnet: v.Subnet}
		s.Networks[id] = n
	}
	if n == nil {
		return 0, "", fmt.Errorf("fai prima una scansione di questa rete")
	}
	count := 0
	for _, vd := range v.Devices {
		key := vd.Key
		if mac := scanner.NormalizeMAC(vd.MAC); mac != "" {
			key = mac
		} else if net.ParseIP(vd.IP).To4() == nil {
			continue
		} else if !strings.HasPrefix(key, "ip:") {
			key = "ip:" + vd.IP
		}
		d := n.Devices[key]
		if d == nil {
			d = &Device{MAC: scanner.NormalizeMAC(vd.MAC), IP: vd.IP, Kind: vd.Kind, Model: vd.Model, Friendly: vd.Friendly, Hostname: vd.Hostname, LastSeen: vd.LastSeen}
			n.Devices[key] = d
		}
		for _, kv := range [][2]*string{{&d.Name, &vd.Name}, {&d.Role, &vd.Role}, {&d.Note, &vd.Note}} {
			if *kv[1] != "" {
				*kv[0] = *kv[1]
			}
		}
		if vd.KindManual != "" && validKind(vd.KindManual) {
			d.KindManual, d.Kind = vd.KindManual, vd.KindManual
		}
		d.Critical = d.Critical || vd.Critical
		count++
	}
	if len(n.Plan) == 0 && len(v.Plan) > 0 {
		n.Plan = v.Plan
	}
	if n.Name == "" && v.Name != "" {
		n.Name = v.Name
	}
	if v.Notes != "" {
		n.Notes = v.Notes
	}
	for _, u := range v.Universes {
		if !slices.Contains(n.Universes, u) {
			n.Universes = append(n.Universes, u)
		}
	}
	for _, r := range v.RDM {
		if !slices.ContainsFunc(n.RDM, func(x RDMSnapshot) bool { return x.At.Equal(r.At) }) {
			n.RDM = append(n.RDM, r)
		}
	}
	sort.Slice(n.RDM, func(i, j int) bool { return n.RDM[i].At.Before(n.RDM[j].At) })
	for _, r := range v.Routing {
		if !slices.ContainsFunc(n.Routing, func(x RoutingSnap) bool { return x.At.Equal(r.At) }) {
			n.Routing = append(n.Routing, r)
		}
	}
	sort.Slice(n.Routing, func(i, j int) bool { return n.Routing[i].At.Before(n.Routing[j].At) })
	if len(n.Routing) > 10 {
		n.Routing = n.Routing[len(n.Routing)-10:]
	}
	if len(n.RDM) > 10 {
		n.RDM = n.RDM[len(n.RDM)-10:]
	}
	for _, sn := range v.Snapshots {
		if !slices.ContainsFunc(n.Snapshots, func(x Snapshot) bool { return x.At.Equal(sn.At) }) {
			n.Snapshots = append(n.Snapshots, sn)
		}
	}
	sort.Slice(n.Snapshots, func(i, j int) bool { return n.Snapshots[i].At.Before(n.Snapshots[j].At) })
	if len(n.Snapshots) > 30 {
		n.Snapshots = n.Snapshots[len(n.Snapshots)-30:]
	}
	s.saveLocked()
	return count, id, nil
}

// SetNotes stores the show's notes of a network ("" = current).
func (s *Store) SetNotes(id, notes string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.netLocked(id)
	if n == nil {
		return false
	}
	n.Notes = notes
	s.saveLocked()
	return true
}

// CriticalUniverses lists the DMX universes marked critical ("sacn:1").
func (s *Store) CriticalUniverses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := s.Networks[s.current]; n != nil {
		return append([]string(nil), n.Universes...)
	}
	return nil
}

// SetUniverseCritical marks or unmarks a universe on the current network.
func (s *Store) SetUniverseCritical(id string, on bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[s.current]
	if n == nil {
		return false
	}
	var out []string
	for _, u := range n.Universes {
		if u != id {
			out = append(out, u)
		}
	}
	if on {
		out = append(out, id)
	}
	n.Universes = out
	s.saveLocked()
	return true
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
