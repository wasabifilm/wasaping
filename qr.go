package main

import (
	"errors"
	"fmt"
	"strings"
)

// A small QR code encoder (byte mode, error correction M, versions 1–6:
// up to 106 bytes, enough for an address like http://192.168.1.13:47474/).
// It follows ISO/IEC 18004; written by hand to keep wasaping free of
// dependencies.

// Per version (index = version): data codewords per block, number of
// blocks, error correction codewords per block, alignment centres.
var qrSpec = [...]struct {
	data, blocks, ec int
	align            []int
}{
	{},
	{16, 1, 10, nil},
	{28, 1, 16, []int{6, 18}},
	{44, 1, 26, []int{6, 22}},
	{32, 2, 18, []int{6, 26}},
	{43, 2, 24, []int{6, 30}},
	{27, 4, 16, []int{6, 34}},
}

type qrCode struct {
	size     int
	dark     [][]bool
	function [][]bool
}

// qrEncode returns the modules of the QR code for text.
func qrEncode(text string) (*qrCode, error) {
	data := []byte(text)
	ver := 0
	for v := 1; v < len(qrSpec); v++ {
		if len(data)+2 <= qrSpec[v].data*qrSpec[v].blocks { // 4+8 bits of header, 4 of terminator
			ver = v
			break
		}
	}
	if ver == 0 {
		return nil, errors.New("testo troppo lungo per il codice QR")
	}
	sp := qrSpec[ver]
	capacity := sp.data * sp.blocks

	// Bit stream: mode 0100 (byte), length, data, terminator, padding.
	var bits []bool
	put := func(v, n int) {
		for i := n - 1; i >= 0; i-- {
			bits = append(bits, v>>i&1 == 1)
		}
	}
	put(4, 4)
	put(len(data), 8)
	for _, b := range data {
		put(int(b), 8)
	}
	put(0, min(4, capacity*8-len(bits)))
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}
	cw := make([]byte, 0, capacity)
	for i := 0; i < len(bits); i += 8 {
		var b byte
		for j := 0; j < 8; j++ {
			if bits[i+j] {
				b |= 1 << (7 - j)
			}
		}
		cw = append(cw, b)
	}
	for pad := byte(0xEC); len(cw) < capacity; pad ^= 0xEC ^ 0x11 {
		cw = append(cw, pad)
	}

	// Error correction per block, then interleaving.
	var blocks, ecs [][]byte
	for b := 0; b < sp.blocks; b++ {
		blk := cw[b*sp.data : (b+1)*sp.data]
		blocks = append(blocks, blk)
		ecs = append(ecs, rsRemainder(blk, sp.ec))
	}
	var final []byte
	for i := 0; i < sp.data; i++ {
		for _, blk := range blocks {
			final = append(final, blk[i])
		}
	}
	for i := 0; i < sp.ec; i++ {
		for _, e := range ecs {
			final = append(final, e[i])
		}
	}

	q := newQR(ver)
	q.drawData(final)
	// The mask with the lowest penalty.
	best, bestScore := 0, -1
	for m := 0; m < 8; m++ {
		q.applyMask(m)
		q.drawFormat(m)
		if s := q.penalty(); bestScore < 0 || s < bestScore {
			best, bestScore = m, s
		}
		q.applyMask(m) // masking twice undoes it
	}
	q.applyMask(best)
	q.drawFormat(best)
	return q, nil
}

func newQR(ver int) *qrCode {
	n := ver*4 + 17
	q := &qrCode{size: n, dark: make([][]bool, n), function: make([][]bool, n)}
	for i := range q.dark {
		q.dark[i], q.function[i] = make([]bool, n), make([]bool, n)
	}
	// Timing patterns.
	for i := 0; i < n; i++ {
		q.set(6, i, i%2 == 0)
		q.set(i, 6, i%2 == 0)
	}
	// Finder patterns with their separators.
	for _, c := range [][2]int{{3, 3}, {n - 4, 3}, {3, n - 4}} {
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				x, y := c[0]+dx, c[1]+dy
				if x < 0 || y < 0 || x >= n || y >= n {
					continue
				}
				d := max(abs(dx), abs(dy))
				q.set(x, y, d != 2 && d != 4)
			}
		}
	}
	// Alignment patterns (not over the finders).
	al := qrSpec[ver].align
	for i, ax := range al {
		for j, ay := range al {
			if (i == 0 && j == 0) || (i == 0 && j == len(al)-1) || (i == len(al)-1 && j == 0) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					q.set(ax+dx, ay+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}
	q.drawFormat(0) // reserves the format areas and the dark module
	return q
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// set draws a function module (x = column, y = row).
func (q *qrCode) set(x, y int, dark bool) {
	q.dark[y][x] = dark
	q.function[y][x] = true
}

func (q *qrCode) drawFormat(mask int) {
	data := mask // error correction M = 00
	rem := data
	for i := 0; i < 10; i++ {
		rem = rem<<1 ^ (rem>>9)*0x537
	}
	bits := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return bits>>i&1 == 1 }
	n := q.size
	for i := 0; i <= 5; i++ {
		q.set(8, i, bit(i))
	}
	q.set(8, 7, bit(6))
	q.set(8, 8, bit(7))
	q.set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		q.set(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		q.set(n-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		q.set(8, n-15+i, bit(i))
	}
	q.set(8, n-8, true)
}

func (q *qrCode) drawData(data []byte) {
	n, i := q.size, 0
	for right := n - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < n; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = n - 1 - vert
				}
				if !q.function[y][x] && i < len(data)*8 {
					q.dark[y][x] = data[i>>3]>>(7-i&7)&1 == 1
					i++
				}
			}
		}
	}
}

