package main

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

func TestOSCInParse(t *testing.T) {
	m := oscMessages(oscMessage("/wasaping/mute", 120))
	if len(m) != 1 || m[0].addr != "/wasaping/mute" || len(m[0].args) != 1 || m[0].args[0] != 120 {
		t.Fatalf("%+v", m)
	}
	// A bundle with two messages.
	a, b := oscMessage("/wasaping/show/start"), oscMessage("/wasaping/scan")
	var p []byte
	p = append(p, []byte("#bundle\x00")...)
	p = append(p, make([]byte, 8)...)
	for _, x := range [][]byte{a, b} {
		p = append(p, byte(len(x)>>24), byte(len(x)>>16), byte(len(x)>>8), byte(len(x)))
		p = append(p, x...)
	}
	if m := oscMessages(p); len(m) != 2 || m[1].addr != "/wasaping/scan" {
		t.Fatalf("bundle: %+v", m)
	}
	if m := oscMessages([]byte("garbage")); len(m) != 0 {
		t.Fatal("garbage parsed")
	}
	if oscIn.run(context.Background(), "/other/thing", nil) {
		t.Fatal("unknown command accepted")
	}
}

// logTempDir is a temporary config folder for tests that write the monitor
// log: the log is appended from a goroutine, and on Windows a folder with an
// open file cannot be removed, so wait for the writes before cleaning up.
func logTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "wasaping-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		time.Sleep(200 * time.Millisecond)
		logMu.Lock()
		logMu.Unlock()
		os.RemoveAll(dir)
	})
	return dir
}

func TestOSCInListener(t *testing.T) {
	dir := logTempDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	l, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	port := l.LocalAddr().(*net.UDPAddr).Port
	l.Close()
	setMu.Lock()
	setCache = &settings{OSCInPort: port}
	setMu.Unlock()
	defer func() {
		setMu.Lock()
		setCache = &settings{}
		setMu.Unlock()
		oscIn.apply(context.Background())
		setMu.Lock()
		setCache = nil
		setMu.Unlock()
	}()
	if err := oscIn.apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	c, _ := net.Dial("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	c.Write(oscMessage("/wasaping/mute", 60))
	c.Close()
	for i := 0; i < 50 && !mon.isMuted(); i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if !mon.isMuted() {
		t.Fatal("mute over OSC did not arrive")
	}
	mon.mute(0)
	if mon.isMuted() {
		t.Fatal("unmute")
	}
}

func TestRoutingDiff(t *testing.T) {
	old := []RouteLink{{"dante", "Stagebox", "1 Kick", "01@Console"}, {"dante", "Stagebox", "2 Snare", "02@Console"}, {"dante", "Stagebox", "3 Spare", ""}}
	now := []RouteLink{{"dante", "Stagebox", "1 Kick", "01@Console"}, {"dante", "Stagebox", "2 Snare", "05@Console"}, {"dante", "Stagebox", "4 New", "09@Console"}}
	ch := diffRouting(old, now)
	if len(ch) != 2 || ch[0].Dest != "2 Snare" || ch[0].Was != "02@Console" || ch[0].Now != "05@Console" || ch[1].Dest != "4 New" || ch[1].Was != "" {
		t.Fatalf("%+v", ch)
	}
	links := danteLinks([]scanner.DanteDevice{{IP: "10.0.0.2", Name: "Stagebox", Rx: []scanner.DanteRx{{Num: 1, Name: "Kick", TxChan: "01", TxDevice: "Console"}, {Num: 2, Name: "Spare"}}}})
	if len(links) != 2 || links[0].Source != "01@Console" || links[1].Source != "" {
		t.Fatalf("%+v", links)
	}
	v := videohubLinks([]scanner.VideoInfo{{IP: "10.0.0.9", Kind: "Videohub", Model: "Smart Videohub", Routes: []scanner.VideoRoute{{Out: 1, OutLbl: "PJ", In: 3, InLbl: "Cam 3"}}}})
	if len(v) != 1 || v[0].Dest != "1 PJ" || v[0].Source != "3 Cam 3" {
		t.Fatalf("%+v", v)
	}
}

func TestShowMarkersReport(t *testing.T) {
	dir := logTempDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	start := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	for _, e := range []MonEvent{
		{At: start.Add(-time.Hour), Name: "Console", IP: "10.0.0.1"}, // before the show
		{At: start, Name: "Show", Kind: "show", Text: showStartText},
		{At: start.Add(10 * time.Minute), Name: "Stagebox", IP: "10.0.0.2"},
		{At: start.Add(11 * time.Minute), Name: "Stagebox", IP: "10.0.0.2", Up: true, DownFor: 60},
		{At: start.Add(time.Hour), Name: "Show", Kind: "show", Text: showEndText},
		{At: start.Add(90 * time.Minute), Name: "Console", IP: "10.0.0.1"}, // after
	} {
		appendMonitorLog(e)
	}
	s, e, ok := lastShow()
	if !ok || !s.Equal(start) || !e.Equal(start.Add(time.Hour)) {
		t.Fatal(s, e, ok)
	}
	rep, err := buildShowReportRange(s, e)
	if err != nil || rep.Drops != 1 || len(rep.Devices) != 1 || rep.Devices[0].Name != "Stagebox" {
		t.Fatalf("%+v %v", rep, err)
	}
}

func TestNetworkChecks(t *testing.T) {
	hosts := []HostView{{Host: scanner.Host{IP: "10.0.0.1", Vendor: "MikroTik"}}, {Host: scanner.Host{IP: "10.0.0.77", Vendor: "TP-Link"}}}
	servers := []scanner.DHCPServer{{IP: "10.0.0.1", Router: "10.0.0.1"}, {IP: "10.0.0.77", Router: "192.168.0.1"}}
	is := dhcpIssues(servers, hosts, "10.0.0.1")
	if len(is) != 2 || !strings.Contains(is[0], "10.0.0.77 (TP-Link)") || !strings.Contains(is[1], "192.168.0.1") {
		t.Fatalf("dhcp issues: %q", is)
	}
	for _, s := range is {
		if en := msgIn(s, "en"); en == s {
			t.Errorf("not translated: %q", s)
		}
	}
	if is := dhcpIssues(servers[:1], hosts, "10.0.0.1"); len(is) != 0 {
		t.Fatalf("one good server should be fine: %q", is)
	}
	calm := trafficNotes(scanner.TrafficReport{RxPPS: 300, NonUnicastPS: -1})
	if len(calm) != 1 || calm[0].Level != "ok" {
		t.Fatalf("calm traffic: %+v", calm)
	}
	busy := trafficNotes(scanner.TrafficReport{RxPPS: 30000, NonUnicastPS: 5000, ArtBroadcast: make([]int, 12),
		Protocols: []scanner.TrafficProto{{Name: "Art-Net", Universes: 12, Mbps: 60}}})
	if len(busy) != 4 || busy[0].Level != "warn" {
		t.Fatalf("busy traffic: %+v", busy)
	}
	for _, m := range []string{
		"IP doppi: 10.0.0.5. Due dispositivi con lo stesso indirizzo si contendono il traffico: apri il dispositivo per vedere chi sono.",
		"Art-Net in broadcast da grandMA3: 4 universi arrivano a tutti i dispositivi della rete (anche Dante, video e Wi-Fi). Meglio l'unicast verso i nodi.",
	} {
		if msgIn(m, "en") == m {
			t.Errorf("not translated: %q", m)
		}
	}
}
