package main

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"

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
