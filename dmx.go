package main

import (
	"context"
	"net"
	"net/http"
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
