package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

var tabsTestSize = image.Pt(400, 200)

// tabsFrame lays w out through r, as one frame of an app, and returns the ops.
func tabsFrame(r *input.Router, now time.Time, w layout.Widget) *op.Ops {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops: ops, Now: now, Source: r.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: tabsTestSize},
	}
	w(gtx)
	r.Frame(ops)
	return ops
}

// tabsRender draws ops in a headless window; it skips where there is no GPU.
func tabsRender(t *testing.T, ops *op.Ops) *image.RGBA {
	t.Helper()
	win, err := headless.NewWindow(tabsTestSize.X, tabsTestSize.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: tabsTestSize})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img
}

func tabsNames(names ...string) []TabsTrigger {
	triggers := make([]TabsTrigger, len(names))
	for i, n := range names {
		triggers[i].Text = n
	}
	return triggers
}

func TestTabsListGeometry(t *testing.T) {
	th := testTheme()
	cases := []struct {
		name    string
		variant TabsVariant
		gaps    int
	}{
		{"default", TabsVariantDefault, 0},
		{"line", TabsVariantLine, 2 * 4}, // gap-1 between three triggers
	}
	for _, tc := range cases {
		s := th.Tabs(new(TabsState), tabsNames("Account", "Password", "Billing")...)
		s.Variant = tc.variant
		d := layoutAt(func(gtx C) D { return s.Layout(gtx) }, 0, 600)
		// h-9, and p-[0.1875rem] around three equally wide triggers.
		if d.Size.Y != 36 || (d.Size.X-6-tc.gaps)%3 != 0 {
			t.Errorf("%s: got %v, want height 36 and three equal triggers inside 3px padding", tc.name, d.Size)
		}
	}
}

func TestTabsTriggersShareTheWidestWidth(t *testing.T) {
	th := testTheme()
	size := func(names ...string) image.Point {
		s := th.Tabs(new(TabsState), tabsNames(names...)...)
		return layoutAt(func(gtx C) D { return s.Layout(gtx) }, 0, 2000).Size
	}
	if got, want := size("a", "a much longer trigger"), size("a much longer trigger", "a much longer trigger"); got != want {
		t.Errorf("flex-1 triggers: got %v, want the widest one's width twice (%v)", got, want)
	}
}

func TestTabsVerticalStacksTriggers(t *testing.T) {
	th := testTheme()
	s := th.Tabs(new(TabsState), tabsNames("Account", "Password", "Billing")...)
	s.Orientation = TabsVertical
	d := layoutAt(func(gtx C) D { return s.Layout(gtx) }, 0, 600)
	h := (d.Size.Y - 6) / 3
	// h-fit: py-1 and the border around a 20px line.
	if (d.Size.Y-6)%3 != 0 || h < 30 {
		t.Errorf("got %v, want three equal rows of at least 30px inside 3px padding", d.Size)
	}
}

func TestTabsGhostHugsItsContent(t *testing.T) {
	s := testTheme().Tabs(new(TabsState), tabsNames("Code", "Review")...)
	s.Variant = TabsVariantGhost
	d := layoutAt(func(gtx C) D { return s.Layout(gtx) }, 0, 600)
	if d.Size.Y >= 36 || d.Size.Y < 26 {
		t.Errorf("h-auto list: got height %d, want text-xs line + py-0.5 + border + p-0.5 (26..35)", d.Size.Y)
	}
}

func TestTabsClickSelectsAndReportsOnce(t *testing.T) {
	th, state := testTheme(), new(TabsState)
	var r input.Router
	w := func(gtx C) D { return th.Tabs(state, tabsNames("One", "Two", "Three")...).Layout(gtx) }
	var list image.Point
	tabsFrame(&r, time.Unix(0, 0), func(gtx C) D { list = w(gtx).Size; return D{} })
	trigger := (list.X - 6) / 3
	at := f32.Pt(float32(3+trigger+trigger/2), float32(list.Y/2))
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse},
	)
	tabsFrame(&r, time.Unix(0, 0), func(gtx C) D { return w(gtx) })
	if state.Selected != 1 {
		t.Fatalf("selected %d, want the clicked second trigger", state.Selected)
	}
	if !state.Changed() || state.Changed() {
		t.Error("Changed must report the pick once")
	}
}

func TestTabsDisabledTriggerIgnoresClicks(t *testing.T) {
	th, state := testTheme(), new(TabsState)
	var r input.Router
	triggers := tabsNames("One", "Two")
	triggers[1].Disabled = true
	w := func(gtx C) D { return th.Tabs(state, triggers...).Layout(gtx) }
	var list image.Point
	tabsFrame(&r, time.Unix(0, 0), func(gtx C) D { list = w(gtx).Size; return D{} })
	trigger := (list.X - 6) / 2
	at := f32.Pt(float32(3+trigger+trigger/2), float32(list.Y/2))
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse},
	)
	tabsFrame(&r, time.Unix(0, 0), func(gtx C) D { return w(gtx) })
	if state.Selected != 0 || state.Changed() {
		t.Errorf("selected %d, want the disabled trigger left alone", state.Selected)
	}
}

