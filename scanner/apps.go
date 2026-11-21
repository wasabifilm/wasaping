package scanner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AppInfo is a server program found on a port: databases, Docker,
// Kubernetes… Version is filled when the program says it without a
// password; NoAuth means it also answered a question that should need one.
type AppInfo struct {
	Port    int    `json:"port"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	NoAuth  bool   `json:"no_auth,omitempty"`
}

// appProbes are the programs we ask for their version, by port. Each one
// only reads: no login, no write, at most one or two small requests.
var appProbes = map[int]func(ctx context.Context, ip string, port int, t time.Duration) *AppInfo{
	3306:  probeMySQL,
	6379:  probeRedis,
	27017: probeMongo,
	1433:  probeMSSQL,
	9200:  probeElastic,
	8086:  probeInflux,
	2375:  probeDocker,
	6443:  probeKube,
	5432:  func(context.Context, string, int, time.Duration) *AppInfo { return &AppInfo{Name: "PostgreSQL"} },
}

// probeApps asks the open ports that have a probe, all at once.
func probeApps(ctx context.Context, ip string, ports []int, timeout time.Duration) []AppInfo {
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []AppInfo
	)
	for _, p := range ports {
		f := appProbes[p]
		if f == nil {
			continue
		}
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			if a := f(ctx, ip, p, timeout); a != nil {
				a.Port = p
				mu.Lock()
				out = append(out, *a)
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	// Stable order: by port.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Port < out[j-1].Port; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// withTitleApp adds the product the web page names (Proxmox VE, iDRAC,
// TrueNAS…) to the programs, unless it is just a device model.
func withTitleApp(apps []AppInfo, title string) []AppInfo {
	p := serverTitle(strings.ToLower(title))
	if p == "" || p == "NETGEAR" {
		return apps
	}
	for _, a := range apps {
		if strings.HasPrefix(a.Name, p) {
			return apps
		}
	}
	return append(apps, AppInfo{Name: p})
}

func dialApp(ctx context.Context, ip string, port int, t time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: t}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	c.SetDeadline(time.Now().Add(t))
	return c, nil
}

// probeMySQL reads the greeting the server sends first: protocol 10, then
// the version as a C string. An error packet (0xff) means "host not
// allowed": still MySQL.
func probeMySQL(ctx context.Context, ip string, port int, t time.Duration) *AppInfo {
	c, err := dialApp(ctx, ip, port, t)
	if err != nil {
		return nil
	}
	defer c.Close()
	b := make([]byte, 256)
	n, _ := io.ReadAtLeast(c, b, 6)
	return parseMySQLGreeting(b[:n])
}

func parseMySQLGreeting(b []byte) *AppInfo {
	if len(b) < 6 {
		return nil
	}
	body := b[4:]
	switch body[0] {
	case 10:
		v := body[1:]
		if i := bytes.IndexByte(v, 0); i > 0 {
			ver := string(v[:i])
			name := "MySQL"
			if strings.Contains(strings.ToLower(ver), "mariadb") {
				name = "MariaDB"
				ver = strings.TrimPrefix(ver, "5.5.5-") // MariaDB 10+ hides behind 5.5.5-
			}
			if j := strings.IndexAny(ver, "-+"); j > 0 {
				ver = ver[:j]
			}
			return &AppInfo{Name: name, Version: ver}
		}
	case 0xff:
		return &AppInfo{Name: "MySQL"}
	}
	return nil
}

// probeRedis asks INFO server: the version when there is no password,
// "-NOAUTH" otherwise.
func probeRedis(ctx context.Context, ip string, port int, t time.Duration) *AppInfo {
	c, err := dialApp(ctx, ip, port, t)
	if err != nil {
		return nil
	}
	defer c.Close()
	c.Write([]byte("INFO server\r\n"))
	// Redis keeps the connection open: read the first line, then the bulk
	// reply it announces ("$<len>").
	r := bufio.NewReader(c)
	line, err := r.ReadString('\n')
	if err != nil {
		return nil
	}
	b := []byte(line)
	if n, err := strconv.Atoi(strings.TrimSpace(line[1:])); line[0] == '$' && err == nil && n > 0 {
		body := make([]byte, min(n, 8192))
		k, _ := io.ReadFull(r, body)
		b = append(b, body[:k]...)
	}
	return parseRedisInfo(b)
}

func parseRedisInfo(b []byte) *AppInfo {
	s := string(b)
	switch {
	case strings.HasPrefix(s, "-NOAUTH"), strings.HasPrefix(s, "-ERR") && strings.Contains(s, "auth"):
		return &AppInfo{Name: "Redis"}
	case strings.HasPrefix(s, "$"):
		a := &AppInfo{Name: "Redis", NoAuth: true}
		for _, l := range strings.Split(s, "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(l), "redis_version:"); ok {
				a.Version = v
			}
		}
		return a
	}
	return nil
}

// probeElastic reads GET /: version.number, or 401 with security on.
func probeElastic(ctx context.Context, ip string, port int, t time.Duration) *AppInfo {
	code, body, _ := appGET(ctx, "http://"+net.JoinHostPort(ip, strconv.Itoa(port))+"/", t)
	var r struct {
		Version struct {
			Number       string `json:"number"`
			Distribution string `json:"distribution"`
		} `json:"version"`
		Tagline string `json:"tagline"`
	}
	switch {
	case code == http.StatusUnauthorized:
		return &AppInfo{Name: "Elasticsearch"}
	case code == http.StatusOK && json.Unmarshal(body, &r) == nil && r.Version.Number != "":
		name := "Elasticsearch"
		if r.Version.Distribution == "opensearch" {
			name = "OpenSearch"
		}
		return &AppInfo{Name: name, Version: r.Version.Number, NoAuth: true}
	}
	return nil
}

// probeInflux: GET /ping answers 204 with the version in a header.
func probeInflux(ctx context.Context, ip string, port int, t time.Duration) *AppInfo {
	_, _, h := appGET(ctx, "http://"+net.JoinHostPort(ip, strconv.Itoa(port))+"/ping", t)
	if v := h.Get("X-Influxdb-Version"); v != "" {
		return &AppInfo{Name: "InfluxDB", Version: strings.TrimPrefix(v, "v")}
	}
	return nil
}

// probeDocker: the plain-HTTP Docker API has no password at all.
func probeDocker(ctx context.Context, ip string, port int, t time.Duration) *AppInfo {
	code, body, _ := appGET(ctx, "http://"+net.JoinHostPort(ip, strconv.Itoa(port))+"/version", t)
	var r struct{ Version string }
	if code == http.StatusOK && json.Unmarshal(body, &r) == nil && r.Version != "" {
		return &AppInfo{Name: "Docker", Version: r.Version, NoAuth: true}
	}
	return nil
}

// probeKube: /version is usually readable without a token.
func probeKube(ctx context.Context, ip string, port int, t time.Duration) *AppInfo {
	code, body, _ := appGET(ctx, "https://"+net.JoinHostPort(ip, strconv.Itoa(port))+"/version", t)
	var r struct {
		GitVersion string `json:"gitVersion"`
	}
	switch {
	case code == http.StatusOK && json.Unmarshal(body, &r) == nil && r.GitVersion != "":
		return &AppInfo{Name: "Kubernetes", Version: strings.TrimPrefix(r.GitVersion, "v")}
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return &AppInfo{Name: "Kubernetes"}
	}
	return nil
}

func appGET(ctx context.Context, u string, t time.Duration) (int, []byte, http.Header) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, nil, nil
	}
	req.Header.Set("User-Agent", "wasaping")
	resp, err := deviceHTTPClient(t).Do(req)
	if err != nil {
		return 0, nil, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, b, resp.Header
}

// probeMSSQL sends a TDS PRELOGIN (asks nothing secret) and reads the
// VERSION option of the answer.
func probeMSSQL(ctx context.Context, ip string, port int, t time.Duration) *AppInfo {
	c, err := dialApp(ctx, ip, port, t)
	if err != nil {
		return nil
	}
	defer c.Close()
	// Options: VERSION (6 bytes), ENCRYPTION (1 byte: 2 = not supported), end.
	opts := []byte{0x00, 0, 11, 0, 6, 0x01, 0, 17, 0, 1, 0xff}
	data := append(opts, 0, 0, 0, 0, 0, 0, 2)
	pkt := []byte{0x12, 0x01, 0, 0, 0, 0, 1, 0}
	binary.BigEndian.PutUint16(pkt[2:], uint16(8+len(data)))
	c.Write(append(pkt, data...))
	b := make([]byte, 512)
	n, _ := io.ReadAtLeast(c, b, 8)
	return parseMSSQLPrelogin(b[:n])
}

func parseMSSQLPrelogin(b []byte) *AppInfo {
	if len(b) < 8 || b[0] != 0x04 {
		return nil
	}
	p := b[8:]
	for i := 0; i+5 <= len(p) && p[i] != 0xff; i += 5 {
		if p[i] != 0x00 {
			continue
		}
		off, ln := int(binary.BigEndian.Uint16(p[i+1:])), int(binary.BigEndian.Uint16(p[i+3:]))
		if ln < 4 || off+4 > len(p) {
			break
		}
		major, minor, build := p[off], p[off+1], binary.BigEndian.Uint16(p[off+2:])
		name := "SQL Server"
		years := map[byte]string{17: "2025", 16: "2022", 15: "2019", 14: "2017", 13: "2016", 12: "2014", 11: "2012", 10: "2008"}
		if y := years[major]; y != "" {
			name += " " + y
		}
		return &AppInfo{Name: name, Version: fmt.Sprintf("%d.%d.%d", major, minor, build)}
	}
	return &AppInfo{Name: "SQL Server"}
}

// probeMongo asks buildInfo (always allowed) for the version, then
// listDatabases, which fails with "requires authentication" when there is
// a password.
func probeMongo(ctx context.Context, ip string, port int, t time.Duration) *AppInfo {
	c, err := dialApp(ctx, ip, port, t)
	if err != nil {
		return nil
	}
	defer c.Close()
	info, ok := mongoCmd(c, 1, bsonDoc("buildInfo", int32(1), "$db", "admin"))
	if !ok {
		return nil
	}
	a := &AppInfo{Name: "MongoDB"}
	a.Version, _ = info["version"].(string)
	if r, ok := mongoCmd(c, 2, bsonDoc("listDatabases", int32(1), "nameOnly", true, "$db", "admin")); ok {
		if v, _ := r["ok"].(float64); v == 1 {
			a.NoAuth = true
		}
	}
	return a
}

// mongoCmd sends one OP_MSG with a command and reads the reply document.
func mongoCmd(c net.Conn, id int32, doc []byte) (map[string]any, bool) {
	msg := make([]byte, 16, 21+len(doc))
	binary.LittleEndian.PutUint32(msg[4:], uint32(id))
	binary.LittleEndian.PutUint32(msg[12:], 2013) // OP_MSG
	msg = append(msg, 0, 0, 0, 0, 0)              // flags, section kind 0
	msg = append(msg, doc...)
	binary.LittleEndian.PutUint32(msg[0:], uint32(len(msg)))
	if _, err := c.Write(msg); err != nil {
		return nil, false
	}
	head := make([]byte, 16)
	if _, err := io.ReadFull(c, head); err != nil {
		return nil, false
	}
	n := int(binary.LittleEndian.Uint32(head))
	if n < 21 || n > 1<<20 || binary.LittleEndian.Uint32(head[12:]) != 2013 {
		return nil, false
	}
	body := make([]byte, n-16)
	if _, err := io.ReadFull(c, body); err != nil {
		return nil, false
	}
	return parseBSON(body[5:])
}

// bsonDoc builds a flat BSON document from key, value pairs (int32,
// string, bool).
func bsonDoc(kv ...any) []byte {
	b := []byte{0, 0, 0, 0}
	for i := 0; i+1 < len(kv); i += 2 {
		k := kv[i].(string)
		switch v := kv[i+1].(type) {
		case int32:
			b = append(b, 0x10)
			b = append(append(b, k...), 0)
			b = binary.LittleEndian.AppendUint32(b, uint32(v))
		case string:
			b = append(b, 0x02)
			b = append(append(b, k...), 0)
			b = binary.LittleEndian.AppendUint32(b, uint32(len(v)+1))
			b = append(append(b, v...), 0)
		case bool:
			b = append(b, 0x08)
			b = append(append(b, k...), 0)
			if v {
				b = append(b, 1)
			} else {
				b = append(b, 0)
			}
		}
	}
	b = append(b, 0)
	binary.LittleEndian.PutUint32(b, uint32(len(b)))
	return b
}

// parseBSON reads the top-level strings, numbers and booleans of a BSON
// document; nested documents and arrays are skipped.
func parseBSON(b []byte) (map[string]any, bool) {
	if len(b) < 5 {
		return nil, false
	}
	n := int(binary.LittleEndian.Uint32(b))
	if n > len(b) || n < 5 {
		return nil, false
	}
	out := map[string]any{}
	i := 4
	for i < n-1 {
		t := b[i]
		i++
		end := bytes.IndexByte(b[i:n], 0)
		if end < 0 {
			return out, false
		}
		k := string(b[i : i+end])
		i += end + 1
		need := func(m int) bool { return i+m <= n }
		switch t {
		case 0x01: // double
			if !need(8) {
				return out, false
			}
			out[k] = math.Float64frombits(binary.LittleEndian.Uint64(b[i:]))
			i += 8
		case 0x02: // string
			if !need(4) {
				return out, false
			}
			l := int(binary.LittleEndian.Uint32(b[i:]))
			if l < 1 || !need(4+l) {
				return out, false
			}
			out[k] = string(b[i+4 : i+4+l-1])
			i += 4 + l
		case 0x03, 0x04: // document, array
			if !need(4) {
				return out, false
			}
			l := int(binary.LittleEndian.Uint32(b[i:]))
			if l < 5 || !need(l) {
				return out, false
			}
			i += l
		case 0x08: // bool
			if !need(1) {
				return out, false
			}
			out[k] = b[i] != 0
			i++
		case 0x10: // int32
			if !need(4) {
				return out, false
			}
			out[k] = float64(int32(binary.LittleEndian.Uint32(b[i:])))
			i += 4
		case 0x12, 0x09, 0x11: // int64, datetime, timestamp
			if !need(8) {
				return out, false
			}
			if t == 0x12 {
				out[k] = float64(int64(binary.LittleEndian.Uint64(b[i:])))
			}
			i += 8
		case 0x07: // ObjectId
			i += 12
		case 0x0A: // null
		case 0x05: // binary
			if !need(5) {
				return out, false
			}
			i += 5 + int(binary.LittleEndian.Uint32(b[i:]))
		default:
			return out, true // unknown type: stop, keep what we have
		}
	}
	return out, true
}
