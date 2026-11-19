package scanner

import (
	"context"
	"net"
	"os"
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
	go showsim.Run(ctx, showsim.Config{
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
			if x.sacn != nil && x.sacn.Name == "grandMA3 FOH (simulatore)" {
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
		ListenDMX(dctx, nil, 4, 500*time.Millisecond, func() string { return "art:0" }, func(us []Universe) {
			for _, u := range us {
				seen[u.ID] = u
			}
		})
		if u, ok := seen["art:0"]; !ok || u.Active == 0 || len(u.Levels) != 512 {
			t.Errorf("Art-Net universe 0 not seen: %+v", seen["art:0"])
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
	})
}
