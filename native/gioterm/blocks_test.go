package gioterm

import "testing"

func area(rects []cellRect) float32 {
	var a float32
	for _, r := range rects {
		a += (r.x1 - r.x0) * (r.y1 - r.y0)
	}
	return a
}

func TestBlockElementCoverage(t *testing.T) {
	cases := []struct {
		r     rune
		area  float32
		alpha uint8
	}{
		{'█', 1, 255}, {'▀', .5, 255}, {'▄', .5, 255}, {'▌', .5, 255}, {'▐', .5, 255},
		{'▁', .125, 255}, {'▏', .125, 255}, {'▛', .75, 255}, {'▚', .5, 255}, {'▝', .25, 255},
		{'░', 1, 64}, {'▒', 1, 128}, {'▓', 1, 192},
	}
	for _, c := range cases {
		rects, alpha, ok := blockElement(c.r)
		if !ok || area(rects) != c.area || alpha != c.alpha {
			t.Errorf("%c: ok=%v area=%v alpha=%d, want area %v alpha %d", c.r, ok, area(rects), alpha, c.area, c.alpha)
		}
	}
	if _, _, ok := blockElement('a'); ok {
		t.Error("'a' reported as a block element")
	}
}
