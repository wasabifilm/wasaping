package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Scanning and the scan-related endpoints.

func handleInfo(w http.ResponseWriter, r *http.Request) {
	ifs, _ := scanner.LocalInterfaces()
	def := ""
	if i, err := scanner.DefaultInterface(); err == nil {
		def = clampNetwork(i)
	}
	for k := range ifs {
		ifs[k].Network = clampNetwork(ifs[k])
	}
	ui, helper, app := uiMode.Load(), helperState.Load(), appModeOn
	remoteWeb := isRemoteWeb(r)
	if remoteWeb { // from another computer: an ordinary browser tab
		ui, helper, app = "browser", "notneeded", false
	}
	writeJSON(w, map[string]any{
		"remote":        remoteWeb,
		"version":       version,
		"interfaces":    ifs,
		"defaultTarget": def,
		"allTargets":    localScanTargets(),
		"hasOUI":        scanner.HasOUI(),
		"os":            runtime.GOOS,
		"app":           app,
		"ui":            ui,
		"helper":        helper,
		"snmp":          firstNonEmpty(loadSettings().SNMPCommunity, "public"),
	})
}

// handleScan streams results as Server-Sent Events.
func handleScan(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var ips []net.IP
	var targets []string
	if q.Get("target") == "all" {
		if targets = localScanTargets(); len(targets) == 0 {
			http.Error(w, "nessuna rete da scansionare", http.StatusBadRequest)
			return
		}
	} else {
		var err error
		if ips, err = scanner.ParseTargets(q.Get("target")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	ports, err := parsePorts(q.Get("ports"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming non supportato", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	var mu sync.Mutex
	send := func(event string, v any) {
		b, _ := json.Marshal(v)
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		fl.Flush()
	}
	lastProgress := time.Time{}
	opt := scanner.Options{
		Ping:       q.Get("ping") != "0",
		ResolveDNS: q.Get("dns") != "0",
		ScanPorts:  ports,
	}
	collect := func(onHost func(HostView), onProgress func(scanner.Progress)) (*ScanResult, error) {
		if targets != nil {
			return runCollectAll(r.Context(), targets, opt, false, onHost, onProgress)
		}
		return runCollect(r.Context(), q.Get("target"), ips, opt, false, onHost, onProgress)
	}
	res, err := collect(func(v HostView) {
		send("host", v)
	}, func(p scanner.Progress) {
		mu.Lock()
		due := time.Since(lastProgress) > 150*time.Millisecond || p.Done == p.Total
		if due {
			lastProgress = time.Now()
		}
		mu.Unlock()
		if due {
			send("progress", p)
		}
	})
	if err != nil {
		send("failure", map[string]string{"error": err.Error()})
		return
	}
	done := *res
	done.Hosts = nil // already streamed one by one
	send("done", done)
}

func handleUpdateOUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	n, err := scanner.UpdateOUI(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]int{"vendors": n})
}

func handleWOL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	if err := scanner.WakeOnLAN(r.URL.Query().Get("mac")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePorts checks a list of ports on one device, on request.
func handlePorts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ip := net.ParseIP(q.Get("ip")).To4()
	if ip == nil {
		http.Error(w, "indirizzo non valido", http.StatusBadRequest)
		return
	}
	ports, err := parsePorts(q.Get("ports"))
	if err != nil || len(ports) == 0 {
		http.Error(w, "porte non valide", http.StatusBadRequest)
		return
	}
	activeScan.Add(1)
	defer activeScan.Add(-1)
	open := scanner.ScanPorts(r.Context(), ip.String(), ports, time.Second)
	writeJSON(w, map[string]any{"ip": ip.String(), "checked": len(ports), "open": open})
}

// handleCheck answers which of the given devices are up (show monitor).
func handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		IPs   []string         `json:"ips"`
		Ports map[string][]int `json:"ports"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil || len(req.IPs) > 512 {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	var ips []string
	for _, ip := range req.IPs {
		if p := net.ParseIP(ip).To4(); p != nil {
			ips = append(ips, p.String())
		}
	}
	for ip, ports := range req.Ports {
		if len(ports) > 32 {
			req.Ports[ip] = ports[:32]
		}
	}
	// Not counted as a scan: the monitor checks every few seconds and must
	// not keep background scans from running.
	writeJSON(w, scanner.Check(r.Context(), ips, req.Ports))
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
