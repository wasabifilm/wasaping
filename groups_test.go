package main

import (
	"strings"
	"testing"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

func TestDeviceGroup(t *testing.T) {
	for kind, want := range map[string]string{
		scanner.KindLightDesk: "lights", scanner.KindLighting: "lights", scanner.KindAudio: "audio",
		scanner.KindProjector: "video", scanner.KindSwitch: "network", scanner.KindComputer: "other", "": "other",
	} {
		if g := deviceGroup(Device{Kind: kind}); g != want {
			t.Errorf("%q: %q, want %q", kind, g, want)
		}
	}
	if g := deviceGroup(Device{Kind: scanner.KindComputer, KindManual: scanner.KindAudio}); g != "audio" {
		t.Errorf("the corrected type decides: %q", g)
	}
	if g := deviceGroup(Device{Kind: scanner.KindComputer, Group: "video"}); g != "video" {
		t.Errorf("the user's department wins: %q", g)
	}
	for _, id := range groupIDs {
		if !validGroup(id) || groupName(id) == "" {
			t.Error(id)
		}
	}
	if validGroup("catering") || validGroup("") {
		t.Error("unknown departments")
	}
}

// A muted department: its events are marked (no sound on the pages, no
// alerts), the others still ring.
func TestMonitorGroupMute(t *testing.T) {
	logTempDir(t)
	m := &monitor{devs: map[string]*MonDevice{
		"a": {Key: "a", Name: "Stagebox", IP: "10.0.0.2", Group: "audio"},
		"l": {Key: "l", Name: "Nodo", IP: "10.0.0.3", Group: "lights"},
	}, order: []string{"a", "l"}}
	m.muteGroup("audio", 5*time.Minute)
	m.mu.Lock()
	m.addEvent(time.Now(), m.devs["a"], false, 0)
	m.addEvent(time.Now(), m.devs["l"], false, 0)
	m.addEventRaw(MonEvent{At: time.Now(), Name: "sACN 1", Kind: "universe"})
	m.addEventRaw(MonEvent{At: time.Now(), Name: "PTP v2", IP: "10.0.0.9", Kind: "ptp", Text: "nessun master del clock"})
	m.mu.Unlock()
	ev := m.events[1:]
	if !ev[0].Muted || ev[0].Group != "audio" {
		t.Errorf("audio muted: %+v", ev[0])
	}
	if ev[1].Muted || ev[1].Group != "lights" {
		t.Errorf("lights still on: %+v", ev[1])
	}
	if ev[2].Group != "lights" || ev[3].Group != "audio" || !ev[3].Muted {
		t.Errorf("universe and clock: %+v %+v", ev[2], ev[3])
	}
	st := m.state(10)
	if len(st.Groups) != 2 || st.Groups[0].ID != "lights" || st.Groups[1].ID != "audio" || st.Groups[1].MutedTill.IsZero() {
		t.Errorf("groups %+v", st.Groups)
	}
	m.muteGroup("audio", 0)
	m.mu.Lock()
	m.addEvent(time.Now(), m.devs["a"], true, 3)
	m.mu.Unlock()
	if m.events[len(m.events)-1].Muted {
		t.Error("audio back on")
	}
}

func TestAlertTextGroup(t *testing.T) {
	title, _ := alertText(MonEvent{At: time.Now(), Name: "Stagebox", IP: "10.0.0.2", Group: "audio"}, "")
	if !strings.HasPrefix(title, "🔴 Audio · ") || !strings.Contains(title, "Stagebox") {
		t.Errorf("title %q", title)
	}
}

func TestBuildHistory(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.Local)
	at := func(daysAgo int, hm string) time.Time {
		t, _ := time.ParseInLocation("15:04", hm, time.Local)
		d := now.AddDate(0, 0, -daysAgo)
		return time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
	}
	ip := "10.0.0.2"
	ev := []LogEvent{
		{At: at(20, "21:00"), Network: "Arena", IP: ip, Event: "non risponde", Down: true}, // older than 14 days
		{At: at(20, "21:02"), Network: "Arena", IP: ip, Event: "di nuovo in linea", Up: true, DownFor: 120},
		{At: at(3, "20:00"), Network: "Arena", IP: ip, Event: "non risponde", Down: true},
		{At: at(3, "20:00"), Network: "Arena", IP: ip, Event: "non risponde", Down: true}, // wasaping restarted: same drop
		{At: at(3, "20:01"), Network: "Arena", IP: ip, Event: "di nuovo in linea", Up: true, DownFor: 60},
		{At: at(1, "22:00"), Network: "Arena", IP: ip, Event: "risponde lento (45 ms)"},
		{At: at(1, "22:05"), Network: "Arena", IP: ip, Event: "di nuovo veloce"},
		{At: at(1, "23:00"), Network: "Casa", IP: ip, Event: "non risponde", Down: true}, // another network
		{At: at(0, "10:00"), Network: "Arena", IP: "10.0.0.3", Event: "non risponde", Down: true},
		{At: at(0, "11:00"), Network: "Arena", IP: ip, Event: "non risponde", Down: true}, // still down
	}
	h := buildHistory(ev, ip, []string{"Arena"}, now)
	if h.Drops != 3 || h.Drops7d != 2 || h.Drops24h != 1 || h.Slow7d != 1 {
		t.Errorf("counts %+v", h)
	}
	if h.Longest != 120 || h.DownAll != 180 { // a drop with no end in the log does not count: wasaping may have been closed
		t.Errorf("longest %v, all %v", h.Longest, h.DownAll)
	}
	if len(h.Days) != 14 || h.Days[13].Day != "2026-11-20" || h.Days[13].Drops != 1 || h.Days[10].Drops != 1 || h.Days[12].Slow != 1 {
		t.Errorf("days %+v", h.Days)
	}
	if len(h.Recent) != 4 || h.Recent[0].Kind != "down" || !h.Recent[0].Open || h.Recent[1].Kind != "slow" || h.Recent[1].Text != "45 ms" || h.Recent[1].Secs != 300 {
		t.Errorf("recent %+v", h.Recent)
	}
	if all := buildHistory(ev, ip, nil, now); all.Drops != 3 { // the drop at «Casa» never ended: today is the same one
		t.Errorf("any network: %d drops", all.Drops)
	}
}

func TestAlarmSound(t *testing.T) {
	if !validSound("moo") || !validSound("beep") || validSound("cowbell") {
		t.Error("sounds")
	}
}
