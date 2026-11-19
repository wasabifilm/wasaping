package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

func TestStoreNewGoneHistory(t *testing.T) {
	s := openStore(filepath.Join(t.TempDir(), "devices.json"))
	ctx := context.Background()
	targets := []net.IP{net.ParseIP("192.168.77.1"), net.ParseIP("192.168.77.2"), net.ParseIP("192.168.77.3")}
	router := scanner.Host{IP: "192.168.77.1", MAC: "98:25:4a:ae:41:91"}
	tv := scanner.Host{IP: "192.168.77.2"}

	// First scan on this network: nothing is flagged as new.
	ss := s.Begin(ctx, targets)
	if v := ss.View(router); v.New {
		t.Error("first scan should not flag devices as new")
	}
	ss.View(tv)
	ss.Finish(targets)
	if !s.Label("", "ip:192.168.77.2", "TV salotto", "Monitor palco", "", true) {
		t.Fatal("label failed")
	}

	// Second scan: the TV now has a MAC (keeps its name), a phone is new,
	// the router is gone.
	ss = s.Begin(ctx, targets)
	v := ss.View(scanner.Host{IP: "192.168.77.2", MAC: "aa:bb:cc:00:11:22"})
	if v.New || v.Name != "TV salotto" || v.Role != "Monitor palco" || !v.Critical {
		t.Errorf("TV: %+v", v)
	}
	if v.History != "11" {
		t.Errorf("history %q", v.History)
	}
	if v := ss.View(scanner.Host{IP: "192.168.77.3"}); !v.New {
		t.Error("phone should be new")
	}
	gone := ss.Finish(targets)
	if len(gone) != 1 || gone[0].Key != router.MAC || gone[0].History != "10" {
		t.Errorf("gone = %+v", gone)
	}
	if c := s.CriticalDevices(); len(c) != 1 || c[0].Name != "TV salotto" {
		t.Errorf("critical = %+v", c)
	}

	// Third scan: another device answers on the router's old IP.
	ss = s.Begin(ctx, targets)
	ss.View(scanner.Host{IP: "192.168.77.2", MAC: "aa:bb:cc:00:11:22"})
	ss.Finish(targets)
	ss = s.Begin(ctx, targets)
	w := ss.View(scanner.Host{IP: "192.168.77.2", MAC: "de:ad:be:ef:00:01"})
	if len(w.Warnings) != 1 {
		t.Errorf("expected IP change warning, got %v", w.Warnings)
	}
}

func TestParsePlan(t *testing.T) {
	excel := "\xef\xbb\xbfNome;IP;Ruolo;MAC\r\ngrandMA3 FOH;10.0.0.10;Console;00-0C-29-4A-10-01\r\n;;;\r\nNodo SX;10.0.0.21;;\r\n"
	p, err := parsePlan([]byte(excel))
	if err != nil || len(p) != 2 || p[0].Name != "grandMA3 FOH" || p[0].MAC != "00:0c:29:4a:10:01" || p[1].IP != "10.0.0.21" {
		t.Fatalf("excel: %v %+v", err, p)
	}
	plain := "Console,10.0.0.10,FOH\nNodo,10.0.0.21\n"
	if p, err := parsePlan([]byte(plain)); err != nil || len(p) != 2 || p[0].Role != "FOH" {
		t.Fatalf("plain: %v %+v", err, p)
	}
	if _, err := parsePlan([]byte("solo testo\n")); err == nil {
		t.Error("expected an error without IPs")
	}
}

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		tag, cur string
		want     bool
	}{{"v0.4.0", "v0.3.0", true}, {"v0.3.0", "v0.3.0", false}, {"v0.3.1", "v0.3.0-rc", true}, {"v0.10.0", "v0.9.9", true}, {"v0.2.0", "v0.3.0", false}, {"", "v0.3.0", false}, {"v1.0.0", "dev", false}} {
		if got := newerVersion(c.tag, c.cur); got != c.want {
			t.Errorf("%s vs %s: got %v", c.tag, c.cur, got)
		}
	}
}

func TestOSCMessage(t *testing.T) {
	m := oscMessage("/wasaping/down", "CL5-FOH", 12)
	want := "/wasaping/down\x00\x00,si\x00CL5-FOH\x00\x00\x00\x00\x0c"
	if string(m) != want {
		t.Errorf("got %q", m)
	}
}

func TestDanteRedundancy(t *testing.T) {
	d := func(net, name string) HostView {
		return HostView{Host: scanner.Host{Friendly: name, Protocols: []string{"Dante"}}, Net: net}
	}
	r := &ScanResult{
		Networks: []NetView{{ID: "a", Auto: "Dante primaria"}, {ID: "b", Auto: "Dante secondaria"}},
		Hosts:    []HostView{d("a", "CL5-FOH"), d("b", "CL5-FOH"), d("a", "Rio3224")},
	}
	got := danteRedundancy(r)
	if len(got) != 1 || !strings.Contains(got[0], "Rio3224") || !strings.Contains(got[0], "Dante secondaria") {
		t.Errorf("got %v", got)
	}
}

