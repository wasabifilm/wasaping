package scanner

import (
	"fmt"
	"slices"
	"strings"
)

// protocolsOf lists the show and media protocols a device speaks, for the
// "Protocolli" column.
func protocolsOf(h Host) []string {
	var p []string
	add := func(ok bool, name string) {
		if ok && !slices.Contains(p, name) {
			p = append(p, name)
		}
	}
	svc := func(s string) bool { return slices.Contains(h.Services, s) }
	add(h.ArtNet != nil || len(h.ArtBroadcast) > 0, "Art-Net")
	add(h.SACN != nil, "sACN")
	add(svc("netaudio-arc") || svc("netaudio-cmc") || svc("netaudio-dbc") || svc("netaudio-chan") || h.Dante != nil, "Dante")
	add(h.PTP != "", "PTP master")
	add(len(h.NDI) > 0 || svc("ndi") || containsInt(h.Ports, 5960), "NDI")
	add(svc("blackmagic") || svc("videohub") || containsInt(h.Ports, 9990) || containsInt(h.Ports, 9993), "Blackmagic")
	add(h.PJLink != nil, "PJLink")
	add(svc("qlab") || containsInt(h.Ports, 53000), "QLab")
	add(svc("millumin"), "Millumin")
	add(svc("madmapper"), "MadMapper")
	add(svc("touchdesigner"), "TouchDesigner")
	add(svc("resolume"), "Resolume")
	add(svc("isadora"), "Isadora")
	add(svc("osc"), "OSC")
	add(containsInt(h.Ports, 30000) || containsInt(h.Ports, 30001), "MA-Net")
	add(containsInt(h.Ports, 49280), "Yamaha RCP")
	add(containsInt(h.Ports, 51325) || containsInt(h.Ports, 51326), "Allen & Heath")
	add(containsInt(h.Ports, 2202), "Shure")
	add(svc("oca"), "AES70")
	add(containsInt(h.Ports, 1710) || serverTitle(strings.ToLower(h.WebTitle)) == "Q-SYS", "Q-SYS")
	add(containsInt(h.Ports, 41794) || containsInt(h.Ports, 41795), "Crestron")
	add(containsInt(h.Ports, 1319), "AMX")
	add(serverTitle(strings.ToLower(h.WebTitle)) == "Companion", "Companion")
	add(svc("airplay"), "AirPlay")
	return p
}

// Exposure lists risky things a device exposes, in plain words.
func Exposure(h Host) []string {
	var w []string
	if h.OffSubnet {
		w = append(w, "Fuori dalla sottorete di questo computer: chi è sulla rete normale non lo raggiunge. Controlla IP e maschera di rete.")
	}
	if containsInt(h.Ports, 23) {
		w = append(w, "Telnet aperto (porta 23): accesso senza cifratura, spesso con password di fabbrica.")
	}
	if containsInt(h.Ports, 21) && h.Gateway {
		w = append(w, "FTP aperto sul router (porta 21): meglio disattivarlo.")
	}
	if h.Gateway && containsInt(h.Ports, 80) && !containsInt(h.Ports, 443) {
		w = append(w, "Pannello del router solo in HTTP: la password viaggia in chiaro.")
	}
	for _, a := range h.Apps {
		switch {
		case a.Name == "Docker":
			w = append(w, "API di Docker aperta senza TLS (porta "+itoa(a.Port)+"): chi la raggiunge controlla il server.")
		case a.NoAuth:
			w = append(w, a.Name+" senza password (porta "+itoa(a.Port)+"): chiunque sulla rete legge i dati.")
		}
	}
	switch serverTitle(strings.ToLower(h.WebTitle)) {
	case "iDRAC", "iLO", "IPMI", "IMM", "XClarity", "CIMC":
		w = append(w, "Gestione remota del server ("+serverTitle(strings.ToLower(h.WebTitle))+") raggiungibile da questa rete: meglio tenerla su una rete separata.")
	}
	if containsInt(h.Ports, 3389) {
		w = append(w, "Desktop remoto di Windows aperto (porta 3389).")
	}
	if h.PJLink != nil && h.PJLink.Errors != "" {
		w = append(w, "Il proiettore segnala: "+h.PJLink.Errors+".")
	}
	if h.PJLink != nil && !h.PJLink.NeedsLogin && h.PJLink.Name != "" {
		// PJLink without password: anyone on the network can switch it off.
		w = append(w, "PJLink senza password: chiunque sulla rete può spegnere il proiettore.")
	}
	if h.Dante != nil && h.OffSubnet {
		w = append(w, "Dispositivo Dante su un'altra sottorete: Dante Controller lo vede, ma l'audio con gli altri dispositivi non passa.")
	}
	if h.ArtNet != nil && strings.Contains(strings.ToLower(h.ArtNet.Report), "fail") {
		w = append(w, "Il nodo Art-Net segnala un problema: "+h.ArtNet.Report)
	}
	return w
}

