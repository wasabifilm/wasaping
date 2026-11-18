// wasaping — a spicy little cross-platform IP scanner (https://github.com/wasabifilm/wasaping).
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

var version = "dev"

const macHiddenNote = "Nota: macOS nasconde gli indirizzi MAC dei dispositivi ai programmi non firmati da uno sviluppatore registrato, quindi MAC e produttore restano vuoti."

const usage = `wasaping %s — a spicy little IP scanner 🟢

Uso:
  wasaping [scan] [opzioni] [target]   scansiona la rete (default: la tua LAN)
  wasaping web [-addr 127.0.0.1:0]     apre l'interfaccia grafica nel browser
  wasaping app                        come web, ma si chiude con la pagina (doppio clic)
  wasaping interfaces                  elenca le interfacce di rete
  wasaping helper install|uninstall    macOS: aiuto per leggere i MAC (password una volta)
  wasaping update-oui                  scarica il database dei produttori (IEEE)
  wasaping version

Target: 192.168.1.0/24 · 192.168.1.10-50 · 10.0.0.1,10.0.0.7

Opzioni di scan:
`

func main() {
	var args []string
	for _, a := range os.Args[1:] {
		if !strings.HasPrefix(a, "-psn_") { // process serial number added by old macOS
			args = append(args, a)
		}
	}
	cmd := "scan"
	if len(args) == 0 && launchedByDoubleClick() {
		cmd = "app"
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && !looksLikeTarget(args[0]) {
		cmd, args = args[0], args[1:]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch cmd {
	case "scan":
		err = runScan(ctx, args)
	case "web", "gui":
		err = runWeb(ctx, args)
	case "app":
		err = runWeb(ctx, append([]string{"-app"}, args...))
	case "helper":
		err = runHelper(args)
	case "interfaces", "if":
		err = runInterfaces()
	case "update-oui":
		fmt.Fprintln(os.Stderr, "Scarico", scanner.OUIURL, "...")
		var n int
		if n, err = scanner.UpdateOUI(ctx); err == nil {
			fmt.Printf("OK: %d produttori salvati in %s\n", n, scanner.OUIPath())
		}
	case "version", "--version":
		fmt.Println("wasaping", version)
	case "help", "-h", "--help":
		fmt.Printf(usage, version)
		scanFlags(new(scanConfig)).PrintDefaults()
	default:
		err = fmt.Errorf("comando sconosciuto %q (prova: wasaping help)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "errore:", err)
		os.Exit(1)
	}
}

func looksLikeTarget(s string) bool {
	return len(s) > 0 && s[0] >= '0' && s[0] <= '9'
}

type scanConfig struct {
	iface   string
	timeout time.Duration
	workers int
	ports   string
	noPing  bool
	noDNS   bool
	format  string
}

func scanFlags(c *scanConfig) *flag.FlagSet {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	fs.StringVar(&c.iface, "i", "", "interfaccia da usare (es. en0, eth0, Wi-Fi)")
	fs.DurationVar(&c.timeout, "timeout", 800*time.Millisecond, "timeout per singolo tentativo")
	fs.IntVar(&c.workers, "workers", 128, "host scansionati in parallelo")
	fs.StringVar(&c.ports, "p", "common", `porte da controllare sugli host attivi: "common", "none" o lista "22,80,443"`)
	fs.BoolVar(&c.noPing, "no-ping", false, "non usare il comando ping di sistema")
	fs.BoolVar(&c.noDNS, "no-dns", false, "non risolvere i nomi host")
	fs.StringVar(&c.format, "o", "table", "formato output: table, json, csv")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, version)
		fs.PrintDefaults()
	}
	return fs
}

func parsePorts(s string) ([]int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none", "0":
		return nil, nil
	case "common":
		return scanner.CommonPorts, nil
	}
	var out []int
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if a, b, ok := strings.Cut(p, "-"); ok {
			lo, e1 := strconv.Atoi(a)
			hi, e2 := strconv.Atoi(b)
			if e1 != nil || e2 != nil || lo < 1 || hi > 65535 || lo > hi {
				return nil, fmt.Errorf("intervallo porte non valido %q", p)
			}
			for i := lo; i <= hi; i++ {
				out = append(out, i)
			}
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("porta non valida %q", p)
		}
		out = append(out, n)
	}
	if len(out) > 2048 {
		return nil, fmt.Errorf("troppe porte (massimo 2048)")
	}
	return out, nil
}

// resolveTarget returns the target spec: explicit, from -i, or the default LAN.
func resolveTarget(spec, iface string) (string, error) {
	if spec != "" {
		return spec, nil
	}
	if iface != "" {
		ifs, err := scanner.LocalInterfaces()
		if err != nil {
			return "", err
		}
		for _, i := range ifs {
			if i.Name == iface {
				return clampNetwork(i), nil
			}
		}
		return "", fmt.Errorf("interfaccia %q non trovata (vedi: wasaping interfaces)", iface)
	}
	i, err := scanner.DefaultInterface()
	if err != nil {
		return "", err
	}
	return clampNetwork(i), nil
}

// clampNetwork limits very large networks (e.g. /16) to the /24 around our IP.
func clampNetwork(i scanner.Interface) string {
	var a, b, c, d, bits int
	if _, err := fmt.Sscanf(i.Network, "%d.%d.%d.%d/%d", &a, &b, &c, &d, &bits); err == nil && bits < 20 {
		parts := strings.Split(i.IP, ".")
		return fmt.Sprintf("%s.%s.%s.0/24", parts[0], parts[1], parts[2])
	}
	return i.Network
}

