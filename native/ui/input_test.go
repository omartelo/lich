package ui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

func inputSize(w layout.Widget, maxX, maxY int) image.Point {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(maxX, maxY)},
	}
	return w(gtx).Size
}

func TestInputGeometry(t *testing.T) {
	th, ed := testTheme(), new(widget.Editor)
	dense := th.Input(ed, "")
	dense.Height = Space(7)
	fixed := th.Input(ed, "")
	fixed.Width = Space(44)
	cases := []struct {
		name string
		w    layout.Widget
		want image.Point
	}{
		{"h-9 w-full", th.Input(ed, "Name").Layout, image.Pt(300, 36)},
		{"h-7", dense.Layout, image.Pt(300, 28)},
		{"w-44", fixed.Layout, image.Pt(176, 36)},
		{"w-44 inside a narrower parent", fixed.Layout, image.Pt(100, 36)},
	}
	for _, tc := range cases {
		maxX := 300
		if strings.Contains(tc.name, "narrower") {
			maxX = 100
		}
		if got := inputSize(tc.w, maxX, 1000); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTextareaGrowsWithItsText(t *testing.T) {
	th, ed := testTheme(), new(widget.Editor)
	ta := th.Textarea(ed, "Prompt")
	if got := inputSize(ta.Layout, 300, 1000); got != image.Pt(300, 64) {
		t.Errorf("empty: got %v, want min-h-16 (300x64)", got)
	}
	ed.SetText(strings.Repeat("line\n", 9))
	grown := inputSize(ta.Layout, 300, 1000).Y
	if grown <= 64 {
		t.Errorf("ten lines: height %d did not grow past min-h-16", grown)
	}
	// max-h-40
	if got := inputSize(ta.Layout, 300, 100).Y; got != 100 {
		t.Errorf("max-h: got %d, want the 100px cap", got)
	}
}

func TestInputGroupKeepsRowHeightAndFillsWidth(t *testing.T) {
	th, ed := testTheme(), new(widget.Editor)
	g := th.InputGroup(ed, "Filter")
	g.End = InputGroupAddon{Items: []layout.Widget{th.InputGroupIcon(icons.Lucide("chevron-down"))}}
	if got := inputSize(g.Layout, 240, 1000); got != image.Pt(240, 36) {
		t.Errorf("got %v, want 240x36", got)
	}
	ta := th.InputGroupTextarea(ed, "Prompt")
	ta.Start = InputGroupAddon{Items: []layout.Widget{th.InputGroupText("To")}}
	if got := inputSize(ta.Layout, 240, 1000); got.X != 240 || got.Y != 64 {
		t.Errorf("textarea group: got %v, want 240 wide and min-h-16", got)
	}
}

// inputRig renders a widget through a router, so focus and typed text travel
// the way they do in the app.
type inputRig struct {
	t      *testing.T
	win    *headless.Window
	router input.Router
	size   image.Point
	margin int
	draw   func(gtx layout.Context) layout.Dimensions
}

const inputRigMargin = 8

func newInputRig(t *testing.T, w, h int, draw func(layout.Context) layout.Dimensions) *inputRig {
	win, err := headless.NewWindow(w+2*inputRigMargin, h+2*inputRigMargin)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	t.Cleanup(win.Release)
	return &inputRig{t: t, win: win, size: image.Pt(w, h), margin: inputRigMargin, draw: draw}
}

// frame lays the widget out once and renders it.
func (r *inputRig) frame() *image.RGBA {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      r.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(r.size),
	}
	defer op.Offset(image.Pt(r.margin, r.margin)).Push(ops).Pop()
	r.draw(gtx)
	r.router.Frame(ops)
	if err := r.win.Frame(ops); err != nil {
		r.t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: r.win.Size()})
	if err := r.win.Screenshot(img); err != nil {
		r.t.Fatal(err)
	}
	return img
}

// ringPixel is a pixel 2px left of the box, inside the 3px ring band.
func (r *inputRig) ringPixel(img *image.RGBA) (rgba [4]uint8) {
	p := img.RGBAAt(r.margin-2, r.margin+r.size.Y/2)
	return [4]uint8{p.R, p.G, p.B, p.A}
}

func TestInputFocusRingDrawsOnlyWhenFocused(t *testing.T) {
	th, ed := testTheme(), new(widget.Editor)
	in := th.Input(ed, "Name")
	rig := newInputRig(t, 160, 36, in.Layout)
	if px := rig.ringPixel(rig.frame()); px[3] != 0 {
		t.Fatalf("unfocused: ring pixel %v, want nothing outside the box", px)
	}
	rig.router.Source().Execute(key.FocusCmd{Tag: ed})
	rig.frame()
	focused := rig.ringPixel(rig.frame())
	if focused[3] == 0 {
		t.Fatalf("focused: no ring outside the box")
	}
	in.Invalid = true
	rig.draw = in.Layout
	invalid := rig.ringPixel(rig.frame())
	if invalid == focused || invalid[0] <= invalid[2] {
		t.Errorf("invalid ring %v: want the reddish destructive, not the focus ring %v", invalid, focused)
	}
}