func (q *qrCode) applyMask(m int) {
	for y := 0; y < q.size; y++ {
		for x := 0; x < q.size; x++ {
			if q.function[y][x] {
				continue
			}
			var inv bool
			switch m {
			case 0:
				inv = (x+y)%2 == 0
			case 1:
				inv = y%2 == 0
			case 2:
				inv = x%3 == 0
			case 3:
				inv = (x+y)%3 == 0
			case 4:
				inv = (x/3+y/2)%2 == 0
			case 5:
				inv = x*y%2+x*y%3 == 0
			case 6:
				inv = (x*y%2+x*y%3)%2 == 0
			case 7:
				inv = ((x+y)%2+x*y%3)%2 == 0
			}
			if inv {
				q.dark[y][x] = !q.dark[y][x]
			}
		}
	}
}

// penalty scores a masked symbol (lower = easier to read).
func (q *qrCode) penalty() int {
	n, score, dark := q.size, 0, 0
	at := func(x, y int, col bool) bool {
		if col {
			return q.dark[x][y]
		}
		return q.dark[y][x]
	}
	for _, col := range []bool{false, true} {
		for a := 0; a < n; a++ {
			run := 1
			var line strings.Builder
			for b := 0; b < n; b++ {
				v := at(b, a, col)
				if v {
					line.WriteByte('1')
				} else {
					line.WriteByte('0')
				}
				if b > 0 && v == at(b-1, a, col) {
					run++
					if run == 5 {
						score += 3
					} else if run > 5 {
						score++
					}
				} else if b > 0 {
					run = 1
				}
			}
			l := "0000" + line.String() + "0000"
			score += 40 * (strings.Count(l, "10111010000") + strings.Count(l, "00001011101"))
		}
	}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if q.dark[y][x] {
				dark++
			}
			if x < n-1 && y < n-1 && q.dark[y][x] == q.dark[y][x+1] && q.dark[y][x] == q.dark[y+1][x] && q.dark[y][x] == q.dark[y+1][x+1] {
				score += 3
			}
		}
	}
	total := n * n
	k := (abs(dark*20-total*10)+total-1)/total - 1
	return score + max(k, 0)*10
}

// rsRemainder is the Reed–Solomon error correction (GF(256), 0x11D).
func rsRemainder(data []byte, n int) []byte {
	mul := func(a, b byte) byte {
		var r byte
		for i := 7; i >= 0; i-- {
			hi := r & 0x80
			r <<= 1
			if hi != 0 {
				r ^= 0x1D
			}
			if b>>i&1 == 1 {
				r ^= a
			}
		}
		return r
	}
	// Generator polynomial: product of (x - α^i), i = 0..n-1.
	gen := make([]byte, n)
	gen[n-1] = 1
	root := byte(1)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			gen[j] = mul(gen[j], root)
			if j+1 < n {
				gen[j] ^= gen[j+1]
			}
		}
		root = mul(root, 2)
	}
	res := make([]byte, n)
	for _, b := range data {
		f := b ^ res[0]
		copy(res, res[1:])
		res[n-1] = 0
		for i := range res {
			res[i] ^= mul(gen[i], f)
		}
	}
	return res
}

// qrSVG draws text as an SVG QR code with the usual quiet zone.
func qrSVG(text string) (string, error) {
	q, err := qrEncode(text)
	if err != nil {
		return "", err
	}
	const quiet = 4
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="100%%" height="100%%" fill="#fff"/><path fill="#171717" d="`, q.size+2*quiet, q.size+2*quiet)
	for y := 0; y < q.size; y++ {
		for x := 0; x < q.size; x++ {
			if q.dark[y][x] {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String(), nil
}
