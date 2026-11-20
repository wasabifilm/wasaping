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

var macHiddenNote = TC("Nota: macOS nasconde gli indirizzi MAC dei dispositivi ai programmi non firmati da uno sviluppatore registrato, quindi MAC e produttore restano vuoti.",
	"Note: macOS hides devices' MAC addresses from apps not signed by a registered developer, so MAC and maker stay empty.")

const usage = `wasaping %s — a spicy little IP scanner 🟢

Uso:
  wasaping [scan] [opzioni] [target]   scansiona la rete (default: la tua LAN)
  wasaping web [-addr 127.0.0.1:0]     apre l'interfaccia grafica nel browser
  wasaping app                        come web, ma si chiude con la pagina (doppio clic)
  wasaping interfaces                  elenca le interfacce di rete
  wasaping dhcp                        chi distribuisce gli indirizzi (avvisa se sono due)
  wasaping traffic [-t 6s]             traffico in arrivo: broadcast, multicast, DMX
  wasaping helper install|uninstall    macOS: aiuto per leggere i MAC (password una volta)
  wasaping web-access password|on|off  interfaccia completa da un altro computer (HTTPS e password)

Show:
  wasaping preshow                     check pre-show (esce con 1 se non è pronto)
  wasaping monitor                     controlla i critici e stampa gli eventi
  wasaping dmx [-t 5s] [-u sacn:1]     universi Art-Net/sACN dal vivo
  wasaping rdm                         apparecchi RDM dietro i nodi Art-Net
  wasaping rdm identify IP U UID [off] fa lampeggiare un apparecchio
  wasaping rdm set IP U UID address|mode|label VALORE
                                       cambia indirizzo DMX, modalità o etichetta (chiede conferma)
  wasaping plan import piano.csv       importa il piano IP (plan: confronto)
  wasaping venue export|import FILE    condivide il locale (.wasaping)
  wasaping report [-since 18:00]       report di fine show dal registro
  wasaping box                         Raspberry Pi: monitor sempre acceso + iPad
  wasaping diag                        file di diagnostica da mandare a chi sviluppa wasaping
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
	case "preshow":
		err = runPreshowCLI(ctx, args)
	case "dhcp":
		err = runDHCPCLI(ctx)
	case "traffic":
		err = runTrafficCLI(ctx, args)
	case "rdm":
		err = runRDMCLI(ctx, args)
	case "dmx":
		err = runDMXCLI(ctx, args)
	case "monitor":
		err = runMonitorCLI(ctx, args)
	case "plan":
		err = runPlanCLI(ctx, args)
	case "venue":
		err = runVenueCLI(ctx, args)
	case "report":
		err = runReportCLI(args)
	case "web-access":
		err = runWebAccessCLI(args)
	case "box":
		err = runBox(ctx, args)
	case "diag":
		err = runDiagCLI(ctx)
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
	fs.IntVar(&c.workers, "workers", 256, "host scansionati in parallelo")
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
	fmt.Fprintf(os.Stderr, TC("🟢 wasaping: scansiono %s (%d indirizzi)...\n", "🟢 wasaping: scanning %s (%d addresses)...\n"), target, len(ips))
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
	sess := devices().Begin(ctx, ips)
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
		w.Write([]string{"ip", "nome", "ruolo", "nome_rete", "hostname", "mac", "produttore", "modello", "tipo", "protocolli", "universi", "porte", "metodo", "rtt_ms", "nuovo", "critico", "note"})
		for _, v := range views {
			w.Write([]string{v.IP, v.Name, v.Role, v.Friendly, v.Hostname, v.MAC, v.Vendor, v.Model, v.Kind, strings.Join(v.Protocols, " "), universes(v.Host),
				joinPorts(v.Ports), v.Method, strconv.FormatInt(v.RTTms, 10), strconv.FormatBool(v.New), strconv.FormatBool(v.Critical), v.Note})
		}
		w.Flush()
		return w.Error()
	default:
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, TC("IP\tNOME\tTIPO\tMAC\tMODELLO/PRODUTTORE\tPROTOCOLLI\tPORTE APERTE", "IP\tNAME\tTYPE\tMAC\tMODEL/MAKER\tPROTOCOLS\tOPEN PORTS"))
		for _, v := range views {
			ip := v.IP
			if v.Self {
				ip += TC(" (tu)", " (you)")
			}
			if v.New {
				ip += " ★"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", ip, dash(firstNonEmpty(v.Name, v.Friendly, v.Hostname)), dash(kindName(v.Kind)), dash(v.MAC), dash(firstNonEmpty(v.Model, vendorCLI(v.Vendor))), dash(strings.Join(v.Protocols, ", ")), dash(joinPorts(v.Ports)))
		}
		tw.Flush()
		fmt.Fprintf(os.Stderr, TC("\n%d dispositivi trovati in %s\n", "\n%d devices found in %s\n"), len(hosts), time.Since(start).Round(100*time.Millisecond))
		if len(fresh) > 0 {
			fmt.Fprintf(os.Stderr, TC("★ Nuovi dall'ultima scansione: %s\n", "★ New since the last scan: %s\n"), strings.Join(fresh, ", "))
		}
		for _, v := range views {
			for _, w := range v.Warnings {
				fmt.Fprintf(os.Stderr, "⚠️  %s: %s\n", firstNonEmpty(v.Name, v.Friendly, v.Hostname, v.IP), msgCLI(w))
			}
		}
		for _, w := range scanner.NetworkIssues(hosts) {
			fmt.Fprintln(os.Stderr, "⚠️ ", msgCLI(w))
		}
		if len(gone) > 0 {
			var g []string
			for _, d := range gone {
				g = append(g, fmt.Sprintf("%s (%s)", firstNonEmpty(d.Name, d.Hostname, d.MAC, d.IP), d.IP))
			}
			fmt.Fprintf(os.Stderr, TC("Non rispondono più: %s\n", "No longer responding: %s\n"), strings.Join(g, ", "))
		}
		if scanner.MACsHidden(hosts) {
			fmt.Fprintln(os.Stderr, macHiddenNote)
			if helper == "missing" {
				fmt.Fprintln(os.Stderr, TC("Per vederli: wasaping helper install (chiede la password una sola volta).", "To see them: wasaping helper install (asks for the password only once)."))
			}
		} else if !scanner.HasOUI() {
			fmt.Fprintln(os.Stderr, TC("Suggerimento: esegui `wasaping update-oui` per vedere i produttori dei dispositivi.", "Tip: run `wasaping update-oui` to see the devices' makers."))
		}
	}
	return nil
}

func vendorCLI(v string) string {
	if v == "(MAC privato/casuale)" {
		return TC(v, "(private/random MAC)")
	}
	return v
}

func kindName(k string) string {
	if cliLang == "en" {
		return kindNamesEN[k]
	}
	return kindNames[k]
}

var kindNamesEN = map[string]string{
	scanner.KindRouter: "router", scanner.KindComputer: "computer", scanner.KindPhone: "phone",
	scanner.KindTablet: "tablet", scanner.KindPrinter: "printer", scanner.KindTV: "TV/media",
	scanner.KindSpeaker: "speaker", scanner.KindNAS: "NAS", scanner.KindCamera: "camera",
	scanner.KindIoT: "home automation", scanner.KindLightDesk: "lighting console", scanner.KindLighting: "lighting (node)",
	scanner.KindAudio: "audio", scanner.KindVideo: "video", scanner.KindProjector: "projector", scanner.KindSwitch: "switch",
}

var kindNames = map[string]string{
	scanner.KindRouter: "router", scanner.KindComputer: "computer", scanner.KindPhone: "telefono",
	scanner.KindTablet: "tablet", scanner.KindPrinter: "stampante", scanner.KindTV: "TV/media",
	scanner.KindSpeaker: "altoparlante", scanner.KindNAS: "NAS", scanner.KindCamera: "videocamera",
	scanner.KindIoT: "domotica", scanner.KindLightDesk: "console luci", scanner.KindLighting: "luci (nodo)",
	scanner.KindAudio: "audio", scanner.KindVideo: "video", scanner.KindProjector: "proiettore", scanner.KindSwitch: "switch",
}

// universes summarizes the DMX universes a device sends or receives.
func universes(h scanner.Host) string {
	var p []string
	list := func(label string, u []int) {
		if len(u) > 0 {
			var n []string
			for _, x := range u {
				n = append(n, strconv.Itoa(x))
			}
			p = append(p, label+" "+strings.Join(n, ","))
		}
	}
	if h.ArtNet != nil {
		list("Art-Net out", h.ArtNet.Out)
		list("Art-Net in", h.ArtNet.In)
	}
	if h.SACN != nil {
		list("sACN", h.SACN.Universes)
	}
	return strings.Join(p, "; ")
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
