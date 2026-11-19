package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func request(h http.Handler, method, url, host, remote string, hdr map[string]string, body string) int {
	r := httptest.NewRequest(method, url, strings.NewReader(body))
	r.Host = host
	r.RemoteAddr = remote
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestAPIFilter(t *testing.T) {
	allowed := map[string]bool{"127.0.0.1:5000": true}
	local := newHandler(false, allowed, func() {})
	cases := []struct {
		name, method, url, host string
		hdr                     map[string]string
		want                    int
	}{
		{"own page", "GET", "/api/lang", "127.0.0.1:5000", nil, 200},
		{"DNS rebinding", "GET", "/api/lang", "evil.example:5000", nil, 403},
		{"other site, fetch metadata", "GET", "/api/lang", "127.0.0.1:5000", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"other site, origin", "POST", "/api/lang", "127.0.0.1:5000", map[string]string{"Origin": "https://evil.example"}, 403},
		{"sandboxed frame", "GET", "/api/lang", "127.0.0.1:5000", map[string]string{"Origin": "null"}, 403},
		{"same origin", "POST", "/api/lang", "127.0.0.1:5000", map[string]string{"Origin": "http://127.0.0.1:5000", "Sec-Fetch-Site": "same-origin"}, 200},
	}
	for _, c := range cases {
		if got := request(local, c.method, c.url, c.host, "127.0.0.1:40000", c.hdr, `{}`); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}

	public := newHandler(true, allowed, func() {})
	// From another computer: looking is fine, changing this computer is not.
	if got := request(public, "GET", "/api/lang", "10.0.0.5:8080", "10.0.0.9:51000", nil, ""); got != 403 {
		t.Errorf("remote /api/lang: got %d, want 403 (local only)", got)
	}
	for _, p := range []string{"/api/save", "/api/helper", "/api/quit", "/api/device", "/api/plan", "/api/alerts", "/api/update/install", "/api/remote", "/api/rdm/identify", "/api/rdm/set", "/api/forget-old"} {
		if got := request(public, "POST", p, "10.0.0.5:8080", "10.0.0.9:51000", nil, `{}`); got != 403 {
			t.Errorf("remote POST %s: got %d, want 403", p, got)
		}
	}
	if got := request(public, "GET", "/api/last", "10.0.0.5:8080", "10.0.0.9:51000", nil, ""); got != 200 {
		t.Errorf("remote GET /api/last: got %d, want 200", got)
	}
	if got := request(public, "GET", "/api/lang", "10.0.0.5:8080", "127.0.0.1:51000", nil, ""); got != 200 {
		t.Errorf("local GET on public server: got %d, want 200", got)
	}
}

func TestAPIValidation(t *testing.T) {
	h := newHandler(false, map[string]bool{"127.0.0.1:5000": true}, func() {})
	do := func(method, url, body string) int {
		return request(h, method, url, "127.0.0.1:5000", "127.0.0.1:40000", map[string]string{"Content-Type": "application/json"}, body)
	}
	if got := do("POST", "/api/universe", `{"id":"../../etc","critical":true}`); got != 400 {
		t.Errorf("bad universe id: %d", got)
	}
	if got := do("GET", "/api/pingtest?host=-c%20999", ""); got != 400 {
		t.Errorf("option as ping target: %d", got)
	}
	if got := do("POST", "/api/save", `{"name":"../x.sh","data":"x"}`); got != 400 {
		t.Errorf("bad file name: %d", got)
	}
	if got := do("POST", "/api/alerts", `{"webhook":"file:///etc/passwd"}`); got != 400 {
		t.Errorf("bad webhook: %d", got)
	}
	if got := do("POST", "/api/rdm/identify", `{"node":"x","uid":"nope"}`); got != 400 {
		t.Errorf("bad identify: %d", got)
	}
}

// The macOS menu calls the page's tools by id: every id must exist there.
func TestToolMenuIDs(t *testing.T) {
	b, err := os.ReadFile("web/js/tools.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range toolMenuItems {
		if it.id != "-" && !strings.Contains(string(b), `id: "`+it.id+`"`) {
			t.Errorf("menu item %q has no tool in tools.js", it.id)
		}
	}
	if spec := toolMenuSpec(); strings.Count(spec, "\n") != len(toolMenuItems) {
		t.Errorf("spec lines %d", strings.Count(spec, "\n"))
	}
}

func TestPrefsAndSettingsFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	setMu.Lock()
	setCache = &settings{RemotePIN: "424242", WebPasswordHash: "x", Telegram: "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789|-100123"}
	setMu.Unlock()
	h := apiMux(func() {})
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := do("POST", "/api/prefs", `{"default_ports":"22,80","loop_pause":30,"alarm_volume":250,"snmp_community":"show"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	s := loadSettings()
	if s.DefaultPorts != "22,80" || s.LoopPause != 30 || s.AlarmVolume != 100 || s.SNMPCommunity != "show" {
		t.Fatalf("%+v", s)
	}
	if w := do("POST", "/api/prefs", `{"default_ports":"abc"}`); w.Code != 400 {
		t.Fatal("bad ports accepted")
	}
	exp := do("GET", "/api/prefs/export", "").Body.String()
	if strings.Contains(exp, "424242") || strings.Contains(exp, "web_password") || !strings.Contains(exp, "wasaping-settings") {
		t.Fatal("export:", exp)
	}
	setMu.Lock()
	setCache = &settings{RemotePIN: "999999"}
	setMu.Unlock()
	if w := do("POST", "/api/prefs/import", exp); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if s := loadSettings(); s.DefaultPorts != "22,80" || s.RemotePIN != "999999" || s.SNMPCommunity != "show" {
		t.Fatalf("import: %+v", s)
	}
	if w := do("POST", "/api/prefs/import", `{"format":"other"}`); w.Code != 400 {
		t.Fatal("wrong file accepted")
	}
	if w := do("GET", "/api/qr?for=remote", ""); w.Code != 200 || !strings.HasPrefix(w.Body.String(), "<svg") {
		t.Fatal("qr", w.Code)
	}
	for _, p := range []string{"/api/prefs", "/api/prefs/reveal", "/api/prefs/import", "/api/qr"} {
		if !localOnly[p] {
			t.Error("not local only:", p)
		}
	}
	setMu.Lock()
	setCache = nil
	setMu.Unlock()
}
