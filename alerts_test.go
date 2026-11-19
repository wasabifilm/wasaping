package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAlertPresets(t *testing.T) {
	var mu sync.Mutex
	got := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasPrefix(r.URL.Path, "/bot123456:"):
			r.ParseForm()
			got["telegram"] = r.URL.Path + " " + string(b)
		case r.URL.Path == "/wasaping-test":
			got["ntfy"] = r.Header.Get("Title") + "|" + r.Header.Get("Priority") + "|" + string(b)
		case r.URL.Path == "/slack":
			var m map[string]string
			json.Unmarshal(b, &m)
			got["slack"] = m["text"]
		}
	}))
	defer srv.Close()
	old := telegramAPI
	telegramAPI = srv.URL
	defer func() { telegramAPI = old }()

	a := alertCfg{Telegram: "123456:ABCDEFGHIJKLMNOPQRSTUVWXYZ|-1001", Ntfy: srv.URL + "/wasaping-test", Slack: srv.URL + "/slack"}
	res := alertTo(a, MonEvent{At: time.Now(), Name: "grandMA3 FOH", IP: "2.0.0.1"})
	if strings.Join(res, " ") != "Telegram ✓ ntfy ✓ Slack ✓" {
		t.Errorf("results %v", res)
	}
	if !strings.Contains(got["telegram"], "/sendMessage") || !strings.Contains(got["telegram"], "chat_id=-1001") || !strings.Contains(got["telegram"], "grandMA3") {
		t.Errorf("telegram %q", got["telegram"])
	}
	if !strings.Contains(got["ntfy"], "grandMA3 FOH|high|") || !strings.Contains(got["ntfy"], "2.0.0.1") {
		t.Errorf("ntfy %q", got["ntfy"])
	}
	if !strings.Contains(got["slack"], "grandMA3 FOH") {
		t.Errorf("slack %q", got["slack"])
	}
	// A failing Telegram call must not show the token.
	telegramAPI = "http://127.0.0.1:1"
	res = alertTo(alertCfg{Telegram: a.Telegram}, MonEvent{At: time.Now(), Name: "x", Up: true})
	if strings.Contains(strings.Join(res, " "), "ABCDEFGHIJ") {
		t.Errorf("token leaked: %v", res)
	}
}
