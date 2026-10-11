package icons

import (
	"image"
	"image/color"
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func segsOf(t *testing.T, d string) []segment {
	t.Helper()
	var b pathBuilder
	if err := b.path(d); err != nil {
		t.Fatalf("path %q: %v", d, err)
	}
	return b.segs
}

func ends(segs []segment) []f32.Point {
	var out []f32.Point
	for _, s := range segs {
		if s.op != 'Z' {
			out = append(out, s.to)
		}
	}
	return out
}

func near(a, b f32.Point) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-3 && math.Abs(float64(a.Y-b.Y)) < 1e-3
}

func TestPathCommands(t *testing.T) {
	cases := []struct {
		d    string
		want []f32.Point
	}{
		{"M2 2h4v4H2z", []f32.Point{f32.Pt(2, 2), f32.Pt(6, 2), f32.Pt(6, 6), f32.Pt(2, 6)}},
		// Coordinates after a moveto are linetos, relative after a relative one.
		{"m1 1 2 0 0 2", []f32.Point{f32.Pt(1, 1), f32.Pt(3, 1), f32.Pt(3, 3)}},
		// Numbers that run together.
		{"M.5.5L1-2", []f32.Point{f32.Pt(0.5, 0.5), f32.Pt(1, -2)}},
		{"M1e1 2E-1", []f32.Point{f32.Pt(10, 0.2)}},
		// A relative command after Z starts from the subpath's start.
		{"M4 4l2 0zl0 3", []f32.Point{f32.Pt(4, 4), f32.Pt(6, 4), f32.Pt(4, 7)}},
	}
	for _, tc := range cases {
		got := ends(segsOf(t, tc.d))
		if len(got) != len(tc.want) {
			t.Errorf("%q: got %v, want %v", tc.d, got, tc.want)
			continue
		}
		for i := range got {
			if !near(got[i], tc.want[i]) {
				t.Errorf("%q: point %d = %v, want %v", tc.d, i, got[i], tc.want[i])
			}
		}
	}
}

func TestPathRejectsGarbage(t *testing.T) {
	for _, d := range []string{"10 10", "M1", "M1 1 A1 1 0 2 1 2 2", "M1 1 X2"} {
		var b pathBuilder
		if err := b.path(d); err == nil {
			t.Errorf("path %q parsed, want an error", d)
		}
	}
}

// A half circle from (0,12) to (24,12) over the top: two quarter cubics, the
// first ending at the top of the circle. Packed flags ("011") read as two
// flags and a coordinate.
func TestArcIsTheCircle(t *testing.T) {
	for _, d := range []string{"M0 12A12 12 0 0 1 24 12", "M0 12a12 12 0 0124 0"} {
		segs := segsOf(t, d)
		if len(segs) != 3 {
			t.Fatalf("%q: got %d segments, want move and two cubics", d, len(segs))
		}
		if !near(segs[1].to, f32.Pt(12, 0)) || !near(segs[2].to, f32.Pt(24, 12)) {
			t.Errorf("%q: quarter ends %v and %v, want (12,0) and (24,12)", d, segs[1].to, segs[2].to)
		}
	}
}

func TestRectRoundsItsCorners(t *testing.T) {
	var b pathBuilder
	b.rect(2, 4, 20, 16, 2, 0)
	if !near(b.segs[0].to, f32.Pt(4, 4)) || !near(b.segs[2].to, f32.Pt(22, 6)) {
		t.Errorf("got start %v and first corner end %v, want (4,4) and (22,6)", b.segs[0].to, b.segs[2].to)
	}
}

func TestEverySetIconParses(t *testing.T) {
	set, err := lucideSet()
	if err != nil {
		t.Fatal(err)
	}
	if len(set) < 1700 {
		t.Errorf("got %d Lucide icons, want the whole set", len(set))
	}
	for name, nodes := range set {
		if _, err := parseLucide(nodes); err != nil {
			t.Errorf("Lucide %q: %v", name, err)
		}
	}
	for name := range lobePaths {
		Lobe(name)
	}
}

func TestUnknownNamePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Lucide(\"no-such-icon\") did not panic")
		}
	}()
	Lucide("no-such-icon")
}

// render draws ic at 24px in opaque white and reports whether (x, y) is
// inked.
func render(t *testing.T, ic *Icon) func(x, y int) bool {
	t.Helper()
	const size = 24
	win, err := headless.NewWindow(size, size)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(size, size))}
	ic.Layout(gtx, size, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return func(x, y int) bool { return img.RGBAAt(x, y).A > 128 }
}

func TestLucideStrokes(t *testing.T) {
	inked := render(t, Lucide("x"))
	if !inked(12, 12) || inked(12, 3) || inked(3, 12) {
		t.Errorf("x: crossing inked %v, top edge %v, left edge %v; want only the crossing", inked(12, 12), inked(12, 3), inked(3, 12))
	}
}

func TestLobeFillsAndKeepsCounters(t *testing.T) {
	inked := render(t, Lobe("opencode"))
	if !inked(6, 12) || inked(12, 12) {
		t.Errorf("opencode: frame inked %v, counter inked %v; want the frame filled and the counter open", inked(6, 12), inked(12, 12))
	}
}
