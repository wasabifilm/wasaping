package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Command-line versions of the show tools: same logic and same archive as
// the window, for SSH sessions and the Raspberry Pi box.

// selectLocalNetwork makes the network of the default interface the
// current one in the archive (a fresh process does not know it yet).
func selectLocalNetwork(ctx context.Context) (NetView, bool) {
	i, err := scanner.DefaultInterface()
	if err != nil {
		return NetView{}, false
	}
	ips, err := scanner.ParseTargets(clampNetwork(i))
	if err != nil {
		return NetView{}, false
	}
	id, _, _ := networkID(ctx, ips)
	if !devices().Select(id) {
		return NetView{}, false
	}
	return devices().Current()
}

func mark(class string) string {
	switch class {
	case "ok":
		return "✓"
	case "bad":
		return "✗"
	}
	return "⚠"
}

func runPreshowCLI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("preshow", flag.ExitOnError)
	asJSON := fs.Bool("json", false, TC("risultato in JSON", "output as JSON"))
	fs.Parse(args)
	selectLocalNetwork(ctx)
	fmt.Fprintln(os.Stderr, TC("🟢 wasaping: check pre-show in corso…", "🟢 wasaping: pre-show check running…"))
	rep, err := runPreshow(ctx, "", cliLang)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(rep)
	} else {
		fmt.Printf("\n%s %s — %s\n%s · %s\n", mark(rep.Verdict), rep.Headline, rep.Summary, rep.Network, rep.At.Format("02/01/2006 15:04"))
		for _, s := range rep.Sections {
			fmt.Printf("\n%s\n", strings.ToUpper(s.Title))
			if s.Note != "" {
				fmt.Println("  " + s.Note)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for _, r := range s.Rows {
				fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n", mark(r.Class), r.Name, r.Role, r.IP, r.Detail)
			}
			tw.Flush()
		}
	}
	if rep.Verdict == "bad" {
		os.Exit(1) // for scripts: not ready
	}
	return nil
}

