package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// IP plan comparison and the pre-show check, in one place for the page,
// the command line and the tests.

// L picks the text for a language ("en" or Italian).
func L(lang, it, en string) string {
	if lang == "en" {
		return en
	}
	return it
}

// PlanRow is one planned device compared with the network.
type PlanRow struct {
	PlanItem
	Class    string `json:"class"`  // ok, bad, warn
	Status   string `json:"status"` // text for people
	FoundKey string `json:"found_key,omitempty"`
	FoundNet string `json:"found_net,omitempty"`
	Found    string `json:"found,omitempty"` // label of the device found
	FoundIP  string `json:"found_ip,omitempty"`
}

// PlanCompare is the plan against the network.
type PlanCompare struct {
	Plan  []PlanItem `json:"plan"`
	Rows  []PlanRow  `json:"rows"`
	Extra []HostView `json:"extra"` // devices that are not in the plan
	OK    int        `json:"ok"`
	Bad   int        `json:"bad"`
	Warn  int        `json:"warn"`
}

func hostLabel(h HostView) string {
	return firstNonEmpty(h.Name, h.Friendly, h.Hostname, h.MAC, h.IP)
}

// comparePlan matches the plan with the hosts found: by MAC when the plan
// has one, else by IP, else by name.
func comparePlan(plan []PlanItem, hosts []HostView, lang string) PlanCompare {
	pc := PlanCompare{Plan: plan, Rows: []PlanRow{}, Extra: []HostView{}}
	var live []HostView
	for _, h := range hosts {
		if !h.Self {
			live = append(live, h)
		}
	}
	byIP, byMAC, byName := map[string]HostView{}, map[string]HostView{}, map[string]HostView{}
	for _, h := range live {
		byIP[h.IP] = h
		if h.MAC != "" {
			byMAC[h.MAC] = h
		}
		byName[strings.ToLower(hostLabel(h))] = h
	}
	ipCount := map[string]int{}
	planIPs := map[string]bool{}
	for _, p := range plan {
		ipCount[p.IP]++
		planIPs[p.IP] = true
	}
	used := map[string]bool{}
	for _, p := range plan {
		row := PlanRow{PlanItem: p}
		atIP, okIP := byIP[p.IP]
		byM, okM := byMAC[scanner.NormalizeMAC(p.MAC)]
		named, okN := byName[strings.ToLower(p.Name)]
		var h *HostView
		switch {
		case ipCount[p.IP] > 1:
			row.Class, row.Status = "warn", L(lang, "IP doppio nel piano", "duplicate IP in the plan")
			if okIP {
				h = &atIP
			}
		case okM && byM.IP == p.IP:
			row.Class, row.Status, h = "ok", "ok", &byM
		case okM:
			row.Class, row.Status, h = "warn", fmt.Sprintf(L(lang, "IP sbagliato: è a %s", "wrong IP: it is at %s"), byM.IP), &byM
		case okIP && p.MAC != "" && atIP.MAC != "" && atIP.MAC != scanner.NormalizeMAC(p.MAC):
			row.Class, row.Status, h = "bad", fmt.Sprintf(L(lang, "IP occupato da %s", "IP taken by %s"), hostLabel(atIP)), &atIP
		case okIP:
			row.Class, row.Status, h = "ok", "ok", &atIP
		case p.Name != "" && okN:
			row.Class, row.Status, h = "warn", fmt.Sprintf(L(lang, "IP sbagliato: è a %s", "wrong IP: it is at %s"), named.IP), &named
		default:
			row.Class, row.Status = "bad", L(lang, "manca", "missing")
		}
		if h != nil {
			used[h.IP] = true
			row.FoundKey, row.FoundNet, row.Found, row.FoundIP = h.Key, h.Net, hostLabel(*h), h.IP
		}
		switch row.Class {
		case "ok":
			pc.OK++
		case "bad":
			pc.Bad++
		default:
			pc.Warn++
		}
		pc.Rows = append(pc.Rows, row)
	}
	for _, h := range live {
		if !used[h.IP] && !planIPs[h.IP] {
			pc.Extra = append(pc.Extra, h)
		}
	}
	return pc
}

// hostsOf returns the hosts of the latest scan on network net ("" = all).
func hostsOf(net string) []HostView {
	lastMu.Lock()
	defer lastMu.Unlock()
	if lastResult == nil {
		return nil
	}
	var out []HostView
	for _, h := range lastResult.Hosts {
		if net == "" || h.Net == net {
			out = append(out, h)
		}
	}
	return out
}

// ---- Pre-show check ----

