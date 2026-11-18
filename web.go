package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

//go:embed web
var webFS embed.FS

func runWeb(ctx context.Context, args []string) error {
	fset := flag.NewFlagSet("web", flag.ExitOnError)
	addr := fset.String("addr", "127.0.0.1:0", "indirizzo di ascolto")
	noOpen := fset.Bool("no-open", false, "non aprire il browser automaticamente")
	appMode := fset.Bool("app", false, "modalità app: finestra propria, si chiude con la finestra")
	hidden := fset.Bool("hidden", false, "macOS: parte solo nella barra dei menu, senza finestra (apertura al login)")
	fset.Parse(args)
	ctx, quit := context.WithCancel(ctx)
	defer quit()
	appModeOn = *appMode

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	host, _, _ := net.SplitHostPort(*addr)
	public := !isLoopbackHost(host)
	allowedHosts := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("[::1]:%d", port):     true,
	}

	static, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/api/info", handleInfo)
	mux.HandleFunc("/api/scan", handleScan)
	mux.HandleFunc("/api/update-oui", handleUpdateOUI)
	mux.HandleFunc("/api/save", handleSave)
	mux.HandleFunc("/api/device", handleDevice)
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
	mux.HandleFunc("/api/last", handleLast)
	mux.HandleFunc("/api/trace", handleTrace)
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

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	if public {
		fmt.Fprintf(os.Stderr, "⚠️  Attenzione: wasaping è raggiungibile da altri computer su %s, senza password.\n"+
			"   Chiunque sulla rete può usarlo per scansionarla. Usa -addr 127.0.0.1:0 per tenerlo privato.\n", ln.Addr())
	}

	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()

	// macOS: switch on the ARP helper if it is installed. Installing it
	// (password) is the user's choice, from the page.
	helperState.Store(setupARP(false))
	go backgroundLoop(ctx)
	switch {
	case *appMode && hasNativeWindow:
		uiMode.Store("native")
		// Ctrl+C or /api/quit while the window is open: leave right away.
		go func() { <-ctx.Done(); os.Exit(0) }()
		nativeWindow(url, *hidden) // blocks on the main thread until the user quits
		quit()
	case *appMode:
		if cmd := startAppWindow(url); cmd != nil {
			uiMode.Store("window")
			fmt.Printf("🟢 wasaping è aperto in una finestra: %s\n   Per uscire chiudi la finestra.\n", url)
			go func() {
				began := time.Now()
				cmd.Wait()
				if time.Since(began) > 5*time.Second {
					quit() // the window was ours and it is closed
				} else {
					go watchHeartbeat(ctx, quit) // handed over to a running browser
				}
			}()
		} else {
			uiMode.Store("browser")
			fmt.Printf("🟢 wasaping è aperto nel browser: %s\n   Per uscire usa il pulsante «Esci» nella pagina, o chiudi questa finestra.\n", url)
			openBrowser(url)
			go watchHeartbeat(ctx, quit)
		}
	default:
		uiMode.Store("browser")
		fmt.Printf("🟢 wasaping è pronto su %s  (Ctrl+C per uscire)\n", url)
		if !*noOpen {
			openBrowser(url)
		}
	}
	if err := <-served; err != http.ErrServerClosed {
		return err
	}
	return nil
}

// localOnly are the endpoints that change this computer, not just look at
// the network.
var localOnly = map[string]bool{
	"/api/save": true, "/api/helper": true, "/api/quit": true, "/api/notify": true,
	"/api/device": true, "/api/network": true, "/api/bg": true, "/api/update-oui": true, "/api/login": true,
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

var (
	appModeOn   bool
	uiMode      atomic.Value // "native", "window" or "browser"
	helperState atomic.Value // "notneeded", "installed", "missing"
	lastPing    atomic.Int64
	activeScan  atomic.Int32
)

// watchHeartbeat stops the app when the page has been gone for a while.
// Browsers slow down timers in background tabs (down to once a minute),
// so the grace period is generous.
func watchHeartbeat(ctx context.Context, quit context.CancelFunc) {
	started := time.Now()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if activeScan.Load() > 0 {
			continue
		}
		last := lastPing.Load()
		if last == 0 {
			if time.Since(started) > 3*time.Minute {
				fmt.Println("Nessuna pagina aperta: chiudo wasaping.")
				quit()
			}
			continue
		}
		if time.Since(time.Unix(0, last)) > 150*time.Second {
			fmt.Println("Pagina chiusa: chiudo wasaping.")
			quit()
		}
	}
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	ifs, _ := scanner.LocalInterfaces()
	def := ""
	if i, err := scanner.DefaultInterface(); err == nil {
		def = clampNetwork(i)
	}
	for k := range ifs {
		ifs[k].Network = clampNetwork(ifs[k])
	}
	writeJSON(w, map[string]any{
		"version":       version,
		"interfaces":    ifs,
		"defaultTarget": def,
		"hasOUI":        scanner.HasOUI(),
		"os":            runtime.GOOS,
		"app":           appModeOn,
		"ui":            uiMode.Load(),
		"helper":        helperState.Load(),
	})
}

// handleScan streams results as Server-Sent Events.
func handleScan(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ips, err := scanner.ParseTargets(q.Get("target"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
	res, err := runCollect(r.Context(), q.Get("target"), ips, scanner.Options{
		Ping:       q.Get("ping") != "0",
		ResolveDNS: q.Get("dns") != "0",
		ScanPorts:  ports,
	}, false, func(v HostView) {
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

// handleDevice saves the user's name and note for a device, or forgets it.
func handleDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Key      string `json:"key"`
		Name     string `json:"name"`
		Role     string `json:"role"`
		Note     string `json:"note"`
		Critical bool   `json:"critical"`
		Forget   bool   `json:"forget"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil || req.Key == "" {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 200 || len(req.Role) > 200 || len(req.Note) > 2000 {
		http.Error(w, "testo troppo lungo", http.StatusBadRequest)
		return
	}
	if req.Forget {
		devices().Forget(req.Key)
	} else if !devices().Label(req.Key, req.Name, req.Role, req.Note, req.Critical) {
		http.Error(w, "dispositivo sconosciuto", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// handleHelper installs or removes the macOS ARP helper (password dialog).
func handleHelper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var err error
	switch r.URL.Query().Get("do") {
	case "install":
		if err = installHelper(); err == nil {
			helperState.Store(setupARP(false))
		}
	case "uninstall":
		if err = uninstallHelper(); err == nil {
			helperState.Store("missing")
		}
	default:
		err = fmt.Errorf("azione sconosciuta")
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"helper": helperState.Load()})
}

// handleNetwork renames the current network ("Arena Verona").
func handleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		if n, ok := devices().Current(); ok {
			writeJSON(w, n)
		} else {
			writeJSON(w, nil)
		}
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil || len(req.Name) > 200 {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	if !devices().RenameNetwork(req.Name) {
		http.Error(w, "fai prima una scansione", http.StatusConflict)
		return
	}
	n, _ := devices().Current()
	writeJSON(w, n)
}

// handleCritical lists the devices marked critical on the current network.
func handleCritical(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, devices().CriticalDevices())
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

// handleNotify shows a system notification (monitor alarms).
func handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	notify(firstNonEmpty(req.Title, "wasaping"), req.Body)
	w.WriteHeader(http.StatusNoContent)
}

// handleLogin reads or sets "Apri al login" (macOS).
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		if err := setLogin(req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
	}
	writeJSON(w, map[string]bool{"available": loginAvailable(), "enabled": loginEnabled()})
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Apri manualmente:", url)
	}
}