func runRDMCLI(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "set" {
		// wasaping rdm set <node> <universe> <uid> address|mode|label <value> [-y]
		if len(args) < 6 {
			return fmt.Errorf(TC("uso: wasaping rdm set <IP nodo> <universo> <UID> address|mode|label <valore> [-y]", "usage: wasaping rdm set <node IP> <universe> <UID> address|mode|label <value> [-y]"))
		}
		u, err := strconv.Atoi(args[2])
		if err != nil {
			return err
		}
		uid := strings.ToUpper(args[3])
		before, err := scanner.RDMInfo(ctx, args[1], u, uid)
		if err != nil {
			return err
		}
		fmt.Printf(TC("%s (%s %s): %s da %q a %q\n", "%s (%s %s): %s from %q to %q\n"), before.Label, before.Maker, before.Model, args[4], rdmField(before, args[4]), args[5])
		if !(len(args) > 6 && args[6] == "-y") {
			fmt.Print(TC("Cambia davvero l'apparecchio. Confermi? [s/N] ", "This really changes the fixture. Confirm? [y/N] "))
			var ans string
			fmt.Scanln(&ans)
			if a := strings.ToLower(ans); a != "s" && a != "si" && a != "sì" && a != "y" && a != "yes" {
				return fmt.Errorf(TC("annullato", "cancelled"))
			}
		}
		if _, err := rdmChange(ctx, args[1], u, uid, args[4], args[5]); err != nil {
			return err
		}
		fmt.Println(TC("✓ cambiato e verificato", "✓ changed and verified"))
		return nil
	}
	if len(args) > 0 && args[0] == "identify" {
		// wasaping rdm identify <node> <universe> <uid> [off]
		if len(args) < 4 {
			return fmt.Errorf(TC("uso: wasaping rdm identify <IP nodo> <universo> <UID> [off]", "usage: wasaping rdm identify <node IP> <universe> <UID> [off]"))
		}
		u, err := strconv.Atoi(args[2])
		if err != nil {
			return err
		}
		on := !(len(args) > 4 && args[4] == "off")
		if err := scanner.RDMIdentify(ctx, args[1], u, strings.ToUpper(args[3]), on); err != nil {
			return err
		}
		fmt.Println(TC("✓ fatto", "✓ done"))
		return nil
	}
	fmt.Fprintln(os.Stderr, TC("🟢 wasaping: cerco i nodi Art-Net…", "🟢 wasaping: looking for Art-Net nodes…"))
	nodes := scanner.FindArtNetNodes(ctx, 2*time.Second)
	if len(nodes) == 0 {
		return fmt.Errorf(TC("nessun nodo Art-Net risponde", "no Art-Net node answers"))
	}
	req := map[string][]int{}
	for ip, n := range nodes {
		u := append(append([]int{}, n.Out...), n.In...)
		if len(u) == 0 {
			u = []int{0}
		}
		req[ip] = u
	}
	fmt.Fprintf(os.Stderr, TC("%d nodi: chiedo gli apparecchi RDM…\n", "%d nodes: asking for RDM fixtures…\n"), len(nodes))
	fx := scanner.RDMDiscover(ctx, req)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, TC("UNIVERSO\tINDIRIZZO\tCANALI\tETICHETTA\tPRODUTTORE\tMODELLO\tMODALITÀ\tUID\tNODO", "UNIVERSE\tADDRESS\tCHANNELS\tLABEL\tMAKER\tMODEL\tMODE\tUID\tNODE"))
	for _, f := range fx {
		addr := "-"
		if f.Address > 0 {
			addr = fmt.Sprintf("%d-%d", f.Address, f.Address+max(f.Footprint, 1)-1)
		}
		fmt.Fprintf(tw, "%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n", f.Universe, addr, f.Footprint, dash(f.Label), dash(f.Maker), dash(f.Model), dash(f.Personality), f.UID, f.Node)
	}
	tw.Flush()
	// Overlapping addresses on the same universe.
	for i, a := range fx {
		for _, b := range fx[i+1:] {
			if a.Universe == b.Universe && a.Address > 0 && b.Address > 0 &&
				a.Address <= b.Address+max(b.Footprint, 1)-1 && b.Address <= a.Address+max(a.Footprint, 1)-1 {
				fmt.Printf(TC("⚠️  Indirizzi sovrapposti nell'universo %d: %s e %s\n", "⚠️  Overlapping addresses in universe %d: %s and %s\n"), a.Universe, firstNonEmpty(a.Label, a.UID), firstNonEmpty(b.Label, b.UID))
			}
		}
	}
	return nil
}

func runDMXCLI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("dmx", flag.ExitOnError)
	dur := fs.Duration("t", 5*time.Second, TC("per quanto ascoltare", "how long to listen"))
	uni := fs.String("u", "", TC("mostra i 512 canali di un universo (es. sacn:1, art:0)", "show the 512 channels of a universe (e.g. sacn:1, art:0)"))
	fs.Parse(args)
	fmt.Fprintf(os.Stderr, TC("🟢 wasaping: ascolto Art-Net e sACN per %s…\n", "🟢 wasaping: listening to Art-Net and sACN for %s…\n"), *dur)
	lctx, cancel := context.WithTimeout(ctx, *dur)
	defer cancel()
	var last []scanner.Universe
	scanner.ListenDMX(lctx, defaultIface(), 64, 500*time.Millisecond, func() scanner.DMXWant { return scanner.DMXWant{Uni: *uni} }, func(u []scanner.Universe) { last = u })
	if len(last) == 0 {
		fmt.Println(TC("Nessun universo in arrivo.", "No universes arriving."))
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, TC("UNIVERSO\tPROTOCOLLO\tSORGENTE\tFRAME/S\tCANALI ATTIVI", "UNIVERSE\tPROTOCOL\tSOURCE\tFRAMES/S\tACTIVE CHANNELS"))
	for _, u := range last {
		var src, fps []string
		for _, s := range u.Sources {
			src = append(src, firstNonEmpty(s.Name, s.IP))
			fps = append(fps, fmt.Sprintf("%.0f", s.FPS))
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%d/512\n", u.Number, u.Protocol, strings.Join(src, ", "), strings.Join(fps, "/"), u.Active)
	}
	tw.Flush()
	for _, u := range last {
		if u.ID == *uni && len(u.Levels) == 512 {
			fmt.Printf("\n%s %d:\n", u.Protocol, u.Number)
			for i := 0; i < 512; i += 16 {
				fmt.Printf("%3d |", i+1)
				for _, v := range u.Levels[i : i+16] {
					fmt.Printf(" %3d", v)
				}
				fmt.Println()
			}
		}
	}
	return nil
}