func runScan(ctx context.Context, args []string) error {
	var c scanConfig
	fs := scanFlags(&c)
	fs.Parse(args)
	target, err := resolveTarget(strings.Join(fs.Args(), ","), c.iface)
	if err != nil {
		return err
	}
	ips, err := scanner.ParseTargets(target)
	if err != nil {
		return err
	}
	ports, err := parsePorts(c.ports)
	if err != nil {
		return err
	}
	helper := setupARP(false)
	tty := c.format == "table" && isTerminal(os.Stderr)
	fmt.Fprintf(os.Stderr, "🟢 wasaping: scansiono %s (%d indirizzi)...\n", target, len(ips))
	start := time.Now()
	hosts, err := scanner.Scan(ctx, scanner.Options{
		Targets:    ips,
		Timeout:    c.timeout,
		Workers:    c.workers,
		Ping:       !c.noPing,
		ScanPorts:  ports,
		ResolveDNS: !c.noDNS,
	}, nil, func(p scanner.Progress) {
		if tty && (p.Done%8 == 0 || p.Done == p.Total) {
			fmt.Fprintf(os.Stderr, "\r  %d/%d", p.Done, p.Total)
		}
	})
	if tty {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
	if err != nil {
		return err
	}
	sess := devices().Begin()
	views := make([]HostView, len(hosts))
	var fresh []string
	for i, h := range hosts {
		views[i] = sess.View(h)
		if views[i].New {
			fresh = append(fresh, firstNonEmpty(views[i].Name, views[i].Hostname, h.IP))
		}
	}
	gone := sess.Finish(ips)
	switch c.format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(views)
	case "csv":
		w := csv.NewWriter(os.Stdout)
		w.Write([]string{"ip", "nome", "hostname", "mac", "vendor", "tipo", "ports", "method", "rtt_ms", "new", "note"})
		for _, v := range views {
			w.Write([]string{v.IP, v.Name, v.Hostname, v.MAC, v.Vendor, v.Kind, joinPorts(v.Ports), v.Method, strconv.FormatInt(v.RTTms, 10), strconv.FormatBool(v.New), v.Note})
		}
		w.Flush()
		return w.Error()
	default:
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "IP\tNOME\tTIPO\tMAC\tPRODUTTORE\tPORTE APERTE")
		for _, v := range views {
			ip := v.IP
			if v.Self {
				ip += " (tu)"
			}
			if v.New {
				ip += " ★"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", ip, dash(firstNonEmpty(v.Name, v.Hostname)), dash(kindNames[v.Kind]), dash(v.MAC), dash(v.Vendor), dash(joinPorts(v.Ports)))
		}
		tw.Flush()
		fmt.Fprintf(os.Stderr, "\n%d dispositivi trovati in %s\n", len(hosts), time.Since(start).Round(100*time.Millisecond))
		if len(fresh) > 0 {
			fmt.Fprintf(os.Stderr, "★ Nuovi dall'ultima scansione: %s\n", strings.Join(fresh, ", "))
		}
		if len(gone) > 0 {
			var g []string
			for _, d := range gone {
				g = append(g, fmt.Sprintf("%s (%s)", firstNonEmpty(d.Name, d.Hostname, d.MAC, d.IP), d.IP))
			}
			fmt.Fprintf(os.Stderr, "Non rispondono più: %s\n", strings.Join(g, ", "))
		}
		if scanner.MACsHidden(hosts) {
			fmt.Fprintln(os.Stderr, macHiddenNote)
			if helper == "missing" {
				fmt.Fprintln(os.Stderr, "Per vederli: wasaping helper install (chiede la password una sola volta).")
			}
		} else if !scanner.HasOUI() {
			fmt.Fprintln(os.Stderr, "Suggerimento: esegui `wasaping update-oui` per vedere i produttori dei dispositivi.")
		}
	}
	return nil
}

var kindNames = map[string]string{
	scanner.KindRouter: "router", scanner.KindComputer: "computer", scanner.KindPhone: "telefono",
	scanner.KindTablet: "tablet", scanner.KindPrinter: "stampante", scanner.KindTV: "TV/media",
	scanner.KindSpeaker: "altoparlante", scanner.KindNAS: "NAS", scanner.KindCamera: "videocamera",
	scanner.KindIoT: "domotica",
}

func runHelper(args []string) error {
	if !helperSupported {
		fmt.Println("Su questo sistema l'aiuto non serve: i MAC si leggono già.")
		return nil
	}
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "install":
		if err := installHelper(); err != nil {
			return err
		}
		fmt.Println("✓ Aiuto installato: wasaping ora vede i MAC senza chiedere più la password.")
	case "uninstall", "remove":
		if err := uninstallHelper(); err != nil {
			return err
		}
		fmt.Println("✓ Aiuto rimosso.")
	case "status":
		if helperInstalled() {
			fmt.Println("Aiuto per i MAC: installato (" + helperPlist + ")")
		} else {
			fmt.Println("Aiuto per i MAC: non installato. Per installarlo: wasaping helper install")
		}
	default:
		return fmt.Errorf("uso: wasaping helper [install|uninstall|status]")
	}
	return nil
}

func runInterfaces() error {
	ifs, err := scanner.LocalInterfaces()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NOME\tIP\tRETE\tMAC")
	for _, i := range ifs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", i.Name, i.IP, i.Network, dash(i.MAC))
	}
	return tw.Flush()
}

func joinPorts(p []int) string {
	s := make([]string, len(p))
	for i, v := range p {
		s[i] = strconv.Itoa(v)
	}
	return strings.Join(s, ",")
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
