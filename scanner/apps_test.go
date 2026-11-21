package scanner

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeTCP runs a one-shot server per connection and returns its port.
func fakeTCP(t *testing.T, serve func(c net.Conn)) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); serve(c) }()
		}
	}()
	return l.Addr().(*net.TCPAddr).Port
}

func TestProbeMySQL(t *testing.T) {
	greet := func(ver string) func(net.Conn) {
		return func(c net.Conn) {
			body := append([]byte{10}, ver...)
			body = append(body, 0, 1, 2, 3, 4)
			pkt := []byte{byte(len(body)), 0, 0, 0}
			c.Write(append(pkt, body...))
			time.Sleep(200 * time.Millisecond)
		}
	}
	ctx := context.Background()
	if a := probeMySQL(ctx, "127.0.0.1", fakeTCP(t, greet("8.0.36-0ubuntu0.22.04.1")), time.Second); a == nil || a.Name != "MySQL" || a.Version != "8.0.36" {
		t.Errorf("mysql: %+v", a)
	}
	if a := probeMySQL(ctx, "127.0.0.1", fakeTCP(t, greet("5.5.5-10.11.6-MariaDB-0+deb12u1")), time.Second); a == nil || a.Name != "MariaDB" || a.Version != "10.11.6" {
		t.Errorf("mariadb: %+v", a)
	}
	if a := parseMySQLGreeting([]byte{20, 0, 0, 0, 0xff, 0x6a, 0x04, 'H', 'o', 's', 't'}); a == nil || a.Name != "MySQL" || a.Version != "" {
		t.Errorf("host not allowed: %+v", a)
	}
}

func TestProbeRedis(t *testing.T) {
	serve := func(auth bool) func(net.Conn) {
		return func(c net.Conn) {
			r := bufio.NewReader(c)
			if l, _ := r.ReadString('\n'); !strings.HasPrefix(l, "INFO") {
				return
			}
			if auth {
				c.Write([]byte("-NOAUTH Authentication required.\r\n"))
			} else {
				info := "# Server\r\nredis_version:7.2.4\r\nredis_mode:standalone\r\n"
				c.Write([]byte("$" + strconv.Itoa(len(info)) + "\r\n" + info + "\r\n"))
			}
			time.Sleep(2 * time.Second) // Redis keeps the connection open
		}
	}
	ctx := context.Background()
	start := time.Now()
	a := probeRedis(ctx, "127.0.0.1", fakeTCP(t, serve(false)), 1500*time.Millisecond)
	if a == nil || a.Version != "7.2.4" || !a.NoAuth {
		t.Errorf("open redis: %+v", a)
	}
	if time.Since(start) > time.Second {
		t.Errorf("waited for the connection to close: %v", time.Since(start))
	}
	if a := probeRedis(ctx, "127.0.0.1", fakeTCP(t, serve(true)), time.Second); a == nil || a.NoAuth || a.Name != "Redis" {
		t.Errorf("redis with password: %+v", a)
	}
}

func TestProbeMongo(t *testing.T) {
	serve := func(auth bool) func(net.Conn) {
		return func(c net.Conn) {
			for {
				head := make([]byte, 16)
				if _, err := io.ReadFull(c, head); err != nil {
					return
				}
				body := make([]byte, int(binary.LittleEndian.Uint32(head))-16)
				if _, err := io.ReadFull(c, body); err != nil {
					return
				}
				q, _ := parseBSON(body[5:])
				var reply []byte
				switch {
				case q["buildInfo"] != nil:
					reply = bsonDoc("version", "7.0.5", "ok", int32(1))
				case q["listDatabases"] != nil && auth:
					reply = bsonDoc("ok", int32(0), "errmsg", "command listDatabases requires authentication", "code", int32(13))
				case q["listDatabases"] != nil:
					reply = bsonDoc("ok", int32(1))
				}
				// "ok" as a double, as real servers send it
				reply = fixOKDouble(reply)
				msg := make([]byte, 16)
				binary.LittleEndian.PutUint32(msg[12:], 2013)
				msg = append(msg, 0, 0, 0, 0, 0)
				msg = append(msg, reply...)
				binary.LittleEndian.PutUint32(msg, uint32(len(msg)))
				c.Write(msg)
			}
		}
	}
	ctx := context.Background()
	if a := probeMongo(ctx, "127.0.0.1", fakeTCP(t, serve(false)), time.Second); a == nil || a.Version != "7.0.5" || !a.NoAuth {
		t.Errorf("open mongo: %+v", a)
	}
	if a := probeMongo(ctx, "127.0.0.1", fakeTCP(t, serve(true)), time.Second); a == nil || a.Version != "7.0.5" || a.NoAuth {
		t.Errorf("mongo with password: %+v", a)
	}
}

