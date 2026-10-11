package ui

import (
	"image/color"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

func TestLabelIsAsTallAsItsFontSize(t *testing.T) {
	th := testTheme()
	for _, size := range []unit.Sp{TextSM, TextXS} {
		l := th.Label("Subject")
		l.Size = size
		if d := layoutAt(l.Layout, 0, 300); d.Size.Y != int(size) {
			t.Errorf("text %v: got height %d, want leading-none (%[1]v)", size, d.Size.Y)
		}
	}
}

func TestLabelUppercaseIsWider(t *testing.T) {
	th := testTheme()
	plain, upper := th.Label("worktree name"), th.Label("worktree name")
	upper.Uppercase = true
	if p, u := layoutAt(plain.Layout, 0, 600), layoutAt(upper.Layout, 0, 600); u.Size.X <= p.Size.X {
		t.Errorf("uppercase %v, plain %v: want the capitals wider", u.Size, p.Size)
	}
}

// labelInk draws l on the page and returns the glyph pixel farthest from it,
// a fully covered one, which is the text's own color.
func labelInk(t *testing.T, l FormLabelStyle) color.RGBA {
	t.Helper()
	ops := tabsFrame(new(input.Router), time.Unix(0, 0), func(gtx C) D {
		paint.Fill(gtx.Ops, Dark.Background)
		return l.Layout(gtx)
	})
	img := tabsRender(t, ops)
	var ink color.RGBA
	far := -1
	for y := 0; y < 30; y++ {
		for x := 0; x < 200; x++ {
			p := img.RGBAAt(x, y)
			if d := int(p.R) - int(Dark.Background.R); d > far {
				far, ink = d, p
			}
		}
	}
	return ink
}

// opacity-50 over the page: foreground (#fafafa) halfway to #09090b.
func TestLabelDisabledIsHalfOpacity(t *testing.T) {
	th := testTheme()
	on, off := th.Label("Subject"), th.Label("Subject")
	off.Disabled = true
	wantOn, wantOff := color.RGBA{R: 250, G: 250, B: 250, A: 255}, color.RGBA{R: 130, G: 130, B: 131, A: 255}
	within := func(a, b color.RGBA) bool {
		d := func(x, y uint8) bool { return int(x)-int(y) <= 2 && int(y)-int(x) <= 2 }
		return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && a.A == b.A
	}
	if got := labelInk(t, on); !within(got, wantOn) {
		t.Errorf("enabled ink %v, want the foreground %v", got, wantOn)
	}
	if got := labelInk(t, off); !within(got, wantOff) {
		t.Errorf("disabled ink %v, want the foreground at half over the page %v", got, wantOff)
	}
}
