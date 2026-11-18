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
	{regexp.MustCompile(`^Alla scansione precedente questo IP era di (.+) \((.+)\): .*$`), "At the previous scan this IP belonged to $1 ($2): device replaced, or an IP conflict."},
	{regexp.MustCompile(`^Nessun router su questa rete.*$`), "No router on this network: normal for an isolated show network, but no internet and no DHCP from a router."},
	{regexp.MustCompile(`^Più master di clock (.+): (.+)\. .*$`), "Several $1 clock masters: $2. Dante and AES67 want a single master per domain."},
	{regexp.MustCompile(`^Universo sACN (\d+) inviato da più sorgenti \((.+)\): .*$`), "sACN universe $1 sent by several sources ($2): check priorities or merging."},
}

// msgCLI translates a scanner message for the command line.
func msgCLI(s string) string {
	if cliLang != "en" {
		return s
	}
	for _, m := range enMessages {
		if m.re.MatchString(s) {
			return m.re.ReplaceAllString(s, m.rep)
		}
	}
	return s
}
