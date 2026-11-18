//go:build darwin

package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Updating wasaping.app from the app: download the release zip, check it
// against SHA256SUMS and its signature, swap the bundle, relaunch. If any
// step fails the installed app is left as it was.

func canSelfUpdate() bool { return appBundlePath() != "" }

func download(url string, limit int64) ([]byte, error) {
	if !strings.HasPrefix(url, "https://github.com/wasabifilm/wasaping/releases/download/") {
		return nil, errors.New("indirizzo di download inatteso")
	}
	c := &http.Client{Timeout: 3 * time.Minute}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("file troppo grande")
	}
	return b, nil
}

func selfUpdate() error {
	s := loadSettings()
	app := appBundlePath()
	if app == "" || s.LatestMacZip == "" || s.LatestSums == "" {
		return errors.New(T("aggiornamento automatico non disponibile: scaricalo dalla pagina", "automatic update not available: download it from the page"))
	}
	sums, err := download(s.LatestSums, 64<<10)
	if err != nil {
		return err
	}
	want := ""
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) == 2 && strings.TrimPrefix(f[1], "*") == "wasaping-macos.zip" {
			want = f[0]
		}
	}
	if want == "" {
		return errors.New("SHA256SUMS senza wasaping-macos.zip")
	}
	zb, err := download(s.LatestMacZip, 200<<20)
	if err != nil {
		return err
	}
	if sum := sha256.Sum256(zb); hex.EncodeToString(sum[:]) != strings.ToLower(want) {
		return errors.New(T("il file scaricato non corrisponde (SHA256)", "the downloaded file does not match (SHA256)"))
	}
	// Extract next to the installed app (same volume: the swap is a rename).
	work, err := os.MkdirTemp(filepath.Dir(app), ".wasaping-update-")
	if err != nil {
		return fmt.Errorf(T("non posso scrivere in %s: %v", "cannot write to %s: %v"), filepath.Dir(app), err)
	}
	defer os.RemoveAll(work)
	zr, err := zip.NewReader(bytes.NewReader(zb), int64(len(zb)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		name := filepath.Clean(f.Name)
		if !strings.HasPrefix(name, "wasaping.app"+string(filepath.Separator)) && name != "wasaping.app" {
			return errors.New("zip inatteso")
		}
		dst := filepath.Join(work, name)
		if !strings.HasPrefix(dst, work+string(filepath.Separator)) {
			return errors.New("zip non sicuro")
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(dst, 0o755)
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return errors.New("zip con collegamenti: non lo apro")
		}
		os.MkdirAll(filepath.Dir(dst), 0o755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode().Perm()|0o600)
		if err == nil {
			_, err = io.Copy(out, io.LimitReader(rc, 200<<20))
			out.Close()
		}
		rc.Close()
		if err != nil {
			return err
		}
	}
	fresh := filepath.Join(work, "wasaping.app")
	plist, _ := os.ReadFile(filepath.Join(fresh, "Contents", "Info.plist"))
	if !bytes.Contains(plist, []byte("eu.wasabi.wasaping")) {
		return errors.New("non è wasaping.app")
	}
	if out, err := exec.Command("codesign", "--verify", "--deep", fresh).CombinedOutput(); err != nil {
		return fmt.Errorf("firma non valida: %s", strings.TrimSpace(string(out)))
	}
	old := app + ".old"
	os.RemoveAll(old)
	if err := os.Rename(app, old); err != nil {
		return err
	}
	if err := os.Rename(fresh, app); err != nil {
		os.Rename(old, app) // put the working app back
		return err
	}
	os.RemoveAll(old)
	// Relaunch the new app once this process has gone.
	exec.Command("/bin/sh", "-c", fmt.Sprintf("sleep 1; /usr/bin/open %q", app)).Start()
	go func() { time.Sleep(500 * time.Millisecond); os.Exit(0) }()
	return nil
}
