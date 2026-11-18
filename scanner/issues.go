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
	add(h.ArtNet != nil, "Art-Net")
	add(h.SACN != nil, "sACN")
	add(svc("netaudio-arc") || svc("netaudio-cmc") || svc("netaudio-dbc"), "Dante")
	add(h.PTP != "", "PTP master")
	add(len(h.NDI) > 0 || svc("ndi") || containsInt(h.Ports, 5960), "NDI")
	add(svc("blackmagic") || svc("videohub") || containsInt(h.Ports, 9990) || containsInt(h.Ports, 9993), "Blackmagic")
	add(h.PJLink != nil, "PJLink")
	add(svc("qlab") || containsInt(h.Ports, 53000), "QLab")
	add(svc("osc"), "OSC")
	add(containsInt(h.Ports, 30000) || containsInt(h.Ports, 30001), "MA-Net")
	add(containsInt(h.Ports, 49280), "Yamaha RCP")
	add(containsInt(h.Ports, 51325) || containsInt(h.Ports, 51326), "Allen & Heath")
	add(containsInt(h.Ports, 2202), "Shure")
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
	for _, h := range hosts {
		if h.Gateway {
			gw = true
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
	if !gw && len(hosts) > 0 {
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
	slices.Sort(out)
	return out
}
