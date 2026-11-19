package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// RDM writes: start address, mode (personality) and label. Each change is
// read back from the fixture and written to rdm-changes.csv, so there is a
// record of who changed what during the night.

var rdmLogMu sync.Mutex

func rdmLogPath() string { return filepath.Join(filepath.Dir(monitorLogPath()), "rdm-changes.csv") }

func appendRDMLog(f scanner.Fixture, field, before, after string) {
	rdmLogMu.Lock()
	defer rdmLogMu.Unlock()
	p := rdmLogPath()
	os.MkdirAll(filepath.Dir(p), 0o700)
	_, err := os.Stat(p)
	head := os.IsNotExist(err)
	fh, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer fh.Close()
	w := csv.NewWriter(fh)
	if head {
		w.Write([]string{"data", "ora", "uid", "etichetta", "nodo", "universo", "campo", "prima", "dopo"})
	}
	now := time.Now()
	w.Write([]string{now.Format("2006-01-02"), now.Format("15:04:05"), f.UID, f.Label, f.Node, strconv.Itoa(f.Universe), field, before, after})
	w.Flush()
}

func rdmField(f scanner.Fixture, field string) string {
	switch field {
	case "address":
		return strconv.Itoa(f.Address)
	case "mode":
		p, _, _ := strings.Cut(f.Personality, "/")
		return p
	case "label":
		return f.Label
	}
	return ""
}

// rdmChange reads the fixture, changes one setting, reads it back and logs it.
func rdmChange(ctx context.Context, node string, universe int, uid, field, value string) (scanner.Fixture, error) {
	before, err := scanner.RDMInfo(ctx, node, universe, uid)
	if err != nil {
		return before, err
	}
	if err := scanner.RDMSet(ctx, node, universe, uid, field, value); err != nil {
		return before, err
	}
	time.Sleep(200 * time.Millisecond) // some fixtures apply the change a moment later
	after, err := scanner.RDMInfo(ctx, node, universe, uid)
	if err != nil {
		return before, fmt.Errorf("modifica inviata, ma l'apparecchio non risponde più: controllalo")
	}
	appendRDMLog(after, field, rdmField(before, field), rdmField(after, field))
	if got := rdmField(after, field); got != strings.TrimSpace(value) {
		return after, fmt.Errorf("l'apparecchio ha confermato, ma ora riporta %q invece di %q", got, value)
	}
	return after, nil
}

// handleRDMSet: POST {node, universe, uid, field, value}. The page asks for
// confirmation twice before calling it.
func handleRDMSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Node     string `json:"node"`
		Universe int    `json:"universe"`
		UID      string `json:"uid"`
		Field    string `json:"field"`
		Value    string `json:"value"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil ||
		net.ParseIP(req.Node).To4() == nil || !reRDMUID.MatchString(req.UID) || req.Universe < 0 || req.Universe > 32767 ||
		(req.Field != "address" && req.Field != "mode" && req.Field != "label") {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	f, err := rdmChange(ctx, req.Node, req.Universe, strings.ToUpper(req.UID), req.Field, strings.TrimSpace(req.Value))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, f)
}
