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
	flag.Parse()
	addr := net.ParseIP(*ip).To4()
	if addr == nil {
		fmt.Fprintln(os.Stderr, "indirizzo non valido:", *ip)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Printf("showsim: nodo Art-Net %s (3 apparecchi RDM), sACN universi 1-2, Art-Net universo 0, PTP master. Ctrl+C per uscire.\n", addr)
	err := showsim.Run(ctx, showsim.Config{NodeIP: addr, NoPTP: *noPTP, Identify: func(uid [6]byte, on bool) {
		fmt.Printf("RDM identifica %X: %v\n", uid, on)
	}})
	if err != nil {
		fmt.Fprintln(os.Stderr, "showsim:", err)
		os.Exit(1)
	}
}
