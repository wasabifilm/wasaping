package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// handleDMX streams the live universes (SSE) while the page shows them, for
// up to 15 minutes. ?u=sacn:1 asks for the levels of one universe; the page
// changes it with POST ?u=… on the same id (?s=session).
var (
	dmxMu     sync.Mutex
	dmxDetail = map[string]string{}
)

func handleDMX(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sess := q.Get("s")
	if r.Method == http.MethodPost {
		dmxMu.Lock()
		dmxDetail[sess] = q.Get("u")
		dmxMu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}
	send, ok := sseStart(w)
	if !ok {
		return
	}
	dmxMu.Lock()
	dmxDetail[sess] = q.Get("u")
	dmxMu.Unlock()
	defer func() { dmxMu.Lock(); delete(dmxDetail, sess); dmxMu.Unlock() }()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	var iface = defaultIface()
	scanner.ListenDMX(ctx, iface, 64, 500*time.Millisecond, func() string {
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
