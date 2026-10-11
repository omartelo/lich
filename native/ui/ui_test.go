package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

// The zinc ramp Tailwind publishes as hex for the oklch values index.css uses.
func TestOKLCHMatchesTailwindHex(t *testing.T) {
	cases := []struct {
		l, c, h float64
		want    color.NRGBA
	}{
		{0.141, 0.005, 285.823, color.NRGBA{0x09, 0x09, 0x0b, 0xff}},
		{0.21, 0.006, 285.885, color.NRGBA{0x18, 0x18, 0x1b, 0xff}},
		{0.274, 0.006, 286.033, color.NRGBA{0x27, 0x27, 0x2a, 0xff}},
		{0.705, 0.015, 286.067, color.NRGBA{0x9f, 0x9f, 0xa9, 0xff}},
		{0.985, 0, 0, color.NRGBA{0xfa, 0xfa, 0xfa, 0xff}},
	}
	for _, tc := range cases {
		got := OKLCH(tc.l, tc.c, tc.h, 1)
		if !near(got, tc.want) {
			t.Errorf("oklch(%v %v %v) = %v, want %v", tc.l, tc.c, tc.h, got, tc.want)
		}
	}
}

// near allows the one-step rounding gap between Tailwind's published hex and
// an exact conversion.
func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) bool { return int(x)-int(y) <= 1 && int(y)-int(x) <= 1 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && a.A == b.A
}

func testTheme() *Theme {
	return &Theme{Palette: Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: Dark.Background}
}

func layoutAt(w layout.Widget, minX, maxX int) layout.Dimensions {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Min: image.Pt(minX, 0), Max: image.Pt(maxX, 1000)},
	}
	return w(gtx)
}

func TestButtonSizes(t *testing.T) {
	th, click, ic := testTheme(), new(widget.Clickable), icons.Lucide("plus")
	cases := []struct {
		size Size
		h    int
	}{{SizeDefault, 36}, {SizeXS, 24}, {SizeSM, 32}, {SizeLG, 40}}
	for _, tc := range cases {
		b := th.Button(click, "Shell")
		b.Size = tc.size
		if d := layoutAt(b.Layout, 0, 500); d.Size.Y != tc.h || d.Size.X >= 500 {
			t.Errorf("size %d: got %v, want height %d at natural width", tc.size, d.Size, tc.h)
		}
	}
	b := th.Button(click, "")
	b.Icon, b.Size = ic, SizeIconSM
	if d := layoutAt(b.Layout, 0, 500); d.Size != image.Pt(32, 32) {
		t.Errorf("icon-sm: got %v, want 32x32", d.Size)
	}
}

func TestButtonFillsMinimumWidth(t *testing.T) {
	b := testTheme().Button(new(widget.Clickable), "Shell")
	if d := layoutAt(b.Layout, 200, 200); d.Size.X != 200 {
		t.Errorf("w-full: got width %d, want 200", d.Size.X)
	}
}

// w-full inside a flex gives a minimum below the maximum: the padding is part
// of that width, not on top of it.
func TestButtonMinimumWidthBelowMaximumIsTheWidth(t *testing.T) {
	b := testTheme().Button(new(widget.Clickable), "Shell")
	if d := layoutAt(b.Layout, 200, 500); d.Size.X != 200 {
		t.Errorf("got width %d under a 200 minimum, want 200", d.Size.X)
	}
}

