package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Show tools: live DMX, critical universes, RDM, switch ports.

// handleSwitchPorts asks the managed switches (SNMP v2c, read-only
// community) which port each device of the latest scan is on.
func handleSwitchPorts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Community string `json:"community"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
	comm := strings.TrimSpace(req.Community)
	if comm == "" {
		comm = "public"
	}
	if len(comm) > 64 {
		http.Error(w, "community troppo lunga", http.StatusBadRequest)
		return
	}
	s := loadSettings()
	s.SNMPCommunity = comm
	saveSettings(s)
	lastMu.Lock()
	var ips []string
	if lastResult != nil {
		for _, h := range lastResult.Hosts {
			if !h.Self {
				ips = append(ips, h.IP)
			}
		}
	}
	lastMu.Unlock()
	if len(ips) == 0 {
		http.Error(w, "fai prima una scansione", http.StatusConflict)
		return
	}
	activeScan.Add(1)
	defer activeScan.Add(-1)
	switches, ports := scanner.SwitchPorts(r.Context(), ips, comm)
	writeJSON(w, map[string]any{"switches": switches, "ports": ports, "community": comm})
}

// handleRDM lists the RDM fixtures behind the Art-Net nodes of the latest scan.
func handleRDM(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { // the latest search, for the patch over the DMX levels
		if last := devices().LastRDM(); last != nil {
			writeJSON(w, last)
		} else {
			writeJSON(w, RDMSnapshot{Fixtures: []scanner.Fixture{}})
		}
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	nodes := map[string][]int{}
	lastMu.Lock()
	if lastResult != nil {
		for _, h := range lastResult.Hosts {
			if h.ArtNet == nil {
				continue
			}
			u := append(append([]int{}, h.ArtNet.Out...), h.ArtNet.In...)
			if len(u) == 0 {
				u = []int{0}
			}
			nodes[h.IP] = u
		}
	}
	lastMu.Unlock()
	if len(nodes) == 0 {
		http.Error(w, "nessun nodo Art-Net nell'ultima scansione", http.StatusConflict)
		return
	}
	activeScan.Add(1)
	defer activeScan.Add(-1)
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	fx := scanner.RDMDiscover(ctx, nodes)
	out := map[string]any{"nodes": len(nodes), "fixtures": fx}
	if prev := devices().AddRDM(fx); prev != nil {
		out["previous"] = prev.At
		out["changes"] = compareFixtures(prev.Fixtures, fx, r.URL.Query().Get("lang"))
	}
	writeJSON(w, out)
}

// handleDMX streams the live universes (SSE) while the page shows them, for
// up to 15 minutes. ?u=sacn:1 asks for the levels of one universe; the page
// changes it with POST ?u=… on the same id (?s=session).
var (
	dmxMu     sync.Mutex
	dmxDetail = map[string]scanner.DMXWant{}
)

// dmxWant reads ?u=sacn:1&ch=40&all=1.
func dmxWant(q url.Values) scanner.DMXWant {
	ch, _ := strconv.Atoi(q.Get("ch"))
	return scanner.DMXWant{Uni: q.Get("u"), Channel: ch, All: q.Get("all") == "1"}
}

func handleDMX(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sess := q.Get("s")
	if r.Method == http.MethodPost {
		dmxMu.Lock()
		dmxDetail[sess] = dmxWant(q)
		dmxMu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}
	send, ok := sseStart(w)
	if !ok {
		return
	}
	dmxMu.Lock()
	dmxDetail[sess] = dmxWant(q)
	dmxMu.Unlock()
	defer func() { dmxMu.Lock(); delete(dmxDetail, sess); dmxMu.Unlock() }()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	var iface = defaultIface()
	scanner.ListenDMX(ctx, iface, 64, 250*time.Millisecond, func() scanner.DMXWant {
		dmxMu.Lock()
		defer dmxMu.Unlock()
		return dmxDetail[sess]
	}, func(u []scanner.Universe) {
		if u == nil {
			u = []scanner.Universe{}
		}
		send("snap", u)
	})
}

func defaultIface() *net.Interface {
	i, err := scanner.DefaultInterface()
	if err != nil {
		return nil
	}
	return scanner.InterfaceFor(net.ParseIP(i.IP))
}

var reUniverseID = regexp.MustCompile(`^(art|sacn):\d{1,5}$`)

// handleUniverse marks a universe as critical for the monitor: POST {id, critical}.
func handleUniverse(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			ID       string `json:"id"`
			Critical bool   `json:"critical"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil || !reUniverseID.MatchString(req.ID) {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		if !devices().SetUniverseCritical(req.ID, req.Critical) {
			http.Error(w, "fai prima una scansione di questa rete", http.StatusConflict)
			return
		}
	}
	writeJSON(w, devices().CriticalUniverses())
}

