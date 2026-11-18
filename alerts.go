package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Alarms to other programs: an OSC message (QLab, consoles, media servers)
// and/or a call to a web address, on every monitor event.

func oscString(s string) []byte {
	b := append([]byte(s), 0)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

// oscMessage builds an OSC message with string and int arguments.
func oscMessage(addr string, args ...any) []byte {
	tags := ","
	var data []byte
	for _, a := range args {
		switch v := a.(type) {
		case string:
			tags += "s"
			data = append(data, oscString(v)...)
		case int:
			tags += "i"
			data = binary.BigEndian.AppendUint32(data, uint32(int32(v)))
		}
	}
	return append(append(oscString(addr), oscString(tags)...), data...)
}

func eventWord(e MonEvent) string {
	switch {
	case e.Text != "":
		return "warning"
	case e.Up:
		return "up"
	}
	return "down"
}

// sendAlert delivers an event to the configured targets (errors are ignored:
// an alarm must never block the monitor).
func sendAlert(e MonEvent) {
	s := loadSettings()
	if s.AlertOSC == "" && s.AlertWebhook == "" {
		return
	}
	alertTo(s.AlertOSC, s.AlertOSCAddr, s.AlertWebhook, e)
}

func alertTo(osc, oscAddr, hook string, e MonEvent) []string {
	var res []string
	netName := ""
	if n, ok := devices().Current(); ok {
		netName = firstNonEmpty(n.Name, n.Auto)
	}
	if osc != "" {
		base := strings.TrimRight(firstNonEmpty(oscAddr, "/wasaping"), "/")
		c, err := net.DialTimeout("udp", osc, 2*time.Second)
		if err != nil {
			res = append(res, "OSC: "+err.Error())
		} else {
			_, err = c.Write(oscMessage(base+"/"+eventWord(e), e.Name, e.IP, firstNonEmpty(e.Text, ""), int(e.DownFor)))
			c.Close()
			if err != nil {
				res = append(res, "OSC: "+err.Error())
			} else {
				res = append(res, "OSC ✓")
			}
		}
	}
	if hook != "" {
		body, _ := json.Marshal(map[string]any{
			"event": eventWord(e), "name": e.Name, "ip": e.IP, "text": e.Text, "kind": firstNonEmpty(e.Kind, "device"),
			"down_for_s": int(e.DownFor), "at": e.At.Format(time.RFC3339), "network": netName, "source": "wasaping " + version,
		})
		c := &http.Client{Timeout: 5 * time.Second}
		resp, err := c.Post(hook, "application/json", bytes.NewReader(body))
		if err != nil {
			res = append(res, "web: "+err.Error())
		} else {
			resp.Body.Close()
			res = append(res, fmt.Sprintf("web: %d", resp.StatusCode))
		}
	}
	return res
}

// handleAlerts: GET the configuration; POST {osc, osc_address, webhook, test}.
func handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			OSC     string `json:"osc"`
			OSCAddr string `json:"osc_address"`
			Webhook string `json:"webhook"`
			Test    bool   `json:"test"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		if req.OSC != "" {
			if _, _, err := net.SplitHostPort(req.OSC); err != nil {
				http.Error(w, T("OSC: scrivi indirizzo:porta, es. 192.168.1.20:53000", "OSC: write address:port, e.g. 192.168.1.20:53000"), http.StatusBadRequest)
				return
			}
		}
		if req.OSCAddr != "" && !strings.HasPrefix(req.OSCAddr, "/") {
			req.OSCAddr = "/" + req.OSCAddr
		}
		if req.Webhook != "" {
			if u, err := url.Parse(req.Webhook); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				http.Error(w, T("indirizzo web non valido", "invalid web address"), http.StatusBadRequest)
				return
			}
		}
		if req.Test {
			res := alertTo(req.OSC, req.OSCAddr, req.Webhook, MonEvent{At: time.Now(), Name: T("Prova di wasaping", "wasaping test"), Text: T("messaggio di prova", "test message")})
			writeJSON(w, map[string]string{"result": strings.Join(res, " · ")})
			return
		}
		s := loadSettings()
		s.AlertOSC, s.AlertOSCAddr, s.AlertWebhook = req.OSC, req.OSCAddr, req.Webhook
		saveSettings(s)
	}
	s := loadSettings()
	writeJSON(w, map[string]string{"osc": s.AlertOSC, "osc_address": s.AlertOSCAddr, "webhook": s.AlertWebhook})
}
