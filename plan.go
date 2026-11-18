package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// The IP plan: the patch list a crew prepares (name, IP, role, MAC),
// imported from a spreadsheet and compared with what is on the network.

// PlanItem is one planned device.
type PlanItem struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Role string `json:"role,omitempty"`
	MAC  string `json:"mac,omitempty"`
}

var planHeaders = map[string]string{
	"nome": "name", "name": "name", "dispositivo": "name", "device": "name", "apparecchio": "name",
	"ip": "ip", "indirizzo": "ip", "indirizzo ip": "ip", "ip address": "ip", "address": "ip",
	"ruolo": "role", "role": "role", "funzione": "role", "posizione": "role", "note": "role",
	"mac": "mac", "mac address": "mac", "indirizzo mac": "mac",
}

// parsePlan reads a CSV with "," or ";" (Excel in Italian uses ";"). The
// first row may be a header; without one the columns are name, IP, role, MAC.
func parsePlan(data []byte) ([]PlanItem, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // BOM from Excel
	first, _, _ := bytes.Cut(data, []byte("\n"))
	sep := ','
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		sep = ';'
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma, r.FieldsPerRecord, r.TrimLeadingSpace = sep, -1, true
	cols := map[string]int{"name": 0, "ip": 1, "role": 2, "mac": 3}
	var out []PlanItem
	row := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("CSV non leggibile: " + err.Error())
		}
		row++
		if row == 1 {
			found := map[string]int{}
			for i, h := range rec {
				if k, ok := planHeaders[strings.ToLower(strings.TrimSpace(h))]; ok {
					if _, dup := found[k]; !dup {
						found[k] = i
					}
				}
			}
			if _, ok := found["ip"]; ok {
				cols = found
				continue
			}
		}
		get := func(k string) string {
			if i, ok := cols[k]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		ip := net.ParseIP(get("ip")).To4()
		if ip == nil {
			continue // empty or comment rows
		}
		out = append(out, PlanItem{Name: get("name"), IP: ip.String(), Role: get("role"), MAC: scanner.NormalizeMAC(get("mac"))})
		if len(out) > 4096 {
			return nil, errors.New("piano troppo grande (massimo 4096 righe)")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("nessuna riga con un indirizzo IP valido")
	}
	return out, nil
}

// handleVenue: GET exports the current network, POST imports a venue file.
func handleVenue(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var v Venue
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&v); err != nil {
			http.Error(w, "file non valido", http.StatusBadRequest)
			return
		}
		n, err := devices().ImportVenue(v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]int{"devices": n})
		return
	}
	v, ok := devices().ExportVenue()
	if !ok {
		http.Error(w, "fai prima una scansione di questa rete", http.StatusConflict)
		return
	}
	writeJSON(w, v)
}

// handlePlan: GET the plan of the current network; POST {csv} to import it,
// {clear: true} to remove it.
func handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			CSV   string `json:"csv"`
			Clear bool   `json:"clear"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		var plan []PlanItem
		if !req.Clear {
			var err error
			if plan, err = parsePlan([]byte(req.CSV)); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if !devices().SetPlan(plan) {
			http.Error(w, "fai prima una scansione di questa rete", http.StatusConflict)
			return
		}
	}
	writeJSON(w, devices().Plan())
}

// handleSwitchPorts asks the managed switches (SNMP v2c, read-only
// community) which port each device of the latest scan is on.
func handleSwitchPorts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Community string `json:"community"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
	comm := strings.TrimSpace(req.Community)
	if comm == "" {
		comm = "public"
	}
	if len(comm) > 64 {
		http.Error(w, "community troppo lunga", http.StatusBadRequest)
		return
	}
	s := loadSettings()
	s.SNMPCommunity = comm
	saveSettings(s)
	lastMu.Lock()
	var ips []string
	if lastResult != nil {
		for _, h := range lastResult.Hosts {
			if !h.Self {
				ips = append(ips, h.IP)
			}
		}
	}
	lastMu.Unlock()
	if len(ips) == 0 {
		http.Error(w, "fai prima una scansione", http.StatusConflict)
		return
	}
	activeScan.Add(1)
	defer activeScan.Add(-1)
	switches, ports := scanner.SwitchPorts(r.Context(), ips, comm)
	writeJSON(w, map[string]any{"switches": switches, "ports": ports, "community": comm})
}

// handleRDM lists the RDM fixtures behind the Art-Net nodes of the latest scan.
func handleRDM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	nodes := map[string][]int{}
	lastMu.Lock()
	if lastResult != nil {
		for _, h := range lastResult.Hosts {
			if h.ArtNet == nil {
				continue
			}
			u := append(append([]int{}, h.ArtNet.Out...), h.ArtNet.In...)
			if len(u) == 0 {
				u = []int{0}
			}
			nodes[h.IP] = u
		}
	}
	lastMu.Unlock()
	if len(nodes) == 0 {
		http.Error(w, "nessun nodo Art-Net nell'ultima scansione", http.StatusConflict)
		return
	}
	activeScan.Add(1)
	defer activeScan.Add(-1)
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	writeJSON(w, map[string]any{"nodes": len(nodes), "fixtures": scanner.RDMDiscover(ctx, nodes)})
}
