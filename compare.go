package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// "What changed since last time": the network compared with an earlier
// snapshot, and RDM fixtures compared with the previous search.

type DevChange struct {
	Kind   string  `json:"kind"` // new, missing, ip, changed
	Now    SnapDev `json:"now"`
	Before SnapDev `json:"before"`
	Detail string  `json:"detail,omitempty"`
}

type CompareResult struct {
	Snapshots []time.Time `json:"snapshots"` // all, oldest first
	Now       time.Time   `json:"now"`
	Before    time.Time   `json:"before"`
	Changes   []DevChange `json:"changes"`
	Same      int         `json:"same"`
}

// compareSnapshots lists what differs between before and now.
func compareSnapshots(before, now Snapshot, lang string) ([]DevChange, int) {
	b := map[string]SnapDev{}
	for _, d := range before.Devices {
		b[d.Key] = d
	}
	var out []DevChange
	same := 0
	seen := map[string]bool{}
	for _, d := range now.Devices {
		seen[d.Key] = true
		o, ok := b[d.Key]
		switch {
		case !ok:
			out = append(out, DevChange{Kind: "new", Now: d})
		case o.IP != d.IP:
			out = append(out, DevChange{Kind: "ip", Now: d, Before: o, Detail: fmt.Sprintf(L(lang, "IP cambiato: %s → %s", "IP changed: %s → %s"), o.IP, d.IP)})
		case o.Model != d.Model && o.Model != "" && d.Model != "":
			out = append(out, DevChange{Kind: "changed", Now: d, Before: o, Detail: fmt.Sprintf(L(lang, "modello: %s → %s", "model: %s → %s"), o.Model, d.Model)})
		default:
			same++
		}
	}
	for _, d := range before.Devices {
		if !seen[d.Key] {
			out = append(out, DevChange{Kind: "missing", Before: d})
		}
	}
	return out, same
}

// handleCompare: GET ?net=…&with=<index of the earlier snapshot>.
func handleCompare(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("net") != "" {
		devices().Select(q.Get("net"))
	}
	snaps := devices().Snapshots()
	res := CompareResult{Changes: []DevChange{}}
	for _, s := range snaps {
		res.Snapshots = append(res.Snapshots, s.At)
	}
	if len(snaps) < 2 {
		writeJSON(w, res)
		return
	}
	now := snaps[len(snaps)-1]
	i := len(snaps) - 2
	if v, err := strconv.Atoi(q.Get("with")); err == nil && v >= 0 && v < len(snaps)-1 {
		i = v
	}
	res.Now, res.Before = now.At, snaps[i].At
	res.Changes, res.Same = compareSnapshots(snaps[i], now, q.Get("lang"))
	writeJSON(w, res)
}

// FixtureChange is a difference between two RDM searches.
type FixtureChange struct {
	UID    string `json:"uid"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

func compareFixtures(before, now []scanner.Fixture, lang string) []FixtureChange {
	b := map[string]scanner.Fixture{}
	for _, f := range before {
		b[f.UID] = f
	}
	var out []FixtureChange
	seen := map[string]bool{}
	for _, f := range now {
		seen[f.UID] = true
		o, ok := b[f.UID]
		label := firstNonEmpty(f.Label, f.Model, f.UID)
		switch {
		case !ok:
			out = append(out, FixtureChange{f.UID, label, L(lang, "nuovo", "new")})
		default:
			if o.Address != f.Address {
				out = append(out, FixtureChange{f.UID, label, fmt.Sprintf(L(lang, "indirizzo %d → %d", "address %d → %d"), o.Address, f.Address)})
			}
			if o.Personality != f.Personality {
				out = append(out, FixtureChange{f.UID, label, fmt.Sprintf(L(lang, "modalità %s → %s", "mode %s → %s"), o.Personality, f.Personality)})
			}
			if o.Label != f.Label && o.Label != "" {
				out = append(out, FixtureChange{f.UID, label, fmt.Sprintf(L(lang, "etichetta «%s» → «%s»", "label “%s” → “%s”"), o.Label, f.Label)})
			}
			if o.Universe != f.Universe {
				out = append(out, FixtureChange{f.UID, label, fmt.Sprintf(L(lang, "universo %d → %d", "universe %d → %d"), o.Universe, f.Universe)})
			}
		}
	}
	for _, f := range before {
		if !seen[f.UID] {
			out = append(out, FixtureChange{f.UID, firstNonEmpty(f.Label, f.Model, f.UID), L(lang, "non risponde più", "no longer answering")})
		}
	}
	return out
}
