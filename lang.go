package main

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// Languages: Italian (the owner's) and English. The page picks its own and
// tells the program, for notifications; the command line follows LANG.

func envLang() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.ToLower(os.Getenv(k)); v != "" && v != "c" && v != "posix" {
			if strings.HasPrefix(v, "it") {
				return "it"
			}
			return "en"
		}
	}
	return "it"
}

// appLang is the language of the window (notifications, menu bar).
func appLang() string {
	if l := loadSettings().Lang; l == "it" || l == "en" {
		return l
	}
	return envLang()
}

// T picks the text for the window's language; TC for the command line.
func T(it, en string) string {
	if appLang() == "en" {
		return en
	}
	return it
}

var cliLang = envLang()

func TC(it, en string) string {
	if cliLang == "en" {
		return en
	}
	return it
}

func handleLang(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Lang string `json:"lang"`
		}
		json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&req)
		if req.Lang == "it" || req.Lang == "en" {
			s := loadSettings()
			if s.Lang != req.Lang {
				s.Lang = req.Lang
				saveSettings(s)
			}
		}
	}
	writeJSON(w, map[string]string{"lang": appLang()})
}

// enMessages translate the scanner's warnings for the English command line.
var enMessages = []struct {
	re  *regexp.Regexp
	rep string
}{
	{regexp.MustCompile(`^Fuori dalla sottorete di questo computer.*$`), "Outside this computer's subnet: devices on the normal network cannot reach it. Check its IP and netmask."},
	{regexp.MustCompile(`^Telnet aperto \(porta 23\).*$`), "Telnet open (port 23): unencrypted access, often with the factory password."},
	{regexp.MustCompile(`^FTP aperto sul router.*$`), "FTP open on the router (port 21): better switch it off."},
	{regexp.MustCompile(`^Pannello del router solo in HTTP.*$`), "Router panel on HTTP only: the password travels in clear text."},
	{regexp.MustCompile(`^Desktop remoto di Windows aperto.*$`), "Windows Remote Desktop open (port 3389)."},
	{regexp.MustCompile(`^PJLink senza password.*$`), "PJLink without password: anyone on the network can switch the projector off."},
	{regexp.MustCompile(`^Il proiettore segnala: (.+)\.$`), "The projector reports: $1."},
	{regexp.MustCompile(`^Il nodo Art-Net segnala un problema: (.+)$`), "The Art-Net node reports a problem: $1"},
	{regexp.MustCompile(`^Conflitto di IP: su Art-Net rispondono più dispositivi con questo indirizzo \((.+)\)\.$`), "IP conflict: several devices answer on Art-Net with this address ($1)."},
	{regexp.MustCompile(`^IP doppio: a questo indirizzo ha risposto anche (.+)\. Due dispositivi con lo stesso IP si rubano il traffico a vicenda \(se hai appena sostituito l'apparecchio, ignora\)\.$`), "Duplicate IP: $1 answered at this address too. Two devices with the same IP steal each other's traffic (if you just replaced the device, ignore this)."},
	{regexp.MustCompile(`^IP doppi: (.+)\. Due dispositivi con lo stesso indirizzo si contendono il traffico: apri il dispositivo per vedere chi sono\.$`), "Duplicate IPs: $1. Two devices with the same address fight over the traffic: open the device to see who they are."},
	{regexp.MustCompile(`^Art-Net in broadcast da (.+): (\d+) (?:universo arriva|universi arrivano) a tutti i dispositivi della rete \(anche Dante, video e Wi-Fi\)\. Meglio l'unicast verso i nodi\.$`), "Art-Net broadcast from $1: $2 universes reach every device on the network (Dante, video and Wi-Fi too). Unicast to the nodes is better."},
	{regexp.MustCompile(`^Più server DHCP: (.+)\. I dispositivi possono prendere indirizzi e router sbagliati: lascia acceso il DHCP su uno solo\.$`), "Several DHCP servers: $1. Devices can get wrong addresses and routers: leave DHCP on in only one."},
	{regexp.MustCompile(`^Il server DHCP (.+) dà come router (.+), ma il router di questa rete è (.+)\.$`), "DHCP server $1 hands out router $2, but this network's router is $3."},
	{regexp.MustCompile(`^per sentire i server DHCP serve la porta 68: avvia wasaping come root, oppure dagli il permesso una volta con «(.+)»$`), "to hear DHCP servers wasaping needs port 68: run it as root, or give it the permission once with «$1»"},
	{regexp.MustCompile(`^porta 68 non disponibile: (.+)$`), "port 68 not available: $1"},
	{regexp.MustCompile(`^non riesco a mandare la richiesta DHCP$`), "cannot send the DHCP request"},
	{regexp.MustCompile(`^Alla scansione precedente questo IP era di (.+) \((.+)\): .*$`), "At the previous scan this IP belonged to $1 ($2): device replaced, or an IP conflict."},
	{regexp.MustCompile(`^Nessun router su questa rete.*$`), "No router on this network: normal for an isolated show network, but no internet and no DHCP from a router."},
	{regexp.MustCompile(`^Dante a frequenze di campionamento diverse \((.+)\): tra loro l\'audio non passa\.$`), "Dante at different sample rates ($1): audio does not pass between them."},
	{regexp.MustCompile(`^Dante: (.+) risponde da (.+) sulla stessa rete\. .*$`), "Dante: $1 answers from $2 on the same network. Are primary and secondary on the same switch? They must be on separate networks."},
	{regexp.MustCompile(`^Dispositivi Dante, ma nessun master del clock PTP sentito .*$`), "Dante devices, but no PTP clock master heard during the scan: check the clock (or, on Linux, that wasaping can listen on ports 319 and 320)."},
	{regexp.MustCompile(`^Dante: (.+) non risulta agganciato al clock .*$`), "Dante: $1 does not seem locked to the clock (no request to the master during the scan). Check it in Dante Controller → Clock Status."},
	{regexp.MustCompile(`^Dispositivo Dante su un\'altra sottorete: .*$`), "Dante device on another subnet: Dante Controller sees it, but audio with the other devices does not pass."},
	{regexp.MustCompile(`^Più master di clock (.+): (.+)\. .*$`), "Several $1 clock masters: $2. Dante and AES67 want a single master per domain."},
	{regexp.MustCompile(`^Universo sACN (\d+) inviato da più sorgenti \((.+)\): .*$`), "sACN universe $1 sent by several sources ($2): check priorities or merging."},
}

// msgCLI translates a scanner message for the command line.
func msgCLI(s string) string { return msgIn(s, cliLang) }

func msgIn(s, lang string) string {
	if lang != "en" {
		return s
	}
	for _, m := range enMessages {
		if m.re.MatchString(s) {
			return m.re.ReplaceAllString(s, m.rep)
		}
	}
	// "<network>: <message>" (several networks): translate the message.
	if net, msg, ok := strings.Cut(s, ": "); ok && len(net) < 80 {
		if t := msgIn(msg, lang); t != msg {
			return strings.Replace(net, "Rete ", "Network ", 1) + ": " + t
		}
	}
	return s
}
