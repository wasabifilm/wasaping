package main

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/wasabifilm/wasaping/scanner"
)

func TestStoreNewAndGone(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "devices.json"), Devices: map[string]*Device{}}
	targets := []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.2"), net.ParseIP("192.168.1.3")}
	router := scanner.Host{IP: "192.168.1.1", MAC: "98:25:4a:ae:41:91"}
	tv := scanner.Host{IP: "192.168.1.2"}

	// First scan ever: nothing is flagged as new.
	ss := s.Begin()
	if v := ss.View(router); v.New {
		t.Error("first scan should not flag devices as new")
	}
	ss.View(tv)
	ss.Finish(targets)
	if !s.Label("ip:192.168.1.2", "TV salotto", "") {
		t.Fatal("label failed")
	}

	// Second scan: the TV now has a MAC (keeps its name), a phone is new,
	// the router is gone.
	ss = s.Begin()
	v := ss.View(scanner.Host{IP: "192.168.1.2", MAC: "aa:bb:cc:00:11:22"})
	if v.New || v.Name != "TV salotto" {
		t.Errorf("TV: new=%v name=%q", v.New, v.Name)
	}
	if v := ss.View(scanner.Host{IP: "192.168.1.3"}); !v.New {
		t.Error("phone should be new")
	}
	gone := ss.Finish(targets)
	if len(gone) != 1 || gone[0].Key != router.MAC {
		t.Errorf("gone = %+v", gone)
	}
}
