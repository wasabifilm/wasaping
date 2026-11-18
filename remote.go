package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Remote view: the monitor on an iPad at FOH or a phone on stage. A second,
// separate listener on the network, read-only, protected by a PIN. It
// serves one page and one state endpoint; nothing on this computer can be
// changed from there.

const defaultRemotePort = 47474

type remoteServer struct {
	mu       sync.Mutex
	srv      *http.Server
	port     int
	tokens   map[string]time.Time // session cookie -> expiry
	failures map[string][]time.Time
}

var remote = &remoteServer{tokens: map[string]time.Time{}, failures: map[string][]time.Time{}}

func newPIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "000000"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func newToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// remoteURL is the address to type on the iPad.
func remoteURL(port int) string {
	ip := "IP-di-questo-computer"
	if i, err := scanner.DefaultInterface(); err == nil {
		ip = i.IP
	}
	return fmt.Sprintf("http://%s:%d/", ip, port)
}

// apply starts or stops the listener to match the settings.
func (rs *remoteServer) apply() error {
	s := loadSettings()
	rs.mu.Lock()
	defer rs.mu.Unlock()
	port := s.RemotePort
	if port == 0 {
		port = defaultRemotePort
	}
	if rs.srv != nil && (!s.RemoteOn || rs.port != port) {
		rs.srv.Close()
		rs.srv = nil
		rs.tokens = map[string]time.Time{} // turning it off logs everybody out
	}
	if !s.RemoteOn || rs.srv != nil {
		return nil
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("porta %d occupata: %v", port, err)
	}
	mux := http.NewServeMux()
	page, _ := fs.ReadFile(webFS, "web/m.html")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	mux.HandleFunc("/login", rs.handleLogin)
	mux.HandleFunc("/state", rs.handleState)
	rs.srv = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; frame-ancestors 'none'")
			w.Header().Set("Cache-Control", "no-store")
			mux.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	rs.port = port
	go rs.srv.Serve(ln)
	return nil
}

func clientIP(r *http.Request) string {
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	return h
}

func (rs *remoteServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	ip := clientIP(r)
	rs.mu.Lock()
	var recent []time.Time
	for _, t := range rs.failures[ip] {
		if time.Since(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	rs.failures[ip] = recent
	blocked := len(recent) >= 5
	rs.mu.Unlock()
	if blocked {
		http.Error(w, "troppi tentativi: riprova tra un minuto", http.StatusTooManyRequests)
		return
	}
	var req struct {
		PIN string `json:"pin"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
	pin := loadSettings().RemotePIN
	if pin == "" || subtle.ConstantTimeCompare([]byte(req.PIN), []byte(pin)) != 1 {
		rs.mu.Lock()
		rs.failures[ip] = append(rs.failures[ip], time.Now())
		rs.mu.Unlock()
		http.Error(w, "PIN sbagliato", http.StatusForbidden)
		return
	}
	tok := newToken()
	rs.mu.Lock()
	rs.tokens[tok] = time.Now().Add(30 * 24 * time.Hour)
	rs.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "wsp", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 30 * 24 * 3600})
	w.WriteHeader(http.StatusNoContent)
}

func (rs *remoteServer) authorized(r *http.Request) bool {
	c, err := r.Cookie("wsp")
	if err != nil {
		return false
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	exp, ok := rs.tokens[c.Value]
	return ok && time.Now().Before(exp)
}

// handleState is everything the remote page shows: the monitor and a
// summary of the latest scan. Read-only.
func (rs *remoteServer) handleState(w http.ResponseWriter, r *http.Request) {
	if !rs.authorized(r) {
		http.Error(w, "serve il PIN", http.StatusUnauthorized)
		return
	}
	out := map[string]any{"monitor": mon.state(50)}
	lastMu.Lock()
	if res := lastResult; res != nil {
		var gone []string
		for _, g := range res.Gone {
			if g.Critical {
				gone = append(gone, firstNonEmpty(g.Name, g.Friendly, g.Hostname, g.IP))
			}
		}
		out["scan"] = map[string]any{"at": res.At, "count": res.Count, "new": res.New, "issues": res.Issues, "critical_gone": gone}
	}
	lastMu.Unlock()
	writeJSON(w, out)
}

// handleRemote (local page only): GET the state; POST {on, new_pin, port}.
func handleRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			On     bool `json:"on"`
			NewPIN bool `json:"new_pin"`
			Port   int  `json:"port"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		s := loadSettings()
		s.RemoteOn = req.On
		if s.RemotePIN == "" || req.NewPIN {
			s.RemotePIN = newPIN()
			remote.mu.Lock()
			remote.tokens = map[string]time.Time{} // a new PIN logs everybody out
			remote.mu.Unlock()
		}
		if req.Port >= 1024 && req.Port <= 65535 {
			s.RemotePort = req.Port
		}
		saveSettings(s)
		if err := remote.apply(); err != nil {
			s.RemoteOn = false
			saveSettings(s)
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
	}
	s := loadSettings()
	port := s.RemotePort
	if port == 0 {
		port = defaultRemotePort
	}
	writeJSON(w, map[string]any{"on": s.RemoteOn, "pin": s.RemotePIN, "port": port, "url": remoteURL(port), "port_s": strconv.Itoa(port)})
}
