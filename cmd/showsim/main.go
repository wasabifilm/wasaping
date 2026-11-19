// Command showsim simulates show gear on the network to test wasaping
// without hardware: an Art-Net node at 2.0.0.10 with three RDM fixtures
// (two with overlapping addresses), a console sending sACN on universes
// 1-2 and Art-Net DMX on universe 0, and a PTPv1 clock master.
//
//	go run ./cmd/showsim             # node on the "wrong" subnet 2.0.0.10
//	go run ./cmd/showsim -ip 192.168.1.20
//
// For RDM the node must announce an address wasaping can reach (-ip with
// this computer's address).
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/wasabifilm/wasaping/internal/showsim"
)

func main() {
	ip := flag.String("ip", "2.0.0.10", "address the Art-Net node announces")
	noPTP := flag.Bool("no-ptp", false, "do not simulate a PTP clock master")
	backup := flag.Bool("backup", true, "a second console on sACN universe 2 (same priority)")
	video := flag.Bool("video", true, "Videohub, HyperDeck, ATEM and vMix on their usual ports")
	dante := flag.Bool("dante", true, "a Dante device answering ARC on UDP 4440 (routing, read-only)")
	flag.Parse()
	addr := net.ParseIP(*ip).To4()
	if addr == nil {
		fmt.Fprintln(os.Stderr, "indirizzo non valido:", *ip)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Printf("showsim: nodo Art-Net %s (3 apparecchi RDM), sACN universi 1-2, Art-Net universo 0, PTP master. Ctrl+C per uscire.\n", addr)
	if *video {
		if err := showsim.RunVideo(ctx, "0.0.0.0", showsim.DefaultVideoPorts); err != nil {
			fmt.Fprintln(os.Stderr, "showsim: video:", err)
		} else {
			fmt.Println("showsim: Videohub 9990, HyperDeck 9993, ATEM 9910, vMix 8088 (in wasaping: Strumenti → Video)")
		}
	}
	if *dante {
		go func() {
			if err := showsim.RunDante(ctx, "0.0.0.0:4440"); err != nil {
				fmt.Fprintln(os.Stderr, "showsim: Dante (porta 4440):", err)
			}
		}()
		fmt.Println("showsim: dispositivo Dante", showsim.DanteName, "su UDP 4440 (in wasaping: Strumenti → Dante, IP di questo computer)")
	}
	err := showsim.Run(ctx, showsim.Config{NodeIP: addr, NoPTP: *noPTP, Backup: *backup, Identify: func(uid [6]byte, on bool) {
		fmt.Printf("RDM identifica %X: %v\n", uid, on)
	}})
	if err != nil {
		fmt.Fprintln(os.Stderr, "showsim:", err)
		os.Exit(1)
	}
}
