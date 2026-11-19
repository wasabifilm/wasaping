package scanner

import (
	"context"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wasabifilm/wasaping/internal/showsim"
)

// TestShowIntegration runs the simulated show gear and checks discovery,
// DMX, RDM and Identify end to end over real sockets.
// Run with: WASAPING_INTEGRATION=1 go test ./scanner -run Integration
func TestShowIntegration(t *testing.T) {
	if os.Getenv("WASAPING_INTEGRATION") == "" {
		t.Skip("set WASAPING_INTEGRATION=1 to run the show integration test")
	}
	ArtNetNodePort = 6455 // the simulated node listens here, replies go to 6454
	defer func() { ArtNetNodePort = 6454 }()
	var identified atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go showsim.Run(ctx, showsim.Config{Backup: true,
		NodeIP: net.IPv4(127, 0, 0, 1), NodePort: 6455, ReplyPort: 6454, DMXTo: net.IPv4(127, 0, 0, 1), NoPTP: true,
		Identify: func(uid [6]byte, on bool) { identified.Store(on) },
	})
	time.Sleep(300 * time.Millisecond)

	t.Run("discovery", func(t *testing.T) {
		got := showDiscover(ctx, nil, 2*time.Second)
		s := got["127.0.0.1"]
		if s == nil || s.artnet == nil || s.artnet.LongName != "Luminode 4 Palco SX" || len(s.artnet.Out) != 4 {
			t.Fatalf("Art-Net node not found: %+v", got)
		}
		sacn := false
		for _, x := range got {
			if x.sacn != nil && strings.HasPrefix(x.sacn.Name, "grandMA3 ") {
				sacn = true
			}
		}
		if !sacn {
			t.Error("sACN source not heard (multicast loopback may be off on this machine)")
		}
	})

	t.Run("dmx", func(t *testing.T) {
		dctx, dcancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		defer dcancel()
		seen := map[string]Universe{}
		ListenDMX(dctx, nil, 4, 500*time.Millisecond, func() DMXWant { return DMXWant{Uni: "art:0"} }, func(us []Universe) {
			for _, u := range us {
				seen[u.ID] = u
			}
		})
		if u, ok := seen["art:0"]; !ok || u.Active == 0 || len(u.Levels) != 512 {
			t.Errorf("Art-Net universe 0 not seen: %+v", seen["art:0"])
		}
	})

	t.Run("dmx-merge", func(t *testing.T) {
		dctx, dcancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		defer dcancel()
		var last Universe
		ListenDMX(dctx, nil, 4, 500*time.Millisecond, func() DMXWant { return DMXWant{Uni: "sacn:2", Channel: 30, All: true} }, func(us []Universe) {
			for _, u := range us {
				if u.ID == "sacn:2" {
					last = u
				}
			}
		})
		if len(last.Sources) != 2 || last.Sources[0].Data == "" || last.Data == "" {
			t.Fatalf("two consoles on sACN 2 expected: %+v", last.Sources)
		}
		// HTP merge at equal priority: FOH sends 200 on 1-24, the backup
		// 100+i on 1-48, so channel 30 is 129 and channel 1 is 200.
		if last.Levels[0] != 200 || last.Levels[29] != 129 || len(last.History) == 0 {
			t.Errorf("merge: ch1 %d ch30 %d history %d", last.Levels[0], last.Levels[29], len(last.History))
		}
		for _, s := range last.Sources {
			if s.Lost != 0 {
				t.Errorf("no frames lost on loopback: %+v", s)
			}
		}
	})

	t.Run("dante", func(t *testing.T) {
		old := DanteARCPort
		DanteARCPort = 14440
		defer func() { DanteARCPort = old }()
		dctx, dcancel := context.WithCancel(ctx)
		defer dcancel()
		go showsim.RunDante(dctx, "127.0.0.1:14440")
		time.Sleep(100 * time.Millisecond)
		devs := DanteRouting(ctx, []string{"127.0.0.1"})
		if len(devs) != 1 || devs[0].Name != showsim.DanteName || devs[0].TxCount != 4 || len(devs[0].Tx) != 4 || devs[0].Tx[1].Name != "Mic 2" || len(devs[0].Rx) != 4 {
			t.Fatalf("dante: %+v", devs)
		}
		rx := devs[0].Rx
		if rx[0].TxChan != "Mix L" || rx[0].TxDevice != "CL5-FOH" || rx[0].State != "ok" ||
			rx[1].State != "bad" || rx[2].State != "none" || rx[3].State != "bad" || rx[3].Detail != "dominio di clock diverso" {
			t.Errorf("rx: %+v", rx)
		}
		if s := SummarizeDante(devs, nil); s.OK != 1 || s.Bad != 2 || s.TxMbps["CL5-FOH"] != 1.5 {
			t.Errorf("summary %+v", s)
		}
	})

	t.Run("video", func(t *testing.T) {
		ports := showsim.VideoPorts{Videohub: 19990, HyperDeck: 19993, ATEM: 19910, VMix: 18088}
		VideohubPort, HyperDeckPort, ATEMPort, VMixPort = ports.Videohub, ports.HyperDeck, ports.ATEM, ports.VMix
		defer func() { VideohubPort, HyperDeckPort, ATEMPort, VMixPort = 9990, 9993, 9910, 8088 }()
		vctx, vcancel := context.WithCancel(ctx)
		defer vcancel()
		if err := showsim.RunVideo(vctx, "127.0.0.1", ports); err != nil {
			t.Fatal(err)
		}
		vh := VideoStatus(ctx, "127.0.0.1", "Videohub")
		if vh.Err != "" || len(vh.Routes) != 4 || vh.Routes[0].OutLbl != "LED Wall" || vh.Routes[0].InLbl != "Grafica" || !vh.Routes[0].Locked {
			t.Errorf("videohub %+v", vh)
		}
		hd := VideoStatus(ctx, "127.0.0.1", "HyperDeck")
		if hd.Err != "" || !strings.Contains(strings.Join(hd.Lines, "|"), "IN REGISTRAZIONE") || len(hd.Warn) != 1 {
			t.Errorf("hyperdeck %+v", hd)
		}
		at := VideoStatus(ctx, "127.0.0.1", "ATEM")
		if at.Err != "" || !strings.HasPrefix(at.Model, "ATEM Television Studio") || at.Version != "2.30" ||
			!strings.Contains(strings.Join(at.Lines, "|"), "M/E 1 programma: Camera 2") {
			t.Errorf("atem %+v", at)
		}
		vm := VideoStatus(ctx, "127.0.0.1", "vMix")
		if vm.Err != "" || vm.Model != "vMix 4K" || vm.Lines[0] != "In onda: Intro.mp4" || !strings.Contains(strings.Join(vm.Lines, "|"), "registra") {
			t.Errorf("vmix %+v", vm)
		}
	})

	t.Run("rdm", func(t *testing.T) {
		fx := RDMDiscover(ctx, map[string][]int{"127.0.0.1": {0}})
		if len(fx) != 3 || fx[0].Label != "Spot SX 1" || fx[1].Address != 40 || fx[2].Maker != "Ayrton" || fx[0].Personality != "2/3" {
			t.Fatalf("fixtures %+v", fx)
		}
		if err := RDMIdentify(ctx, "127.0.0.1", 0, fx[2].UID, true); err != nil || !identified.Load() {
			t.Errorf("identify: %v (callback %v)", err, identified.Load())
		}
		// Change the address of the overlapping fixture, then read it back.
		if err := RDMSet(ctx, "127.0.0.1", 0, fx[2].UID, "address", "101"); err != nil {
			t.Fatalf("set address: %v", err)
		}
		if err := RDMSet(ctx, "127.0.0.1", 0, fx[2].UID, "label", "Wash palco"); err != nil {
			t.Fatalf("set label: %v", err)
		}
		f, err := RDMInfo(ctx, "127.0.0.1", 0, fx[2].UID)
		if err != nil || f.Address != 101 || f.Label != "Wash palco" {
			t.Errorf("after set: %+v %v", f, err)
		}
		if err := RDMSet(ctx, "127.0.0.1", 0, fx[2].UID, "address", "600"); err == nil {
			t.Error("address 600 should be refused")
		}
	})
}
