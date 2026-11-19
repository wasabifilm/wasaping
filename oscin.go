package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Commands over OSC (UDP), from QLab, a lighting console, Companion /
// Stream Deck… Only these, nothing that changes devices or settings:
//
//	/wasaping/monitor/start   /wasaping/monitor/stop
//	/wasaping/show/start      /wasaping/show/end      (markers for the report)
//	/wasaping/mute [seconds]  /wasaping/unmute
//	/wasaping/scan            /wasaping/preshow
//
// Off by default; Settings → Rete e accesso turns it on, with a port.

const defaultOSCInPort = 53001

var oscCommands = []string{"/wasaping/monitor/start", "/wasaping/monitor/stop", "/wasaping/show/start", "/wasaping/show/end",
	"/wasaping/mute", "/wasaping/unmute", "/wasaping/scan", "/wasaping/preshow"}

type oscServer struct {
	mu     sync.Mutex
	conn   *net.UDPConn
	port   int
	last   string // latest command, for the settings page
	lastAt time.Time
}

var oscIn = &oscServer{}

// apply starts or stops the listener to match the settings.
func (o *oscServer) apply(ctx context.Context) error {
	port := loadSettings().OSCInPort
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.conn != nil && o.port == port {
		return nil
	}
	if o.conn != nil {
		o.conn.Close()
		o.conn = nil
	}
	o.port = port
	if port == 0 {
		return nil
	}
	c, err := net.ListenUDP("udp", &net.UDPAddr{Port: port})
	if err != nil {
		return fmt.Errorf(T("porta OSC %d non disponibile: %v", "OSC port %d not available: %v"), port, err)
	}
	o.conn = c
	go o.serve(ctx, c)
	return nil
}

func (o *oscServer) serve(ctx context.Context, c *net.UDPConn) {
	buf := make([]byte, 2048)
	var recent []time.Time
	for {
		n, _, err := c.ReadFromUDP(buf)
		if err != nil {
			return // closed
		}
		// At most 20 commands every 10 seconds: a loop in a cue list must
		// not keep wasaping scanning.
		now := time.Now()
		k := 0
		for _, t := range recent {
			if now.Sub(t) < 10*time.Second {
				recent[k] = t
				k++
			}
		}
		recent = recent[:k]
		if len(recent) >= 20 {
			continue
		}
		for _, m := range oscMessages(buf[:n]) {
			if o.run(ctx, m.addr, m.args) {
				recent = append(recent, now)
			}
		}
	}
}

type oscMsg struct {
	addr string
	args []float64
}

// oscMessages parses a packet: one message or a bundle of them. Numeric
// arguments (i, f) are kept, the others skipped.
func oscMessages(p []byte) []oscMsg {
	if strings.HasPrefix(string(p), "#bundle\x00") {
		var out []oscMsg
		for i := 16; i+4 <= len(p); {
			size := int(binary.BigEndian.Uint32(p[i:]))
			i += 4
			if size <= 0 || i+size > len(p) {
				break
			}
			out = append(out, oscMessages(p[i:i+size])...)
			i += size
		}
		return out
	}
	addr, rest, ok := oscReadString(p)
	if !ok || !strings.HasPrefix(addr, "/") {
		return nil
	}
	m := oscMsg{addr: addr}
	tags, rest, ok := oscReadString(rest)
	if ok && strings.HasPrefix(tags, ",") {
		for _, t := range tags[1:] {
			switch t {
			case 'i':
				if len(rest) < 4 {
					return []oscMsg{m}
				}
				m.args = append(m.args, float64(int32(binary.BigEndian.Uint32(rest))))
				rest = rest[4:]
			case 'f':
				if len(rest) < 4 {
					return []oscMsg{m}
				}
				m.args = append(m.args, float64(math.Float32frombits(binary.BigEndian.Uint32(rest))))
				rest = rest[4:]
			case 's':
				_, rest, _ = oscReadString(rest)
			default:
				return []oscMsg{m} // other types: stop reading arguments
			}
		}
	}
	return []oscMsg{m}
}

// oscString reads a 0-terminated string padded to 4 bytes.
func oscReadString(p []byte) (string, []byte, bool) {
	i := strings.IndexByte(string(p), 0)
	if i < 0 {
		return "", nil, false
	}
	n := (i + 4) &^ 3
	if n > len(p) {
		n = len(p)
	}
	return string(p[:i]), p[n:], true
}

// run carries out a command; false when it is not one of ours.
func (o *oscServer) run(ctx context.Context, addr string, args []float64) bool {
	addr = strings.ToLower(strings.TrimSuffix(addr, "/"))
	switch addr {
	case "/wasaping/monitor/start":
		mon.set(true, 0)
	case "/wasaping/monitor/stop":
		mon.set(false, 0)
	case "/wasaping/show/start":
		mon.set(true, 0)
		mon.markShow(true)
	case "/wasaping/show/end":
		mon.markShow(false)
	case "/wasaping/mute":
		sec := 300
		if len(args) > 0 && args[0] > 0 {
			sec = min(int(args[0]), 3600)
		}
		mon.mute(time.Duration(sec) * time.Second)
	case "/wasaping/unmute":
		mon.mute(0)
	case "/wasaping/scan":
		select {
		case bgKick <- struct{}{}:
		default:
		}
	case "/wasaping/preshow":
		go func() {
			c, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			activeScan.Add(1)
			rep, err := runPreshow(c, "", loadSettings().Lang)
			activeScan.Add(-1)
			text := "pre-show: " + T("errore", "error")
			if err == nil {
				text = "pre-show: " + rep.Headline
			}
			mon.mu.Lock()
			mon.addEventRaw(MonEvent{At: time.Now(), Name: "Pre-show", Kind: "note", Text: text})
			mon.mu.Unlock()
		}()
	default:
		return false
	}
	o.mu.Lock()
	o.last, o.lastAt = addr, time.Now()
	o.mu.Unlock()
	return true
}

func (o *oscServer) status() map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	return map[string]any{"on": o.conn != nil, "port": firstPositive(o.port, loadSettings().OSCInPort, defaultOSCInPort),
		"last": o.last, "last_at": o.lastAt, "commands": oscCommands, "ips": localIPv4s()}
}

// handleOSCIn: GET the state; POST {port} (0 = off).
func handleOSCIn(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Port int `json:"port"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil || (req.Port != 0 && (req.Port < 1024 || req.Port > 65535)) {
			http.Error(w, T("porta non valida (1024–65535)", "invalid port (1024–65535)"), http.StatusBadRequest)
			return
		}
		s := loadSettings()
		s.OSCInPort = req.Port
		saveSettings(s)
		if err := oscIn.apply(context.Background()); err != nil {
			s.OSCInPort = 0
			saveSettings(s)
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
	}
	writeJSON(w, oscIn.status())
}

func localIPv4s() []string {
	var out []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() {
			out = append(out, n.IP.String())
		}
	}
	return out
}
