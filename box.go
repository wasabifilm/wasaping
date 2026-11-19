package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
)

// Box mode: a small always-on computer on the show network (a Raspberry
// Pi): monitor on, automatic scans, the read-only remote view for the
// iPad, and the full interface only on 127.0.0.1 (reach it with an SSH
// tunnel), so nothing that changes the box is exposed on the network.

const boxUIPort = 47480

func runBox(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("box", flag.ExitOnError)
	repeat := fs.Int("repeat", 5, TC("minuti tra una scansione e l'altra", "minutes between scans"))
	every := fs.Int("every", 5, TC("secondi tra un controllo del monitor e l'altro", "seconds between monitor checks"))
	port := fs.Int("port", defaultRemotePort, TC("porta della vista per iPad/telefono", "port of the iPad/phone view"))
	newPin := fs.Bool("new-pin", false, TC("genera un nuovo PIN", "generate a new PIN"))
	fs.Parse(args)

	s := loadSettings()
	s.Repeat, s.MonitorOn, s.MonitorEvery, s.RemoteOn, s.RemotePort = *repeat, true, *every, true, *port
	if s.RemotePIN == "" || *newPin {
		s.RemotePIN = newPIN()
	}
	saveSettings(s)
	selectLocalNetwork(ctx)

	fmt.Println(TC("🟢 wasaping box", "🟢 wasaping box"))
	fmt.Printf(TC("   Monitor su iPad o telefono: %s   PIN %s\n", "   Monitor on iPad or phone: %s   PIN %s\n"), remoteURL(*port), s.RemotePIN)
	fmt.Printf(TC("   Interfaccia completa (dal tuo computer): ssh -L %d:127.0.0.1:%d UTENTE@IP-DEL-BOX, poi http://127.0.0.1:%d/\n",
		"   Full interface (from your computer): ssh -L %d:127.0.0.1:%d USER@BOX-IP, then http://127.0.0.1:%d/\n"), boxUIPort, boxUIPort, boxUIPort)
	if s.WebAccess && s.WebPasswordHash != "" {
		port := s.WebPort
		if port == 0 {
			port = defaultWebPort
		}
		fmt.Printf(TC("   Interfaccia completa con password: %s\n", "   Full interface with password: %s\n"), webAccessURL(port))
	}
	fmt.Printf(TC("   Scansione ogni %d min, monitor ogni %d s.\n", "   Scan every %d min, monitor every %d s.\n"), *repeat, *every)
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		fmt.Println(TC("   Nota: per ascoltare il clock PTP su Linux servono i permessi per le porte basse (vedi docs/raspberry.md).",
			"   Note: on Linux, listening to the PTP clock needs low-port permission (see docs/raspberry.md)."))
	}
	return runWeb(ctx, []string{"-addr", "127.0.0.1:" + strconv.Itoa(boxUIPort), "-no-open"})
}
