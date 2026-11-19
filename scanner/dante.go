package scanner

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

// Dante routing, read-only, over ARC: the control protocol Dante Controller
// uses (UDP 4440). Audinate does not publish it; the messages here follow
// what open-source projects (netaudio and others) reconstructed. wasaping
// only asks: device name, channel counts, transmit channels and, for each
// receive channel, where it is subscribed and with what status.

// DanteARCPort is where devices listen for ARC (the tests use another port).
var DanteARCPort = 4440

// DanteChannel is a transmit channel.
type DanteChannel struct {
	Num  int    `json:"num"`
	Name string `json:"name"`
}

// DanteRx is a receive channel and its subscription.
type DanteRx struct {
	Num      int    `json:"num"`
	Name     string `json:"name"`
	TxChan   string `json:"tx_channel,omitempty"`
	TxDevice string `json:"tx_device,omitempty"`
	Status   int    `json:"status"`
	State    string `json:"state"`  // "ok", "bad", "none" (not subscribed), "wait"
	Detail   string `json:"detail"` // in words
}

// DanteDevice is what one device says over ARC.
type DanteDevice struct {
	IP      string         `json:"ip"`
	Name    string         `json:"name,omitempty"`
	TxCount int            `json:"tx_count"`
	RxCount int            `json:"rx_count"`
	Tx      []DanteChannel `json:"tx,omitempty"`
	Rx      []DanteRx      `json:"rx,omitempty"`
	Err     string         `json:"error,omitempty"`
}

const (
	arcName     = 0x1002
	arcCounts   = 0x1000
	arcTx       = 0x2000
	arcTxFriend = 0x2010
	arcRx       = 0x3000
)

// subscription states, as in Audinate's API (rx status).
var danteRxStatus = map[int][2]string{
	0:   {"none", "non collegato"},
	1:   {"bad", "non risolto: il trasmettitore non si trova sulla rete"},
	2:   {"wait", "risolto, in attesa"},
	3:   {"bad", "risoluzione fallita"},
	4:   {"ok", "collegato a sé stesso"},
	7:   {"wait", "inattivo"},
	8:   {"wait", "in corso"},
	9:   {"ok", "collegato (unicast)"},
	10:  {"ok", "collegato (multicast)"},
	14:  {"ok", "collegato (manuale)"},
	15:  {"bad", "nessuna connessione"},
	16:  {"bad", "formato del canale incompatibile"},
	17:  {"bad", "formato del flusso incompatibile"},
	18:  {"bad", "nessuna risorsa in ricezione"},
	19:  {"bad", "errore in ricezione"},
	20:  {"bad", "il trasmettitore non ha più flussi liberi"},
	21:  {"bad", "errore del trasmettitore"},
	22:  {"bad", "QoS: banda insufficiente in ricezione"},
	23:  {"bad", "QoS: banda insufficiente in trasmissione"},
	24:  {"bad", "il trasmettitore rifiuta l'indirizzo"},
	25:  {"bad", "messaggio non valido"},
	26:  {"bad", "latenza del canale troppo alta per questo ricevitore"},
	27:  {"bad", "dominio di clock diverso"},
	28:  {"bad", "non supportato"},
	29:  {"bad", "collegamento del ricevitore giù"},
	30:  {"bad", "collegamento del trasmettitore giù"},
	31:  {"bad", "protocollo dinamico"},
	32:  {"bad", "canale inesistente sul trasmettitore"},
	33:  {"bad", "errore di pianificazione del trasmettitore"},
	34:  {"bad", "il trasmettitore non accetta abbonamenti a sé stesso"},
	35:  {"wait", "trasmettitore non pronto"},
	36:  {"wait", "ricevitore non pronto"},
	37:  {"bad", "il trasmettitore ha raggiunto il limite di ricevitori"},
	38:  {"bad", "canale cifrato"},
	39:  {"bad", "risposta inattesa dal trasmettitore"},
	255: {"bad", "errore di sistema"},
}

func danteState(code int) (string, string) {
	if s, ok := danteRxStatus[code&0xFF]; ok {
		return s[0], s[1]
	}
	return "bad", fmt.Sprintf("stato %d", code)
}

type arcConn struct {
	c   *net.UDPConn
	seq uint16
}

func (a *arcConn) ask(ctx context.Context, op uint16, args []byte) ([]byte, error) {
	if args == nil {
		args = []byte{0, 0}
	}
	a.seq++
	req := []byte{0x27, 0xFF, 0, 0}
	req = binary.BigEndian.AppendUint16(req, a.seq)
	req = binary.BigEndian.AppendUint16(req, op)
	req = append(req, args...)
	binary.BigEndian.PutUint16(req[2:], uint16(len(req)))
	buf := make([]byte, 2048)
	for try := 0; try < 2 && ctx.Err() == nil; try++ {
		if _, err := a.c.Write(req); err != nil {
			return nil, err
		}
		a.c.SetReadDeadline(time.Now().Add(600 * time.Millisecond))
		for {
			n, err := a.c.Read(buf)
			if err != nil {
				break // timeout: try again
			}
			b := buf[:n]
			if n >= 10 && b[0] == 0x27 && binary.BigEndian.Uint16(b[4:]) == a.seq && binary.BigEndian.Uint16(b[6:]) == op {
				diag("dante-arc", a.c.RemoteAddr().String(), b)
				return append([]byte(nil), b...), nil
			}
		}
	}
	return nil, fmt.Errorf("nessuna risposta ARC")
}

