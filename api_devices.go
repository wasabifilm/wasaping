package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

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
		Kind     *string `json:"kind"`  // set only to correct the type ("" = automatic)
		Group    *string `json:"group"` // set only to change the department ("" = from the type)
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil || req.Key == "" {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 200 || len(req.Role) > 200 || len(req.Note) > 2000 {
		http.Error(w, "testo troppo lungo", http.StatusBadRequest)
		return
	}
	if req.Group != nil {
		if *req.Group != "" && !validGroup(*req.Group) {
			http.Error(w, "reparto sconosciuto", http.StatusBadRequest)
			return
		}
		if !devices().SetGroup(req.Net, req.Key, *req.Group) {
			http.Error(w, "dispositivo sconosciuto", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
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

// handleForgetOld cleans a network's list: POST {net, days, named, dry}
// (dry: only count, for the page to show the numbers).
func handleForgetOld(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Net   string `json:"net"`
		Days  int    `json:"days"`
		Named bool   `json:"named"`
		Dry   bool   `json:"dry"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil || req.Days < 0 || req.Days > 3650 {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"removed": devices().ForgetOld(req.Net, req.Days, req.Named, req.Dry)})
}

// handleProject: GET ?nets=id1,id2&name=… saves the show's project (the
// networks given, or the current one); POST {project, target} opens a
// project or an older venue file: target "own" puts each network in its
// own place (created if new), "" merges a single venue into the current one.
func handleProject(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Project json.RawMessage `json:"project"`
			Target  string          `json:"target"`
			Notes   *string         `json:"notes"`
			Net     string          `json:"net"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&req); err != nil {
			http.Error(w, "file non valido", http.StatusBadRequest)
			return
		}
		if req.Notes != nil { // just the show's notes
			if len(*req.Notes) > 20000 || !devices().SetNotes(req.Net, *req.Notes) {
				http.Error(w, "note non salvate", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		venues, name, err := parseProject(req.Project)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		target := ""
		if req.Target == "own" {
			target = "own"
		}
		total, nets := 0, []string{}
		for _, v := range venues {
			n, id, err := devices().ImportVenueInto(v, target)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			total += n
			nets = append(nets, id)
		}
		writeJSON(w, map[string]any{"devices": total, "networks": nets, "name": name})
		return
	}
	q := r.URL.Query()
	p := Project{Format: "wasaping-project", Version: 1, Name: strings.TrimSpace(q.Get("name")), SavedAt: time.Now(), App: "wasaping " + version}
	ids := strings.Split(q.Get("nets"), ",")
	for _, id := range ids {
		if v, ok := devices().ExportNetwork(strings.TrimSpace(id)); ok && !slices.ContainsFunc(p.Venues, func(x Venue) bool { return x.ID == v.ID }) {
			p.Venues = append(p.Venues, v)
		}
	}
	if len(p.Venues) == 0 {
		http.Error(w, "fai prima una scansione di questa rete", http.StatusConflict)
		return
	}
	if p.Name == "" {
		p.Name = firstNonEmpty(p.Venues[0].Name, p.Venues[0].Auto)
	}
	p.Notes = p.Venues[0].Notes
	writeJSON(w, p)
}

// parseProject reads a project file, or a venue file of older versions.
func parseProject(b []byte) ([]Venue, string, error) {
	var head struct {
		Format string `json:"format"`
	}
	if json.Unmarshal(b, &head) != nil {
		return nil, "", fmt.Errorf("file non valido")
	}
	switch head.Format {
	case "wasaping-project":
		var p Project
		if err := json.Unmarshal(b, &p); err != nil || len(p.Venues) == 0 || len(p.Venues) > 32 {
			return nil, "", fmt.Errorf("progetto non valido")
		}
		return p.Venues, p.Name, nil
	case "wasaping-venue":
		var v Venue
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, "", fmt.Errorf("file non valido")
		}
		return []Venue{v}, v.Name, nil
	}
	return nil, "", fmt.Errorf("non è un progetto di wasaping")
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
			CSV   string      `json:"csv"`
			Items *[]PlanItem `json:"items"` // from the editor in the page
			Clear bool        `json:"clear"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		var plan []PlanItem
		if req.Items != nil {
			var err error
			if plan, err = checkPlanItems(*req.Items); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		} else if !req.Clear {
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

func validKind(k string) bool { return validKinds[k] }

var validKinds = map[string]bool{
	scanner.KindRouter: true, scanner.KindComputer: true, scanner.KindPhone: true, scanner.KindTablet: true,
	scanner.KindPrinter: true, scanner.KindTV: true, scanner.KindSpeaker: true, scanner.KindNAS: true,
	scanner.KindCamera: true, scanner.KindIoT: true, scanner.KindLightDesk: true, scanner.KindLighting: true,
	scanner.KindAudio: true, scanner.KindVideo: true, scanner.KindProjector: true, scanner.KindSwitch: true,
}

// Errors of the page (it has no console in the app window): kept for the
// diagnostic file and printed on the terminal.
var (
	clientLogMu sync.Mutex
	clientLog   []string
)

func handleClientLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		clientLogMu.Lock()
		defer clientLogMu.Unlock()
		writeJSON(w, clientLog)
		return
	}
	var req struct {
		Msg   string `json:"msg"`
		Where string `json:"where"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req) != nil {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	line := time.Now().Format("15:04:05") + " " + req.Msg + " @ " + req.Where
	fmt.Fprintln(os.Stderr, "pagina:", line)
	clientLogMu.Lock()
	clientLog = append(clientLog, line)
	if len(clientLog) > 100 {
		clientLog = clientLog[len(clientLog)-100:]
	}
	clientLogMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
