package showsim

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// Video gear: a Videohub (TCP), a HyperDeck (TCP), an ATEM (UDP) and vMix
// (HTTP), answering like the real ones to what wasaping asks.

// VideoPorts are where the simulated video devices listen.
type VideoPorts struct{ Videohub, HyperDeck, ATEM, VMix int }

// DefaultVideoPorts are the real ones.
var DefaultVideoPorts = VideoPorts{9990, 9993, 9910, 8088}

const videohubPrelude = "PROTOCOL PREAMBLE:\nVersion: 2.8\n\nVIDEOHUB DEVICE:\nDevice present: true\nModel name: Blackmagic Smart Videohub 12x12 (simulatore)\nFriendly name: Videohub Regia\nVideo inputs: 4\nVideo outputs: 4\n\n" +
	"INPUT LABELS:\n0 Camera 1\n1 Camera 2\n2 Grafica\n3 Playback\n\nOUTPUT LABELS:\n0 LED Wall\n1 Proiettore SX\n2 Proiettore DX\n3 Monitor regia\n\n" +
	"VIDEO OUTPUT LOCKS:\n0 L\n1 U\n2 U\n3 U\n\nVIDEO OUTPUT ROUTING:\n0 2\n1 0\n2 0\n3 1\n\nEND PRELUDE:\n\n"

func serveTCP(ctx context.Context, addr string, handle func(net.Conn)) error {
	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); handle(c) }()
		}
	}()
	return nil
}

func hyperdeck(c net.Conn) {
	fmt.Fprint(c, "500 connection info:\r\nprotocol version: 1.11\r\nmodel: HyperDeck Studio HD Plus (simulatore)\r\n\r\n")
	sc := bufio.NewScanner(c)
	for sc.Scan() {
		cmd := strings.TrimSpace(sc.Text())
		switch {
		case cmd == "device info":
			fmt.Fprint(c, "204 device info:\r\nprotocol version: 1.11\r\nmodel: HyperDeck Studio HD Plus (simulatore)\r\nunique id: 0123456789\r\nslot count: 2\r\nsoftware version: 8.1\r\n\r\n")
		case cmd == "transport info":
			fmt.Fprint(c, "208 transport info:\r\nstatus: record\r\nspeed: 0\r\nslot id: 1\r\ndisplay timecode: 01:02:03:04\r\ntimecode: 01:02:03:04\r\nvideo format: 1080p50\r\nloop: false\r\n\r\n")
		case cmd == "slot info: slot id: 1":
			fmt.Fprint(c, "202 slot info:\r\nslot id: 1\r\nstatus: mounted\r\nvolume name: SSD-SHOW\r\nrecording time: 1200\r\nvideo format: 1080p50\r\n\r\n")
		case cmd == "slot info: slot id: 2":
			fmt.Fprint(c, "202 slot info:\r\nslot id: 2\r\nstatus: empty\r\n\r\n")
		case cmd == "quit":
			fmt.Fprint(c, "200 ok\r\n")
			return
		default:
			fmt.Fprint(c, "100 syntax error\r\n")
		}
	}
}

func atemCmd(name string, data []byte) []byte {
	b := make([]byte, 8, 8+len(data))
	binary.BigEndian.PutUint16(b, uint16(8+len(data)))
	copy(b[4:], name)
	return append(b, data...)
}

func atemPkt(flags byte, sid, ack, pid uint16, payload []byte) []byte {
	n := 12 + len(payload)
	b := make([]byte, 12, n)
	b[0], b[1] = flags<<3|byte(n>>8&7), byte(n)
	binary.BigEndian.PutUint16(b[2:], sid)
	binary.BigEndian.PutUint16(b[4:], ack)
	binary.BigEndian.PutUint16(b[10:], pid)
	return append(b, payload...)
}

func atemState() []byte {
	pin := make([]byte, 44)
	copy(pin, "ATEM Television Studio HD8 (simulatore)")
	var p []byte
	p = append(p, atemCmd("_ver", []byte{0, 2, 0, 30})...)
	p = append(p, atemCmd("_pin", pin)...)
	for id, name := range map[uint16]string{1: "Camera 1", 2: "Camera 2", 3: "Grafica"} {
		d := make([]byte, 36)
		binary.BigEndian.PutUint16(d, id)
		copy(d[2:22], name)
		copy(d[22:26], fmt.Sprintf("CAM%d", id))
		p = append(p, atemCmd("InPr", d)...)
	}
	p = append(p, atemCmd("PrgI", []byte{0, 0, 0, 2})...)
	p = append(p, atemCmd("PrvI", []byte{0, 0, 0, 1, 0, 0, 0, 0})...)
	p = append(p, atemCmd("InCm", []byte{1, 0, 0, 0})...)
	return p
}

func atemServe(ctx context.Context, addr string) error {
	pc, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); pc.Close() }()
	go func() {
		buf := make([]byte, 1500)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if n < 12 {
				continue
			}
			if buf[0]>>3&0x02 != 0 { // hello: accept, then send the state
				pc.WriteTo(atemPkt(0x02, 0x53AB, 0, 0, []byte{0x02, 0, 0, 0, 0, 0, 0, 0}), from)
				pc.WriteTo(atemPkt(0x01, 0x8001, 0, 1, atemState()), from)
			}
		}
	}()
	return nil
}

const vmixXML = `<vmix><version>27.0.0.49</version><edition>4K</edition><inputs><input key="a" number="1" type="Capture" title="Camera palco"/><input key="b" number="2" type="Video" title="Intro.mp4"/></inputs><active>2</active><preview>1</preview><recording>True</recording><external>False</external><streaming>False</streaming></vmix>`

// RunVideo starts the simulated video devices on host (e.g. "0.0.0.0").
func RunVideo(ctx context.Context, host string, p VideoPorts) error {
	at := func(port int) string { return fmt.Sprintf("%s:%d", host, port) }
	if err := serveTCP(ctx, at(p.Videohub), func(c net.Conn) { fmt.Fprint(c, videohubPrelude); bufio.NewReader(c).ReadString(0) }); err != nil {
		return err
	}
	if err := serveTCP(ctx, at(p.HyperDeck), hyperdeck); err != nil {
		return err
	}
	if err := atemServe(ctx, at(p.ATEM)); err != nil {
		return err
	}
	ln, err := net.Listen("tcp4", at(p.VMix))
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, vmixXML)
	})}
	go func() { <-ctx.Done(); srv.Close() }()
	go srv.Serve(ln)
	return nil
}
