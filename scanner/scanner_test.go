package scanner

import "testing"

func TestParseTargets(t *testing.T) {
	cases := map[string]int{
		"192.168.1.0/24":            254,
		"10.0.0.5":                  1,
		"192.168.1.10-20":           11,
		"192.168.1.250-192.168.2.5": 12,
		"10.0.0.1, 10.0.0.2":        2,
		"10.0.0.0/30":               2,
	}
	for spec, want := range cases {
		ips, err := ParseTargets(spec)
		if err != nil {
			t.Fatalf("%s: %v", spec, err)
		}
		if len(ips) != want {
			t.Errorf("%s: got %d, want %d", spec, len(ips), want)
		}
	}
	for _, bad := range []string{"10.0.0.0/8", "abc", "10.0.0.20-10", ""} {
		if _, err := ParseTargets(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestParseARPOutput(t *testing.T) {
	mac := `? (192.168.1.1) at a4:2b:8:1:c:fe on en0 ifscope [ethernet]
? (192.168.1.255) at ff:ff:ff:ff:ff:ff on en0 ifscope [ethernet]
? (192.168.1.40) at (incomplete) on en0 ifscope [ethernet]`
	win := `Interface: 192.168.1.20 --- 0x7
  Internet Address      Physical Address      Type
  192.168.1.1           a4-2b-08-01-0c-fe     dynamic
  224.0.0.22            01-00-5e-00-00-16     static`
	for name, out := range map[string]string{"mac": mac, "windows": win} {
		tbl := map[string]string{}
		parseARPOutput(out, tbl)
		if len(tbl) != 1 || tbl["192.168.1.1"] != "a4:2b:08:01:0c:fe" {
			t.Errorf("%s: unexpected table %v", name, tbl)
		}
	}
}

func TestRandomMAC(t *testing.T) {
	if !IsRandomMAC("da:a1:19:00:00:01") || IsRandomMAC("a4:2b:08:01:0c:fe") {
		t.Error("locally administered bit detection is wrong")
	}
}

func TestPingRTT(t *testing.T) {
	cases := map[string]float64{
		"64 bytes from 192.168.1.1: icmp_seq=0 ttl=64 time=3.412 ms": 3.412,
		"Reply from 192.168.1.1: bytes=32 time<1ms TTL=64":           1,
		"Risposta da 192.168.1.1: byte=32 durata=12ms TTL=64":        12,
		"Antwort von 192.168.1.1: Bytes=32 Zeit=4ms TTL=64":          4,
		"Request timeout for icmp_seq 0":                             0,
	}
	for out, want := range cases {
		if got := pingRTT(out).Seconds() * 1000; got != want {
			t.Errorf("%q: got %v ms, want %v", out, got, want)
		}
	}
}

func TestHiddenMACPlaceholder(t *testing.T) {
	if normalizeMAC("2:0:0:0:0:0") != "" {
		t.Error("macOS placeholder MAC should be treated as unknown")
	}
}

func TestMDNSRoundTrip(t *testing.T) {
	q := dnsQuery([]string{reverseName("192.168.1.20")}, 12)
	// Build a response: same question, then one PTR answer pointing back to it.
	resp := append([]byte{}, q...)
	resp[2] = 0x84
	resp[7] = 1 // ANCOUNT
	resp = append(resp, 0xC0, 12, 0, 12, 0, 1, 0, 0, 0, 120)
	target := []byte{9, 'm', 'a', 'c', 'b', 'o', 'o', 'k', '-', '1', 5, 'l', 'o', 'c', 'a', 'l', 0}
	resp = append(resp, 0, byte(len(target)))
	resp = append(resp, target...)
	got := parseMDNSPTR(resp)
	if got["192.168.1.20"] != "macbook-1.local" {
		t.Errorf("unexpected %v", got)
	}
}

func TestNetbiosStatus(t *testing.T) {
	q := netbiosStatusQuery()
	resp := append([]byte{}, q[:12]...)
	resp[2], resp[5], resp[7] = 0x84, 0, 1
	resp = append(resp, q[12:12+34]...)
	resp = append(resp, 0, 0x21, 0, 1, 0, 0, 0, 0, 0, 0, 2)
	add := func(n string, suffix byte, flags uint16) {
		b := make([]byte, 18)
		copy(b, n+"               ")
		b[15], b[16], b[17] = suffix, byte(flags>>8), byte(flags)
		resp = append(resp, b...)
	}
	add("WORKGROUP", 0, 0x8400)
	add("UFFICIO-PC", 0, 0x0400)
	if got, group := parseNetbiosStatus(resp); got != "UFFICIO-PC" || group != "WORKGROUP" {
		t.Errorf("got %q in %q", got, group)
	}
}

func TestWindowsRoute(t *testing.T) {
	out := `===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0      192.168.1.1     192.168.1.20     25
        127.0.0.0        255.0.0.0         On-link         127.0.0.1    331`
	if got := parseWindowsRoute(out); got != "192.168.1.1" {
		t.Errorf("got %q", got)
	}
}

func TestGuessKind(t *testing.T) {
	cases := []struct {
		h    Host
		want string
	}{
		{Host{Gateway: true}, KindRouter},
		{Host{Hostname: "Android_46GKIU2H.local"}, KindPhone},
		{Host{Ports: []int{62078}}, KindPhone},
		{Host{Hostname: "MacBook-Pro.local"}, KindComputer},
		{Host{Services: []string{"ipp"}}, KindPrinter},
		{Host{Ports: []int{554}}, KindCamera},
		{Host{}, ""},
	}
	for _, c := range cases {
		if got := GuessKind(c.h); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.h, got, c.want)
		}
	}
}

func TestEmbeddedOUI(t *testing.T) {
	db := loadEmbeddedOUI()
	if len(db) < 10000 {
		t.Fatalf("embedded OUI has only %d vendors", len(db))
	}
	if v := db["44a8fc"]; v != "Apple, Inc." {
		t.Errorf("44a8fc = %q", v)
	}
}