// runMonitorCLI watches the critical devices and universes of the local
// network and prints every event, until Ctrl+C.
func runMonitorCLI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	every := fs.Int("every", 5, TC("secondi tra un controllo e l'altro", "seconds between checks"))
	fs.Parse(args)
	n, ok := selectLocalNetwork(ctx)
	crit, unis := devices().CriticalDevices(), devices().CriticalUniverses()
	if !ok || len(crit)+len(unis) == 0 {
		return fmt.Errorf(TC("nessun dispositivo o universo critico su questa rete: segnali dalla finestra o importa un locale (wasaping venue import)",
			"no critical device or universe on this network: mark them in the window or import a venue (wasaping venue import)"))
	}
	fmt.Printf(TC("🟢 wasaping: monitor di %d dispositivi e %d universi su %s, ogni %d s (Ctrl+C per uscire)\n", "🟢 wasaping: watching %d devices and %d universes on %s, every %d s (Ctrl+C to quit)\n"),
		len(crit), len(unis), firstNonEmpty(n.Name, n.Auto), *every)
	mon.mu.Lock()
	mon.on, mon.every = true, max(2, *every)
	mon.mu.Unlock()
	mon.startListeners(ctx)
	seen := 0
	for ctx.Err() == nil {
		mon.check(ctx)
		st := mon.state(1000)
		for _, e := range st.Events {
			if e.ID <= seen {
				continue
			}
			seen = e.ID
			what := TC("non risponde", "not responding")
			switch {
			case e.Text != "":
				what = e.Text
			case e.Up:
				what = fmt.Sprintf(TC("di nuovo in linea dopo %s", "back online after %s"), shortDur(e.DownFor))
			}
			fmt.Printf("%s  %s  %s\n", e.At.Format("15:04:05"), e.Name, what)
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Duration(mon.every) * time.Second):
		}
	}
	return nil
}

func runPlanCLI(ctx context.Context, args []string) error {
	if _, ok := selectLocalNetwork(ctx); !ok {
		return fmt.Errorf(TC("rete sconosciuta: fai prima una scansione (wasaping)", "unknown network: scan first (wasaping)"))
	}
	if len(args) >= 2 && args[0] == "import" {
		b, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		plan, err := parsePlan(b)
		if err != nil {
			return err
		}
		devices().SetPlan(plan)
		fmt.Printf(TC("✓ piano importato: %d dispositivi\n", "✓ plan imported: %d devices\n"), len(plan))
		return nil
	}
	plan := devices().Plan()
	if len(plan) == 0 {
		fmt.Println(TC("Nessun piano IP. Per importarlo: wasaping plan import piano.csv", "No IP plan. To import one: wasaping plan import plan.csv"))
		return nil
	}
	fmt.Fprintln(os.Stderr, TC("🟢 wasaping: scansiono e confronto con il piano…", "🟢 wasaping: scanning and comparing with the plan…"))
	i, _ := scanner.DefaultInterface()
	t := clampNetwork(i)
	ips, _ := scanner.ParseTargets(t)
	res, err := runCollect(ctx, t, ips, scanner.Options{Ping: true, ResolveDNS: true}, false, nil, nil)
	if err != nil {
		return err
	}
	pc := comparePlan(plan, res.Hosts, cliLang)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range pc.Rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", mark(r.Class), dash(r.Name), r.IP, dash(r.Role), r.Status)
	}
	for _, h := range pc.Extra {
		fmt.Fprintf(tw, "⚠\t%s\t%s\t\t%s\n", hostLabel(h), h.IP, TC("non previsto", "not planned"))
	}
	tw.Flush()
	return nil
}