func TestComparePlan(t *testing.T) {
	h := func(ip, mac, name string) HostView {
		return HostView{Host: scanner.Host{IP: ip, MAC: mac}, Name: name, Key: firstNonEmpty(mac, "ip:"+ip)}
	}
	hosts := []HostView{
		h("10.0.0.10", "00:0c:29:4a:10:01", "Console"),
		h("10.0.0.30", "aa:bb:cc:00:00:30", "TV"),      // planned at .99 by MAC
		h("10.0.0.40", "aa:bb:cc:00:00:40", "Intruso"), // takes the IP of the stagebox
		h("10.0.0.77", "", "Extra"),
	}
	plan := []PlanItem{
		{Name: "Console", IP: "10.0.0.10"},
		{Name: "TV", IP: "10.0.0.99", MAC: "aa:bb:cc:00:00:30"},
		{Name: "Stagebox", IP: "10.0.0.40", MAC: "aa:bb:cc:00:00:41"},
		{Name: "Media server", IP: "10.0.0.50"},
		{Name: "Dup A", IP: "10.0.0.60"}, {Name: "Dup B", IP: "10.0.0.60"},
	}
	pc := comparePlan(plan, hosts, "it")
	want := []string{"ok", "IP sbagliato: è a 10.0.0.30", "IP occupato da Intruso", "manca", "IP doppio nel piano", "IP doppio nel piano"}
	for i, w := range want {
		if pc.Rows[i].Status != w {
			t.Errorf("row %d (%s): %q, want %q", i, plan[i].Name, pc.Rows[i].Status, w)
		}
	}
	if len(pc.Extra) != 1 || pc.Extra[0].IP != "10.0.0.77" || pc.OK != 1 || pc.Bad != 2 {
		t.Errorf("extra %v ok %d bad %d", pc.Extra, pc.OK, pc.Bad)
	}
	if en := comparePlan(plan, hosts, "en"); en.Rows[3].Status != "missing" {
		t.Errorf("english: %q", en.Rows[3].Status)
	}
}

func TestShowReport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := monitorLogPath()
	os.MkdirAll(filepath.Dir(p), 0o700)
	day := time.Now().Format("2006-01-02")
	log := "data,ora,rete,dispositivo,ip,evento,durata_s\n" +
		day + ",00:00:10,Arena,CL5-FOH,10.0.0.30,non risponde,\n" +
		day + ",00:00:22,Arena,CL5-FOH,10.0.0.30,di nuovo in linea,12\n" +
		day + ",00:01:00,Arena,CL5-FOH,10.0.0.30,non risponde,\n" +
		day + ",00:01:05,Arena,CL5-FOH,10.0.0.30,di nuovo in linea,5\n" +
		day + ",00:02:00,Arena,PTP PTPv1,10.0.0.30,nessun master del clock,\n"
	os.WriteFile(p, []byte(log), 0o600)
	since := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	rep, err := buildShowReport(since)
	if err != nil || rep.Drops != 3 || len(rep.Devices) != 2 {
		t.Fatalf("%v %+v", err, rep)
	}
	var cl5 DeviceSummary
	for _, d := range rep.Devices {
		if d.Name == "CL5-FOH" {
			cl5 = d
		}
	}
	if cl5.Drops != 2 || cl5.DownTotal != 17 || cl5.Longest != 12 || cl5.StillDown {
		t.Errorf("CL5: %+v", cl5)
	}
	base := time.Now().Add(-time.Hour)
	var samples []rttSample
	for i := 0; i < 1000; i++ {
		v := int64(2)
		if i == 550 {
			v = 90
		}
		if i%100 == 0 {
			v = -1
		}
		samples = append(samples, rttSample{base.Add(time.Duration(i) * time.Second), "CL5-FOH", "10.0.0.30", v})
	}
	appendRTTLog(samples)
	rtt := readRTT(base.Add(-time.Minute))
	if len(rtt) != 1 || rtt[0].Max != 90 || rtt[0].Loss != 1 || len(rtt[0].Points) > 400 {
		t.Errorf("rtt %+v", rtt)
	}
	worst := false
	for _, p := range rtt[0].Points {
		if p[1] == 90 {
			worst = true
		}
	}
	if !worst {
		t.Error("thinning lost the worst value")
	}
	if at, ok := parseSince("18:00", time.Date(2026, 11, 19, 1, 0, 0, 0, time.Local)); !ok || at.Day() != 18 || at.Hour() != 18 {
		t.Errorf("since 18:00 after midnight: %v", at)
	}
}

func TestCompare(t *testing.T) {
	before := Snapshot{Devices: []SnapDev{{Key: "a", IP: "10.0.0.1", Model: "X"}, {Key: "b", IP: "10.0.0.2"}, {Key: "c", IP: "10.0.0.3", Model: "Y"}}}
	now := Snapshot{Devices: []SnapDev{{Key: "a", IP: "10.0.0.1", Model: "X"}, {Key: "b", IP: "10.0.0.9"}, {Key: "c", IP: "10.0.0.3", Model: "Z"}, {Key: "d", IP: "10.0.0.4"}}}
	ch, same := compareSnapshots(before, now, "it")
	kinds := map[string]int{}
	for _, c := range ch {
		kinds[c.Kind]++
	}
	if same != 1 || kinds["ip"] != 1 || kinds["changed"] != 1 || kinds["new"] != 1 || kinds["missing"] != 0 {
		t.Errorf("same %d kinds %v", same, kinds)
	}
	f := func(uid string, addr int, pers string) scanner.Fixture {
		return scanner.Fixture{UID: uid, Label: uid, Address: addr, Personality: pers}
	}
	fc := compareFixtures([]scanner.Fixture{f("A", 1, "1/2"), f("B", 40, "2/3")}, []scanner.Fixture{f("A", 1, "2/2"), f("C", 80, "1/1")}, "it")
	got := map[string]string{}
	for _, c := range fc {
		got[c.UID] += c.Detail + ";"
	}
	if got["A"] != "modalità 1/2 → 2/2;" || got["B"] != "non risponde più;" || got["C"] != "nuovo;" {
		t.Errorf("fixtures %v", got)
	}
}
