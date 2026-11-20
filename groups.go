package main

import (
	"github.com/wasabifilm/wasaping/scanner"
)

// Departments: the Monitor sorts the critical devices into lights, audio,
// video and network, so that the crew sees at once whose problem it is,
// and each department can be silenced on its own.

// groupIDs in the order the Monitor shows them.
var groupIDs = []string{"lights", "audio", "video", "network", "other"}

func validGroup(g string) bool {
	for _, id := range groupIDs {
		if g == id {
			return true
		}
	}
	return false
}

// groupOfKind is the department a device type belongs to.
func groupOfKind(kind string) string {
	switch kind {
	case scanner.KindLightDesk, scanner.KindLighting:
		return "lights"
	case scanner.KindAudio, scanner.KindSpeaker:
		return "audio"
	case scanner.KindVideo, scanner.KindProjector, scanner.KindCamera, scanner.KindTV:
		return "video"
	case scanner.KindRouter, scanner.KindSwitch:
		return "network"
	}
	return "other"
}

// deviceGroup: the department chosen by the user, else the one of its type.
func deviceGroup(d Device) string {
	if d.Group != "" {
		return d.Group
	}
	return groupOfKind(firstNonEmpty(d.KindManual, d.Kind))
}

// groupName is the department as people read it.
func groupName(id string) string {
	switch id {
	case "lights":
		return T("Luci", "Lights")
	case "audio":
		return "Audio"
	case "video":
		return "Video"
	case "network":
		return T("Rete", "Network")
	}
	return T("Altro", "Other")
}

// groupNameIT is the department in the log and the alerts' JSON (always
// Italian, like the other events in monitor-log.csv).
func groupNameIT(id string) string {
	switch id {
	case "lights":
		return "Luci"
	case "network":
		return "Rete"
	case "audio", "video":
		return groupName(id)
	case "":
		return ""
	}
	return "Altro"
}
