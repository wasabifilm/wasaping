package scanner

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Video gear, read on demand (never during the scans: some devices accept
// few control connections): Blackmagic Videohub (TCP 9990, documented text
// protocol), HyperDeck (TCP 9993, documented), ATEM (UDP 9910, protocol
// reconstructed by the community), vMix (HTTP API on 8088), Resolume (HTTP
// API), and what can only be recognized: OBS, Watchout.

// Ports, variables for the tests.
var (
	VideohubPort  = 9990
	HyperDeckPort = 9993
	ATEMPort      = 9910
	VMixPort      = 8088
)

// VideoInfo is the state of one video device.
type VideoInfo struct {
	IP      string   `json:"ip"`
	Kind    string   `json:"kind"` // ATEM, HyperDeck, Videohub, vMix, Resolume, OBS, Watchout
	Model   string   `json:"model,omitempty"`
	Version string   `json:"version,omitempty"`
	Lines   []string `json:"lines,omitempty"` // state in words: "Programma: Camera 1"
	Warn    []string `json:"warn,omitempty"`
	// Videohub: who goes where.
	Routes []VideoRoute `json:"routes,omitempty"`
	Err    string       `json:"error,omitempty"`
}

// VideoRoute is one Videohub output and the input on it.
type VideoRoute struct {
	Out    int    `json:"out"`
	OutLbl string `json:"out_label"`
	In     int    `json:"in"`
	InLbl  string `json:"in_label"`
	Locked bool   `json:"locked,omitempty"`
}

// VideoCandidate says whether a scanned host is worth asking, and how.
func VideoCandidate(h Host) string {
	svc := func(s string) bool { return slices.Contains(h.Services, s) }
	switch {
	case containsInt(h.Ports, 9990) || svc("videohub"):
		return "Videohub"
	case containsInt(h.Ports, 9993):
		return "HyperDeck"
	case containsInt(h.Ports, 8088):
		return "vMix"
	case containsInt(h.Ports, 4455):
		return "OBS"
	case containsInt(h.Ports, 3039) || containsInt(h.Ports, 3040):
		return "Watchout"
	case strings.Contains(strings.ToLower(h.WebTitle), "resolume"):
		return "Resolume"
	case svc("blackmagic") || strings.HasPrefix(strings.ToLower(h.Vendor), "blackmagic"):
		return "ATEM" // a Blackmagic device that is neither of the above: try ATEM
	}
	return ""
}

// VideoStatus asks one device, by kind.
func VideoStatus(ctx context.Context, ip, kind string) VideoInfo {
	v := VideoInfo{IP: ip, Kind: kind}
	var err error
	switch kind {
	case "Videohub":
		err = readVideohub(ctx, ip, &v)
	case "HyperDeck":
		err = readHyperDeck(ctx, ip, &v)
	case "ATEM":
		err = readATEM(ctx, ip, &v)
	case "vMix":
		err = readVMix(ctx, ip, &v)
	case "Resolume":
		err = readResolume(ctx, ip, &v)
	case "OBS":
		v.Model, v.Lines = "OBS Studio", []string{"Controllo remoto (obs-websocket) attivo sulla porta 4455."}
	case "Watchout":
		v.Model, v.Lines = "Dataton Watchout", []string{"Porte di Watchout aperte (3039/3040)."}
	default:
		err = fmt.Errorf("tipo sconosciuto")
	}
	if err != nil {
		v.Err = err.Error()
	}
	return v
}

// ---- Videohub ----

func readVideohub(ctx context.Context, ip string, v *VideoInfo) error {
	d := net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(VideohubPort)))
	if err != nil {
		return fmt.Errorf("non raggiungibile sulla porta %d", VideohubPort)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	blocks := map[string][]string{}
	cur := ""
	sc := bufio.NewScanner(io.LimitReader(c, 1<<20))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case line == "":
			cur = ""
		case strings.HasSuffix(line, ":") && cur == "":
			cur = strings.TrimSuffix(line, ":")
			if cur == "END PRELUDE" {
				goto done
			}
		case cur != "":
			blocks[cur] = append(blocks[cur], line)
		}
	}
