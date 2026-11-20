package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

// The public address of this network, its provider and country, as LanScan
// shows. It needs a request to an internet service (ipinfo.io), so it is
// off until turned on in Settings; the answer is kept for 10 minutes.

// PublicInfo is what the service says about our public address.
type PublicInfo struct {
	On      bool   `json:"on"`
	IP      string `json:"ip,omitempty"`
	Org     string `json:"org,omitempty"` // provider, e.g. "AS1267 Wind Tre S.p.A."
	Country string `json:"country,omitempty"`
	City    string `json:"city,omitempty"`
	Error   string `json:"error,omitempty"`
}

// publicIPURL is a variable for the tests.
var publicIPURL = "https://ipinfo.io/json"

var pubCache struct {
	sync.Mutex
	at   time.Time
	info PublicInfo
}

func publicIPReset() {
	pubCache.Lock()
	pubCache.at = time.Time{}
	pubCache.Unlock()
}

func publicIP(ctx context.Context) PublicInfo {
	if !loadSettings().PublicIP {
		return PublicInfo{}
	}
	pubCache.Lock()
	defer pubCache.Unlock()
	if time.Since(pubCache.at) < 10*time.Minute {
		return pubCache.info
	}
	info := PublicInfo{On: true}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, publicIPURL, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "wasaping")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		var d struct{ IP, Org, Country, City string }
		if resp.StatusCode != http.StatusOK {
			info.Error = resp.Status
		} else if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&d); err != nil {
			info.Error = T("risposta non valida", "invalid answer")
		} else {
			info.IP, info.Org, info.Country, info.City = d.IP, d.Org, d.Country, d.City
		}
	} else {
		info.Error = T("nessuna risposta (niente internet?)", "no answer (no internet?)")
	}
	pubCache.info = info
	pubCache.at = time.Now()
	if info.Error != "" {
		pubCache.at = time.Now().Add(-9 * time.Minute) // try again in a minute
	}
	return info
}

// handlePublicIP: GET the public address (off: {on: false}).
func handlePublicIP(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, publicIP(r.Context()))
}
