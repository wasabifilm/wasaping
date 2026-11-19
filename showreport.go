package main

import (
	"encoding/csv"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// End-of-show report: what happened during the night, from the monitor
// log (monitor-log.csv): drops per device, total and longest downtime, the
// timeline of events.

// LogEvent is one line of the monitor log.
type LogEvent struct {
	At      time.Time `json:"at"`
	Network string    `json:"network,omitempty"`
	Name    string    `json:"name"`
	IP      string    `json:"ip,omitempty"`
	Event   string    `json:"event"` // as written in the log
	Down    bool      `json:"down"`  // a drop (not a return or a warning)
	Up      bool      `json:"up"`    // a return
	DownFor float64   `json:"down_for,omitempty"`
}

// DeviceSummary adds up one device's night.
type DeviceSummary struct {
	Name      string  `json:"name"`
	IP        string  `json:"ip,omitempty"`
	Drops     int     `json:"drops"`
	DownTotal float64 `json:"down_total"` // seconds
	Longest   float64 `json:"longest"`    // seconds
	StillDown bool    `json:"still_down,omitempty"`
}

// ShowReport is the summary from a moment on.
type ShowReport struct {
	Since   time.Time       `json:"since"`
	Until   time.Time       `json:"until"`
	Devices []DeviceSummary `json:"devices"`
	Events  []LogEvent      `json:"events"`
	Drops   int             `json:"drops"`
}

func monitorLogPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "wasaping", "monitor-log.csv")
}

// readMonitorLog reads the events at or after since.
func readMonitorLog(since time.Time) ([]LogEvent, error) {
	f, err := os.Open(monitorLogPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	var out []LogEvent
	for i, rec := range rows {
		if i == 0 || len(rec) < 7 {
			continue // header
		}
		at, err := time.ParseInLocation("2006-01-02 15:04:05", rec[0]+" "+rec[1], time.Local)
		if err != nil || at.Before(since) {
			continue
		}
		e := LogEvent{At: at, Network: rec[2], Name: rec[3], IP: rec[4], Event: rec[5]}
		switch rec[5] {
		case "non risponde":
			e.Down = true
		case "di nuovo in linea":
			e.Up = true
			e.DownFor, _ = strconv.ParseFloat(rec[6], 64)
		default:
			if strings.HasPrefix(rec[5], "nessun master") {
				e.Down = true
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// buildShowReport sums up the log from since to now.
func buildShowReport(since time.Time) (ShowReport, error) {
	ev, err := readMonitorLog(since)
	rep := ShowReport{Since: since, Until: time.Now(), Events: ev}
	if err != nil {
		return rep, err
	}
	by := map[string]*DeviceSummary{}
	lastDown := map[string]time.Time{}
	for _, e := range ev {
		d := by[e.Name]
		if d == nil {
			d = &DeviceSummary{Name: e.Name, IP: e.IP}
			by[e.Name] = d
		}
		switch {
		case e.Down:
			d.Drops++
			rep.Drops++
			d.StillDown = true
			lastDown[e.Name] = e.At
		case e.Up:
			d.StillDown = false
			d.DownTotal += e.DownFor
			d.Longest = max(d.Longest, e.DownFor)
		}
	}
	for name, d := range by {
		if d.StillDown { // dropped and never came back: count until now
			s := rep.Until.Sub(lastDown[name]).Seconds()
			d.DownTotal += s
			d.Longest = max(d.Longest, s)
		}
		if d.Drops > 0 {
			rep.Devices = append(rep.Devices, *d)
		}
	}
	sort.Slice(rep.Devices, func(i, j int) bool {
		if rep.Devices[i].DownTotal != rep.Devices[j].DownTotal {
			return rep.Devices[i].DownTotal > rep.Devices[j].DownTotal
		}
		return rep.Devices[i].Name < rep.Devices[j].Name
	})
	return rep, nil
}

// parseSince accepts "18:00" (today, or yesterday if that is in the
// future), a date-time "2006-01-02 15:04", or a duration like "6h".
func parseSince(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return now.Add(-12 * time.Hour), true
	}
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return now.Add(-d), true
	}
	if t, err := time.ParseInLocation("15:04", s, time.Local); err == nil {
		at := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		if at.After(now) {
			at = at.AddDate(0, 0, -1)
		}
		return at, true
	}
	for _, f := range []string{"2006-01-02 15:04", "2006-01-02T15:04", time.RFC3339} {
		if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// handleShowReport: GET ?since=18:00
func handleShowReport(w http.ResponseWriter, r *http.Request) {
	since, ok := parseSince(r.URL.Query().Get("since"), time.Now())
	if !ok {
		http.Error(w, T("ora non valida: usa 18:00, 6h o 2026-11-18 18:00", "invalid time: use 18:00, 6h or 2026-11-18 18:00"), http.StatusBadRequest)
		return
	}
	rep, err := buildShowReport(since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, rep)
}