var reRDMUID = regexp.MustCompile(`^[0-9A-Fa-f]{4}:[0-9A-Fa-f]{8}$`)

// handleRDMIdentify makes a fixture flash (or stop): POST {node, universe, uid, on}.
func handleRDMIdentify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Node     string `json:"node"`
		Universe int    `json:"universe"`
		UID      string `json:"uid"`
		On       bool   `json:"on"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil ||
		net.ParseIP(req.Node).To4() == nil || !reRDMUID.MatchString(req.UID) || req.Universe < 0 || req.Universe > 32767 {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := scanner.RDMIdentify(ctx, req.Node, req.Universe, strings.ToUpper(req.UID), req.On); err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleDante reads the Dante routing of the Dante devices of the latest
// scan (plus ?ip=… for one more): GET.
func handleDante(w http.ResponseWriter, r *http.Request) {
	var ips []string
	rate := map[string]int{}
	lastMu.Lock()
	if lastResult != nil {
		for _, h := range lastResult.Hosts {
			if h.Dante != nil && !h.Self {
				ips = append(ips, h.IP)
				if h.Dante.Name != "" {
					rate[h.Dante.Name] = h.Dante.SampleRate
				}
			}
		}
	}
	lastMu.Unlock()
	if ip := net.ParseIP(r.URL.Query().Get("ip")).To4(); ip != nil && !slices.Contains(ips, ip.String()) {
		ips = append(ips, ip.String())
	}
	if len(ips) == 0 {
		writeJSON(w, map[string]any{"devices": []scanner.DanteDevice{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	devs := scanner.DanteRouting(ctx, ips)
	since, changes := devices().AddRouting("dante", danteLinks(devs))
	writeJSON(w, map[string]any{"devices": devs, "summary": scanner.SummarizeDante(devs, rate), "rates": rate, "changes": changes, "changes_since": since})
}

// handleVideo reads the video gear of the latest scan (plus ?ip=…&kind=…):
// GET. Blackmagic, vMix, Resolume… are asked now; projectors, NDI sources
// and PTPv2 clocks come from the scan.
func handleVideo(w http.ResponseWriter, r *http.Request) {
	type target struct {
		ip, kind string
		asked    bool // typed by the user: show it even when it does not answer
	}
	var targets []target
	var projectors, ndi, ptp []map[string]any
	lastMu.Lock()
	if lastResult != nil {
		for _, h := range lastResult.Hosts {
			if h.Self {
				continue
			}
			name := firstNonEmpty(h.Name, h.Friendly, h.Hostname, h.IP)
			if k := scanner.VideoCandidate(h.Host); k != "" {
				targets = append(targets, target{h.IP, k, false})
			}
			if h.PJLink != nil {
				projectors = append(projectors, map[string]any{"ip": h.IP, "name": name, "pjlink": h.PJLink})
			}
			if len(h.NDI) > 0 {
				ndi = append(ndi, map[string]any{"ip": h.IP, "name": name, "sources": h.NDI})
			}
			if strings.HasPrefix(h.PTP, "PTPv2") {
				ptp = append(ptp, map[string]any{"ip": h.IP, "name": name, "kind": h.PTP})
			}
		}
	}
	lastMu.Unlock()
	q := r.URL.Query()
	if ip := net.ParseIP(q.Get("ip")).To4(); ip != nil {
		kind := q.Get("kind")
		if !slices.Contains([]string{"ATEM", "HyperDeck", "Videohub", "vMix", "Resolume"}, kind) {
			http.Error(w, "tipo non valido", http.StatusBadRequest)
			return
		}
		targets = append(targets, target{ip.String(), kind, true})
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	out := make([]scanner.VideoInfo, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func() { defer wg.Done(); out[i] = scanner.VideoStatus(ctx, t.ip, t.kind) }()
	}
	wg.Wait()
	// Found by a guess (a Blackmagic device, port 8088…) and not answering:
	// it is not what it looked like, leave it out.
	var devs []scanner.VideoInfo
	for i, v := range out {
		if v.Err == "" || targets[i].asked || targets[i].kind == "Videohub" || targets[i].kind == "HyperDeck" {
			devs = append(devs, v)
		}
	}
	since, changes := devices().AddRouting("videohub", videohubLinks(devs))
	writeJSON(w, map[string]any{"devices": devs, "projectors": projectors, "ndi": ndi, "ptp2": ptp, "changes": changes, "changes_since": since})
}
