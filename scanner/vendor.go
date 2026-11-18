package scanner

import (
	"context"
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

var (
	ouiOnce sync.Once
	ouiDB   map[string]string
)

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
			ouiDB, _ = loadOUI(OUIPath())
		}
	})
	key := strings.ReplaceAll(mac, ":", "")
	if len(key) < 6 {
		return ""
	}
	return ouiDB[key[:6]]
}

// HasOUI reports whether the vendor database has been downloaded.
func HasOUI() bool {
	_, err := os.Stat(OUIPath())
	return err == nil
}

// IsRandomMAC reports a locally administered address (e.g. iPhone/Android private Wi-Fi address).
func IsRandomMAC(mac string) bool {
	if len(mac) < 2 {
		return false
	}
	b, err := strconv.ParseUint(mac[:2], 16, 8)
	return err == nil && b&0x02 != 0
}