// What the browser paints for each translucent class, over the page (#09090b),
// composited in sRGB; Gio would blend each about twice as bright.
func TestButtonVariantFillsAreTheBrowsersColors(t *testing.T) {
	th := testTheme()
	cases := []struct {
		name     string
		variant  Variant
		hovered  bool
		disabled bool
		want     color.NRGBA
	}{
		{"outline dark:bg-input/30", VariantOutline, false, false, color.NRGBA{R: 20, G: 20, B: 22, A: 255}},
		{"outline dark:hover:bg-input/50", VariantOutline, true, false, color.NRGBA{R: 27, G: 27, B: 29, A: 255}},
		{"ghost dark:hover:bg-muted/50", VariantGhost, true, false, color.NRGBA{R: 24, G: 24, B: 27, A: 255}},
		{"destructive dark:bg-destructive/20", VariantDestructive, false, false, color.NRGBA{R: 58, G: 27, B: 29, A: 255}},
		{"default hover:bg-primary/80", VariantDefault, true, false, color.NRGBA{R: 184, G: 184, B: 187, A: 255}},
		{"default disabled:opacity-50", VariantDefault, false, true, color.NRGBA{R: 119, G: 119, B: 121, A: 255}},
	}
	for _, tc := range cases {
		b := th.Button(new(widget.Clickable), "Shell")
		b.Variant, b.Disabled = tc.variant, tc.disabled
		img := toggleRender(t, image.Pt(80, 36), time.Time{}, func(gtx C) D { return b.draw(gtx, tc.hovered) })
		if !toggleIs(img, image.Pt(4, 18), tc.want) {
			t.Errorf("%s: pixel %v, want %v", tc.name, img.RGBAAt(4, 18), tc.want)
		}
	}
}

// border-input is on the page, not on the fill: bg-clip-padding stops the fill
// short of it. A whole pixel row, not half of two.
func TestButtonOutlineBorderIsInputOverThePage(t *testing.T) {
	b := testTheme().Button(new(widget.Clickable), "Shell")
	b.Variant = VariantOutline
	img := toggleRender(t, image.Pt(80, 36), time.Time{}, func(gtx C) D { return b.draw(gtx, false) })
	want := color.NRGBA{R: 45, G: 45, B: 47, A: 255}
	if !toggleIs(img, image.Pt(20, 0), want) {
		t.Errorf("top row %v, want input over the page %v", img.RGBAAt(20, 0), want)
	}
	if got := img.RGBAAt(20, 35); !near2(color.NRGBA(got), want) {
		t.Errorf("bottom row %v, want input over the page %v", got, want)
	}
}

func TestSeparatorIsBorderOverThePage(t *testing.T) {
	th := testTheme()
	img := toggleRender(t, image.Pt(40, 4), time.Time{}, th.Separator)
	if want := (color.NRGBA{R: 33, G: 33, B: 35, A: 255}); !toggleIs(img, image.Pt(20, 0), want) {
		t.Errorf("pixel %v, want border (white at 10%%) over the page %v", img.RGBAAt(20, 0), want)
	}
}

// index.css's thumb is currentColor at 25%, 40% hovered.
func TestScrollAreaThumbIsForegroundOverThePage(t *testing.T) {
	list := testTheme().ScrollArea(new(widget.List))
	if want := (color.NRGBA{R: 69, G: 69, B: 71, A: 255}); !near2(list.Indicator.Color, want) {
		t.Errorf("thumb %v, want foreground at 25%% over the page %v", list.Indicator.Color, want)
	}
	if want := (color.NRGBA{R: 105, G: 105, B: 107, A: 255}); !near2(list.Indicator.HoverColor, want) {
		t.Errorf("hovered thumb %v, want foreground at 40%% over the page %v", list.Indicator.HoverColor, want)
	}
}

