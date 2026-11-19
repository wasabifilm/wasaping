package main

import (
	"encoding/csv"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// RTTSeries is a device's response times over the period of the report.
type RTTSeries struct {
	Name   string     `json:"name"`
	IP     string     `json:"ip,omitempty"`
	Points [][2]int64 `json:"points"` // unix seconds, ms (-1 = no answer)
	Avg    float64    `json:"avg"`
	Max    int64      `json:"max"`
	Loss   float64    `json:"loss"` // percent of checks without answer
}

type rttSample struct {
	at   time.Time
	name string
	ip   string
	rtt  int64
}

var rttMu sync.Mutex

func rttLogPath() string { return filepath.Join(filepath.Dir(monitorLogPath()), "rtt-log.csv") }

// appendRTTLog keeps every check of the critical devices, for the report.
func appendRTTLog(s []rttSample) {
	if len(s) == 0 {
		return
	}
	rttMu.Lock()
	defer rttMu.Unlock()
	p := rttLogPath()
	os.MkdirAll(filepath.Dir(p), 0o700)
	if st, err := os.Stat(p); err == nil && st.Size() > 20<<20 {
		os.Rename(p, p+".old") // keep the size bounded: one old file is enough
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	for _, x := range s {
		w.Write([]string{strconv.FormatInt(x.at.Unix(), 10), x.name, x.ip, strconv.FormatInt(x.rtt, 10)})
	}
	w.Flush()
}

// readRTT returns the series of each device since `since`, thinned to at
// most 400 points each (the worst value of each group is kept).
func readRTT(since time.Time) []RTTSeries {
	rttMu.Lock()
	defer rttMu.Unlock()
	by := map[string]*RTTSeries{}
	var order []string
	for _, p := range []string{rttLogPath() + ".old", rttLogPath()} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		r := csv.NewReader(f)
		r.FieldsPerRecord = 4
		for {
			rec, err := r.Read()
			if err != nil {
				if err == io.EOF {
					break
				}
				continue
			}
			t, err1 := strconv.ParseInt(rec[0], 10, 64)
			v, err2 := strconv.ParseInt(rec[3], 10, 64)
			if err1 != nil || err2 != nil || t < since.Unix() {
				continue
			}
			s := by[rec[1]]
			if s == nil {
				s = &RTTSeries{Name: rec[1], IP: rec[2]}
				by[rec[1]] = s
				order = append(order, rec[1])
			}
			s.Points = append(s.Points, [2]int64{t, v})
		}
		f.Close()
	}
	var out []RTTSeries
	for _, name := range order {
		s := by[name]
		var sum, n, lost int64
		for _, p := range s.Points {
			if p[1] < 0 {
				lost++
				continue
			}
			sum += p[1]
			n++
			s.Max = max(s.Max, p[1])
		}
		if n > 0 {
			s.Avg = float64(sum) / float64(n)
		}
		s.Loss = 100 * float64(lost) / float64(len(s.Points))
		if len(s.Points) > 400 { // thin out, keeping the worst of each group
			step := (len(s.Points) + 399) / 400
			var thin [][2]int64
			for i := 0; i < len(s.Points); i += step {
				w := s.Points[i]
				for _, p := range s.Points[i:min(i+step, len(s.Points))] {
					if p[1] < 0 || (w[1] >= 0 && p[1] > w[1]) {
						w = p
					}
				}
				thin = append(thin, w)
			}
			s.Points = thin
		}
		out = append(out, *s)
	}
	return out
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
	RTT     []RTTSeries     `json:"rtt"`
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
	rep := ShowReport{Since: since, Until: time.Now(), Events: ev, RTT: readRTT(since)}
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