done:
	diag("videohub", ip, []byte(fmt.Sprint(blocks)))
	if len(blocks) == 0 {
		return fmt.Errorf("nessuna risposta dal Videohub")
	}
	kv := func(block, key string) string {
		for _, l := range blocks[block] {
			if k, val, ok := strings.Cut(l, ": "); ok && k == key {
				return val
			}
		}
		return ""
	}
	v.Model = kv("VIDEOHUB DEVICE", "Model name")
	v.Version = kv("PROTOCOL PREAMBLE", "Version")
	if fn := kv("VIDEOHUB DEVICE", "Friendly name"); fn != "" {
		v.Lines = append(v.Lines, "Nome: "+fn)
	}
	v.Lines = append(v.Lines, fmt.Sprintf("%s ingressi, %s uscite", kv("VIDEOHUB DEVICE", "Video inputs"), kv("VIDEOHUB DEVICE", "Video outputs")))
	if kv("VIDEOHUB DEVICE", "Device present") == "needs_update" {
		v.Warn = append(v.Warn, "Il Videohub chiede un aggiornamento del firmware.")
	}
	numbered := func(block string) map[int]string {
		m := map[int]string{}
		for _, l := range blocks[block] {
			if a, b, ok := strings.Cut(l, " "); ok {
				if n, err := strconv.Atoi(a); err == nil {
					m[n] = b
				}
			}
		}
		return m
	}
	in, out, locks := numbered("INPUT LABELS"), numbered("OUTPUT LABELS"), numbered("VIDEO OUTPUT LOCKS")
	for o, s := range numbered("VIDEO OUTPUT ROUTING") {
		i, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		v.Routes = append(v.Routes, VideoRoute{Out: o + 1, OutLbl: out[o], In: i + 1, InLbl: in[i], Locked: locks[o] == "L" || locks[o] == "O"})
	}
	slices.SortFunc(v.Routes, func(a, b VideoRoute) int { return a.Out - b.Out })
	return nil
}

// ---- HyperDeck ----

type hdConn struct {
	c  net.Conn
	rd *bufio.Reader
}

// cmd sends a command and returns the response's code and fields.
func (h *hdConn) cmd(s string) (int, map[string]string, error) {
	if s != "" {
		if _, err := h.c.Write([]byte(s + "\r\n")); err != nil {
			return 0, nil, err
		}
	}
	for {
		line, err := h.rd.ReadString('\n')
		if err != nil {
			return 0, nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		code, rest, _ := strings.Cut(line, " ")
		n, err := strconv.Atoi(code)
		if err != nil {
			continue
		}
		fields := map[string]string{}
		if strings.HasSuffix(rest, ":") { // multi-line, until an empty line
			for {
				l, err := h.rd.ReadString('\n')
				l = strings.TrimRight(l, "\r\n")
				if err != nil || l == "" {
					break
				}
				if k, v, ok := strings.Cut(l, ": "); ok {
					fields[k] = v
				}
			}
		}
		if n >= 500 && s != "" { // asynchronous notification: not our answer
			continue
		}
		return n, fields, nil
	}
}

var hdStatus = map[string]string{"preview": "anteprima", "stopped": "fermo", "play": "in riproduzione", "forward": "avanti", "rewind": "indietro", "jog": "jog", "shuttle": "shuttle", "record": "IN REGISTRAZIONE"}

func readHyperDeck(ctx context.Context, ip string, v *VideoInfo) error {
	d := net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(HyperDeckPort)))
	if err != nil {
		return fmt.Errorf("non raggiungibile sulla porta %d", HyperDeckPort)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(4 * time.Second))
	h := &hdConn{c: c, rd: bufio.NewReader(io.LimitReader(c, 1<<20))}
	if code, f, err := h.cmd(""); err == nil && code == 500 { // connection info
		v.Model, v.Version = f["model"], f["protocol version"]
	} else if code == 120 {
		return fmt.Errorf("il HyperDeck ha già un altro controllo collegato")
	}
	slots := 1
	if code, f, err := h.cmd("device info"); err == nil && code == 204 {
		v.Model = firstNonEmpty(f["model"], v.Model)
		if n, err := strconv.Atoi(f["slot count"]); err == nil && n > 0 && n <= 8 {
			slots = n
		}
		if sw := f["software version"]; sw != "" {
			v.Version = sw
		}
	}
	if code, f, err := h.cmd("transport info"); err == nil && code == 208 {
		st := firstNonEmpty(hdStatus[f["status"]], f["status"])
		v.Lines = append(v.Lines, "Stato: "+st+", timecode "+f["display timecode"])
		if f["video format"] != "" {
			v.Lines = append(v.Lines, "Formato: "+f["video format"])
		}
	}
	for i := 1; i <= slots; i++ {
		code, f, err := h.cmd(fmt.Sprintf("slot info: slot id: %d", i))
		if err != nil || code != 202 {
			continue
		}
		if f["status"] != "mounted" {
			v.Lines = append(v.Lines, fmt.Sprintf("Slot %d: vuoto", i))
			continue
		}
		secs, _ := strconv.Atoi(f["recording time"])
		left := fmt.Sprintf("%d min", secs/60)
		if secs >= 3600 {
			left = fmt.Sprintf("%d h %02d min", secs/3600, secs/60%60)
		}
		v.Lines = append(v.Lines, fmt.Sprintf("Slot %d: %s, %s di registrazione liberi", i, f["volume name"], left))
		if secs > 0 && secs < 30*60 {
			v.Warn = append(v.Warn, fmt.Sprintf("Slot %d: meno di 30 minuti di registrazione liberi.", i))
		}
	}
	h.c.Write([]byte("quit\r\n"))
	return nil
}

