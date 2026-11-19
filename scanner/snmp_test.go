package scanner

import (
	"context"
	"net"
	"sort"
	"testing"
)

// fakeSwitch answers SNMP GetRequest/GetBulk from a fixed table.
func fakeSwitch(t *testing.T, table map[string]varbind) (stop func()) {
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Skip(err)
	}
	snmpPort = c.LocalAddr().(*net.UDPAddr).Port
	var keys []oid
	for k := range table {
		keys = append(keys, parseOID(k))
	}
	sort.Slice(keys, func(i, j int) bool { return oidLess(keys[i], keys[j]) })
	go func() {
		buf := make([]byte, 4096)
		for {
			n, from, err := c.ReadFromUDP(buf)
			if err != nil {
				return
			}
			r := &berReader{b: buf[:n]}
			_, msg, _ := r.next()
			m := &berReader{b: msg}
			m.next()
			_, comm, _ := m.next()
			pduTag, pdu, _ := m.next()
			p := &berReader{b: pdu}
			_, id, _ := p.next()
			_, _, _ = p.next()
			_, rep, _ := p.next()
			_, list, _ := p.next()
			l := &berReader{b: list}
			_, vb, _ := l.next()
			v := &berReader{b: vb}
			_, o, _ := v.next()
			req := decodeOID(o)
			var vbs []byte
			add := func(k oid, val varbind) {
				vbs = append(vbs, tlv(0x30, append(berOID(k), tlv(val.tag, val.value)...))...)
			}
			if string(comm) != "public" {
				continue
			}
			if pduTag == 0xA0 { // get: sysName, sysDescr
				for l2 := (&berReader{b: list}); l2.off < len(l2.b); {
					_, vb, _ := l2.next()
					v := &berReader{b: vb}
					_, o, _ := v.next()
					k := decodeOID(o)
					add(k, table[k.String()])
				}
			} else {
				count := 0
				for _, k := range keys {
					if oidLess(req, k) && count < int(decodeInt(rep)) {
						add(k, table[k.String()])
						count++
					}
				}
				if count == 0 {
					add(req, varbind{tag: 0x82})
				}
			}
			body := append(append(append(tlv(0x02, id), berInt(0)...), berInt(0)...), tlv(0x30, vbs)...)
			resp := tlv(0x30, append(append(berInt(1), tlv(0x04, comm)...), tlv(0xA2, body)...))
			c.WriteToUDP(resp, from)
		}
	}()
	return func() { c.Close(); snmpPort = 161 }
}

func oidLess(a, b oid) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func TestSwitchPorts(t *testing.T) {
	str := func(s string) varbind { return varbind{tag: 0x04, value: []byte(s)} }
	num := func(n int64) varbind { b := berInt(n); return varbind{tag: 0x02, value: b[2:]} }
	table := map[string]varbind{
		"1.3.6.1.2.1.1.5.0": str("GigaCore26i"),
		"1.3.6.1.2.1.1.1.0": str("Luminex GigaCore"),
		// BRIDGE-MIB fdb: mac -> bridge port. Port 26 is the uplink (2 MACs).
		"1.3.6.1.2.1.17.4.3.1.2.0.12.41.74.16.1":  num(14), // 00:0c:29:4a:10:01 on port 14
		"1.3.6.1.2.1.17.4.3.1.2.0.29.193.48.48.1": num(26),
		"1.3.6.1.2.1.17.4.3.1.2.0.29.193.48.49.1": num(26),
		"1.3.6.1.2.1.17.1.4.1.2.14":               num(1014),
		"1.3.6.1.2.1.17.1.4.1.2.26":               num(1026),
		"1.3.6.1.2.1.31.1.1.1.1.1014":             str("Gi1/14"),
		"1.3.6.1.2.1.31.1.1.1.1.1026":             str("Gi1/26"),
		"1.3.6.1.2.1.31.1.1.1.18.1014":            str("grandMA3 FOH"),
		"1.3.6.1.2.1.31.1.1.1.15.1014":            {tag: 0x42, value: []byte{0x64}}, // 100 Mbit/s
		// LLDP: on local port 26 there is another switch, port 1/1
		"1.0.8802.1.1.2.1.3.7.1.4.26":     str("Gi1/26"),
		"1.0.8802.1.1.2.1.4.1.1.9.0.26.1": str("GigaCore Palco"),
		"1.0.8802.1.1.2.1.4.1.1.8.0.26.1": str("Gi1/1"),
	}
	defer fakeSwitch(t, table)()
	sw, ports := SwitchPorts(context.Background(), []string{"127.0.0.1"}, "public")
	if len(sw) != 1 || sw[0].Name != "GigaCore26i" || sw[0].MACs != 3 || sw[0].Ports["Gi1/26"] != 2 {
		t.Fatalf("switches %+v", sw)
	}
	if n := sw[0].Neighbors; len(n) != 1 || n[0].LocalPort != "Gi1/26" || n[0].Name != "GigaCore Palco" || n[0].RemotePort != "Gi1/1" {
		t.Errorf("lldp %+v", n)
	}
	p := ports["00:0c:29:4a:10:01"]
	if p.Port != "Gi1/14" || p.Alias != "grandMA3 FOH" || p.Shared != 1 || p.Mbps != 100 {
		t.Errorf("port %+v", p)
	}
	if q := ports["00:1d:c1:30:30:01"]; q.Port != "Gi1/26" || q.Shared != 2 {
		t.Errorf("uplink %+v", q)
	}
	if sw, _ := SwitchPorts(context.Background(), []string{"127.0.0.1"}, "wrong"); len(sw) != 0 {
		t.Error("wrong community should not answer")
	}
}
