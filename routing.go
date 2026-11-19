package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// The patch of the show: who sends to whom on Dante (subscriptions) and on
// a Videohub (outputs). Saved per venue each time it is read, so the next
// time in the same venue wasaping says what changed since the rehearsal.

// RouteLink is one connection: a Dante receive channel or a Videohub output.
type RouteLink struct {
	Kind   string `json:"kind"`   // "dante" or "videohub"
	Device string `json:"device"` // receiving device / the Videohub
	Dest   string `json:"dest"`   // receive channel / output
	Source string `json:"source"` // "channel@device" / input, "" = nothing
}

// RoutingSnap is the patch read in one session.
type RoutingSnap struct {
	At    time.Time   `json:"at"`
	Links []RouteLink `json:"links"`
}

// RouteChange is a difference between two snapshots.
type RouteChange struct {
	RouteLink
	Was string `json:"was"` // the source before ("" = not there)
	Now string `json:"now"` // the source now ("" = gone)
}

func (l RouteLink) key() string { return l.Kind + "\x00" + l.Device + "\x00" + l.Dest }

func danteLinks(devs []scanner.DanteDevice) []RouteLink {
	var out []RouteLink
	for _, d := range devs {
		if d.Err != "" {
			continue
		}
		name := firstNonEmpty(d.Name, d.IP)
		for _, rx := range d.Rx {
			src := ""
			if rx.TxChan != "" || rx.TxDevice != "" {
				src = rx.TxChan + "@" + rx.TxDevice
			}
			out = append(out, RouteLink{"dante", name, firstNonEmpty(rx.Name, fmt.Sprint(rx.Num)), src})
		}
	}
	return out
}

func videohubLinks(devs []scanner.VideoInfo) []RouteLink {
	var out []RouteLink
	for _, v := range devs {
		if v.Kind != "Videohub" || v.Err != "" {
			continue
		}
		name := firstNonEmpty(v.Model, "Videohub") + " " + v.IP
		for _, r := range v.Routes {
			out = append(out, RouteLink{"videohub", name, fmt.Sprintf("%d %s", r.Out, r.OutLbl), fmt.Sprintf("%d %s", r.In, r.InLbl)})
		}
	}
	return out
}

// AddRouting records the patch of one kind on the current network. Within
// the same session (an hour) the latest snapshot is updated instead of
// adding one. It returns the changes against the previous session.
func (s *Store) AddRouting(kind string, links []RouteLink) (since time.Time, changes []RouteChange) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Networks[s.current]
	if n == nil || len(links) == 0 {
		return
	}
	now := time.Now()
	if l := len(n.Routing); l == 0 || now.Sub(n.Routing[l-1].At) > time.Hour {
		n.Routing = append(n.Routing, RoutingSnap{At: now})
		if len(n.Routing) > 10 {
			n.Routing = n.Routing[len(n.Routing)-10:]
		}
	}
	cur := &n.Routing[len(n.Routing)-1]
	keep := cur.Links[:0]
	for _, l := range cur.Links {
		if l.Kind != kind {
			keep = append(keep, l)
		}
	}
	cur.Links = append(keep, links...)
	cur.At = now
	s.saveLocked()
	// The previous session that read this kind.
	for i := len(n.Routing) - 2; i >= 0; i-- {
		var old []RouteLink
		for _, l := range n.Routing[i].Links {
			if l.Kind == kind {
				old = append(old, l)
			}
		}
		if len(old) > 0 {
			return n.Routing[i].At, diffRouting(old, links)
		}
	}
	return
}

func diffRouting(old, now []RouteLink) []RouteChange {
	was := map[string]RouteLink{}
	for _, l := range old {
		was[l.key()] = l
	}
	var out []RouteChange
	seen := map[string]bool{}
	for _, l := range now {
		seen[l.key()] = true
		o, ok := was[l.key()]
		switch {
		case !ok && l.Source != "":
			out = append(out, RouteChange{l, "", l.Source})
		case ok && o.Source != l.Source:
			out = append(out, RouteChange{l, o.Source, l.Source})
		}
	}
	for k, o := range was {
		if !seen[k] && o.Source != "" {
			out = append(out, RouteChange{o, o.Source, ""})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Device != out[j].Device {
			return out[i].Device < out[j].Device
		}
		return out[i].Dest < out[j].Dest
	})
	return out
}
