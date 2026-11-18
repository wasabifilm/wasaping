package main

import (
	"context"
	"net"
	"path/filepath"
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
	if !s.Label("ip:192.168.77.2", "TV salotto", "Monitor palco", "", true) {
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
