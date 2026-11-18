package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Once a day, ask GitHub for the latest release and tell the user when a
// newer version exists. The request carries no data about the user or the
// network; it can be turned off.

const releasesAPI = "https://api.github.com/repos/wasabifilm/wasaping/releases/latest"

// newerVersion reports whether tag (v1.2.3) is newer than cur.
func newerVersion(tag, cur string) bool {
	parse := func(v string) []int {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		v, _, _ = strings.Cut(v, "-")
		var out []int
		for _, p := range strings.Split(v, ".") {
			n, err := strconv.Atoi(p)
			if err != nil {
				return nil
			}
			out = append(out, n)
		}
		return out
	}
	a, b := parse(tag), parse(cur)
	if a == nil || b == nil {
		return false
	}
	for i := 0; i < max(len(a), len(b)); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func checkUpdates(ctx context.Context) {
	s := loadSettings()
	if s.NoUpdateCheck || !strings.HasPrefix(version, "v") || time.Since(s.UpdateChecked) < 20*time.Hour {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	req.Header.Set("User-Agent", "wasaping/"+version)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var rel struct {
		Tag    string `json:"tag_name"`
		URL    string `json:"html_url"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&rel) != nil {
		return
	}
	s = loadSettings()
	s.UpdateChecked, s.LatestVersion, s.LatestURL = time.Now(), rel.Tag, rel.URL
	s.LatestMacZip, s.LatestSums = "", ""
	for _, a := range rel.Assets {
		switch a.Name {
		case "wasaping-macos.zip":
			s.LatestMacZip = a.URL
		case "SHA256SUMS":
			s.LatestSums = a.URL
		}
	}
	saveSettings(s)
}

func updateLoop(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(15 * time.Second): // not while the app is starting
	}
	for {
		checkUpdates(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(6 * time.Hour):
		}
	}
}

// handleUpdate: GET says whether a newer version exists; POST {off: true}
// turns the check off (and {off: false} back on).
func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Off bool `json:"off"`
		}
		json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
		s := loadSettings()
		s.NoUpdateCheck = req.Off
		saveSettings(s)
	}
	s := loadSettings()
	writeJSON(w, map[string]any{
		"current": version, "latest": s.LatestVersion, "url": s.LatestURL, "off": s.NoUpdateCheck,
		"newer":       !s.NoUpdateCheck && newerVersion(s.LatestVersion, version),
		"can_install": canSelfUpdate() && s.LatestMacZip != "" && s.LatestSums != "",
	})
}

// handleUpdateInstall updates wasaping.app in place (macOS) and relaunches it.
func handleUpdateInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	if err := selfUpdate(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"message": T("Aggiornato: wasaping si riavvia.", "Updated: wasaping restarts.")})
}
