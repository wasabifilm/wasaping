package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Full interface from another computer on the network: HTTPS with a
// self-signed certificate (made here, kept in the config folder) and a
// password. A third listener, off unless switched on in Settings; what only
// makes sense on this computer (quitting, the macOS password dialog, the
// update, this very setting) stays local.

const defaultWebPort = 47443

type remoteWebKey struct{}

func isRemoteWeb(r *http.Request) bool { return r.Context().Value(remoteWebKey{}) != nil }

// Never from another computer, even with the password.
var remoteWebBlocked = map[string]bool{
	"/api/quit": true, "/api/helper": true, "/api/update/install": true, "/api/webaccess": true,
	"/api/save": true, "/api/diag": true, "/api/ping": true, "/api/login": true,
}

type webAccessServer struct {
	mu     sync.Mutex
	srv    *http.Server
	port   int
	tokens map[string]time.Time
	limit  loginLimit
}

var webAcc = &webAccessServer{tokens: map[string]time.Time{}}

// ---- Password: PBKDF2-HMAC-SHA256 (written out: the standard library has
// it only from Go 1.24) ----

const pbkdf2Iter = 210_000

func pbkdf2SHA256(password, salt []byte, iter int) []byte {
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1})
	u := mac.Sum(nil)
	out := append([]byte(nil), u...)
	for i := 1; i < iter; i++ {
		mac.Reset()
		mac.Write(u)
		u = mac.Sum(u[:0])
		for k := range out {
			out[k] ^= u[k]
		}
	}
	return out
}

func hashPassword(pw string) (hash, salt string) {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(pbkdf2SHA256([]byte(pw), b, pbkdf2Iter)), hex.EncodeToString(b)
}

func checkPassword(pw, hash, salt string) bool {
	s, err1 := hex.DecodeString(salt)
	h, err2 := hex.DecodeString(hash)
	if err1 != nil || err2 != nil || len(h) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(pbkdf2SHA256([]byte(pw), s, pbkdf2Iter), h) == 1
}

// ---- Certificate ----

func certPaths() (string, string) {
	dir := filepath.Dir(monitorLogPath())
	return filepath.Join(dir, "tls-cert.pem"), filepath.Join(dir, "tls-key.pem")
}

