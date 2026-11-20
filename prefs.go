package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Preferences of the page (Settings → Scansione, Sistema), the data folder,
// export and import of the settings, and QR codes of the access addresses.

type prefsView struct {
	SNMPCommunity string       `json:"snmp_community"`
	DefaultPorts  string       `json:"default_ports"`
	LoopPause     int          `json:"loop_pause"`
	AlarmVolume   int          `json:"alarm_volume"`
	SavedRanges   []SavedRange `json:"saved_ranges"`
	TextSize      int          `json:"text_size"`
	Colorblind    bool         `json:"colorblind"`
	PublicIP      bool         `json:"public_ip"`
	DataDir       string       `json:"data_dir"`
	Files         []dataFile   `json:"files"`
}

type dataFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func prefsOf(s settings) prefsView {
	v := prefsView{SNMPCommunity: firstNonEmpty(s.SNMPCommunity, "public"), DefaultPorts: firstNonEmpty(s.DefaultPorts, "common"),
		LoopPause: s.LoopPause, AlarmVolume: s.AlarmVolume, DataDir: dataDir(),
		SavedRanges: s.SavedRanges, TextSize: firstPositive(s.TextSize, 100), Colorblind: s.Colorblind, PublicIP: s.PublicIP}
	if v.SavedRanges == nil {
		v.SavedRanges = []SavedRange{}
	}
	if v.LoopPause == 0 {
		v.LoopPause = 10
	}
	if v.AlarmVolume == 0 {
		v.AlarmVolume = 60
	}
	if ents, err := os.ReadDir(v.DataDir); err == nil {
		for _, e := range ents {
			if i, err := e.Info(); err == nil && !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				v.Files = append(v.Files, dataFile{e.Name(), i.Size()})
			}
		}
	}
	return v
}

func dataDir() string { return filepath.Dir(settingsPath()) }

// validPorts accepts what the port menu can say.
func validPorts(p string) bool {
	if p == "common" || p == "none" {
		return true
	}
	_, err := parsePorts(p)
	return err == nil && len(p) <= 200
}

