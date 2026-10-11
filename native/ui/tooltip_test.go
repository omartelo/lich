package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// tooltipRig draws one invisible trigger inside a Root, so whatever
// ink a frame holds is the bubble.
type tooltipRig struct {
	t         *testing.T
	th        *Theme
	win       *headless.Window
	size      image.Point
	router    input.Router
	layer     Root
	state     Tooltip
	label     string
	side      Side
	triggerAt image.Point
	now       time.Time
}

var tooltipTestTrigger = image.Pt(40, 20)

func newTooltipRig(t *testing.T, label string, triggerAt image.Point) *tooltipRig {
	size := image.Pt(300, 200)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	t.Cleanup(win.Release)
	r := &tooltipRig{t: t, th: testTheme(), win: win, size: size, label: label, triggerAt: triggerAt, now: time.Unix(1000, 0)}
	r.frame()
	return r
}

func (r *tooltipRig) frame() *image.RGBA {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Now:         r.now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(r.size),
		Source:      r.router.Source(),
	}
	r.layer.Layout(gtx, func(gtx C) D {
		defer op.Offset(r.triggerAt).Push(gtx.Ops).Pop()
		gtx.Constraints = layout.Exact(tooltipTestTrigger)
		tip := r.th.Tooltip(&r.layer, &r.state, r.label)
		tip.Side = r.side
		tip.Layout(gtx, func(gtx C) D { return D{Size: gtx.Constraints.Min} })
		return D{Size: r.size}
	})
	r.router.Frame(ops)
	if err := r.win.Frame(ops); err != nil {
		r.t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: r.size})
	if err := r.win.Screenshot(img); err != nil {
		r.t.Fatal(err)
	}
	return img
}

func (r *tooltipRig) pointer(kind pointer.Kind, at image.Point) {
	r.router.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: f32.Pt(float32(at.X), float32(at.Y)), Buttons: tooltipButtons(kind)})
}

func tooltipButtons(kind pointer.Kind) pointer.Buttons {
	if kind == pointer.Press {
		return pointer.ButtonPrimary
	}
	return 0
}

func (r *tooltipRig) hoverTrigger() {
	r.pointer(pointer.Move, r.triggerAt.Add(tooltipTestTrigger.Div(2)))
	r.frame()
}

// openFully hovers past the delay and through the enter animation.
func (r *tooltipRig) openFully() *image.RGBA {
	r.hoverTrigger()
	r.after(tooltipOpenDelay)
	return r.after(tooltipDuration)
}

func (r *tooltipRig) after(d time.Duration) *image.RGBA {
	r.now = r.now.Add(d)
	return r.frame()
}

// tooltipInk is the bounding box of the opaque pixels of img.
func tooltipInk(img *image.RGBA) image.Rectangle {
	var ink image.Rectangle
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if img.RGBAAt(x, y).A > 128 {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return ink
}

func TestTooltipOpensAfterTheDelay(t *testing.T) {
	r := newTooltipRig(t, "Settings", image.Pt(130, 100))
	r.hoverTrigger()
	if ink := tooltipInk(r.after(tooltipOpenDelay - time.Millisecond)); !ink.Empty() {
		t.Fatalf("drawn at %v before the open delay", ink)
	}
	r.after(time.Millisecond)
	ink := tooltipInk(r.after(tooltipDuration))
	if ink.Empty() {
		t.Fatal("nothing drawn after the open delay and the enter animation")
	}
	if ink.Max.Y > r.triggerAt.Y || ink.Min.Y >= r.triggerAt.Y-4 {
		t.Errorf("bubble at %v, want it above the trigger at y=%d with its arrow reaching down to it", ink, r.triggerAt.Y)
	}
	if center := (ink.Min.X + ink.Max.X) / 2; abs(center-(r.triggerAt.X+tooltipTestTrigger.X/2)) > 1 {
		t.Errorf("bubble centered at x=%d, want the trigger's center", center)
	}
}

func TestTooltipFlipsBelowAtTheWindowTop(t *testing.T) {
	r := newTooltipRig(t, "Settings", image.Pt(130, 2))
	ink := tooltipInk(r.openFully())
	if ink.Empty() || ink.Min.Y < r.triggerAt.Y+tooltipTestTrigger.Y-5 {
		t.Errorf("bubble at %v, want it flipped below the trigger", ink)
	}
}

func TestTooltipFlipsAcrossOnTheSide(t *testing.T) {
	r := newTooltipRig(t, "Settings", image.Pt(258, 90))
	r.side = SideRight
	ink := tooltipInk(r.openFully())
	if ink.Empty() || ink.Max.X > r.triggerAt.X+tooltipTestTrigger.X/2 {
		t.Errorf("bubble at %v, want it flipped to the trigger's left", ink)
	}
}

// Near the left edge the bubble slides in to the collision padding, while
// the arrow still points at the trigger's center.
func TestTooltipShiftsInsideTheWindowAndAimsTheArrow(t *testing.T) {
	r := newTooltipRig(t, "Open the pull requests of this project", image.Pt(0, 100))
	img := r.openFully()
	ink := tooltipInk(img)
	if ink.Min.X != int(collisionPadding) {
		t.Errorf("bubble starts at x=%d, want the collision padding %v", ink.Min.X, collisionPadding)
	}
	tipRow := r.triggerAt.Y - 2
	center := tooltipTestTrigger.X / 2
	if img.RGBAAt(center, tipRow).A == 0 {
		t.Errorf("no arrow above the trigger's center at (%d,%d)", center, tipRow)
	}
	if far := ink.Max.X - 20; img.RGBAAt(far, tipRow).A != 0 {
		t.Errorf("ink at (%d,%d), want the arrow only at the trigger's center", far, tipRow)
	}
}

func TestTooltipClosesOnPressAndStaysShut(t *testing.T) {
	r := newTooltipRig(t, "Settings", image.Pt(130, 100))
	r.openFully()
	r.pointer(pointer.Press, r.triggerAt.Add(tooltipTestTrigger.Div(2)))
	r.frame()
	if ink := tooltipInk(r.after(tooltipDuration + tooltipOpenDelay)); !ink.Empty() {
		t.Errorf("drawn at %v after a press, want it shut until the pointer leaves", ink)
	}
}

func TestTooltipClosesOnLeave(t *testing.T) {
	r := newTooltipRig(t, "Settings", image.Pt(130, 100))
	r.openFully()
	r.pointer(pointer.Move, image.Pt(10, 10))
	r.frame()
	if ink := tooltipInk(r.after(tooltipDuration)); !ink.Empty() {
		t.Errorf("drawn at %v after the exit animation", ink)
	}
}

func TestTooltipWithoutLabelDrawsNothing(t *testing.T) {
	r := newTooltipRig(t, "", image.Pt(130, 100))
	if ink := tooltipInk(r.openFully()); !ink.Empty() {
		t.Errorf("drawn at %v, want an empty label to leave the trigger bare", ink)
	}
}

func abs(n int) int { return max(n, -n) }
