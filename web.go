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
	appMode := fset.Bool("app", false, "modalità app: si chiude quando la pagina viene chiusa")
	fset.Parse(args)
	ctx, quit := context.WithCancel(ctx)
	defer quit()
	appModeOn = *appMode

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
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
		// Block DNS-rebinding: only answer requests addressed to localhost.
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowedHosts[r.Host] && !strings.HasPrefix(*addr, "0.0.0.0") {
				http.Error(w, "host non consentito", http.StatusForbidden)
				return
			}
			mux.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	if *appMode {
		fmt.Printf("🟢 wasaping è aperto nel browser: %s\n   Per uscire usa il pulsante «Esci» nella pagina, o chiudi questa finestra.\n", url)
	} else {
		fmt.Printf("🟢 wasaping è pronto su %s  (Ctrl+C per uscire)\n", url)
	}
	if !*noOpen {
		openBrowser(url)
	}
	if *appMode {
		go watchHeartbeat(ctx, quit)
	}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

var (
	appModeOn  bool
	lastPing   atomic.Int64
	activeScan atomic.Int32
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
	activeScan.Add(1)
	defer activeScan.Add(-1)
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
	start := time.Now()
	hosts, err := scanner.Scan(r.Context(), scanner.Options{
		Targets:    ips,
		Ping:       q.Get("ping") != "0",
		ResolveDNS: q.Get("dns") != "0",
		ScanPorts:  ports,
	}, func(h scanner.Host) {
		send("host", h)
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
	send("done", map[string]any{"count": len(hosts), "seconds": time.Since(start).Seconds()})
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