// ---- ATEM ----

func atemPacket(flags byte, sid uint16, ack, pid uint16, payload []byte) []byte {
	n := 12 + len(payload)
	b := make([]byte, 12, n)
	b[0], b[1] = flags<<3|byte(n>>8&7), byte(n)
	binary.BigEndian.PutUint16(b[2:], sid)
	binary.BigEndian.PutUint16(b[4:], ack)
	binary.BigEndian.PutUint16(b[10:], pid)
	return append(b, payload...)
}

const (
	atemAckReq = 0x01
	atemHello  = 0x02
	atemAck    = 0x10
)

func readATEM(ctx context.Context, ip string, v *VideoInfo) error {
	c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP(ip), Port: ATEMPort})
	if err != nil {
		return err
	}
	defer c.Close()
	sid := uint16(0x5300 | time.Now().UnixNano()&0xFF)
	hello := atemPacket(atemHello, sid, 0, 0, []byte{0x01, 0, 0, 0, 0, 0, 0, 0})
	c.Write(hello)
	deadline := time.Now().Add(2500 * time.Millisecond)
	cmds := map[string][][]byte{}
	got, done := false, false
	buf := make([]byte, 2048)
	for time.Now().Before(deadline) && !done && ctx.Err() == nil {
		c.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
		n, err := c.Read(buf)
		if err != nil {
			if !got {
				c.Write(hello)
			}
			continue
		}
		b := buf[:n]
		if n < 12 {
			continue
		}
		flags := b[0] >> 3
		sid = binary.BigEndian.Uint16(b[2:])
		if flags&atemHello != 0 {
			if n > 12 && b[12] == 0x03 {
				return fmt.Errorf("l'ATEM non accetta altri collegamenti (troppi client)")
			}
			got = true
			c.Write(atemPacket(atemAck, sid, 0, 0, nil))
			continue
		}
		if flags&atemAckReq != 0 {
			got = true
			c.Write(atemPacket(atemAck, sid, binary.BigEndian.Uint16(b[10:]), 0, nil))
		}
		for o := 12; o+8 <= n; {
			l := int(binary.BigEndian.Uint16(b[o:]))
			if l < 8 || o+l > n {
				break
			}
			name := string(b[o+4 : o+8])
			cmds[name] = append(cmds[name], append([]byte(nil), b[o+8:o+l]...))
			if name == "InCm" { // end of the initial state
				done = true
			}
			o += l
		}
	}
	if !got {
		return fmt.Errorf("nessuna risposta ATEM (porta UDP %d)", ATEMPort)
	}
	if p := cmds["_pin"]; len(p) > 0 {
		v.Model = cString(p[0])
	}
	if p := cmds["_ver"]; len(p) > 0 && len(p[0]) >= 4 {
		v.Version = fmt.Sprintf("%d.%d", binary.BigEndian.Uint16(p[0]), binary.BigEndian.Uint16(p[0][2:]))
	}
	names := map[uint16]string{}
	for _, d := range cmds["InPr"] {
		if len(d) >= 26 {
			names[binary.BigEndian.Uint16(d)] = cString(d[2:22])
		}
	}
	src := func(id uint16) string {
		if n := names[id]; n != "" {
			return n
		}
		return fmt.Sprintf("sorgente %d", id)
	}
	for _, d := range cmds["PrgI"] {
		if len(d) >= 4 {
			v.Lines = append(v.Lines, fmt.Sprintf("M/E %d programma: %s", d[0]+1, src(binary.BigEndian.Uint16(d[2:]))))
		}
	}
	for _, d := range cmds["PrvI"] {
		if len(d) >= 4 {
			v.Lines = append(v.Lines, fmt.Sprintf("M/E %d anteprima: %s", d[0]+1, src(binary.BigEndian.Uint16(d[2:]))))
		}
	}
	diag("atem", ip, []byte(fmt.Sprint(len(cmds), " comandi")))
	return nil
}