// NetworkIssues lists problems of the network as a whole.
func NetworkIssues(hosts []Host) []string {
	var out []string
	gw := false
	masters := map[string][]string{}
	sacnUni := map[int][]string{}
	direct := 0 // devices on this computer's own network (not via a VPN or router)
	for _, h := range hosts {
		if h.Gateway {
			gw = true
		}
		if h.Via == "" && !h.OffSubnet && !h.Self {
			direct++
		}
		if h.PTP != "" {
			masters[h.PTP] = append(masters[h.PTP], h.IP)
		}
		if h.SACN != nil {
			for _, u := range h.SACN.Universes {
				sacnUni[u] = append(sacnUni[u], h.IP)
			}
		}
	}
	if !gw && direct > 0 {
		out = append(out, "Nessun router su questa rete: normale per una rete show isolata, ma niente internet e niente DHCP dal router.")
	}
	for kind, ips := range masters {
		if len(ips) > 1 {
			out = append(out, fmt.Sprintf("Più master di clock %s: %s. Dante e AES67 vogliono un solo master per dominio.", kind, strings.Join(ips, ", ")))
		}
	}
	for u, ips := range sacnUni {
		if len(ips) > 1 {
			out = append(out, fmt.Sprintf("Universo sACN %d inviato da più sorgenti (%s): controlla le priorità o il merge.", u, strings.Join(ips, ", ")))
		}
	}
	var dup []string
	for _, h := range hosts {
		if len(h.OtherMACs) > 0 {
			dup = append(dup, h.IP)
		}
	}
	if len(dup) > 0 {
		out = append(out, fmt.Sprintf("IP doppi: %s. Due dispositivi con lo stesso indirizzo si contendono il traffico: apri il dispositivo per vedere chi sono.", strings.Join(dup, ", ")))
	}
	for _, h := range hosts {
		if len(h.ArtBroadcast) > 0 {
			n := fmt.Sprintf("%d universi arrivano", len(h.ArtBroadcast))
			if len(h.ArtBroadcast) == 1 {
				n = "1 universo arriva"
			}
			out = append(out, fmt.Sprintf("Art-Net in broadcast da %s: %s a tutti i dispositivi della rete (anche Dante, video e Wi-Fi). Meglio l'unicast verso i nodi.", firstNonEmpty(h.Friendly, h.Hostname, h.IP), n))
		}
	}
	out = append(out, danteIssues(hosts)...)
	slices.Sort(out)
	return out
}

// danteName is how a Dante device is called in the messages.
func danteName(h Host) string {
	if h.Dante != nil && h.Dante.Name != "" {
		return h.Dante.Name
	}
	if h.Friendly != "" {
		return h.Friendly
	}
	return h.IP
}

// danteIssues: what stops Dante audio, readable without Audinate's
// protocol: sample rates, primary and secondary mixed up, the clock.
func danteIssues(hosts []Host) []string {
	var out []string
	var dante []Host
	for _, h := range hosts {
		if h.Dante != nil && !h.Self {
			dante = append(dante, h)
		}
	}
	if len(dante) == 0 {
		return nil
	}
	// 1. Different sample rates: those devices cannot exchange audio.
	byRate := map[int][]string{}
	for _, h := range dante {
		if h.Dante.SampleRate > 0 {
			byRate[h.Dante.SampleRate] = append(byRate[h.Dante.SampleRate], danteName(h))
		}
	}
	if len(byRate) > 1 {
		rates := make([]int, 0, len(byRate))
		for r := range byRate {
			rates = append(rates, r)
		}
		slices.Sort(rates)
		var parts []string
		for _, r := range rates {
			parts = append(parts, fmt.Sprintf("%g kHz: %s", float64(r)/1000, strings.Join(byRate[r], ", ")))
		}
		out = append(out, "Dante a frequenze di campionamento diverse ("+strings.Join(parts, "; ")+"): tra loro l'audio non passa.")
	}
	// 2. The same device answering twice on one network: primary and
	// secondary are on the same switch (or the device is in switched mode
	// with two cables on the same network).
	byDev := map[string][]string{}
	for _, h := range dante {
		key := h.Dante.ID
		if key == "" {
			key = "name:" + h.Dante.Name
		}
		if key != "name:" {
			byDev[key] = append(byDev[key], h.IP)
		}
	}
	for key, ips := range byDev {
		if len(ips) > 1 {
			name := strings.TrimPrefix(key, "name:")
			for _, h := range dante {
				if h.IP == ips[0] {
					name = danteName(h)
				}
			}
			slices.Sort(ips)
			out = append(out, fmt.Sprintf("Dante: %s risponde da %s sulla stessa rete. Primaria e secondaria sono collegate allo stesso switch? Devono stare su reti separate.", name, strings.Join(ips, " e ")))
		}
	}
	// 3. The clock: who follows the master. Only when some device is seen
	// following, so that a missing Delay_Req means something.
	master, follows := false, false
	for _, h := range hosts {
		master = master || h.PTP == "PTPv1" || strings.HasPrefix(h.PTP, "PTPv2")
		follows = follows || h.PTPFollower
	}
	if !master && len(dante) > 1 {
		out = append(out, "Dispositivi Dante, ma nessun master del clock PTP sentito durante la scansione: controlla il clock (o, su Linux, che wasaping possa ascoltare le porte 319 e 320).")
	}
	if master && follows {
		var lost []string
		for _, h := range dante {
			if h.PTP == "" && !h.PTPFollower {
				lost = append(lost, danteName(h))
			}
		}
		if len(lost) > 0 && len(lost) < len(dante) {
			slices.Sort(lost)
			out = append(out, "Dante: "+strings.Join(lost, ", ")+" non risulta agganciato al clock (nessuna richiesta al master durante la scansione). Controllalo in Dante Controller → Clock Status.")
		}
	}
	return out
}
