// Command ouigen downloads the IEEE OUI registry and writes the compact,
// gzipped copy embedded in wasaping (scanner/oui.txt.gz): one
// "aabbcc<TAB>Vendor" line per prefix, sorted. Run: make oui
package main

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

const url = "https://standards-oui.ieee.org/oui/oui.csv"

func main() {
	out := "scanner/oui.txt.gz"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := run(out); err != nil {
		fmt.Fprintln(os.Stderr, "ouigen:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "wasaping-ouigen")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}
	r := csv.NewReader(resp.Body)
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
			name := strings.Join(strings.Fields(rec[2]), " ")
			if name != "" {
				db[strings.ToLower(rec[1])] = name
			}
		}
	}
	if len(db) < 10000 {
		return fmt.Errorf("solo %d produttori: file IEEE inatteso", len(db))
	}
	keys := make([]string, 0, len(db))
	for k := range db {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	f, err := os.Create(out + ".tmp")
	if err != nil {
		return err
	}
	zw, _ := gzip.NewWriterLevel(f, gzip.BestCompression)
	for _, k := range keys {
		fmt.Fprintf(zw, "%s\t%s\n", k, db[k])
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Printf("%d produttori → %s\n", len(db), out)
	return os.Rename(out+".tmp", out)
}