type ReportRow struct {
	Class  string `json:"class"` // ok, warn, bad
	Name   string `json:"name"`
	Role   string `json:"role,omitempty"`
	IP     string `json:"ip,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type ReportSection struct {
	Title string      `json:"title"`
	Note  string      `json:"note,omitempty"`
	Rows  []ReportRow `json:"rows"`
}

type Report struct {
	Verdict  string          `json:"verdict"` // ok, warn, bad
	Headline string          `json:"headline"`
	Summary  string          `json:"summary"`
	Network  string          `json:"network"`
	At       time.Time       `json:"at"`
	Sections []ReportSection `json:"sections"`
}

// runPreshow checks everything before the show: a fresh scan (if the last
// one is older than two minutes), critical devices and universes, the IP
// plan, network problems and device warnings.
func runPreshow(ctx context.Context, net, lang string) (*Report, error) {
	lastMu.Lock()
	stale := lastResult == nil || time.Since(lastResult.At) > 2*time.Minute
	lastMu.Unlock()
	if stale {
		target := ""
		if n, ok := devices().Current(); ok && n.Subnet != "" {
			target = n.Subnet
		} else if i, err := scanner.DefaultInterface(); err == nil {
			target = clampNetwork(i)
		}
		ips, err := scanner.ParseTargets(target)
		if err != nil {
			return nil, err
		}
		if _, err := runCollect(ctx, target, ips, scanner.Options{Ping: true, ResolveDNS: true, ScanPorts: scanner.CommonPorts}, false, nil, nil); err != nil {
			return nil, err
		}
	}
	if net != "" {
		devices().Select(net)
	}
	hosts := hostsOf(net)
	rep := &Report{At: time.Now()}
	if n, ok := devices().Current(); ok {
		rep.Network = firstNonEmpty(n.Name, n.Auto)
	}
	red, yellow := 0, 0

	// Critical devices.
	crit := devices().CriticalDevices()
	sec := ReportSection{Title: L(lang, "Dispositivi critici", "Critical devices"), Rows: []ReportRow{}}
	if len(crit) == 0 {
		sec.Note = L(lang, "Nessun dispositivo segnato come critico.", "No device marked as critical.")
	} else {
		ips, ports := []string{}, map[string][]int{}
		for _, d := range crit {
			ips = append(ips, d.IP)
			if len(d.Ports) > 0 {
				ports[d.IP] = d.Ports[:min(8, len(d.Ports))]
			}
		}
		res := scanner.Check(ctx, ips, ports)
		for i, d := range crit {
			row := ReportRow{Name: firstNonEmpty(d.Name, d.Friendly, d.Hostname, d.IP), Role: d.Role, IP: d.IP}
			if res[i].Alive {
				row.Class, row.Detail = "ok", fmt.Sprintf("%d ms", res[i].RTTms)
			} else {
				row.Class, row.Detail = "bad", L(lang, "NON RISPONDE", "NOT RESPONDING")
				red++
			}
			sec.Rows = append(sec.Rows, row)
		}
	}
	rep.Sections = append(rep.Sections, sec)

	// Critical universes.
	unis := devices().CriticalUniverses()
	sec = ReportSection{Title: L(lang, "Universi critici", "Critical universes"), Rows: []ReportRow{}}
	if len(unis) == 0 {
		sec.Note = L(lang, "Nessun universo segnato come critico (Strumenti → Universi DMX).", "No universe marked as critical (Tools → DMX universes).")
	} else {
		heard := map[string]scanner.Universe{}
		lctx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
		scanner.ListenDMX(lctx, defaultIface(), 64, 500*time.Millisecond, func() scanner.DMXWant { return scanner.DMXWant{} }, func(us []scanner.Universe) {
			for _, u := range us {
				heard[u.ID] = u
			}
		})
		cancel()
		for _, id := range unis {
			row := ReportRow{Name: uniNameLang(id, lang)}
			u, ok := heard[id]
			switch {
			case !ok || len(u.Sources) == 0:
				row.Class, row.Detail = "bad", L(lang, "NON ARRIVA", "NOT ARRIVING")
				red++
			default:
				var names []string
				fps := 0.0
				for _, s := range u.Sources {
					names = append(names, firstNonEmpty(s.Name, s.IP))
					fps = max(fps, s.FPS)
				}
				row.Role = strings.Join(names, ", ")
				row.Class, row.Detail = "ok", fmt.Sprintf(L(lang, "%.0f frame/s", "%.0f frames/s"), fps)
				if len(u.Sources) > 1 {
					row.Class = "warn"
					row.Detail += L(lang, " · più sorgenti", " · several sources")
					yellow++
				}
			}
			sec.Rows = append(sec.Rows, row)
		}
	}
	rep.Sections = append(rep.Sections, sec)

	// IP plan.
	sec = ReportSection{Title: L(lang, "Piano IP", "IP plan"), Rows: []ReportRow{}}
	if plan := devices().Plan(); len(plan) == 0 {
		sec.Note = L(lang, "Nessun piano IP importato.", "No IP plan imported.")
	} else {
		pc := comparePlan(plan, hosts, lang)
		for _, r := range pc.Rows {
			if r.Class != "ok" {
				sec.Rows = append(sec.Rows, ReportRow{Class: r.Class, Name: firstNonEmpty(r.Name, "—"), Role: r.Role, IP: r.IP, Detail: r.Status})
			}
		}
		red, yellow = red+pc.Bad, yellow+pc.Warn
		if len(pc.Extra) > 0 {
			yellow++
		}
		sec.Note = fmt.Sprintf(L(lang, "%d su %d a posto", "%d of %d OK"), pc.OK, len(pc.Rows))
		if len(pc.Extra) > 0 {
			sec.Note += fmt.Sprintf(L(lang, " · %d dispositivi non previsti", " · %d devices not planned"), len(pc.Extra))
		}
		sec.Note += "."
	}
	rep.Sections = append(rep.Sections, sec)

	// Dante routing: broken subscriptions stop the audio.
	var danteIPs []string
	for _, h := range hosts {
		if h.Dante != nil && !h.Self {
			danteIPs = append(danteIPs, h.IP)
		}
	}
	if len(danteIPs) > 0 {
		sec = ReportSection{Title: "Dante", Rows: []ReportRow{}}
		dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		devs := scanner.DanteRouting(dctx, danteIPs)
		cancel()
		sum := scanner.SummarizeDante(devs, nil)
		for _, d := range devs {
			if d.Err != "" {
				continue
			}
			for _, r := range d.Rx {
				if r.State == "bad" {
					sec.Rows = append(sec.Rows, ReportRow{Class: "bad", Name: d.Name + " · " + r.Name, IP: d.IP, Role: strings.Trim(r.TxChan+" @ "+r.TxDevice, " @"), Detail: r.Detail})
					red++
				}
			}
		}
		sec.Note = fmt.Sprintf(L(lang, "%d canali collegati, %d con problemi (lettura del routing, sperimentale).", "%d channels connected, %d with problems (routing read, experimental)."), sum.OK, sum.Bad)
		rep.Sections = append(rep.Sections, sec)
	}

	// Network problems and device warnings.
	lastMu.Lock()
	var issues []string
	if lastResult != nil {
		issues = lastResult.Issues
	}
	lastMu.Unlock()
	sec = ReportSection{Title: L(lang, "Problemi di rete", "Network problems"), Rows: []ReportRow{}}
	for _, is := range issues {
		sec.Rows = append(sec.Rows, ReportRow{Class: "warn", Name: msgLang(is, lang)})
		yellow++
	}
	if len(issues) == 0 {
		sec.Note = L(lang, "Nessuno.", "None.")
	}
	rep.Sections = append(rep.Sections, sec)
	sec = ReportSection{Title: L(lang, "Avvisi sui dispositivi", "Device warnings"), Rows: []ReportRow{}}
	for _, h := range hosts {
		for _, w := range h.Warnings {
			sec.Rows = append(sec.Rows, ReportRow{Class: "warn", Name: hostLabel(h), IP: h.IP, Detail: msgLang(w, lang)})
		}
	}
	if len(sec.Rows) == 0 {
		sec.Note = L(lang, "Nessuno.", "None.")
	} else {
		yellow++
	}
	rep.Sections = append(rep.Sections, sec)

	switch {
	case red > 0:
		rep.Verdict, rep.Headline = "bad", L(lang, "NON PRONTO", "NOT READY")
		rep.Summary = fmt.Sprintf(L(lang, "%d problemi da risolvere prima dello show.", "%d problems to fix before the show."), red)
	case yellow > 0:
		rep.Verdict, rep.Headline = "warn", L(lang, "PRONTO CON AVVISI", "READY WITH WARNINGS")
		rep.Summary = L(lang, "Niente di bloccante, ma qualcosa da controllare.", "Nothing blocking, but something to check.")
	default:
		rep.Verdict, rep.Headline = "ok", L(lang, "PRONTO", "READY")
		rep.Summary = L(lang, "Tutto a posto.", "All good.")
	}
	return rep, nil
}

func uniNameLang(id, lang string) string {
	proto, num, _ := strings.Cut(id, ":")
	return map[string]string{"art": "Art-Net", "sacn": "sACN"}[proto] + L(lang, " universo ", " universe ") + num
}

// msgLang translates a scanner message for English output.
func msgLang(s, lang string) string { return msgIn(s, lang) }

// handlePreshow runs the pre-show check: GET ?net=…&lang=….
func handlePreshow(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	activeScan.Add(1)
	defer activeScan.Add(-1)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	rep, err := runPreshow(ctx, q.Get("net"), q.Get("lang"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, rep)
}
