package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

// toggleRig lays a widget out through an input.Router, so a pointer event
// queued on it reaches the widget the way a window's would.
type toggleRig struct {
	router input.Router
	ops    op.Ops
	at     image.Point // where the widget sits in the window
	now    time.Time
	widget layout.Widget
}

func newToggleRig(at image.Point, w layout.Widget) *toggleRig {
	r := &toggleRig{at: at, widget: w, now: time.Unix(1000, 0)}
	r.frame()
	return r
}

func (r *toggleRig) frame() D {
	r.ops.Reset()
	gtx := layout.Context{
		Ops:         &r.ops,
		Source:      r.router.Source(),
		Now:         r.now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(400, 200)},
	}
	stack := op.Offset(r.at).Push(gtx.Ops)
	d := r.widget(gtx)
	stack.Pop()
	r.router.Frame(&r.ops)
	return d
}

// click presses and releases the primary button at p, relative to the
// widget's origin, then lays the widget out again so it sees the click.
func (r *toggleRig) click(p image.Point) {
	pos := f32.Pt(float32(r.at.X+p.X), float32(r.at.Y+p.Y))
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos},
	)
	r.frame()
}

// toggleRender draws w once at now into a transparent window of size and
// returns the pixels.
func toggleRender(t *testing.T, size image.Point, now time.Time, w layout.Widget) *image.RGBA {
	t.Helper()
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	ops := new(op.Ops)
	w(layout.Context{
		Ops:         ops,
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: size},
	})
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img
}

// toggleIs reports whether the pixel at p is want, drawn opaque or at the
// alpha want carries, to within one rounding step per channel.
func toggleIs(img *image.RGBA, p image.Point, want color.NRGBA) bool {
	got := img.RGBAAt(p.X, p.Y)
	wantPre := color.RGBAModel.Convert(want).(color.RGBA)
	d := func(a, b uint8) bool { return int(a)-int(b) <= 2 && int(b)-int(a) <= 2 }
	return d(got.R, wantPre.R) && d(got.G, wantPre.G) && d(got.B, wantPre.B) && d(got.A, wantPre.A)
}

func TestToggleSizes(t *testing.T) {
	th := testTheme()
	cases := []struct {
		size ToggleSize
		h    int
	}{{ToggleSizeDefault, 36}, {ToggleSizeSM, 32}, {ToggleSizeLG, 40}}
	for _, tc := range cases {
		tg := th.Toggle(new(widget.Bool), "Bold")
		tg.Size = tc.size
		if d := layoutAt(tg.Layout, 0, 500); d.Size.Y != tc.h || d.Size.X <= tc.h {
			t.Errorf("size %d: got %v, want height %d at natural width", tc.size, d.Size, tc.h)
		}
	}
}

// min-w-9: a toggle of one narrow glyph or an icon alone is still a square.
func TestToggleIsNoNarrowerThanItsHeight(t *testing.T) {
	th := testTheme()
	icon := th.Toggle(new(widget.Bool), "")
	icon.Icon = icons.Lucide("bold")
	if d := layoutAt(icon.Layout, 0, 500); d.Size != image.Pt(36, 36) {
		t.Errorf("icon only: got %v, want 36x36", d.Size)
	}
	narrow := th.Toggle(new(widget.Bool), "i")
	narrow.Size = ToggleSizeLG
	if d := layoutAt(narrow.Layout, 0, 500); d.Size.X != 40 {
		t.Errorf("one glyph: got width %d, want the 40 of min-w-10", d.Size.X)
	}
}

func TestToggleClickFlipsPressed(t *testing.T) {
	th, pressed := testTheme(), new(widget.Bool)
	rig := newToggleRig(image.Pt(20, 20), th.Toggle(pressed, "Bold").Layout)
	rig.click(image.Pt(10, 10))
	if !pressed.Value {
		t.Fatal("click did not press the toggle")
	}
	rig.click(image.Pt(10, 10))
	if pressed.Value {
		t.Error("second click did not release it")
	}
}

func TestToggleDisabledIgnoresClicks(t *testing.T) {
	th, pressed := testTheme(), new(widget.Bool)
	tg := th.Toggle(pressed, "Bold")
	tg.Disabled = true
	rig := newToggleRig(image.Pt(20, 20), tg.Layout)
	rig.click(image.Pt(10, 10))
	if pressed.Value {
		t.Error("a disabled toggle was pressed")
	}
}

func TestTogglePressedIsAccentFill(t *testing.T) {
	th := testTheme()
	draw := func(pressed bool, mod func(*ToggleStyle)) *image.RGBA {
		state := &widget.Bool{Value: pressed}
		tg := th.Toggle(state, "Bold")
		if mod != nil {
			mod(&tg)
		}
		return toggleRender(t, image.Pt(80, 36), time.Time{}, tg.Layout)
	}
	if img := draw(true, nil); !toggleIs(img, image.Pt(3, 18), th.Accent) {
		t.Errorf("pressed: pixel %v, want accent %v", img.RGBAAt(3, 18), th.Accent)
	}
	if img := draw(false, nil); img.RGBAAt(3, 18).A != 0 {
		t.Errorf("unpressed: pixel %v, want transparent", img.RGBAAt(3, 18))
	}
	// opacity-50 over the page: accent (#27272a) halfway to #09090b.
	disabled := draw(true, func(tg *ToggleStyle) { tg.Disabled = true })
	if want := (color.NRGBA{R: 24, G: 24, B: 27, A: 255}); !toggleIs(disabled, image.Pt(3, 18), want) {
		t.Errorf("disabled pressed: pixel %v, want accent at half over the page %v", disabled.RGBAAt(3, 18), want)
	}
}

func TestToggleOutlineHasBorderDefaultHasNone(t *testing.T) {
	th := testTheme()
	edge := func(v ToggleVariant) color.RGBA {
		tg := th.Toggle(new(widget.Bool), "Bold")
		tg.Variant = v
		return toggleRender(t, image.Pt(80, 36), time.Time{}, tg.Layout).RGBAAt(40, 0)
	}
	// border-input is white at 15% over the page.
	if p, want := edge(ToggleVariantOutline), (color.NRGBA{R: 45, G: 45, B: 47, A: 255}); !near2(color.NRGBA(p), want) {
		t.Errorf("outline: top edge %v, want input over the page %v", p, want)
	}
	if p := edge(ToggleVariantDefault); p.A != 0 {
		t.Errorf("default: top edge %v, want none", p)
	}
}

// A text-xs item carries text-xs's 16px line, whatever size it was built from.
func TestToggleTextSizeBringsItsLineHeight(t *testing.T) {
	th := testTheme()
	content := func(gtx C) D {
		return th.toggleContent(gtx, toggleSpec{text: "Diff", textSize: TextXS}, th.Foreground)
	}
	if d := layoutAt(content, 0, 500); d.Size.Y != 16 {
		t.Errorf("got line height %d, want text-xs's 16", d.Size.Y)
	}
}
