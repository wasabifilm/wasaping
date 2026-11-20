package main

import (
	"math"
	"net"
	"net/http"
	"slices"

	"strings"
	"time"
)

// A device's problem history, from the Monitor's log: when it dropped,
// for how long, when it was slow. Handy to find the cable or the switch
// port that fails only now and then.

// HistItem is one problem: a drop, a slow period or a warning.
type HistItem struct {
	At   time.Time `json:"at"`
	Kind string    `json:"kind"`           // "down", "slow", "note"
	Secs float64   `json:"secs,omitempty"` // how long (drop, slow period)
	Open bool      `json:"open,omitempty"` // no end in the log (still on, or wasaping was closed)
	Text string    `json:"text,omitempty"`
}

// HistDay sums up one day.
type HistDay struct {
	Day      string  `json:"day"` // 2006-01-02
	Drops    int     `json:"drops"`
	DownSecs float64 `json:"down_s"`
	Slow     int     `json:"slow"`
}

// DevHistory is what the device panel shows.
type DevHistory struct {
	IP        string     `json:"ip"`
	Since     time.Time  `json:"since,omitempty"` // first line about it in the log
	Drops     int        `json:"drops"`
	Drops24h  int        `json:"drops_24h"`
	Drops7d   int        `json:"drops_7d"`
	Down7d    float64    `json:"down_7d_s"`
	DownAll   float64    `json:"down_s"`
	Longest   float64    `json:"longest_s"`
	LongestAt time.Time  `json:"longest_at,omitempty"`
	LastDrop  time.Time  `json:"last_drop,omitempty"`
	Slow7d    int        `json:"slow_7d"`
	Days      []HistDay  `json:"days"`   // the latest 14, oldest first
	Recent    []HistItem `json:"recent"` // the latest 20, newest first
	RTT       *RTTSeries `json:"rtt,omitempty"`
}

const histDays = 14

// buildHistory reads the log for one IP on one network (labels: the names
// the network had in the log; none = any network).
func buildHistory(ev []LogEvent, ip string, labels []string, now time.Time) DevHistory {
	h := DevHistory{IP: ip, Days: make([]HistDay, histDays), Recent: []HistItem{}}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	for i := range h.Days {
		h.Days[i].Day = today.AddDate(0, 0, i-histDays+1).Format("2006-01-02")
	}
	day := func(t time.Time) *HistDay {
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		i := int(math.Round(d.Sub(today).Hours()/24)) + histDays - 1
		if i < 0 || i >= histDays {
			return nil
		}
		return &h.Days[i]
	}
	var items []HistItem
	var downAt, slowAt time.Time
	slowText := ""
	addDrop := func(start time.Time, secs float64, open bool) {
		items = append(items, HistItem{At: start, Kind: "down", Secs: secs, Open: open})
		h.DownAll += secs
		if secs > h.Longest {
			h.Longest, h.LongestAt = secs, start
		}
		if now.Sub(start) < 7*24*time.Hour {
			h.Down7d += secs
		}
		if d := day(start); d != nil {
			d.DownSecs += secs
		}
	}
	countDrop := func(at time.Time) {
		h.Drops++
		h.LastDrop = at
		if now.Sub(at) < 24*time.Hour {
			h.Drops24h++
		}
		if now.Sub(at) < 7*24*time.Hour {
			h.Drops7d++
		}
		if d := day(at); d != nil {
			d.Drops++
		}
	}
	for _, e := range ev {
		if e.IP != ip || (len(labels) > 0 && !slices.Contains(labels, e.Network)) {
			continue
		}
		if h.Since.IsZero() {
			h.Since = e.At
		}
		switch {
		case e.Down && e.Event == "non risponde":
			if downAt.IsZero() { // a second "down" (wasaping restarted) is the same drop
				downAt = e.At
				countDrop(e.At)
			}
		case e.Up:
			start := downAt
			if start.IsZero() { // the drop began before the log
				start = e.At.Add(-time.Duration(e.DownFor) * time.Second)
				countDrop(start)
			}
			addDrop(start, e.DownFor, false)
			downAt = time.Time{}
		case strings.HasPrefix(e.Event, "risponde lento"):
			if slowAt.IsZero() {
				slowAt, slowText = e.At, strings.TrimSuffix(strings.TrimPrefix(e.Event, "risponde lento ("), ")") // "45 ms"
				if now.Sub(e.At) < 7*24*time.Hour {
					h.Slow7d++
				}
				if d := day(e.At); d != nil {
					d.Slow++
				}
			}
		case e.Event == "di nuovo veloce":
			if !slowAt.IsZero() {
				items = append(items, HistItem{At: slowAt, Kind: "slow", Secs: e.At.Sub(slowAt).Seconds(), Text: slowText})
				slowAt = time.Time{}
			}
		default:
			items = append(items, HistItem{At: e.At, Kind: "note", Text: e.Event})
		}
	}
	if !downAt.IsZero() {
		items = append(items, HistItem{At: downAt, Kind: "down", Secs: now.Sub(downAt).Seconds(), Open: true})
	}
	if !slowAt.IsZero() {
		items = append(items, HistItem{At: slowAt, Kind: "slow", Secs: now.Sub(slowAt).Seconds(), Open: true, Text: slowText})
	}
	slices.SortStableFunc(items, func(a, b HistItem) int { return b.At.Compare(a.At) })
	if len(items) > 20 {
		items = items[:20]
	}
	h.Recent = append(h.Recent, items...)
	return h
}

// NetLabels are the names a network is written with in the log.
func (s *Store) NetLabels(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.netLocked(id)
	if n == nil {
		return nil
	}
	var out []string
	for _, l := range []string{n.Name, n.Auto} {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// handleHistory: GET ?ip=&net= (network id, "" = current).
func handleHistory(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if net.ParseIP(ip).To4() == nil {
		http.Error(w, "IP non valido", http.StatusBadRequest)
		return
	}
	now := time.Now()
	ev, err := readMonitorLog(now.AddDate(0, 0, -90))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h := buildHistory(ev, ip, devices().NetLabels(r.URL.Query().Get("net")), now)
	for _, s := range readRTT(now.Add(-24 * time.Hour)) {
		if s.IP == ip {
			s := s
			h.RTT = &s
			break
		}
	}
	if h.RTT != nil && len(h.RTT.Points) > 120 { // the panel draws a small chart
		step := (len(h.RTT.Points) + 119) / 120
		var thin [][2]int64
		for i := 0; i < len(h.RTT.Points); i += step {
			w := h.RTT.Points[i]
			for _, p := range h.RTT.Points[i:min(i+step, len(h.RTT.Points))] {
				if p[1] < 0 || (w[1] >= 0 && p[1] > w[1]) {
					w = p
				}
			}
			thin = append(thin, w)
		}
		h.RTT.Points = thin
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, h)
}
