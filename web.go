package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"time"
)

//go:embed web
var webFS embed.FS

func runWeb(ctx context.Context, args []string) error {
	fset := flag.NewFlagSet("web", flag.ExitOnError)
	addr := fset.String("addr", "127.0.0.1:0", "indirizzo di ascolto")
	noOpen := fset.Bool("no-open", false, "non aprire il browser automaticamente")
	appMode := fset.Bool("app", false, "modalità app: finestra propria, si chiude con la finestra")
	hidden := fset.Bool("hidden", false, "macOS: parte solo nella barra dei menu, senza finestra (apertura al login)")
	fset.Parse(args)
	ctx, quit := context.WithCancel(ctx)
	defer quit()
	appModeOn = *appMode

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	host, _, _ := net.SplitHostPort(*addr)
	public := !isLoopbackHost(host)
	allowedHosts := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("[::1]:%d", port):     true,
	}

	srv := &http.Server{
		Handler:           newHandler(public, allowedHosts, quit),
		ReadHeaderTimeout: 10 * time.Second,
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	if public {
		fmt.Fprintf(os.Stderr, "⚠️  Attenzione: wasaping è raggiungibile da altri computer su %s, senza password.\n"+
			"   Chiunque sulla rete può usarlo per scansionarla. Usa -addr 127.0.0.1:0 per tenerlo privato.\n", ln.Addr())
	}

	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()

	// macOS: switch on the ARP helper if it is installed. Installing it
	// (password) is the user's choice, from the page.
	helperState.Store(setupARP(false))
	go backgroundLoop(ctx)
	go mon.loop(ctx)
	if err := oscIn.apply(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	go updateLoop(ctx)
	if err := remote.apply(); err != nil {
		fmt.Fprintln(os.Stderr, "Accesso da iPad/telefono non attivo:", err)
	}
	if err := webAcc.apply(); err != nil {
		fmt.Fprintln(os.Stderr, "Accesso completo dalla rete non attivo:", err)
	}
	switch {
	case *appMode && hasNativeWindow:
		uiMode.Store("native")
		// Ctrl+C or /api/quit while the window is open: leave right away.
		go func() { <-ctx.Done(); os.Exit(0) }()
		nativeWindow(url, *hidden) // blocks on the main thread until the user quits
		quit()
	case *appMode:
		if cmd := startAppWindow(url); cmd != nil {
			uiMode.Store("window")
			fmt.Printf("🟢 wasaping è aperto in una finestra: %s\n   Per uscire chiudi la finestra.\n", url)
			go func() {
				began := time.Now()
				cmd.Wait()
				if time.Since(began) > 5*time.Second {
					quit() // the window was ours and it is closed
				} else {
					go watchHeartbeat(ctx, quit) // handed over to a running browser
				}
			}()
		} else {
			uiMode.Store("browser")
			fmt.Printf("🟢 wasaping è aperto nel browser: %s\n   Per uscire usa il pulsante «Esci» nella pagina, o chiudi questa finestra.\n", url)
			openBrowser(url)
			go watchHeartbeat(ctx, quit)
		}
	default:
		uiMode.Store("browser")
		fmt.Printf("🟢 wasaping è pronto su %s  (Ctrl+C per uscire)\n", url)
		if !*noOpen {
			openBrowser(url)
		}
	}
	if err := <-served; err != http.ErrServerClosed {
		return err
	}
	return nil
}

var (
	appModeOn   bool
	uiMode      atomic.Value // "native", "window" or "browser"
	helperState atomic.Value // "notneeded", "installed", "missing"
	lastPing    atomic.Int64
	activeScan  atomic.Int32
)

// watchHeartbeat stops the app when the page has been gone for a while.
// Browsers slow down timers in background tabs (down to once a minute),
// so the grace period is generous.
func watchHeartbeat(ctx context.Context, quit context.CancelFunc) {
	started := time.Now()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if activeScan.Load() > 0 {
			continue
		}
		last := lastPing.Load()
		if last == 0 {
			if time.Since(started) > 3*time.Minute {
				fmt.Println("Nessuna pagina aperta: chiudo wasaping.")
				quit()
			}
			continue
		}
		if time.Since(time.Unix(0, last)) > 150*time.Second {
			fmt.Println("Pagina chiusa: chiudo wasaping.")
			quit()
		}
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Apri manualmente:", url)
	}
}
