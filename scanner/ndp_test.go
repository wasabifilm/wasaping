package scanner

import "testing"

func TestParseNeighborsV6(t *testing.T) {
	mac := "98:25:4a:ae:41:91"
	darwin := `Neighbor                                Linklayer Address  Netif Expire    St Flgs Prbs
fe80::1%lo0                             (incomplete)         lo0 permanent R
fe80::9a25:4aff:feae:4191%en0           98:25:4a:ae:41:91    en0 8h14m39s  S      R
2a02:aa1::1                             98:25:4a:ae:41:91    en0 8h        S      R`
	if a := parseNeighborsV6(darwin, "darwin")[mac]; len(a) != 2 || a[0] != "2a02:aa1::1" {
		t.Fatalf("darwin %v", a)
	}
	linux := `fe80::9a25:4aff:feae:4191 dev eth0 lladdr 98:25:4a:ae:41:91 router REACHABLE
fe80::5 dev eth0 FAILED`
	if a := parseNeighborsV6(linux, "linux")[mac]; len(a) != 1 {
		t.Fatalf("linux %v", a)
	}
	win := `Interface 12: Ethernet

Internet Address                              Physical Address   Type
--------------------------------------------  -----------------  -----------
fe80::9a25:4aff:feae:4191                     98-25-4a-ae-41-91  Reachable (Router)
fe80::7                                       00-00-00-00-00-00  Unreachable
ff02::1                                       33-33-00-00-00-01  Permanent`
	r := parseNeighborsV6(win, "windows")
	if a := r[mac]; len(a) != 1 || len(r) != 1 {
		t.Fatalf("windows %v", r)
	}
}