// handlePrefs: GET the preferences; POST changes only the fields given.
func handlePrefs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			SNMPCommunity *string       `json:"snmp_community"`
			DefaultPorts  *string       `json:"default_ports"`
			LoopPause     *int          `json:"loop_pause"`
			AlarmVolume   *int          `json:"alarm_volume"`
			SavedRanges   *[]SavedRange `json:"saved_ranges"`
			TextSize      *int          `json:"text_size"`
			Colorblind    *bool         `json:"colorblind"`
			PublicIP      *bool         `json:"public_ip"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		s := loadSettings()
		if req.SNMPCommunity != nil {
			c := strings.TrimSpace(*req.SNMPCommunity)
			if len(c) > 64 {
				http.Error(w, T("community troppo lunga", "community too long"), http.StatusBadRequest)
				return
			}
			s.SNMPCommunity = c
		}
		if req.DefaultPorts != nil {
			p := strings.TrimSpace(*req.DefaultPorts)
			if !validPorts(p) {
				http.Error(w, T("porte non valide: es. 22,80,443 o 1-1024", "invalid ports: e.g. 22,80,443 or 1-1024"), http.StatusBadRequest)
				return
			}
			s.DefaultPorts = p
		}
		if req.LoopPause != nil {
			if *req.LoopPause < 3 || *req.LoopPause > 3600 {
				http.Error(w, "pausa non valida", http.StatusBadRequest)
				return
			}
			s.LoopPause = *req.LoopPause
		}
		if req.AlarmVolume != nil {
			s.AlarmVolume = min(max(*req.AlarmVolume, 10), 100)
		}
		if req.SavedRanges != nil {
			sr, err := cleanRanges(*req.SavedRanges)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.SavedRanges = sr
		}
		if req.TextSize != nil {
			s.TextSize = min(max(*req.TextSize, 70), 175)
		}
		if req.Colorblind != nil {
			s.Colorblind = *req.Colorblind
		}
		if req.PublicIP != nil {
			s.PublicIP = *req.PublicIP
			publicIPReset()
		}
		saveSettings(s)
	}
	writeJSON(w, prefsOf(loadSettings()))
}

// handleReveal opens the data folder in the file manager (this computer only).
func handleReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	dir := dataDir()
	os.MkdirAll(dir, 0o755)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	go cmd.Wait()
	w.WriteHeader(http.StatusNoContent)
}

// settingsFile is what travels to another computer: no PIN, no password,
// nothing about this machine's state.
type settingsFile struct {
	Format        string       `json:"format"` // "wasaping-settings"
	Version       int          `json:"version"`
	Exported      string       `json:"exported,omitempty"`
	Repeat        int          `json:"repeat_minutes"`
	MonitorEvery  int          `json:"monitor_every,omitempty"`
	SNMPCommunity string       `json:"snmp_community,omitempty"`
	NoUpdateCheck bool         `json:"no_update_check,omitempty"`
	DefaultPorts  string       `json:"default_ports,omitempty"`
	LoopPause     int          `json:"loop_pause,omitempty"`
	AlarmVolume   int          `json:"alarm_volume,omitempty"`
	SavedRanges   []SavedRange `json:"saved_ranges,omitempty"`
	TextSize      int          `json:"text_size,omitempty"`
	Colorblind    bool         `json:"colorblind,omitempty"`
	PublicIP      bool         `json:"public_ip,omitempty"`
	alertCfg
}

func handleSettingsExport(w http.ResponseWriter, r *http.Request) {
	s := loadSettings()
	writeJSON(w, settingsFile{Format: "wasaping-settings", Version: 1, Exported: time.Now().Format(time.RFC3339),
		Repeat: s.Repeat, MonitorEvery: s.MonitorEvery, SNMPCommunity: s.SNMPCommunity, NoUpdateCheck: s.NoUpdateCheck,
		DefaultPorts: s.DefaultPorts, LoopPause: s.LoopPause, AlarmVolume: s.AlarmVolume, alertCfg: alertsOf(s),
		SavedRanges: s.SavedRanges, TextSize: s.TextSize, Colorblind: s.Colorblind, PublicIP: s.PublicIP})
}

func handleSettingsImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var f settingsFile
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&f); err != nil || f.Format != "wasaping-settings" {
		http.Error(w, T("non è un file di impostazioni di wasaping", "not a wasaping settings file"), http.StatusBadRequest)
		return
	}
	a, err := cleanAlerts(f.alertCfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if f.DefaultPorts != "" && !validPorts(f.DefaultPorts) {
		f.DefaultPorts = ""
	}
	s := loadSettings()
	s.Repeat = min(max(f.Repeat, 0), 24*60)
	if f.MonitorEvery >= 1 && f.MonitorEvery <= 300 {
		s.MonitorEvery = f.MonitorEvery
	}
	if len(f.SNMPCommunity) <= 64 {
		s.SNMPCommunity = strings.TrimSpace(f.SNMPCommunity)
	}
	s.NoUpdateCheck, s.DefaultPorts = f.NoUpdateCheck, f.DefaultPorts
	if f.LoopPause == 0 || (f.LoopPause >= 3 && f.LoopPause <= 3600) {
		s.LoopPause = f.LoopPause
	}
	if f.AlarmVolume == 0 || (f.AlarmVolume >= 10 && f.AlarmVolume <= 100) {
		s.AlarmVolume = f.AlarmVolume
	}
	s.AlertOSC, s.AlertOSCAddr, s.AlertWebhook, s.Telegram, s.Ntfy, s.Slack = a.OSC, a.OSCAddr, a.Webhook, a.Telegram, a.Ntfy, a.Slack
	if sr, err := cleanRanges(f.SavedRanges); err == nil {
		s.SavedRanges = sr
	}
	if f.TextSize == 0 || (f.TextSize >= 70 && f.TextSize <= 175) {
		s.TextSize = f.TextSize
	}
	s.Colorblind, s.PublicIP = f.Colorblind, f.PublicIP
	saveSettings(s)
	writeJSON(w, prefsOf(s))
}

// handleQR draws the address of the iPad page (for=remote) or of the full
// web access (for=web) as a QR code to frame with the device.
func handleQR(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost { // labels: {texts: [...]} -> {svgs: [...]}
		var req struct {
			Texts []string `json:"texts"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil || len(req.Texts) > 1000 {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		out := make([]string, len(req.Texts))
		for i, t := range req.Texts {
			out[i], _ = qrSVG(t) // "" when too long
		}
		writeJSON(w, map[string]any{"svgs": out})
		return
	}
	s := loadSettings()
	var url string
	switch r.URL.Query().Get("for") {
	case "remote":
		url = remoteURL(firstPositive(s.RemotePort, defaultRemotePort))
	case "web":
		url = webAccessURL(firstPositive(s.WebPort, defaultWebPort))
	default:
		http.Error(w, "for=remote o for=web", http.StatusBadRequest)
		return
	}
	svg, err := qrSVG(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(svg))
}

func firstPositive(v ...int) int {
	for _, x := range v {
		if x > 0 {
			return x
		}
	}
	return 0
}

// SavedRange is a scan target with a name, e.g. "Palco" = 10.0.0.1-50.
type SavedRange struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

// cleanRanges checks saved ranges: at most 50, a name, a valid target.
func cleanRanges(in []SavedRange) ([]SavedRange, error) {
	if len(in) > 50 {
		return nil, fmt.Errorf(T("troppi intervalli salvati (massimo 50)", "too many saved ranges (50 at most)"))
	}
	var out []SavedRange
	seen := map[string]bool{}
	for _, r := range in {
		r.Name, r.Target = strings.TrimSpace(r.Name), strings.TrimSpace(r.Target)
		if r.Name == "" || len([]rune(r.Name)) > 40 {
			return nil, fmt.Errorf(T("nome dell'intervallo non valido (1–40 caratteri)", "invalid range name (1–40 characters)"))
		}
		if _, err := scanner.ParseTargets(r.Target); err != nil {
			return nil, fmt.Errorf("%s: %v", r.Name, err)
		}
		if seen[r.Target] {
			continue
		}
		seen[r.Target] = true
		out = append(out, r)
	}
	return out, nil
}
