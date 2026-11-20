package scanner

import (
	"net"
	"testing"
)

func TestRoutedNetworksParsing(t *testing.T) {
	mac := `Routing tables

Internet:
Destination        Gateway            Flags               Netif Expire
default            link#27            UCSIg               utun6
10.10.10/24        link#27            UCS                 utun6
10.10.10.2         10.10.10.2         UH                  utun6
192.168.1          link#15            UCS                 en0
192.168.3          link#27            UCS                 utun6
192.168.3.1        link#27            UHWIi               utun6
172.20             192.168.1.254      UGSc                en0
224.0.0/4          link#27            UmCSI               utun6
255.255.255.255/32 link#27            UCSI                utun6
`
	_, en0, _ := net.ParseCIDR("192.168.1.0/24")
	got := keepRouted(parseNetstatRoutes(mac), []*net.IPNet{en0})
	want := []string{"10.10.10.0/24 VPN (utun6)", "172.20.0.0/16 router 192.168.1.254", "192.168.3.0/24 VPN (utun6)"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, r := range got {
		if r.Network+" "+r.Label != want[i] {
			t.Errorf("%d: got %q, want %q", i, r.Network+" "+r.Label, want[i])
		}
	}
	if InRouted("192.168.3.40", got) != "VPN (utun6)" || InRouted("192.168.1.40", got) != "" {
		t.Error("InRouted")
	}

	linux := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0101A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"eth0\t0001A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n" +
		"wg0\t0003A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0\n"
	got = keepRouted(parseProcRoute(linux), []*net.IPNet{en0})
	if len(got) != 1 || got[0].Network != "192.168.3.0/24" || got[0].Label != "VPN (wg0)" {
		t.Fatalf("linux: %+v", got)
	}

	win := `IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0      192.168.1.1     192.168.1.13     25
      192.168.1.0    255.255.255.0         On-link      192.168.1.13    281
      192.168.3.0    255.255.255.0         10.8.0.1         10.8.0.6     35
        224.0.0.0        240.0.0.0         On-link         127.0.0.1    331
`
	got = keepRouted(parseRoutePrint(win), []*net.IPNet{en0})
	if len(got) != 1 || got[0].Network != "192.168.3.0/24" || got[0].Gateway != "10.8.0.1" {
		t.Fatalf("windows: %+v", got)
	}
}
