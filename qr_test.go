package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestQREncode(t *testing.T) {
	for _, s := range []string{"http://192.168.1.13:47474/", "https://10.0.0.5:47443/", strings.Repeat("x", 100), "https://example-host-name.local:47443/a/b/c/d/e/f/g", "https://wasaping-box-arena.local:47443/a-longer-path-for-version-four-and-five/xyz"} {
		q, err := qrEncode(s)
		if err != nil {
			t.Fatal(err)
		}
		if q.size < 21 || (q.size-17)%4 != 0 {
			t.Fatalf("size %d", q.size)
		}
		// Finder corners are dark, the separator next to them is light.
		if !q.dark[0][0] || !q.dark[0][q.size-1] || !q.dark[q.size-1][0] || q.dark[7][7] {
			t.Fatal("finder patterns")
		}
		// Optional: write a PNG to check it with a real reader.
		if p := os.Getenv("WASAPING_QR_PNG"); p != "" && s == firstNonEmpty(os.Getenv("WASAPING_QR_TEXT"), "http://192.168.1.13:47474/") {
			const scale, quiet = 10, 4
			n := (q.size + 2*quiet) * scale
			img := image.NewGray(image.Rect(0, 0, n, n))
			for y := 0; y < n; y++ {
				for x := 0; x < n; x++ {
					mx, my := x/scale-quiet, y/scale-quiet
					c := color.Gray{255}
					if mx >= 0 && my >= 0 && mx < q.size && my < q.size && q.dark[my][mx] {
						c = color.Gray{0}
					}
					img.SetGray(x, y, c)
				}
			}
			f, _ := os.Create(p)
			png.Encode(f, img)
			f.Close()
		}
	}
	if _, err := qrEncode(strings.Repeat("x", 200)); err == nil {
		t.Fatal("too long text accepted")
	}
	if svg, err := qrSVG("http://a/"); err != nil || !strings.HasPrefix(svg, "<svg") {
		t.Fatal("svg")
	}
}