// ---- vMix and Resolume (HTTP) ----

var videoHTTP = &http.Client{Timeout: 3 * time.Second}

func readVMix(ctx context.Context, ip string, v *VideoInfo) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s:%d/api", ip, VMixPort), nil)
	resp, err := videoHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("API di vMix non raggiungibile")
	}
	defer resp.Body.Close()
	var x struct {
		Version string `xml:"version"`
		Edition string `xml:"edition"`
		Inputs  []struct {
			Number string `xml:"number,attr"`
			Title  string `xml:"title,attr"`
		} `xml:"inputs>input"`
		Active    string `xml:"active"`
		Preview   string `xml:"preview"`
		Recording string `xml:"recording"`
		Streaming string `xml:"streaming"`
		External  string `xml:"external"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&x); err != nil || x.Version == "" {
		return fmt.Errorf("risposta di vMix non valida")
	}
	v.Model, v.Version = strings.TrimSpace("vMix "+x.Edition), x.Version
	title := func(n string) string {
		for _, i := range x.Inputs {
			if i.Number == n {
				return i.Title
			}
		}
		return n
	}
	v.Lines = append(v.Lines, "In onda: "+title(x.Active), "Anteprima: "+title(x.Preview))
	yes := func(s string) bool { return strings.EqualFold(s, "true") }
	var st []string
	if yes(x.Recording) {
		st = append(st, "registra")
	}
	if yes(x.Streaming) {
		st = append(st, "in streaming")
	}
	if yes(x.External) {
		st = append(st, "uscita esterna attiva")
	}
	if len(st) > 0 {
		v.Lines = append(v.Lines, strings.Join(st, ", "))
	}
	return nil
}

func readResolume(ctx context.Context, ip string, v *VideoInfo) error {
	v.Model = "Resolume"
	for _, port := range []int{8080, 80} {
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s:%d/api/v1/product", ip, port), nil)
		resp, err := videoHTTP.Do(req)
		if err != nil {
			continue
		}
		var p struct {
			Name     string `json:"name"`
			Major    int    `json:"major"`
			Minor    int    `json:"minor"`
			Micro    int    `json:"micro"`
			Revision int    `json:"revision"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&p)
		resp.Body.Close()
		if err == nil && p.Name != "" {
			v.Model, v.Version = "Resolume "+strings.TrimPrefix(p.Name, "Resolume "), fmt.Sprintf("%d.%d.%d", p.Major, p.Minor, p.Micro)
			v.Lines = append(v.Lines, fmt.Sprintf("API web attiva sulla porta %d.", port))
			return nil
		}
	}
	v.Lines = append(v.Lines, "Interfaccia web di Resolume trovata (API non disponibile).")
	return nil
}