func runVenueCLI(ctx context.Context, args []string) error {
	if _, ok := selectLocalNetwork(ctx); !ok {
		return fmt.Errorf(TC("rete sconosciuta: fai prima una scansione (wasaping)", "unknown network: scan first (wasaping)"))
	}
	switch {
	case len(args) >= 2 && args[0] == "import":
		b, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		venues, _, err := parseProject(b) // a project (0.8) or a venue file
		if err != nil {
			return err
		}
		total := 0
		for _, v := range venues {
			n, _, err := devices().ImportVenueInto(v, "own")
			if err != nil {
				return err
			}
			total += n
		}
		fmt.Printf(TC("✓ importato: %d reti, %d dispositivi\n", "✓ imported: %d networks, %d devices\n"), len(venues), total)
	case len(args) >= 1 && args[0] == "export":
		v, _ := devices().ExportVenue()
		b, _ := json.MarshalIndent(v, "", "  ")
		if len(args) >= 2 {
			return os.WriteFile(args[1], b, 0o644)
		}
		os.Stdout.Write(append(b, '\n'))
	default:
		return fmt.Errorf(TC("uso: wasaping venue export [file] | import file", "usage: wasaping venue export [file] | import file"))
	}
	return nil
}

func runReportCLI(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	sinceS := fs.String("since", "", TC("da quando: 18:00, 6h o 2026-11-18 18:00 (default: 12 ore fa)", "since: 18:00, 6h or 2026-11-18 18:00 (default: 12 hours ago)"))
	fs.Parse(args)
	since, ok := parseSince(*sinceS, time.Now())
	if !ok {
		return fmt.Errorf(TC("ora non valida", "invalid time"))
	}
	rep, err := buildShowReport(since)
	if err != nil {
		return err
	}
	fmt.Printf(TC("Report dalle %s alle %s: %d cadute\n", "Report from %s to %s: %d drops\n"), rep.Since.Format("02/01 15:04"), rep.Until.Format("02/01 15:04"), rep.Drops)
	if len(rep.Devices) > 0 {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, TC("\nDISPOSITIVO\tCADUTE\tGIÙ IN TUTTO\tIL PIÙ LUNGO", "\nDEVICE\tDROPS\tTOTAL DOWN\tLONGEST"))
		for _, d := range rep.Devices {
			still := ""
			if d.StillDown {
				still = TC(" (ancora giù)", " (still down)")
			}
			fmt.Fprintf(tw, "%s%s\t%d\t%s\t%s\n", d.Name, still, d.Drops, shortDur(d.DownTotal), shortDur(d.Longest))
		}
		tw.Flush()
	}
	if len(rep.RTT) > 0 {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, TC("\nTEMPI DI RISPOSTA\tMEDIA\tMASSIMO\tCONTROLLI SENZA RISPOSTA", "\nRESPONSE TIMES\tAVERAGE\tMAXIMUM\tCHECKS WITHOUT ANSWER"))
		for _, s := range rep.RTT {
			fmt.Fprintf(tw, "%s\t%.0f ms\t%d ms\t%.1f%%\n", s.Name, s.Avg, s.Max, s.Loss)
		}
		tw.Flush()
	}
	sort.Slice(rep.Events, func(i, j int) bool { return rep.Events[i].At.Before(rep.Events[j].At) })
	if len(rep.Events) > 0 {
		fmt.Println(TC("\nCRONOLOGIA", "\nTIMELINE"))
		for _, e := range rep.Events {
			fmt.Printf("%s  %s  %s\n", e.At.Format("02/01 15:04:05"), e.Name, e.Event)
		}
	}
	return nil
}
