package showsim

import (
	"context"
	"encoding/binary"
	"net"
)

// A Dante device answering ARC (read-only requests), with the reply layout
// wasaping's client reads: 4 transmit channels, 4 receive channels — one
// connected, one unresolved, one free, one with a clock domain problem.

// DanteName is the simulated device's name.
const DanteName = "Rio3224-SIM"

type danteRx struct {
	name, txChan, txDev string
	status              uint16
}

var danteTx = []string{"Mic 1", "Mic 2", "Mic 3", "Mic 4"}

var danteRxs = []danteRx{
	{"Ch 1", "Mix L", "CL5-FOH", 9},
	{"Ch 2", "Mix R", "CL5-OFF", 1},
	{"Ch 3", "", "", 0},
	{"Ch 4", "Click", "QLab-Mac", 27},
}

// arcReply builds a reply: header (10 bytes), 2 spare bytes, the records,
// then the labels they point to.
func arcReply(req []byte, records [][]byte, labels map[int]string, fix func(rec []byte, off map[int]int)) []byte {
	b := append([]byte{}, req[:8]...)
	b = append(b, 0, 1, 0, byte(len(records)))
	start := len(b)
	for _, r := range records {
		b = append(b, r...)
	}
	off := map[int]int{}
	for k := 0; k < len(labels); k++ {
		off[k] = len(b)
		b = append(append(b, labels[k]...), 0)
	}
	if fix != nil {
		fix(b[start:], off)
	}
	binary.BigEndian.PutUint16(b[2:], uint16(len(b)))
	return b
}

func danteAnswer(req []byte) []byte {
	if len(req) < 10 || req[0] != 0x27 {
		return nil
	}
	switch binary.BigEndian.Uint16(req[6:]) {
	case 0x1002: // name
		b := append(append([]byte{}, req[:8]...), 0, 1)
		b = append(append(b, DanteName...), 0)
		binary.BigEndian.PutUint16(b[2:], uint16(len(b)))
		return b
	case 0x1000: // counts
		b := append(append([]byte{}, req[:8]...), 0, 1, 0, 0, 0, byte(len(danteTx)), 0, byte(len(danteRxs)))
		binary.BigEndian.PutUint16(b[2:], uint16(len(b)))
		return b
	case 0x2000: // transmit channels
		labels := map[int]string{}
		recs := make([][]byte, len(danteTx))
		for i, n := range danteTx {
			labels[i] = n
			recs[i] = make([]byte, 8)
			binary.BigEndian.PutUint16(recs[i], uint16(i+1))
		}
		return arcReply(req, recs, labels, func(rec []byte, off map[int]int) {
			for i := range danteTx {
				binary.BigEndian.PutUint16(rec[i*8+6:], uint16(off[i]))
			}
		})
	case 0x3000: // receive channels
		labels := map[int]string{}
		recs := make([][]byte, len(danteRxs))
		for i, r := range danteRxs {
			labels[i*3], labels[i*3+1], labels[i*3+2] = r.name, r.txChan, r.txDev
			recs[i] = make([]byte, 20)
			binary.BigEndian.PutUint16(recs[i], uint16(i+1))
			binary.BigEndian.PutUint16(recs[i][14:], r.status)
		}
		return arcReply(req, recs, labels, func(rec []byte, off map[int]int) {
			for i, r := range danteRxs {
				o := rec[i*20:]
				binary.BigEndian.PutUint16(o[10:], uint16(off[i*3]))
				if r.txDev != "" {
					binary.BigEndian.PutUint16(o[6:], uint16(off[i*3+1]))
					binary.BigEndian.PutUint16(o[8:], uint16(off[i*3+2]))
				}
			}
		})
	}
	return nil
}

// RunDante answers ARC on addr (e.g. "0.0.0.0:4440") until ctx ends.
func RunDante(ctx context.Context, addr string) error {
	pc, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); pc.Close() }()
	buf := make([]byte, 1500)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			return nil
		}
		if r := danteAnswer(buf[:n]); r != nil {
			pc.WriteTo(r, from)
		}
	}
}
