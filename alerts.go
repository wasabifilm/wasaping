package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
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

// alertCfg is where alarms go.
type alertCfg struct {
	OSC      string `json:"osc"`
	OSCAddr  string `json:"osc_address"`
	Webhook  string `json:"webhook"`
	Telegram string `json:"telegram"` // "bot-token|chat-id"
	Ntfy     string `json:"ntfy"`     // https://ntfy.sh/topic
	Slack    string `json:"slack"`    // Slack incoming webhook
}

func (a alertCfg) empty() bool {
	return a.OSC == "" && a.Webhook == "" && a.Telegram == "" && a.Ntfy == "" && a.Slack == ""
}

func alertsOf(s settings) alertCfg {
	return alertCfg{s.AlertOSC, s.AlertOSCAddr, s.AlertWebhook, s.Telegram, s.Ntfy, s.Slack}
}

// sendAlert delivers an event to the configured targets (errors are ignored:
// an alarm must never block the monitor).
func sendAlert(e MonEvent) {
	if a := alertsOf(loadSettings()); !a.empty() {
		alertTo(a, e)
	}
}

// alertText is the message for people (phone notifications, chat).
func alertText(e MonEvent, netName string) (title, body string) {
	who := e.Name
	if e.IP != "" && e.IP != e.Name {
		who += " (" + e.IP + ")"
	}
	switch {
	case e.Text != "":
		title, body = "⚠ "+e.Name, who+": "+e.Text
	case e.Up:
		title = "✅ " + T("Di nuovo in linea: ", "Back online: ") + e.Name
		body = who + T(" è di nuovo in linea", " is back online")
		if e.DownFor > 0 {
			body += fmt.Sprintf(T(" dopo %s", " after %s"), (time.Duration(e.DownFor) * time.Second).String())
		}
	default:
		title = "🔴 " + T("Non risponde: ", "Not responding: ") + e.Name
		body = who + T(" non risponde", " is not responding")
	}
	if e.Group != "" {
		title = strings.Replace(title, " ", " "+groupName(e.Group)+" · ", 1) // "🔴 Audio · Non risponde: …"
	}
	if netName != "" {
		body += " · " + netName
	}
	return title, body + " · wasaping " + e.At.Format("15:04:05")
}

// telegramAPI is a variable so that tests can point it to a fake server.
var telegramAPI = "https://api.telegram.org"

var alertClient = &http.Client{Timeout: 5 * time.Second}

func httpResult(name string, resp *http.Response, err error) string {
	if err != nil {
		return name + ": " + err.Error()
	}
	resp.Body.Close()
	if resp.StatusCode/100 == 2 {
		return name + " ✓"
	}
	return fmt.Sprintf("%s: %d", name, resp.StatusCode)
}

func alertTo(a alertCfg, e MonEvent) []string {
	var res []string
	netName := ""
	if n, ok := devices().Current(); ok {
		netName = firstNonEmpty(n.Name, n.Auto)
	}
	if a.OSC != "" {
		base := strings.TrimRight(firstNonEmpty(a.OSCAddr, "/wasaping"), "/")
		c, err := net.DialTimeout("udp", a.OSC, 2*time.Second)
		if err != nil {
			res = append(res, "OSC: "+err.Error())
		} else {
			_, err = c.Write(oscMessage(base+"/"+eventWord(e), e.Name, e.IP, firstNonEmpty(e.Text, ""), int(e.DownFor), groupNameIT(e.Group)))
			c.Close()
			if err != nil {
				res = append(res, "OSC: "+err.Error())
			} else {
				res = append(res, "OSC ✓")
			}
		}
	}
	if a.Webhook != "" {
		body, _ := json.Marshal(map[string]any{
			"event": eventWord(e), "name": e.Name, "ip": e.IP, "text": e.Text, "kind": firstNonEmpty(e.Kind, "device"),
			"down_for_s": int(e.DownFor), "at": e.At.Format(time.RFC3339), "network": netName, "source": "wasaping " + version,
			"group": e.Group, "group_name": groupNameIT(e.Group),
		})
		resp, err := alertClient.Post(a.Webhook, "application/json", bytes.NewReader(body))
		res = append(res, httpResult("web", resp, err))
	}
	title, text := alertText(e, netName)
	if token, chat, ok := strings.Cut(a.Telegram, "|"); ok {
		resp, err := alertClient.PostForm(telegramAPI+"/bot"+token+"/sendMessage", url.Values{"chat_id": {chat}, "text": {title + "\n" + text}})
		r := httpResult("Telegram", resp, err)
		res = append(res, strings.ReplaceAll(r, token, "…")) // never show the token in errors
	}
	if a.Ntfy != "" {
		req, _ := http.NewRequest("POST", a.Ntfy, strings.NewReader(text))
		req.Header.Set("Title", title)
		req.Header.Set("Tags", "wasaping")
		if eventWord(e) == "down" {
			req.Header.Set("Priority", "high")
		}
		resp, err := alertClient.Do(req)
		res = append(res, httpResult("ntfy", resp, err))
	}
	if a.Slack != "" {
		body, _ := json.Marshal(map[string]string{"text": "*" + title + "*\n" + text})
		resp, err := alertClient.Post(a.Slack, "application/json", bytes.NewReader(body))
		res = append(res, httpResult("Slack", resp, err))
	}
	return res
}

