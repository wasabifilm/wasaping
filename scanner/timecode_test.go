package scanner

import (
	"encoding/binary"
	"testing"
	"time"
)

func tcPacket(h, m, s, f, typ int) []byte {
	b := make([]byte, 19)
	copy(b, "Art-Net\x00")
	binary.LittleEndian.PutUint16(b[8:], 0x9700)
	b[11] = 14
	b[14], b[15], b[16], b[17], b[18] = byte(f), byte(s), byte(m), byte(h), byte(typ)
	return b
}

func TestParseArtTimeCode(t *testing.T) {
	_, h, m, s, f, typ, ok := parseArtTimeCode(tcPacket(1, 2, 3, 4, 1))
	if !ok || h != 1 || m != 2 || s != 3 || f != 4 || typ != 1 {
		t.Fatalf("got %d:%d:%d:%d type %d ok %v", h, m, s, f, typ, ok)
	}
	if _, _, _, _, _, _, ok := parseArtTimeCode(tcPacket(1, 2, 3, 25, 1)); ok {
		t.Error("frame 25 in EBU 25 accepted")
	}
	if _, _, _, _, _, _, ok := parseArtTimeCode(tcPacket(1, 2, 3, 4, 7)); ok {
		t.Error("unknown type accepted")
	}
	if _, _, _, _, _, _, ok := parseArtTimeCode(tcPacket(1, 2, 3, 4, 1)[:18]); ok {
		t.Error("short packet accepted")
	}
}

func TestTimecodeStates(t *testing.T) {
	var tc timecodes
	t0 := time.Date(2026, 11, 20, 20, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	// 2 s running at 25 fps from 01:00:00:00.
	for i := 0; i < 50; i++ {
		tc.feed(at(i*40), "10.0.0.5", 0, 1, 0, i/25, i%25, 1)
	}
	s := tc.snapshot(at(49*40 + 10))
	if len(s) != 1 || s[0].State != "running" || s[0].Time != "01:00:01:24" || s[0].Type != "EBU 25" {
		t.Fatalf("running: %+v", s)
	}
	// Same value for a second: stopped.
	for i := 0; i < 25; i++ {
		tc.feed(at(2000+i*40), "10.0.0.5", 0, 1, 0, 1, 24, 1)
	}
	s = tc.snapshot(at(3000))
	if s[0].State != "stopped" || s[0].Events[0].Kind != "stop" {
		t.Fatalf("stopped: %+v", s[0])
	}
	// Located elsewhere and running again: a jump and a start.
	tc.feed(at(3040), "10.0.0.5", 0, 2, 0, 0, 0, 1)
	tc.feed(at(3080), "10.0.0.5", 0, 2, 0, 0, 1, 1)
	s = tc.snapshot(at(3100))
	if s[0].State != "running" || s[0].Events[0].Kind != "start" || s[0].Events[1].Kind != "jump" ||
		s[0].Events[1].From != "01:00:01:24" || s[0].Events[1].To != "02:00:00:00" {
		t.Fatalf("jump: %+v", s[0].Events)
	}
	// Normal running is never a jump, even with a packet lost now and then.
	tc.feed(at(3200), "10.0.0.5", 0, 2, 0, 0, 4, 1)
	s = tc.snapshot(at(3210))
	if s[0].Events[0].Kind != "start" {
		t.Fatalf("lost packet seen as a jump: %+v", s[0].Events[0])
	}
	// No packets for 3 s: silent.
	s = tc.snapshot(at(6300))
	if s[0].State != "silent" || s[0].Events[0].Kind != "silent" {
		t.Fatalf("silent: %+v", s[0])
	}
}
