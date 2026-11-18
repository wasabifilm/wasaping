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
