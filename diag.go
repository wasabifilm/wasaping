package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Diagnostic bundle: a zip with what wasaping heard on the network during a
// fresh scan, so that gear it does not recognize (or names wrong) can be
// fixed from real answers. Private details are left out or masked.

type diagPacket struct {
	T    time.Time `json:"t"`
	Kind string    `json:"kind"`
	From string    `json:"from"`
	Len  int       `json:"len"`
	Data string    `json:"b64"`
}

// buildDiag scans the local network while recording, and returns the zip.
func buildDiag(ctx context.Context) ([]byte, error) {
	var mu sync.Mutex
	var packets []diagPacket
	total := 0
	scanner.SetDiag(func(kind, from string, data []byte) {
		mu.Lock()
		defer mu.Unlock()
		if len(packets) >= 5000 || total > 8<<20 {
			return
		}
		total += len(data)
		packets = append(packets, diagPacket{time.Now(), kind, from, len(data), base64.StdEncoding.EncodeToString(data)})
	})
	defer scanner.SetDiag(nil)

	target := ""
	if n, ok := devices().Current(); ok && n.Subnet != "" {
		target = n.Subnet
	} else if i, err := scanner.DefaultInterface(); err == nil {
		target = clampNetwork(i)
	}
	ips, err := scanner.ParseTargets(target)
	if err != nil {
		return nil, err
	}
	res, err := runCollect(ctx, target, ips, scanner.Options{Ping: true, ResolveDNS: true, ScanPorts: scanner.CommonPorts}, false, nil, nil)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(name string, v any) {
		w, _ := z.Create(name)
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		e.Encode(v)
	}
	ifs, _ := scanner.LocalInterfaces()
	add("info.json", map[string]any{
		"version": version, "os": runtime.GOOS, "arch": runtime.GOARCH, "go": runtime.Version(), "at": time.Now(),
		"target": target, "interfaces": ifs, "gateway": scanner.DefaultGateway(), "helper": helperState.Load(),
		"ui": uiMode.Load(), "lang": appLang(), "mac_hidden": res.MacHidden,
	})
	// The scan without what the user wrote (names, roles, notes).
	hosts := make([]HostView, len(res.Hosts))
	for i, h := range res.Hosts {
		h.Name, h.Role, h.Note = "", "", ""
		hosts[i] = h
	}
	add("scan.json", map[string]any{"count": res.Count, "seconds": res.Seconds, "issues": res.Issues, "hosts": hosts})
	// Types corrected by hand: what wasaping guessed and what it really is.
	var fixes []map[string]any
	for _, h := range res.Hosts {
		if h.AutoKind != "" {
			fixes = append(fixes, map[string]any{"ip": h.IP, "mac": h.MAC, "vendor": h.Vendor, "model": h.Model, "friendly": h.Friendly,
				"services": h.Services, "ports": h.Ports, "protocols": h.Protocols, "guessed": h.AutoKind, "really": h.Kind})
		}
	}
	add("corrections.json", fixes)
	mu.Lock()
	pw, _ := z.Create("packets.jsonl")
	for _, p := range packets {
		b, _ := json.Marshal(p)
		pw.Write(append(b, '\n'))
	}
	mu.Unlock()
	s := loadSettings()
	mask := func(v string) string {
		if v == "" {
			return ""
		}
		return "•••"
	}
	s.RemotePIN, s.AlertWebhook, s.AlertOSC, s.Telegram, s.SNMPCommunity, s.Ntfy, s.Slack = mask(s.RemotePIN), mask(s.AlertWebhook), mask(s.AlertOSC), mask(s.Telegram), mask(s.SNMPCommunity), mask(s.Ntfy), mask(s.Slack)
	s.WebPasswordHash, s.WebPasswordSalt = mask(s.WebPasswordHash), ""
	add("settings.json", s)
	if f, err := os.Open(monitorLogPath()); err == nil {
		var lines []string
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		f.Close()
		if len(lines) > 500 {
			lines = lines[len(lines)-500:]
		}
		w, _ := z.Create("monitor-log.csv")
		w.Write([]byte(strings.Join(lines, "\n") + "\n"))
	}
	w, _ := z.Create("LEGGIMI.txt")
	fmt.Fprintf(w, "Diagnostica di wasaping %s, %s.\nContiene la scansione della rete, le risposte ricevute dai dispositivi (packets.jsonl) e le correzioni fatte a mano.\nNon contiene i nomi, i ruoli e le note che hai scritto, né PIN, password o indirizzi degli allarmi.\n", version, time.Now().Format("2006-01-02 15:04"))
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// handleDiag builds the bundle and saves it in Downloads.
func handleDiag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	activeScan.Add(1)
	defer activeScan.Add(-1)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	b, err := buildDiag(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	path := filepath.Join(downloadsDir(), "wasaping-diagnostica-"+time.Now().Format("2006-01-02-1504")+".zip")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	revealFile(path)
	writeJSON(w, map[string]any{"path": path, "size": len(b)})
}

func runDiagCLI(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, TC("🟢 wasaping: scansione con registrazione delle risposte…", "🟢 wasaping: scanning and recording the answers…"))
	b, err := buildDiag(ctx)
	if err != nil {
		return err
	}
	path := "wasaping-diagnostica-" + time.Now().Format("2006-01-02-1504") + ".zip"
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	fmt.Printf(TC("✓ %s (%d KB): mandalo a chi sviluppa wasaping.\n", "✓ %s (%d KB): send it to wasaping's developer.\n"), path, len(b)/1024)
	return nil
}
