package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/io/input"
)

func TestSkeletonSize(t *testing.T) {
	th := testTheme()
	cases := []struct {
		name          string
		width, height float32
		max           int
		want          image.Point
	}{
		{"h-3 w-56", 56, 3, 600, image.Pt(224, 12)},
		{"w-full is the row", 0, 3, 300, image.Pt(300, 12)},
		{"max-w-full clamps", 80, 6, 100, image.Pt(100, 24)},
	}
	for _, tc := range cases {
		s := th.Skeleton(Space(tc.width), Space(tc.height))
		if d := layoutAt(s.Layout, 300, tc.max); d.Size != tc.want {
			t.Errorf("%s: got %v, want %v, whatever the minimum constraint", tc.name, d.Size, tc.want)
		}
	}
}

// skeletonAt renders one skeleton at a point of the pulse and reads its fill.
func skeletonAt(t *testing.T, at time.Duration) color.NRGBA {
	t.Helper()
	s := testTheme().Skeleton(Space(10), Space(10))
	ops := tabsFrame(new(input.Router), time.Unix(100, 0).Add(at), func(gtx C) D { return s.Layout(gtx) })
	p := tabsRender(t, ops).RGBAAt(5, 5)
	return color.NRGBA{R: p.R, G: p.G, B: p.B, A: p.A}
}

// animate-pulse: full opacity at both ends of its 2s, half of it at 1s. The
// browser lays bg-foreground/20 (#fafafa at 20%) over the page (#09090b) in
// sRGB: #39393b, and half of that opacity, #212123.
func TestSkeletonPulsesBetweenFullAndHalf(t *testing.T) {
	full, half, again := skeletonAt(t, 0), skeletonAt(t, time.Second), skeletonAt(t, 2*time.Second)
	wantFull, wantHalf := color.NRGBA{R: 57, G: 57, B: 59, A: 255}, color.NRGBA{R: 33, G: 33, B: 35, A: 255}
	close := func(a, b color.NRGBA) bool {
		d := func(x, y uint8) bool { return int(x)-int(y) <= 1 && int(y)-int(x) <= 1 }
		return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && a.A == b.A
	}
	if !close(full, wantFull) || !close(again, wantFull) {
		t.Errorf("at the ends of the period: %v and %v, want bg-foreground/20 over the page %v", full, again, wantFull)
	}
	if !close(half, wantHalf) {
		t.Errorf("at 1s: %v, want half of that opacity %v", half, wantHalf)
	}
}

func TestSkeletonAsksForTheNextFrame(t *testing.T) {
	s := testTheme().Skeleton(Space(10), Space(10))
	var r input.Router
	tabsFrame(&r, time.Unix(100, 0), func(gtx C) D { return s.Layout(gtx) })
	if _, ok := r.WakeupTime(); !ok {
		t.Error("a pulsing skeleton must keep the window redrawing")
	}
}
