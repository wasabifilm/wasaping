package main

import "strings"

// The tools in the macOS menu bar (Rete, Show, Progetto). The ids are the
// ones of web/js/tools.js: the menu asks the page to run them.
type toolMenuItem struct {
	group, id, it, en, key string
	shift                  bool
}

var toolMenuItems = []toolMenuItem{
	{"show", "preshow", "Pre-show", "Pre-show", "p", true},
	{"show", "monitor", "Monitor", "Monitor", "m", true},
	{"show", "-", "", "", "", false},
	{"show", "dmx", "DMX", "DMX", "d", true},
	{"show", "rdm", "RDM", "RDM", "", false},
	{"show", "dante", "Dante", "Dante", "", false},
	{"show", "video", "Video", "Video", "", false},
	{"net", "plan", "Piano IP", "IP plan", "i", true},
	{"net", "free", "IP liberi", "Free IPs", "", false},
	{"net", "map", "Mappa della rete", "Network map", "", false},
	{"net", "dhcp", "Server DHCP", "DHCP servers", "", false},
	{"net", "traffic", "Traffico", "Traffic", "", false},
	{"net", "compare", "Confronta con l'ultima volta", "Compare with last time", "", false},
	{"project", "report", "Report di fine show", "End-of-show report", "", false},
	{"project", "-", "", "", "", false},
	{"project", "save", "Salva progetto…", "Save project…", "s", false},
	{"project", "open", "Apri progetto…", "Open project…", "o", false},
	{"project", "-", "", "", "", false},
	{"project", "cleanup", "Pulisci i dispositivi vecchi…", "Clean up old devices…", "", false},
	{"project", "-", "", "", "", false},
	{"project", "print", "Stampa elenco IP…", "Print IP list…", "p", false},
	{"project", "csv", "Esporta CSV", "Export CSV", "", false},
	{"project", "json", "Esporta JSON", "Export JSON", "", false},
	{"project", "-", "", "", "", false},
	{"project", "diag", "Salva diagnostica", "Save diagnostics", "", false},
}

// toolMenuSpec is the menu for the native window: one line per item,
// "Menu\tid\tLabel\tkey\tshift", in the app's language.
func toolMenuSpec() string {
	titles := map[string]string{"show": "Show", "net": T("Rete", "Network"), "project": T("Progetto", "Project")}
	var b strings.Builder
	for _, it := range toolMenuItems {
		shift := "0"
		if it.shift {
			shift = "1"
		}
		b.WriteString(titles[it.group] + "\t" + it.id + "\t" + T(it.it, it.en) + "\t" + it.key + "\t" + shift + "\n")
	}
	return b.String()
}
