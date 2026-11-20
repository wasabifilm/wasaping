package scanner

import (
	"encoding/binary"
	"fmt"
	"sort"
	"time"
)

// Art-Net timecode (ArtTimeCode, OpCode 0x9700): who sends it, the current
// time, and whether it runs, stops or jumps. Consoles, media servers and
// timecode converters broadcast it on port 6454.

// TimecodeEvent is a change in a timecode stream.
type TimecodeEvent struct {
	At   time.Time `json:"at"`
	Kind string    `json:"kind"` // start, stop, jump, silent, type
	From string    `json:"from,omitempty"`
	To   string    `json:"to,omitempty"`
}

// TimecodeSource is the live state of one timecode stream.
type TimecodeSource struct {
	IP     string          `json:"ip"`
	Stream int             `json:"stream,omitempty"` // Art-Net 4 stream id
	Type   string          `json:"type"`             // "EBU 25", …
	Time   string          `json:"time"`             // HH:MM:SS:FF
	State  string          `json:"state"`            // running, stopped, silent (no packets)
	Since  time.Time       `json:"since"`            // of the state
	PPS    int             `json:"pps"`              // packets in the last second
	Events []TimecodeEvent `json:"events,omitempty"` // latest first
}

var tcTypes = [4]struct {
	name string
	fps  int     // frames counted per second
	rate float64 // real frames per second
}{{"Film 24", 24, 24}, {"EBU 25", 25, 25}, {"DF 29,97", 30, 29.97}, {"SMPTE 30", 30, 30}}

// parseArtTimeCode decodes an ArtTimeCode packet.
func parseArtTimeCode(b []byte) (stream, h, m, s, f, typ int, ok bool) {
	if len(b) < 19 || string(b[:8]) != "Art-Net\x00" || binary.LittleEndian.Uint16(b[8:]) != 0x9700 {
		return
	}
	f, s, m, h, typ = int(b[14]), int(b[15]), int(b[16]), int(b[17]), int(b[18])
	if typ > 3 || h > 23 || m > 59 || s > 59 || f >= tcTypes[typ].fps {
		return
	}
	return int(b[13]), h, m, s, f, typ, true
}

type tcTrack struct {
	ip                string
	stream            int
	typ               int
	frames            int // total frames since 00:00:00:00
	text              string
	state             string
	since             time.Time
	lastPkt, lastMove time.Time
	pkts              []time.Time
	events            []TimecodeEvent
}

// timecodes follows every stream heard; not safe for concurrent use.
type timecodes struct{ m map[string]*tcTrack }

func (t *timecodes) event(k *tcTrack, e TimecodeEvent) {
	k.events = append([]TimecodeEvent{e}, k.events...)
	if len(k.events) > 30 {
		k.events = k.events[:30]
	}
}

func (t *timecodes) setState(k *tcTrack, now time.Time, state string) {
	if k.state != state {
		k.state, k.since = state, now
	}
}

// feed records one ArtTimeCode packet.
func (t *timecodes) feed(now time.Time, ip string, stream, h, m, s, f, typ int) {
	if t.m == nil {
		t.m = map[string]*tcTrack{}
	}
	key := fmt.Sprintf("%s/%d", ip, stream)
	k := t.m[key]
	frames := ((h*60+m)*60+s)*tcTypes[typ].fps + f
	text := fmt.Sprintf("%02d:%02d:%02d:%02d", h, m, s, f)
	if k == nil {
		k = &tcTrack{ip: ip, stream: stream, typ: typ, frames: frames, text: text, state: "stopped", since: now, lastPkt: now}
		t.m[key] = k
	}
	if typ != k.typ {
		t.event(k, TimecodeEvent{At: now, Kind: "type", From: tcTypes[k.typ].name, To: tcTypes[typ].name})
		k.typ = typ
	}
	if frames != k.frames {
		// A jump: backwards, or further than the time since the previous
		// packet explains (plus half a second of slack).
		diff := frames - k.frames
		expected := now.Sub(k.lastPkt).Seconds() * tcTypes[typ].rate
		if diff < 0 || float64(diff) > expected+tcTypes[typ].rate/2 {
			t.event(k, TimecodeEvent{At: now, Kind: "jump", From: k.text, To: text})
		}
		if k.state != "running" {
			t.event(k, TimecodeEvent{At: now, Kind: "start", To: text})
			t.setState(k, now, "running")
		}
		k.lastMove = now
	} else if k.state == "silent" {
		t.setState(k, now, "stopped") // packets again, same value
	}
	k.frames, k.text, k.lastPkt = frames, text, now
	k.pkts = append(k.pkts, now)
}

// snapshot updates the states (running → stopped after half a second
// without movement, → silent after 2 s without packets) and returns them.
func (t *timecodes) snapshot(now time.Time) []TimecodeSource {
	var out []TimecodeSource
	for key, k := range t.m {
		if now.Sub(k.lastPkt) > 10*time.Minute {
			delete(t.m, key)
			continue
		}
		if k.state == "running" && now.Sub(k.lastMove) > 500*time.Millisecond {
			t.event(k, TimecodeEvent{At: now, Kind: "stop", To: k.text})
			t.setState(k, now, "stopped")
		}
		if k.state != "silent" && now.Sub(k.lastPkt) > 2*time.Second {
			t.event(k, TimecodeEvent{At: now, Kind: "silent", To: k.text})
			t.setState(k, now, "silent")
		}
		n := 0
		for _, p := range k.pkts {
			if now.Sub(p) <= time.Second {
				k.pkts[n] = p
				n++
			}
		}
		k.pkts = k.pkts[:n]
		out = append(out, TimecodeSource{IP: k.ip, Stream: k.stream, Type: tcTypes[k.typ].name, Time: k.text,
			State: k.state, Since: k.since, PPS: n, Events: append([]TimecodeEvent(nil), k.events...)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IP != out[j].IP {
			return out[i].IP < out[j].IP
		}
		return out[i].Stream < out[j].Stream
	})
	return out
}
