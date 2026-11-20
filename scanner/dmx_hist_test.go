package scanner

import "testing"

func TestThinHistory(t *testing.T) {
	end := int64(1_000_000)
	var h [][2]int64
	for ms := end - 300_000; ms <= end; ms += 100 { // 5 minutes, a point every 100 ms
		v := int64(100)
		if ms == end-150_000 {
			v = 255 // one flash
		}
		h = append(h, [2]int64{ms, v})
	}
	ten := thinHistory(h, 10, end)
	if ten[0][0] != end-10_000 || len(ten) > histPoints {
		t.Errorf("10 s: %d points from %d", len(ten), ten[0][0])
	}
	all := thinHistory(h, 300, end)
	if len(all) > histPoints {
		t.Errorf("thinned to %d points", len(all))
	}
	flash := false
	for _, p := range all {
		flash = flash || p[1] == 255
	}
	if !flash {
		t.Error("the flash was lost when thinning")
	}
	for i := 1; i < len(all); i++ {
		if all[i][0] < all[i-1][0] {
			t.Fatal("points out of order")
		}
	}
	// A value set before the window starts is carried to its start.
	if got := thinHistory([][2]int64{{end - 60_000, 42}}, 10, end); len(got) != 1 || got[0] != [2]int64{end - 10_000, 42} {
		t.Errorf("carried %v", got)
	}
}
