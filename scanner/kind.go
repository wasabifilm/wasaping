package scanner

import (
	"slices"
	"strings"
)

// Device kinds shown as icons in the interface.
const (
	KindRouter   = "router"
	KindComputer = "computer"
	KindPhone    = "phone"
	KindTablet   = "tablet"
	KindPrinter  = "printer"
	KindTV       = "tv"
	KindSpeaker  = "speaker"
	KindNAS      = "nas"
	KindCamera   = "camera"
	KindIoT      = "iot"
)

// GuessKind infers what a device is from what it shows on the network:
// open ports, Bonjour services, its name and vendor. It returns "" when
// there is nothing to go on.
func GuessKind(h Host) string {
	name := strings.ToLower(h.Hostname)
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
	case h.Gateway:
		return KindRouter
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
	case port(554) || has(name, "cam", "ipcam") || has(vendor, "hikvision", "dahua", "axis", "reolink"):
		return KindCamera
	case svc("smb", "afpovertcp", "ssh", "sftp-ssh", "rfb", "companion-link") || port(3389, 548, 5900):
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