func TestTabsOnlySelectedPanelIsLaidOut(t *testing.T) {
	th := testTheme()
	state := &TabsState{Selected: 1}
	var laid [3]int
	panel := func(i int) layout.Widget {
		return func(gtx C) D { laid[i]++; return D{} }
	}
	s := th.Tabs(state, tabsNames("One", "Two", "Three")...)
	layoutAt(func(gtx C) D { return s.Layout(gtx, panel(0), panel(1), panel(2)) }, 0, 600)
	if laid != [3]int{0, 1, 0} {
		t.Errorf("panels laid out %v, want only the second", laid)
	}
}

func TestTabsNoPanelForSelectionDrawsListOnly(t *testing.T) {
	s := testTheme().Tabs(&TabsState{Selected: 2}, tabsNames("One", "Two", "Three")...)
	d := layoutAt(func(gtx C) D { return s.Layout(gtx, func(gtx C) D { return D{Size: image.Pt(1, 1000)} }) }, 0, 600)
	if d.Size.Y != 36 {
		t.Errorf("got height %d, want the bare list (36)", d.Size.Y)
	}
}

func tabsPixel(img *image.RGBA, x, y int) color.RGBA { return img.RGBAAt(x, y) }

func TestTabsLineIndicatorSitsUnderTheActiveTrigger(t *testing.T) {
	th := testTheme()
	s := th.Tabs(&TabsState{Selected: 1}, tabsNames("One", "Two", "Three")...)
	s.Variant = TabsVariantLine
	var list image.Point
	ops := tabsFrame(new(input.Router), time.Unix(0, 0), func(gtx C) D { list = s.Layout(gtx).Size; return D{} })
	img := tabsRender(t, ops)
	trigger := (list.X - 6 - 2*4) / 3
	row := list.Y - 1
	active := tabsPixel(img, 3+trigger+4+trigger/2, row)
	inactive := tabsPixel(img, 3+trigger/2, row)
	if active.A != 255 || active.R < 200 {
		t.Errorf("under the active trigger: %v, want the foreground bar", active)
	}
	if inactive.A != 0 {
		t.Errorf("under an inactive trigger: %v, want nothing", inactive)
	}
}

func TestTabsDefaultActiveTriggerIsFilled(t *testing.T) {
	th := testTheme()
	s := th.Tabs(&TabsState{Selected: 0}, tabsNames("One", "Two")...)
	var list image.Point
	ops := tabsFrame(new(input.Router), time.Unix(0, 0), func(gtx C) D { list = s.Layout(gtx).Size; return D{} })
	img := tabsRender(t, ops)
	trigger := (list.X - 6) / 2
	y := list.Y / 2
	active, inactive := tabsPixel(img, 3+4, y), tabsPixel(img, 3+trigger+4, y)
	// dark:data-active:bg-input/30 over the list's bg-muted (#27272a), the list
	// itself where no trigger is active.
	if want := (color.NRGBA{R: 49, G: 49, B: 52, A: 255}); !toggleIs(img, image.Pt(3+4, y), want) {
		t.Errorf("active %v, want input/30 over the list %v", active, want)
	}
	if !toggleIs(img, image.Pt(3+trigger+4, y), th.Muted) {
		t.Errorf("inactive %v, want the list's muted %v", inactive, th.Muted)
	}
	// dark:data-active:border-input lands on that fill.
	if want := (color.NRGBA{R: 80, G: 80, B: 83, A: 255}); !toggleIs(img, image.Pt(3, list.Y/2), want) {
		t.Errorf("border %v, want input over the active fill %v", tabsPixel(img, 3, list.Y/2), want)
	}
}

func TestTabsFocusRingAppearsAroundFocusedTrigger(t *testing.T) {
	th := testTheme()
	var r input.Router
	s := th.Tabs(&TabsState{Selected: 0}, tabsNames("One", "Two")...)
	w := func(gtx C) D { return s.Layout(gtx) }
	tabsFrame(&r, time.Unix(0, 0), w)
	// The list's padding is where the ring lands, on the left of the first trigger.
	before := tabsPixel(tabsRender(t, tabsFrame(&r, time.Unix(0, 0), w)), 1, 18)
	r.MoveFocus(key.FocusForward)
	tabsFrame(&r, time.Unix(0, 0), w)
	after := tabsPixel(tabsRender(t, tabsFrame(&r, time.Unix(0, 0), w)), 1, 18)
	if before == after {
		t.Errorf("pixel left of the first trigger stayed %v, want the focus ring", after)
	}
}