// The box of an input over the page (#09090b), as the browser composites it:
// bg-input/30 for the fill, border-input over the fill, ring-ring/50 over the
// page. Gio would blend each about twice as bright.
func TestInputBoxPaintsTheBrowsersColors(t *testing.T) {
	th, ed := testTheme(), new(widget.Editor)
	in := th.Input(ed, "Name")
	rig := newInputRig(t, 160, 36, in.Layout)
	img := rig.frame()
	at := func(x, y int) image.Point { return image.Pt(rig.margin+x, rig.margin+y) }
	if want := (color.NRGBA{R: 20, G: 20, B: 22, A: 255}); !toggleIs(img, at(4, 18), want) {
		t.Errorf("fill %v, want input/30 over the page %v", img.RGBAAt(at(4, 18).X, at(4, 18).Y), want)
	}
	if want := (color.NRGBA{R: 55, G: 55, B: 57, A: 255}); !toggleIs(img, at(80, 0), want) {
		t.Errorf("border %v, want input over the fill %v", img.RGBAAt(at(80, 0).X, at(80, 0).Y), want)
	}

	rig.router.Source().Execute(key.FocusCmd{Tag: ed})
	rig.frame()
	img = rig.frame()
	if want := (color.NRGBA{R: 61, G: 61, B: 66, A: 255}); !toggleIs(img, at(-2, 18), want) {
		t.Errorf("focus ring %v, want ring/50 over the page %v", img.RGBAAt(at(-2, 18).X, at(-2, 18).Y), want)
	}
	if want := th.Ring; !toggleIs(img, at(80, 0), want) {
		t.Errorf("focused border %v, want ring %v", img.RGBAAt(at(80, 0).X, at(80, 0).Y), want)
	}
}

// disabled:opacity-50 fades the finished box: the fill (#14141 6) at half over
// the page.
func TestInputDisabledFadesOntoThePage(t *testing.T) {
	th := testTheme()
	in := th.Input(new(widget.Editor), "Name")
	in.Disabled = true
	rig := newInputRig(t, 160, 36, in.Layout)
	p := image.Pt(rig.margin+4, rig.margin+18)
	if img, want := rig.frame(), (color.NRGBA{R: 14, G: 14, B: 16, A: 255}); !toggleIs(img, p, want) {
		t.Errorf("fill %v, want input/30 at half over the page %v", img.RGBAAt(p.X, p.Y), want)
	}
}

// An addon's button hovers over the group's fill, so its theme draws on it.
func TestInputGroupInnerDrawsOnTheFill(t *testing.T) {
	th := testTheme()
	g := th.InputGroup(new(widget.Editor), "Filter")
	if want := (color.NRGBA{R: 20, G: 20, B: 22, A: 255}); !near2(g.Inner().Surface, want) {
		t.Errorf("surface %v, want input/30 over the page %v", g.Inner().Surface, want)
	}
	g.Disabled = true
	if want := (color.NRGBA{R: 14, G: 14, B: 16, A: 255}); !near2(g.Inner().Surface, want) {
		t.Errorf("disabled surface %v, want the fill at half over the page %v", g.Inner().Surface, want)
	}
}

func TestInputTakesTypedTextUnlessDisabled(t *testing.T) {
	th, ed := testTheme(), new(widget.Editor)
	in := th.Input(ed, "Name")
	rig := newInputRig(t, 160, 36, func(gtx layout.Context) layout.Dimensions { return in.Layout(gtx) })
	rig.frame()
	rig.router.Source().Execute(key.FocusCmd{Tag: ed})
	rig.frame()
	rig.router.Queue(key.EditEvent{Text: "skipo"})
	rig.frame()
	if got := ed.Text(); got != "skipo" {
		t.Errorf("typed text: got %q, want %q", got, "skipo")
	}

	off, edOff := th.Input(new(widget.Editor), "Name"), new(widget.Editor)
	off.Editor, off.Disabled = edOff, true
	rig = newInputRig(t, 160, 36, off.Layout)
	rig.frame()
	rig.router.Source().Execute(key.FocusCmd{Tag: edOff})
	rig.frame()
	rig.router.Queue(key.EditEvent{Text: "skipo"})
	img := rig.frame()
	if edOff.Text() != "" {
		t.Errorf("disabled: got %q, want it untouched", edOff.Text())
	}
	if px := rig.ringPixel(img); px[3] != 0 {
		t.Errorf("disabled: ring pixel %v, want none", px)
	}
}

// The text line of an h-9 input is half its height: a press above it, in the
// padding, still has to focus the field.
func TestPressInPaddingFocusesTheEditor(t *testing.T) {
	th, ed := testTheme(), new(widget.Editor)
	in := th.Input(ed, "Name")
	rig := newInputRig(t, 160, 36, in.Layout)
	rig.frame()
	at := image.Pt(rig.margin+40, rig.margin+2)
	rig.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: inputPointF32(at)})
	rig.frame()
	rig.frame()
	if px := rig.ringPixel(rig.frame()); px[3] == 0 {
		t.Error("press in the top padding left the field unfocused")
	}
}

func inputPointF32(p image.Point) f32.Point { return f32.Pt(float32(p.X), float32(p.Y)) }
