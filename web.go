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
	"time"

	"github.com/USERNAME/wasaping/scanner"
)

//go:embed web
var webFS embed.FS

func runWeb(ctx context.Context, args []string) error {
	fset := flag.NewFlagSet("web", flag.ExitOnError)
	addr := fset.String("addr", "127.0.0.1:0", "indirizzo di ascolto")
	noOpen := fset.Bool("no-open", false, "non aprire il browser automaticamente")
	fset.Parse(args)

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
	fmt.Printf("🟢 wasaping è pronto su %s  (Ctrl+C per uscire)\n", url)
	if !*noOpen {
		openBrowser(url)
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
