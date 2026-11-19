package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/wasabifilm/wasaping/scanner"
)

// The IP plan: the patch list a crew prepares (name, IP, role, MAC),
// imported from a spreadsheet and compared with what is on the network.

// PlanItem is one planned device.
type PlanItem struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Role string `json:"role,omitempty"`
	MAC  string `json:"mac,omitempty"`
}

var planHeaders = map[string]string{
	"nome": "name", "name": "name", "dispositivo": "name", "device": "name", "apparecchio": "name",
	"ip": "ip", "indirizzo": "ip", "indirizzo ip": "ip", "ip address": "ip", "address": "ip",
	"ruolo": "role", "role": "role", "funzione": "role", "posizione": "role", "note": "role",
	"mac": "mac", "mac address": "mac", "indirizzo mac": "mac",
}

// parsePlan reads a CSV with "," or ";" (Excel in Italian uses ";"). The
// first row may be a header; without one the columns are name, IP, role, MAC.
func parsePlan(data []byte) ([]PlanItem, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // BOM from Excel
	first, _, _ := bytes.Cut(data, []byte("\n"))
	sep := ','
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		sep = ';'
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma, r.FieldsPerRecord, r.TrimLeadingSpace = sep, -1, true
	cols := map[string]int{"name": 0, "ip": 1, "role": 2, "mac": 3}
	var out []PlanItem
	row := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("CSV non leggibile: " + err.Error())
		}
		row++
		if row == 1 {
			found := map[string]int{}
			for i, h := range rec {
				if k, ok := planHeaders[strings.ToLower(strings.TrimSpace(h))]; ok {
					if _, dup := found[k]; !dup {
						found[k] = i
					}
				}
			}
			if _, ok := found["ip"]; ok {
				cols = found
				continue
			}
		}
		get := func(k string) string {
			if i, ok := cols[k]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		ip := net.ParseIP(get("ip")).To4()
		if ip == nil {
			continue // empty or comment rows
		}
		out = append(out, PlanItem{Name: get("name"), IP: ip.String(), Role: get("role"), MAC: scanner.NormalizeMAC(get("mac"))})
		if len(out) > 4096 {
			return nil, errors.New("piano troppo grande (massimo 4096 righe)")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("nessuna riga con un indirizzo IP valido")
	}
	return out, nil
}