// fixOKDouble rewrites the int32 "ok" of a test document as a double.
func fixOKDouble(doc []byte) []byte {
	m, _ := parseBSON(doc)
	var kv []any
	out := []byte{0, 0, 0, 0}
	for k, v := range m {
		if k == "ok" {
			out = append(out, 0x01)
			out = append(append(out, k...), 0)
			out = binary.LittleEndian.AppendUint64(out, math.Float64bits(v.(float64)))
			continue
		}
		if s, ok := v.(string); ok {
			kv = append(kv, k, s)
		}
		if f, ok := v.(float64); ok {
			kv = append(kv, k, int32(f))
		}
	}
	rest := bsonDoc(kv...)
	out = append(out, rest[4:len(rest)-1]...)
	out = append(out, 0)
	binary.LittleEndian.PutUint32(out, uint32(len(out)))
	return out
}

func TestProbeMSSQL(t *testing.T) {
	port := fakeTCP(t, func(c net.Conn) {
		head := make([]byte, 8)
		if _, err := io.ReadFull(c, head); err != nil || head[0] != 0x12 {
			return
		}
		io.ReadFull(c, make([]byte, int(binary.BigEndian.Uint16(head[2:]))-8))
		opts := []byte{0x00, 0, 11, 0, 6, 0x01, 0, 17, 0, 1, 0xff, 16, 0, 0x10, 0x7a, 0, 0, 2}
		pkt := []byte{0x04, 0x01, 0, byte(8 + len(opts)), 0, 0, 1, 0}
		c.Write(append(pkt, opts...))
	})
	if a := probeMSSQL(context.Background(), "127.0.0.1", port, time.Second); a == nil || a.Name != "SQL Server 2022" || a.Version != "16.0.4218" {
		t.Errorf("mssql: %+v", a)
	}
}

func TestProbeHTTPApps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`{"name":"n1","version":{"number":"8.12.0"},"tagline":"You Know, for Search"}`))
		case "/version":
			w.Write([]byte(`{"Version":"25.0.3","ApiVersion":"1.44"}`))
		case "/ping":
			w.Header().Set("X-Influxdb-Version", "v2.7.5")
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	ctx := context.Background()
	if a := probeElastic(ctx, "127.0.0.1", port, time.Second); a == nil || a.Version != "8.12.0" || !a.NoAuth {
		t.Errorf("elastic: %+v", a)
	}
	if a := probeDocker(ctx, "127.0.0.1", port, time.Second); a == nil || a.Version != "25.0.3" || !a.NoAuth {
		t.Errorf("docker: %+v", a)
	}
	if a := probeInflux(ctx, "127.0.0.1", port, time.Second); a == nil || a.Version != "2.7.5" {
		t.Errorf("influx: %+v", a)
	}
}

func TestServerTitles(t *testing.T) {
	for title, want := range map[string]string{
		"iDRAC9 - Login":                         "iDRAC",
		"iLO 5":                                  "iLO",
		"Camilo's page":                          "",
		"VMware ESXi":                            "VMware ESXi",
		"TrueNAS - 192.168.1.5":                  "TrueNAS",
		"Home Assistant":                         "Home Assistant",
		"pve1 - Proxmox Virtual Environment":     "Proxmox VE",
		"Q-SYS Core Manager":                     "Q-SYS",
		"Cisco Integrated Management Controller": "CIMC",
		"NETGEAR GS724T":                         "NETGEAR",
	} {
		if got := serverTitle(strings.ToLower(title)); got != want {
			t.Errorf("%q: got %q, want %q", title, got, want)
		}
	}
	if boringTitle("iDRAC9 - Login") {
		t.Error("iDRAC login page thrown away")
	}
}

func TestAppKindsAndWarnings(t *testing.T) {
	cases := []struct {
		h    Host
		want string
	}{
		{Host{Ports: []int{443}, WebTitle: "iDRAC9 - Login"}, KindComputer},
		{Host{Ports: []int{80}, WebTitle: "TrueNAS"}, KindNAS},
		{Host{Ports: []int{8123}}, KindIoT},
		{Host{Ports: []int{1710, 80}}, KindAudio},
		{Host{Ports: []int{41794}}, KindVideo},
		{Host{Ports: []int{22, 3306}}, KindComputer},
		{Host{Services: []string{"oca"}}, KindAudio},
	}
	for _, c := range cases {
		if got := GuessKind(c.h); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.h, got, c.want)
		}
	}
	w := strings.Join(Exposure(Host{
		WebTitle: "iLO 5",
		Apps:     []AppInfo{{Port: 6379, Name: "Redis", NoAuth: true}, {Port: 2375, Name: "Docker", NoAuth: true}, {Port: 3306, Name: "MySQL"}},
	}), "\n")
	for _, s := range []string{"Redis senza password (porta 6379)", "API di Docker aperta senza TLS (porta 2375)", "Gestione remota del server (iLO)"} {
		if !strings.Contains(w, s) {
			t.Errorf("missing %q in:\n%s", s, w)
		}
	}
	if strings.Contains(w, "MySQL") {
		t.Error("MySQL with a password should not warn")
	}
	if p := protocolsOf(Host{Ports: []int{1710, 41794}, Services: []string{"oca"}}); strings.Join(p, ",") != "AES70,Q-SYS,Crestron" {
		t.Errorf("protocols %v", p)
	}
}