var (
	reTGToken = regexp.MustCompile(`^\d{5,}:[A-Za-z0-9_-]{20,}$`)
	reTGChat  = regexp.MustCompile(`^(-?\d{1,20}|@[A-Za-z0-9_]{4,})$`)
)

func validWebURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// handleAlerts: GET the configuration; POST alertCfg (+ test).
func handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			alertCfg
			Test bool `json:"test"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		a, err := cleanAlerts(req.alertCfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Test {
			res := alertTo(a, MonEvent{At: time.Now(), Name: T("Prova di wasaping", "wasaping test"), Text: T("messaggio di prova", "test message")})
			writeJSON(w, map[string]string{"result": strings.Join(res, " · ")})
			return
		}
		s := loadSettings()
		s.AlertOSC, s.AlertOSCAddr, s.AlertWebhook, s.Telegram, s.Ntfy, s.Slack = a.OSC, a.OSCAddr, a.Webhook, a.Telegram, a.Ntfy, a.Slack
		saveSettings(s)
	}
	writeJSON(w, alertsOf(loadSettings()))
}

// cleanAlerts checks and normalizes the alert destinations (settings page
// and settings import).
func cleanAlerts(a alertCfg) (alertCfg, error) {
	for _, p := range []*string{&a.OSC, &a.OSCAddr, &a.Webhook, &a.Telegram, &a.Ntfy, &a.Slack} {
		*p = strings.TrimSpace(*p)
	}
	if a.OSC != "" {
		if _, _, err := net.SplitHostPort(a.OSC); err != nil {
			return a, errors.New(T("OSC: scrivi indirizzo:porta, es. 192.168.1.20:53000", "OSC: write address:port, e.g. 192.168.1.20:53000"))
		}
	}
	if a.OSCAddr != "" && !strings.HasPrefix(a.OSCAddr, "/") {
		a.OSCAddr = "/" + a.OSCAddr
	}
	if a.Webhook != "" && !validWebURL(a.Webhook) {
		return a, errors.New(T("indirizzo web non valido", "invalid web address"))
	}
	if a.Telegram != "" {
		token, chat, _ := strings.Cut(a.Telegram, "|")
		if !reTGToken.MatchString(strings.TrimSpace(token)) || !reTGChat.MatchString(strings.TrimSpace(chat)) {
			return a, errors.New(T("Telegram: serve il token del bot (da @BotFather) e l'id della chat", "Telegram: the bot token (from @BotFather) and the chat id are needed"))
		}
		a.Telegram = strings.TrimSpace(token) + "|" + strings.TrimSpace(chat)
	}
	if a.Ntfy != "" && !strings.Contains(a.Ntfy, "://") {
		a.Ntfy = "https://ntfy.sh/" + strings.TrimPrefix(a.Ntfy, "/") // just the topic name
	}
	if a.Ntfy != "" && !validWebURL(a.Ntfy) {
		return a, errors.New(T("ntfy: scrivi il nome dell'argomento o l'indirizzo completo", "ntfy: write the topic name or the full address"))
	}
	if a.Slack != "" && !strings.HasPrefix(a.Slack, "https://") {
		return a, errors.New(T("Slack: incolla l'indirizzo del webhook (https://hooks.slack.com/…)", "Slack: paste the webhook address (https://hooks.slack.com/…)"))
	}
	return a, nil
}
