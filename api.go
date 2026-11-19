package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"
)

// The local web API: routes, and the filter in front of them.

// newHandler builds the server for the page. public: listening on the
// network (-addr 0.0.0.0); allowed: the Host values accepted otherwise.
func newHandler(public bool, allowedHosts map[string]bool, quit func()) http.Handler {
	mux := apiMux(quit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block DNS rebinding: only answer requests addressed to localhost.
		if !public && !allowedHosts[r.Host] {
			http.Error(w, "host non consentito", http.StatusForbidden)
			return
		}
		// Block other websites open in the browser from driving the API.
		if crossSite(r) {
			http.Error(w, "richiesta da un altro sito non consentita", http.StatusForbidden)
			return
		}
		// Reachable from the network (-addr 0.0.0.0): what acts on this
		// computer (files, password dialog, quitting, archive) stays local.
		if public && localOnly[r.URL.Path] && !fromLoopback(r) {
			http.Error(w, "consentito solo da questo computer", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// apiMux has the page and every route, without the filter.
func apiMux(quit func()) *http.ServeMux {
	static, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	if dir := devWebDir(); dir != "" {
		mux.Handle("/", devFiles(dir))
		mux.HandleFunc("/api/dev", handleDev)
	} else {
		mux.Handle("/", http.FileServer(http.FS(static)))
	}
	mux.HandleFunc("/api/info", handleInfo)
	mux.HandleFunc("/api/scan", handleScan)
	mux.HandleFunc("/api/update-oui", handleUpdateOUI)
	mux.HandleFunc("/api/save", handleSave)
	mux.HandleFunc("/api/device", handleDevice)
	mux.HandleFunc("/api/forget-old", handleForgetOld)
	mux.HandleFunc("/api/project", handleProject)
	mux.HandleFunc("/api/clientlog", handleClientLog)
	mux.HandleFunc("/api/wol", handleWOL)
	mux.HandleFunc("/api/ports", handlePorts)
	mux.HandleFunc("/api/helper", handleHelper)
	mux.HandleFunc("/api/network", handleNetwork)
	mux.HandleFunc("/api/check", handleCheck)
	mux.HandleFunc("/api/critical", handleCritical)
	mux.HandleFunc("/api/notify", handleNotify)
	mux.HandleFunc("/api/pingtest", handlePingTest)
	mux.HandleFunc("/api/bg", handleBackground)
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/monitor", handleMonitor)
	mux.HandleFunc("/api/remote", handleRemote)
	mux.HandleFunc("/api/plan", handlePlan)
	mux.HandleFunc("/api/dmx", handleDMX)
	mux.HandleFunc("/api/switchports", handleSwitchPorts)
	mux.HandleFunc("/api/update", handleUpdate)
	mux.HandleFunc("/api/lang", handleLang)
	mux.HandleFunc("/api/rdm", handleRDM)
	mux.HandleFunc("/api/rdm/identify", handleRDMIdentify)
	mux.HandleFunc("/api/rdm/set", handleRDMSet)
	mux.HandleFunc("/api/dante", handleDante)
	mux.HandleFunc("/api/video", handleVideo)
	mux.HandleFunc("/api/webaccess", handleWebAccess)
	mux.HandleFunc("/api/preshow", handlePreshow)
	mux.HandleFunc("/api/showreport", handleShowReport)
	mux.HandleFunc("/api/diag", handleDiag)
	mux.HandleFunc("/api/compare", handleCompare)
	mux.HandleFunc("/api/universe", handleUniverse)
	mux.HandleFunc("/api/alerts", handleAlerts)
	mux.HandleFunc("/api/update/install", handleUpdateInstall)
	mux.HandleFunc("/api/last", handleLast)
	mux.HandleFunc("/api/trace", handleTrace)
	mux.HandleFunc("/api/prefs", handlePrefs)
	mux.HandleFunc("/api/prefs/reveal", handleReveal)
	mux.HandleFunc("/api/prefs/export", handleSettingsExport)
	mux.HandleFunc("/api/prefs/import", handleSettingsImport)
	mux.HandleFunc("/api/qr", handleQR)
	mux.HandleFunc("/api/oscin", handleOSCIn)
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		lastPing.Store(time.Now().UnixNano())
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "usa POST", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		go func() { time.Sleep(200 * time.Millisecond); quit() }()
	})
	return mux
}

// localOnly are the endpoints that change this computer, not just look at
// the network.
var localOnly = map[string]bool{
	"/api/save": true, "/api/helper": true, "/api/quit": true, "/api/notify": true,
	"/api/device": true, "/api/forget-old": true, "/api/project": true, "/api/network": true, "/api/bg": true, "/api/update-oui": true, "/api/login": true, "/api/monitor": true, "/api/remote": true, "/api/plan": true, "/api/switchports": true, "/api/update": true, "/api/lang": true, "/api/universe": true, "/api/alerts": true, "/api/rdm": true, "/api/rdm/identify": true, "/api/rdm/set": true, "/api/preshow": true, "/api/diag": true, "/api/compare": true, "/api/update/install": true, "/api/webaccess": true,
	"/api/prefs": true, "/api/prefs/reveal": true, "/api/prefs/export": true, "/api/prefs/import": true, "/api/qr": true, "/api/oscin": true,
}

func fromLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	return err == nil && isLoopbackHost(host)
}

func isLoopbackHost(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// crossSite reports requests started by another website (CSRF), using the
// Fetch Metadata and Origin headers that browsers attach.
func crossSite(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return true
	}
	if o := r.Header.Get("Origin"); o == "null" {
		return true // sandboxed frames, local files: never our page
	} else if o != "" {
		return strings.TrimPrefix(strings.TrimPrefix(o, "http://"), "https://") != r.Host
	}
	return false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

func sseStart(w http.ResponseWriter) (func(string, any), bool) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming non supportato", http.StatusInternalServerError)
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	return func(event string, v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		fl.Flush()
	}, true
}
