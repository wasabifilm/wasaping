package scanner

import (
	"slices"
	"strings"
)

// Device kinds shown as icons in the interface.
const (
	KindRouter    = "router"
	KindComputer  = "computer"
	KindPhone     = "phone"
	KindTablet    = "tablet"
	KindPrinter   = "printer"
	KindTV        = "tv"
	KindSpeaker   = "speaker"
	KindNAS       = "nas"
	KindCamera    = "camera"
	KindIoT       = "iot"
	KindLightDesk = "lightdesk" // lighting console / controller
	KindLighting  = "lighting"  // Art-Net / sACN node, fixture, dimmer
	KindAudio     = "audio"     // Dante device, digital mixer, stagebox, wireless mics
	KindVideo     = "video"     // ATEM, HyperDeck, Videohub, NDI, media server
	KindProjector = "projector"
	KindSwitch    = "switch"
)

// GuessKind infers what a device is from what it shows on the network:
// show protocols, open ports, Bonjour services, model, name and vendor.
// It returns "" when there is nothing to go on.
func GuessKind(h Host) string {
	name := strings.ToLower(h.Hostname + " " + h.Friendly)
	model := strings.ToLower(h.Model)
	vendor := strings.ToLower(h.Vendor)
	port := func(p ...int) bool {
		for _, x := range p {
			if slices.Contains(h.Ports, x) {
				return true
			}
		}
		return false
	}
	svc := func(s ...string) bool {
		for _, x := range s {
			if slices.Contains(h.Services, x) {
				return true
			}
		}
		return false
	}
	has := func(s string, words ...string) bool {
		for _, w := range words {
			if strings.Contains(s, w) {
				return true
			}
		}
		return false
	}
	switch {
	case h.Gateway, h.UPnP == "InternetGatewayDevice":
		return KindRouter
	// Show gear first: it is what matters on a show network.
	case h.PJLink != nil || port(4352) || has(name, "projector", "proiettore"):
		return KindProjector
	case h.ArtNet != nil && (h.ArtNet.Style == "console" || h.ArtNet.Style == "configuratore"),
		has(name, "grandma", "gma3", "gma2", "onpc", "chamsys", "magicq", "hog4", "eos ", "etc-eos", "avolites"),
		port(30000, 30001) && has(vendor, "ma lighting"):
		return KindLightDesk
	case h.ArtNet != nil && h.ArtNet.Style == "media server", len(h.NDI) > 0,
		svc("blackmagic", "videohub", "ndi") || port(9990, 9993, 9910, 5960),
		has(name, "atem", "hyperdeck", "videohub", "resolume", "millumin", "disguise", "pixera"):
		return KindVideo
	case h.ArtNet != nil || h.SACN != nil:
		return KindLighting
	case svc("netaudio-arc", "netaudio-cmc", "netaudio-dbc") || port(4440, 49280, 51325, 51326, 2202) ||
		has(vendor, "audinate", "yamaha", "allen & heath", "digico", "shure", "sennheiser", "focusrite", "rme", "behringer", "midas"):
		return KindAudio
	case svc("qlab"):
		return KindComputer
	case has(model, "iphone"):
		return KindPhone
	case has(model, "ipad"):
		return KindTablet
	case has(model, "appletv", "chromecast", "google tv", "bravia") || h.UPnP == "MediaRenderer" && !has(name+model, "sonos", "speaker"):
		return KindTV
	case has(model, "audioaccessory", "homepod", "sonos"):
		return KindSpeaker
	case has(model, "macbook", "imac", "macmini", "mac mini", "macpro", "mac studio", "mac1", "mac14", "mac15", "mac16"):
		return KindComputer
	case svc("ipp", "ipps", "printer", "pdl-datastream", "scanner", "uscan") || port(9100, 631) ||
		has(name, "printer", "laserjet", "deskjet", "officejet", "epson", "brother", "canon"):
		return KindPrinter
	case has(name, "ipad"):
		return KindTablet
	case has(name, "iphone", "android", "galaxy", "pixel", "redmi", "oneplus") || port(62078):
		return KindPhone
	case has(name, "macbook", "imac", "mac-mini", "macmini", "mac-studio", "macpro", "desktop-", "laptop-", "-pc", "thinkpad"):
		return KindComputer
	case has(name, "appletv", "apple-tv", "tv", "chromecast", "roku", "firetv", "bravia") ||
		svc("googlecast", "mediaremotetv") || port(8008, 8009):
		return KindTV
	case has(name, "homepod", "sonos", "echo", "speaker") || svc("sonos", "spotify-connect"):
		return KindSpeaker
	case has(name, "nas", "diskstation", "synology", "qnap") || has(vendor, "synology", "qnap", "western digital"):
		return KindNAS
	case has(vendor, "cisco", "netgear", "ubiquiti", "luminex", "pathport", "mikrotik", "hirschmann", "d-link") && !port(62078):
		return KindSwitch
	case port(554) || has(name, "cam", "ipcam") || has(vendor, "hikvision", "dahua", "axis", "reolink"):
		return KindCamera
	case svc("smb", "afpovertcp", "ssh", "sftp-ssh", "rfb", "companion-link", "workstation") || port(3389, 548, 5900):
		return KindComputer
	case svc("airplay", "raop"):
		return KindSpeaker
	case port(1883) || svc("hap", "matter", "matterc", "homekit") ||
		has(vendor, "espressif", "tuya", "shelly", "philips lighting", "signify", "xiaomi",
			"dreame", "roborock", "ecovacs", "irobot", "nuki", "netatmo", "tado", "ring llc"):
		return KindIoT
	case port(53) && port(80, 443):
		return KindRouter
	case port(445, 139):
		return KindComputer
	}
	return ""
}

func containsInt(a []int, x int) bool { return slices.Contains(a, x) }

func joinStrings(a []string) string { return strings.Join(a, ", ") }

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}