func near2(a, b color.NRGBA) bool {
	d := func(x, y uint8) bool { return int(x)-int(y) <= 2 && int(y)-int(x) <= 2 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && a.A == b.A
}

func TestLabelTruncatesToOneLine(t *testing.T) {
	th := testTheme()
	long := th.Text(TextSM, "a session label far too long for the card it sits in")
	one := layoutAt(th.Text(TextSM, "a").Layout, 0, 60)
	if d := layoutAt(long.Layout, 0, 60); d.Size.Y != one.Size.Y || d.Size.X > 60 {
		t.Errorf("got %v, want one line (height %d) within 60px", d.Size, one.Size.Y)
	}
}

func TestLayoutTailClipsToWidthOnOneLine(t *testing.T) {
	th := testTheme()
	short := th.Text(TextXS, "~/try")
	natural := layoutAt(short.Layout, 0, 500)
	if d := layoutAt(short.LayoutTail, 0, 500); d.Size != natural.Size {
		t.Errorf("short path: got %v, want its natural %v", d.Size, natural.Size)
	}
	long := th.Text(TextXS, "~/.local/share/lich/worktrees/e48f04f46a4a/quiet-tundra")
	if d := layoutAt(long.LayoutTail, 0, 80); d.Size.X != 80 || d.Size.Y != natural.Size.Y {
		t.Errorf("long path: got %v, want 80 wide and one line (%d)", d.Size, natural.Size.Y)
	}
}

// A card row hands its children Min.X = Max.X: the "…" must not take the whole
// row and leave the tail no room.
func TestLayoutTailDrawsTailUnderExactConstraints(t *testing.T) {
	th := testTheme()
	const w, h = 120, 20
	win, err := headless.NewWindow(w, h)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(w, h))}
	th.Text(TextXS, "~/.local/share/lich/worktrees/e48f04f46a4a/quiet-tundra").LayoutTail(gtx)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	ellipsis := layoutAt(th.Text(TextXS, "…").Layout, 0, w).Size.X
	inked := 0
	for y := 0; y < h; y++ {
		for x := ellipsis + 2; x < w; x++ {
			if img.RGBAAt(x, y).A > 0 {
				inked++
			}
		}
	}
	if inked == 0 {
		t.Error("nothing drawn right of the ellipsis, want the path's tail")
	}
}

func TestMixOKLab(t *testing.T) {
	red, blue := OKLCH(0.637, 0.237, 25.331, 1), OKLCH(0.623, 0.214, 259.815, 1)
	if got := MixOKLab(red, blue, 0); !near(got, red) {
		t.Errorf("weight 0: got %v, want %v", got, red)
	}
	if got := MixOKLab(red, blue, 1); !near(got, blue) {
		t.Errorf("weight 1: got %v, want %v", got, blue)
	}
	// Toward transparent the color keeps its hue and only fades.
	if got := MixOKLab(red, color.NRGBA{}, 0.9); !near(color.NRGBA{R: got.R, G: got.G, B: got.B, A: 255}, red) || got.A < 25 || got.A > 26 {
		t.Errorf("red 10%% over transparent: got %v, want red at alpha 25-26 (10%% of 255)", got)
	}
}

func TestOverCompositesInSRGB(t *testing.T) {
	got := Over(color.NRGBA{R: 0x18, G: 0x18, B: 0x1b, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 26})
	if want := (color.NRGBA{R: 0x2f, G: 0x2f, B: 0x31, A: 255}); !near(got, want) {
		t.Errorf("white at 10%% over zinc-900: got %v, want the browser's %v", got, want)
	}
}

// The line box is CSS's: text-sm is 20px tall a line, text-xs 16px, and a
// wrapped label is that times its lines.
func TestLabelLineHeightIsTheCSSLineBox(t *testing.T) {
	th := testTheme()
	if d := layoutAt(th.Text(TextSM, "Shell 1").Layout, 0, 500); d.Size.Y != 20 {
		t.Errorf("text-sm: got height %d, want 20", d.Size.Y)
	}
	if d := layoutAt(th.Text(TextXS, "main").Layout, 0, 500); d.Size.Y != 16 {
		t.Errorf("text-xs: got height %d, want 16", d.Size.Y)
	}
	wrapped := th.Text(TextSM, "one two three four five six seven")
	wrapped.MaxLines = 0
	if d := layoutAt(wrapped.Layout, 0, 60); d.Size.Y%20 != 0 || d.Size.Y < 40 {
		t.Errorf("wrapped text-sm: got height %d, want a multiple of 20 over several lines", d.Size.Y)
	}
}

func TestThemeOverCompositesOntoItsSurface(t *testing.T) {
	th := testTheme().On(Dark.Sidebar)
	if got, want := th.Over(Alpha(Dark.Accent, 0.6)), Over(Dark.Sidebar, Alpha(Dark.Accent, 0.6)); got != want {
		t.Errorf("accent/60 on the sidebar: got %v, want %v", got, want)
	}
	defer func() {
		if recover() == nil {
			t.Error("Over on a theme with no surface did not panic")
		}
	}()
	(&Theme{Palette: Dark}).Over(Dark.Border)
}
