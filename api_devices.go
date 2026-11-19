package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/wasabifilm/wasaping/scanner"
)

// Devices, networks (venues), IP plan and settings that live on this computer.

// handleDevice saves the user's name and note for a device, or forgets it.
func handleDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Key      string  `json:"key"`
		Net      string  `json:"net"`
		Name     string  `json:"name"`
		Role     string  `json:"role"`
		Note     string  `json:"note"`
		Critical bool    `json:"critical"`
		Forget   bool    `json:"forget"`
		Kind     *string `json:"kind"` // set only to correct the type ("" = automatic)
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil || req.Key == "" {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 200 || len(req.Role) > 200 || len(req.Note) > 2000 {
		http.Error(w, "testo troppo lungo", http.StatusBadRequest)
		return
	}
	if req.Kind != nil {
		if *req.Kind != "" && !validKinds[*req.Kind] {
			http.Error(w, "tipo sconosciuto", http.StatusBadRequest)
			return
		}
		if !devices().SetKind(req.Net, req.Key, *req.Kind) {
			http.Error(w, "dispositivo sconosciuto", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req.Forget {
		devices().Forget(req.Net, req.Key)
	} else if !devices().Label(req.Net, req.Key, req.Name, req.Role, req.Note, req.Critical) {
		http.Error(w, "dispositivo sconosciuto", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		Name   string `json:"name"`
		Select string `json:"select"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil || len(req.Name) > 200 {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	if req.Select != "" {
		if !devices().Select(req.Select) {
			http.Error(w, "rete sconosciuta", http.StatusNotFound)
			return
		}
		n, _ := devices().Current()
		writeJSON(w, n)
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
	q := r.URL.Query()
	if q.Get("net") != "" {
		devices().Select(q.Get("net"))
	}
	writeJSON(w, comparePlan(devices().Plan(), hostsOf(q.Get("net")), q.Get("lang")))
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

var validKinds = map[string]bool{
	scanner.KindRouter: true, scanner.KindComputer: true, scanner.KindPhone: true, scanner.KindTablet: true,
	scanner.KindPrinter: true, scanner.KindTV: true, scanner.KindSpeaker: true, scanner.KindNAS: true,
	scanner.KindCamera: true, scanner.KindIoT: true, scanner.KindLightDesk: true, scanner.KindLighting: true,
	scanner.KindAudio: true, scanner.KindVideo: true, scanner.KindProjector: true, scanner.KindSwitch: true,
}