// localCert loads the certificate, or makes a new one when missing, expired
// or not valid for the current address of this computer.
func localCert() (tls.Certificate, error) {
	cp, kp := certPaths()
	ip := ""
	if i, err := scanner.DefaultInterface(); err == nil {
		ip = i.IP
	}
	if c, err := tls.LoadX509KeyPair(cp, kp); err == nil {
		if x, err := x509.ParseCertificate(c.Certificate[0]); err == nil && time.Now().Before(x.NotAfter.Add(-24*time.Hour)) &&
			(ip == "" || x.VerifyHostname(ip) == nil) {
			return c, nil
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	host, _ := os.Hostname()
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "wasaping " + host, Organization: []string{"wasaping"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(3, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	if host != "" {
		tpl.DNSNames = append(tpl.DNSNames, host, strings.TrimSuffix(host, ".local")+".local")
	}
	ifs, _ := scanner.LocalInterfaces()
	for _, i := range ifs {
		if p := net.ParseIP(i.IP); p != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, p)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	kb, _ := x509.MarshalECPrivateKey(key)
	os.MkdirAll(filepath.Dir(cp), 0o700)
	os.WriteFile(kp, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600)
	os.WriteFile(cp, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	return tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}))
}

// certFingerprint is what to compare with the browser's certificate details.
func certFingerprint() string {
	cp, _ := certPaths()
	b, err := os.ReadFile(cp)
	if err != nil {
		return ""
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return ""
	}
	sum := sha256.Sum256(blk.Bytes)
	h := strings.ToUpper(hex.EncodeToString(sum[:]))
	var parts []string
	for i := 0; i < len(h); i += 2 {
		parts = append(parts, h[i:i+2])
	}
	return strings.Join(parts, ":")
}

// ---- Server ----

func webAccessURL(port int) string {
	ip := "IP-di-questo-computer"
	if i, err := scanner.DefaultInterface(); err == nil {
		ip = i.IP
	}
	return fmt.Sprintf("https://%s:%d/", ip, port)
}

func (wa *webAccessServer) apply() error {
	s := loadSettings()
	on := s.WebAccess && s.WebPasswordHash != ""
	port := s.WebPort
	if port == 0 {
		port = defaultWebPort
	}
	wa.mu.Lock()
	defer wa.mu.Unlock()
	if wa.srv != nil && (!on || wa.port != port) {
		wa.srv.Close()
		wa.srv = nil
		wa.tokens = map[string]time.Time{}
	}
	if !on || wa.srv != nil {
		return nil
	}
	cert, err := localCert()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("porta %d occupata: %v", port, err)
	}
	wa.srv = &http.Server{
		Handler:           wa.handler(apiMux(func() {})),
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ErrorLog:          log.New(io.Discard, "", 0), // browsers refusing the self-signed certificate are not news
	}
	wa.port = port
	go wa.srv.Serve(tls.NewListener(ln, wa.srv.TLSConfig))
	return nil
}

func (wa *webAccessServer) authorized(r *http.Request) bool {
	c, err := r.Cookie("wsa")
	if err != nil {
		return false
	}
	wa.mu.Lock()
	defer wa.mu.Unlock()
	exp, ok := wa.tokens[c.Value]
	return ok && time.Now().Before(exp)
}

func (wa *webAccessServer) handler(mux http.Handler) http.Handler {
	return wa.handlerWith(mux, func(pw string) bool {
		s := loadSettings()
		return checkPassword(pw, s.WebPasswordHash, s.WebPasswordSalt)
	})
}

// handlerWith takes the password check as a function, for the tests.
func (wa *webAccessServer) handlerWith(mux http.Handler, passwordOK func(string) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=86400")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if crossSite(r) {
			http.Error(w, "richiesta da un altro sito non consentita", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/login":
			wa.handleLogin(w, r, passwordOK)
			return
		case "/logout":
			if c, err := r.Cookie("wsa"); err == nil && r.Method == http.MethodPost {
				wa.mu.Lock()
				delete(wa.tokens, c.Value)
				wa.mu.Unlock()
			}
			http.SetCookie(w, &http.Cookie{Name: "wsa", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !wa.authorized(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "serve la password", http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}
		if remoteWebBlocked[r.URL.Path] {
			http.Error(w, T("consentito solo dal computer dove gira wasaping", "only allowed on the computer running wasaping"), http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), remoteWebKey{}, true)))
	})
}

const loginPage = `<!doctype html><html lang="it"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>wasaping</title><style>
body{margin:0;min-height:100vh;display:grid;place-items:center;background:#171717;color:#eee;font:16px "Open Sans",system-ui,sans-serif}
form{background:#222;padding:28px;border-radius:14px;width:min(320px,calc(100vw - 32px));box-sizing:border-box}
h1{margin:0 0 4px;font-size:24px}h1 b{color:#81d742}p{color:#aaa;font-size:14px;margin:0 0 18px}
input{width:100%%;box-sizing:border-box;padding:10px;border-radius:8px;border:1px solid #444;background:#111;color:#eee;font-size:16px}
button{margin-top:12px;width:100%%;padding:10px;border:0;border-radius:8px;background:#81d742;color:#171717;font-weight:700;font-size:16px}
.err{color:#ff7b72;margin-top:10px;font-size:14px}</style>
<form method="post" action="/login"><h1><b>●</b> wasaping</h1><p>%s</p>
<input type="password" name="password" autocomplete="current-password" autofocus aria-label="password">
<button>%s</button>%s</form></html>`

func (wa *webAccessServer) handleLogin(w http.ResponseWriter, r *http.Request, passwordOK func(string) bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	page := func(msg string, code int) {
		w.WriteHeader(code)
		e := ""
		if msg != "" {
			e = `<div class="err">` + html.EscapeString(msg) + `</div>`
		}
		fmt.Fprintf(w, loginPage, "Accesso completo da un altro computer · Full access from another computer", "Entra · Sign in", e)
	}
	if r.Method != http.MethodPost {
		page("", http.StatusOK)
		return
	}
	ip := clientIP(r)
	if wa.limit.blocked(ip) {
		page("Troppi tentativi: riprova tra un minuto · Too many attempts: try again in a minute", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if !passwordOK(r.PostFormValue("password")) {
		wa.limit.fail(ip)
		page("Password sbagliata · Wrong password", http.StatusForbidden)
		return
	}
	b := make([]byte, 32)
	rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)
	wa.mu.Lock()
	for k, exp := range wa.tokens {
		if time.Now().After(exp) {
			delete(wa.tokens, k)
		}
	}
	wa.tokens[tok] = time.Now().Add(12 * time.Hour)
	wa.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "wsa", Value: tok, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 3600})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleWebAccess (this computer only): GET the state; POST {on, password, port}.
func handleWebAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			On       bool   `json:"on"`
			Password string `json:"password"`
			Port     int    `json:"port"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil || req.Port < 0 || req.Port > 65535 {
			http.Error(w, "richiesta non valida", http.StatusBadRequest)
			return
		}
		s := loadSettings()
		if req.Password != "" {
			if len([]rune(req.Password)) < 8 {
				http.Error(w, T("la password deve avere almeno 8 caratteri", "the password needs at least 8 characters"), http.StatusBadRequest)
				return
			}
			s.WebPasswordHash, s.WebPasswordSalt = hashPassword(req.Password)
			webAcc.mu.Lock()
			webAcc.tokens = map[string]time.Time{} // a new password logs everybody out
			webAcc.mu.Unlock()
		}
		if req.On && s.WebPasswordHash == "" {
			http.Error(w, T("scegli prima una password", "choose a password first"), http.StatusBadRequest)
			return
		}
		s.WebAccess = req.On
		if req.Port != 0 {
			s.WebPort = req.Port
		}
		saveSettings(s)
		if err := webAcc.apply(); err != nil {
			s.WebAccess = false
			saveSettings(s)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	s := loadSettings()
	port := s.WebPort
	if port == 0 {
		port = defaultWebPort
	}
	out := map[string]any{"on": s.WebAccess && s.WebPasswordHash != "", "has_password": s.WebPasswordHash != "", "port": port, "url": webAccessURL(port)}
	if s.WebAccess {
		out["fingerprint"] = certFingerprint()
	}
	writeJSON(w, out)
}

// readSecret reads a line without echo where `stty` exists (macOS, Linux).
func readSecret(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	if runtime.GOOS != "windows" {
		if c := exec.Command("stty", "-echo"); true {
			c.Stdin = os.Stdin
			if c.Run() == nil {
				defer func() {
					r := exec.Command("stty", "echo")
					r.Stdin = os.Stdin
					r.Run()
					fmt.Fprintln(os.Stderr)
				}()
			}
		}
	}
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

// runWebAccessCLI: wasaping web-access on|off|password|status. The setting
// is read by the running wasaping when it starts (restart the service).
func runWebAccessCLI(args []string) error {
	s := loadSettings()
	cmd := "status"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "password":
		pw := readSecret(TC("Nuova password (almeno 8 caratteri): ", "New password (at least 8 characters): "))
		if len([]rune(pw)) < 8 {
			return fmt.Errorf(TC("la password deve avere almeno 8 caratteri", "the password needs at least 8 characters"))
		}
		if readSecret(TC("Ripeti la password: ", "Repeat the password: ")) != pw {
			return fmt.Errorf(TC("le due password non coincidono", "the two passwords differ"))
		}
		s.WebPasswordHash, s.WebPasswordSalt = hashPassword(pw)
		s.WebAccess = true
	case "on":
		if s.WebPasswordHash == "" {
			return fmt.Errorf(TC("scegli prima una password: wasaping web-access password", "choose a password first: wasaping web-access password"))
		}
		s.WebAccess = true
	case "off":
		s.WebAccess = false
	case "status":
	default:
		return fmt.Errorf(TC("uso: wasaping web-access on|off|password|status", "usage: wasaping web-access on|off|password|status"))
	}
	if cmd != "status" {
		saveSettings(s)
	}
	port := s.WebPort
	if port == 0 {
		port = defaultWebPort
	}
	if s.WebAccess && s.WebPasswordHash != "" {
		localCert() // ready, so that the fingerprint can be shown now
		fmt.Printf(TC("Accesso completo dalla rete: attivo su %s\n", "Full access from the network: on at %s\n"), webAccessURL(port))
		fmt.Printf(TC("Impronta del certificato (SHA-256): %s\n", "Certificate fingerprint (SHA-256): %s\n"), certFingerprint())
		if cmd != "status" {
			fmt.Println(TC("Riavvia wasaping (o il servizio) perché la modifica abbia effetto.", "Restart wasaping (or the service) for the change to take effect."))
		}
	} else {
		fmt.Println(TC("Accesso completo dalla rete: spento", "Full access from the network: off"))
	}
	return nil
}
