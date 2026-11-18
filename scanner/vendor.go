package scanner

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OUIURL is the official IEEE registry of MAC address prefixes.
const OUIURL = "https://standards-oui.ieee.org/oui/oui.csv"

// embeddedOUI is a compact copy of the IEEE registry built into wasaping
// (regenerate with `make oui`), so vendors work offline from the first scan.
//
//go:embed oui.txt.gz
var embeddedOUI []byte

var (
	ouiOnce sync.Once
	ouiDB   map[string]string
)

// loadEmbeddedOUI decodes the built-in "aabbcc<TAB>Vendor" list.
func loadEmbeddedOUI() map[string]string {
	db := make(map[string]string, 41000)
	zr, err := gzip.NewReader(bytes.NewReader(embeddedOUI))
	if err != nil {
		return db
	}
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), "\t"); ok && len(k) == 6 {
			db[k] = v
		}
	}
	return db
}

// OUIPath is where the vendor database is cached.
func OUIPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "wasaping", "oui.csv")
}

// UpdateOUI downloads the IEEE OUI database into the cache.
func UpdateOUI(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, OUIURL, nil)
	req.Header.Set("User-Agent", "wasaping")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download fallito: %s", resp.Status)
	}
	path := OUIPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return 0, err
	}
	f.Close()
	db, err := loadOUI(tmp)
	if err != nil || len(db) == 0 {
		os.Remove(tmp)
		return 0, fmt.Errorf("file OUI non valido: %v", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return 0, err
	}
	ouiDB = db
	return len(db), nil
}

func loadOUI(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	db := map[string]string{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(rec) >= 3 && len(rec[1]) == 6 {
			db[strings.ToLower(rec[1])] = strings.TrimSpace(rec[2])
		}
	}
	return db, nil
}

// Vendor returns the manufacturer for a MAC, if the OUI database is present.
func Vendor(mac string) string {
	if mac == "" {
		return ""
	}
	if IsRandomMAC(mac) {
		return "(MAC privato/casuale)"
	}
	ouiOnce.Do(func() {
		if ouiDB == nil {
			// A copy downloaded with update-oui is newer than the built-in one.
			if db, err := loadOUI(OUIPath()); err == nil && len(db) > 0 {
				ouiDB = db
			} else {
				ouiDB = loadEmbeddedOUI()
			}
		}
	})
	key := strings.ReplaceAll(mac, ":", "")
	if len(key) < 6 {
		return ""
	}
	v := ouiDB[key[:6]]
	if v == "IEEE Registration Authority" {
		// Block split into smaller ones (MA-M/MA-S): the real maker is unknown here.
		return ""
	}
	return v
}

// HasOUI reports whether a vendor database is available (always, since
// one is built in; kept for the interface).
func HasOUI() bool { return len(embeddedOUI) > 0 }

// IsRandomMAC reports a locally administered address (e.g. iPhone/Android private Wi-Fi address).
func IsRandomMAC(mac string) bool {
	if len(mac) < 2 {
		return false
	}
	b, err := strconv.ParseUint(mac[:2], 16, 8)
	return err == nil && b&0x02 != 0
}
