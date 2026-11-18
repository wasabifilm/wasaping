// Command winres builds the Windows resource files (rsrc_windows_*.syso)
// that give wasaping.exe its icon. Go links any .syso in the package
// automatically. Standard library only. Run: make winicon
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

var sizes = []int{16, 24, 32, 48, 64, 128, 256}

func main() {
	src := "assets/icon.png"
	f, err := os.Open(src)
	if err != nil {
		fail(err)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		fail(err)
	}
	var icons [][]byte
	for _, s := range sizes {
		var b bytes.Buffer
		if err := png.Encode(&b, scale(img, s)); err != nil {
			fail(err)
		}
		icons = append(icons, b.Bytes())
	}
	// amd64: IMAGE_REL_AMD64_ADDR32NB = 3; arm64: IMAGE_REL_ARM64_ADDR32NB = 2.
	for _, t := range []struct {
		arch    string
		machine uint16
		reloc   uint16
	}{{"amd64", 0x8664, 3}, {"arm64", 0xAA64, 2}} {
		out := fmt.Sprintf("rsrc_windows_%s.syso", t.arch)
		if err := os.WriteFile(out, coff(icons, t.machine, t.reloc), 0o644); err != nil {
			fail(err)
		}
		fmt.Println("→", out)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "winres:", err)
	os.Exit(1)
}

// scale shrinks a square image to n×n by averaging (premultiplied) pixels.
func scale(src image.Image, n int) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, n, n))
	fx, fy := float64(b.Dx())/float64(n), float64(b.Dy())/float64(n)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			x0, x1 := b.Min.X+int(float64(x)*fx), b.Min.X+int(float64(x+1)*fx)
			y0, y1 := b.Min.Y+int(float64(y)*fy), b.Min.Y+int(float64(y+1)*fy)
			var r, g, bl, a, cnt float64
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					c := color.NRGBAModel.Convert(src.At(xx, yy)).(color.NRGBA)
					al := float64(c.A) / 255
					r += float64(c.R) * al
					g += float64(c.G) * al
					bl += float64(c.B) * al
					a += al
					cnt++
				}
			}
			if a > 0 {
				dst.SetNRGBA(x, y, color.NRGBA{uint8(r / a), uint8(g / a), uint8(bl / a), uint8(255 * a / cnt)})
			}
		}
	}
	return dst
}

// coff writes a COFF object with one .rsrc section holding RT_ICON entries
// (PNG images, ids 1..n) and an RT_GROUP_ICON (id 1) that lists them.
func coff(icons [][]byte, machine, relocType uint16) []byte {
	const (
		rtIcon      = 3
		rtGroupIcon = 14
		lang        = 0x0409
	)
	n := len(icons)
	le := binary.LittleEndian
	dir := func(b *bytes.Buffer, entries int) {
		// characteristics, time stamp, version, then named entries = 0 (low
		// 16 bits) and id entries = entries (high 16 bits).
		binary.Write(b, le, [4]uint32{0, 0, 0, uint32(entries) << 16})
	}
	// Layout: root dir, type dirs, name dirs, language dirs, data entries, data.
	const dirHdr, entry, dataEntry = 16, 8, 16
	rootSize := dirHdr + 2*entry
	iconTypeDir := rootSize
	groupTypeDir := iconTypeDir + dirHdr + n*entry
	langDirs := groupTypeDir + dirHdr + entry // n+1 language dirs, each with one entry
	dataEntries := langDirs + (n+1)*(dirHdr+entry)
	dataStart := dataEntries + (n+1)*dataEntry

	var group bytes.Buffer
	binary.Write(&group, le, [3]uint16{0, 1, uint16(n)})
	for i, png := range icons {
		w := byte(sizes[i])
		if sizes[i] >= 256 {
			w = 0
		}
		group.Write([]byte{w, w, 0, 0})
		binary.Write(&group, le, uint16(1))  // planes
		binary.Write(&group, le, uint16(32)) // bit count
		binary.Write(&group, le, uint32(len(png)))
		binary.Write(&group, le, uint16(i+1))
	}
	blobs := append(append([][]byte{}, icons...), group.Bytes())
	offsets := make([]int, len(blobs))
	off := dataStart
	for i, b := range blobs {
		offsets[i] = off
		off += (len(b) + 7) &^ 7
	}

	var s bytes.Buffer
	dir(&s, 2)
	binary.Write(&s, le, [2]uint32{rtIcon, 0x80000000 | uint32(iconTypeDir)})
	binary.Write(&s, le, [2]uint32{rtGroupIcon, 0x80000000 | uint32(groupTypeDir)})
	dir(&s, n)
	for i := 0; i < n; i++ {
		binary.Write(&s, le, [2]uint32{uint32(i + 1), 0x80000000 | uint32(langDirs+i*(dirHdr+entry))})
	}
	dir(&s, 1)
	binary.Write(&s, le, [2]uint32{1, 0x80000000 | uint32(langDirs+n*(dirHdr+entry))})
	for i := 0; i <= n; i++ {
		dir(&s, 1)
		binary.Write(&s, le, [2]uint32{lang, uint32(dataEntries + i*dataEntry)})
	}
	var relocs []uint32
	for i, b := range blobs {
		relocs = append(relocs, uint32(s.Len()))
		binary.Write(&s, le, [4]uint32{uint32(offsets[i]), uint32(len(b)), 0, 0})
	}
	for _, b := range blobs {
		s.Write(b)
		for s.Len()%8 != 0 {
			s.WriteByte(0)
		}
	}
	raw := s.Bytes()

	const fileHdr, secHdr = 20, 40
	relocPtr := fileHdr + secHdr + len(raw)
	symPtr := relocPtr + len(relocs)*10
	var o bytes.Buffer
	binary.Write(&o, le, machine)
	binary.Write(&o, le, uint16(1))      // sections
	binary.Write(&o, le, uint32(0))      // timestamp
	binary.Write(&o, le, uint32(symPtr)) // symbol table
	binary.Write(&o, le, uint32(1))      // symbols
	binary.Write(&o, le, uint16(0))      // optional header
	binary.Write(&o, le, uint16(0))      // characteristics
	o.WriteString(".rsrc\x00\x00\x00")
	binary.Write(&o, le, [4]uint32{0, 0, uint32(len(raw)), fileHdr + secHdr})
	binary.Write(&o, le, uint32(relocPtr))
	binary.Write(&o, le, uint32(0))
	binary.Write(&o, le, uint16(len(relocs)))
	binary.Write(&o, le, uint16(0))
	binary.Write(&o, le, uint32(0x40000040)) // initialized data, readable
	o.Write(raw)
	for _, r := range relocs {
		binary.Write(&o, le, r)         // where
		binary.Write(&o, le, uint32(0)) // symbol 0: the section
		binary.Write(&o, le, relocType)
	}
	o.WriteString(".rsrc\x00\x00\x00")
	binary.Write(&o, le, uint32(0)) // value
	binary.Write(&o, le, uint16(1)) // section number
	binary.Write(&o, le, uint16(0)) // type
	o.WriteByte(3)                  // static
	o.WriteByte(0)                  // aux
	binary.Write(&o, le, uint32(4)) // empty string table
	return o.Bytes()
}