// arcLabel reads the NUL-terminated text at an offset of the reply.
func arcLabel(b []byte, off int) string {
	if off <= 0 || off >= len(b) {
		return ""
	}
	end := off
	for end < len(b) && b[end] != 0 && end-off < 64 {
		end++
	}
	return string(b[off:end])
}

func pageArgs(start int) []byte {
	return []byte{0, 0, 0, 1, byte(start >> 8), byte(start), 0, 0}
}

// danteQuery asks one device.
func danteQuery(ctx context.Context, ip string) DanteDevice {
	d := DanteDevice{IP: ip}
	c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP(ip), Port: DanteARCPort})
	if err != nil {
		d.Err = err.Error()
		return d
	}
	defer c.Close()
	a := &arcConn{c: c, seq: uint16(time.Now().UnixNano())}
	b, err := a.ask(ctx, arcName, nil)
	if err != nil {
		d.Err = "il dispositivo non risponde in ARC (porta 4440)"
		return d
	}
	d.Name = arcLabel(b, 10)
	if b, err := a.ask(ctx, arcCounts, nil); err == nil && len(b) >= 16 {
		d.TxCount, d.RxCount = int(binary.BigEndian.Uint16(b[12:])), int(binary.BigEndian.Uint16(b[14:]))
	}
	d.TxCount, d.RxCount = min(d.TxCount, 1024), min(d.RxCount, 1024)
	// Transmit channels, 16 per page: number and name (8-byte records).
	for start := 1; start <= d.TxCount; start += 16 {
		b, err := a.ask(ctx, arcTx, pageArgs(start))
		if err != nil {
			break
		}
		for i := 0; i < min(16, d.TxCount-start+1); i++ {
			o := 12 + i*8
			if o+8 > len(b) {
				break
			}
			num := int(binary.BigEndian.Uint16(b[o:]))
			if num == 0 {
				break
			}
			d.Tx = append(d.Tx, DanteChannel{Num: num, Name: arcLabel(b, int(binary.BigEndian.Uint16(b[o+6:])))})
		}
	}
	// Receive channels, 16 per page (20-byte records): number, offsets of
	// the transmit channel, transmit device and own name, then the status.
	for start := 1; start <= d.RxCount; start += 16 {
		b, err := a.ask(ctx, arcRx, pageArgs(start))
		if err != nil {
			break
		}
		for i := 0; i < min(16, d.RxCount-start+1); i++ {
			o := 12 + i*20
			if o+20 > len(b) {
				break
			}
			num := int(binary.BigEndian.Uint16(b[o:]))
			if num == 0 {
				break
			}
			rx := DanteRx{Num: num, Name: arcLabel(b, int(binary.BigEndian.Uint16(b[o+10:])))}
			rx.TxDevice = arcLabel(b, int(binary.BigEndian.Uint16(b[o+8:])))
			if off := int(binary.BigEndian.Uint16(b[o+6:])); off != 0 {
				rx.TxChan = arcLabel(b, off)
			} else if rx.TxDevice != "" {
				rx.TxChan = rx.Name // same name on both sides
			}
			rx.Status = int(binary.BigEndian.Uint16(b[o+14:]))
			rx.State, rx.Detail = danteState(rx.Status)
			if rx.TxDevice == "" && rx.State != "ok" {
				rx.State, rx.Detail = "none", "non collegato"
			}
			d.Rx = append(d.Rx, rx)
		}
	}
	return d
}

// DanteRouting asks every device in ips (at most 16 at a time).
func DanteRouting(ctx context.Context, ips []string) []DanteDevice {
	out := make([]DanteDevice, len(ips))
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	for i, ip := range ips {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = danteQuery(ctx, ip)
		}()
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// DanteSummary counts the subscriptions: ok, broken, and the estimated
// unicast bandwidth each transmitter sends (about 1.5 Mbit/s per channel at
// 48 kHz with packet overhead; double at 96 kHz).
type DanteSummary struct {
	OK, Bad, Wait int
	TxMbps        map[string]float64 // transmitting device -> estimated Mbit/s
}

func SummarizeDante(devs []DanteDevice, rate map[string]int) DanteSummary {
	s := DanteSummary{TxMbps: map[string]float64{}}
	for _, d := range devs {
		for _, r := range d.Rx {
			switch r.State {
			case "ok":
				s.OK++
				per := 1.5
				if rate[r.TxDevice] >= 88200 {
					per = 3
				}
				if r.TxDevice != d.Name {
					s.TxMbps[r.TxDevice] += per
				}
			case "bad":
				s.Bad++
			case "wait":
				s.Wait++
			}
		}
	}
	return s
}
