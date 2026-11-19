package main

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPBKDF2(t *testing.T) {
	// RFC 7914 §11 test vector for PBKDF2-HMAC-SHA256.
	got := pbkdf2SHA256([]byte("passwd"), []byte("salt"), 1)
	want := "55ac046e56e3089fec1691c22544b605"
	if h := strings.ToLower(strings.TrimSpace(hexs(got[:16]))); h != want {
		t.Fatalf("pbkdf2 %s, want %s", h, want)
	}
	h, s := hashPassword("palco-sx-2026")
	if !checkPassword("palco-sx-2026", h, s) || checkPassword("palco-sx-2027", h, s) || checkPassword("x", "", "") {
		t.Error("checkPassword")
	}
}

func hexs(b []byte) string {
	const d = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, x := range b {
		out = append(out, d[x>>4], d[x&15])
	}
	return string(out)
}

func TestWebAccessHandler(t *testing.T) {
	wa := &webAccessServer{tokens: map[string]time.Time{}, failures: map[string][]time.Time{}}
	inner := http.NewServeMux()
	inner.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		if !isRemoteWeb(r) {
			t.Error("remote flag missing")
		}
		w.Write([]byte("ok"))
	})
	h, salt := hashPassword("una-password-lunga")
	wa.tokens = map[string]time.Time{}
	// checkPassword reads the settings: test the handler with a stub.
	srv := httptest.NewTLSServer(wa.handlerWith(inner, func(pw string) bool { return checkPassword(pw, h, salt) }))
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	c := srv.Client()
	c.Jar = jar
	c.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	get := func(p string) int {
		r, err := c.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	login := func(pw string) int {
		r, err := c.PostForm(srv.URL+"/login", url.Values{"password": {pw}})
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if get("/api/info") != 401 || get("/") != 303 {
		t.Error("no password: must be refused")
	}
	if login("sbagliata") != 403 {
		t.Error("wrong password accepted")
	}
	if login("una-password-lunga") != 303 {
		t.Fatal("right password refused")
	}
	if get("/api/info") != 200 {
		t.Error("after login: refused")
	}
	if get("/api/quit") != 403 || get("/api/helper") != 403 {
		t.Error("local-only endpoints reachable from the network")
	}
	// Too many wrong passwords: blocked for a minute, even the right one.
	for i := 0; i < 5; i++ {
		login("no")
	}
	if login("una-password-lunga") != 429 {
		t.Error("rate limit missing")
	}
}
