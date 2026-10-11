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
	"gioui.org/widget"
)

var dialogTestWindow = image.Pt(800, 600)

// dialogRig is a window holding a full-size clickable under a confirm dialog.
type dialogRig struct {
	router  input.Router
	ops     op.Ops
	now     time.Time
	th      *Theme
	state   DialogState
	under   widget.Clickable
	confirm widget.Clickable
	open    bool
}

// frame lays the window out once, dialog first as Layout asks, and reports
// whether the dialog was dismissed and the content under it clicked.
func (r *dialogRig) frame() (dismissed, underClicked bool) {
	r.ops.Reset()
	gtx := layout.Context{
		Ops:         &r.ops,
		Now:         r.now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(dialogTestWindow),
		Source:      r.router.Source(),
	}
	d := r.th.Dialog(&r.state, r.open, "Discard changes?", "The worktree keeps uncommitted work.")
	d.Actions = []layout.Widget{r.th.Button(&r.confirm, "Discard").Layout}
	dismissed = d.Layout(gtx)
	underClicked = r.under.Clicked(gtx)
	r.under.Layout(gtx, func(gtx C) D { return Fill(gtx, color.NRGBA{B: 0xff, A: 0xff}, 0) })
	r.router.Frame(&r.ops)
	return dismissed, underClicked
}

// settle opens the dialog and runs its transition to the end.
func (r *dialogRig) settle() {
	r.open = true
	r.frame()
	r.now = r.now.Add(dialogTransition)
	r.frame()
}

func (r *dialogRig) click(at image.Point) {
	pos := f32.Pt(float32(at.X), float32(at.Y))
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos},
	)
}

func newDialogRig() *dialogRig {
	return &dialogRig{th: testTheme(), now: time.Unix(0, 0)}
}

func TestDialogClosedLetsClicksThrough(t *testing.T) {
	r := newDialogRig()
	r.frame()
	r.click(image.Pt(10, 10))
	if _, clicked := r.frame(); !clicked {
		t.Fatal("closed dialog: the content under it did not get the click")
	}
}

func TestDialogScrimPressDismissesAndBlocksContent(t *testing.T) {
	r := newDialogRig()
	r.settle()
	r.click(image.Pt(10, 10))
	dismissed, clicked := r.frame()
	if !dismissed {
		t.Error("press on the scrim did not dismiss")
	}
	if clicked {
		t.Error("press on the scrim reached the content under it")
	}
}

func TestDialogPanelPressKeepsItOpen(t *testing.T) {
	r := newDialogRig()
	r.settle()
	r.click(dialogTestWindow.Div(2))
	if dismissed, clicked := r.frame(); dismissed || clicked {
		t.Errorf("press on the panel: dismissed %v, content clicked %v; want neither", dismissed, clicked)
	}
}

func TestDialogEscapeDismisses(t *testing.T) {
	r := newDialogRig()
	r.settle()
	r.router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	if dismissed, _ := r.frame(); !dismissed {
		t.Error("Escape did not dismiss")
	}
}

// Opening focuses the panel, so Tab walks the dialog's own controls in order:
// the footer action, then the ×, which Space presses.
func TestDialogTakesFocusAndCloseButtonDismisses(t *testing.T) {
	r := newDialogRig()
	r.router.Queue(key.FocusEvent{})
	r.settle()
	// app.Window turns an unclaimed Tab into this call.
	r.router.MoveFocus(key.FocusForward)
	r.frame()
	if !r.router.Source().Focused(&r.confirm) {
		t.Fatal("Tab after opening did not land on the footer action")
	}
	r.router.MoveFocus(key.FocusForward)
	r.frame()
	r.router.Queue(key.Event{Name: key.NameSpace, State: key.Press}, key.Event{Name: key.NameSpace, State: key.Release})
	if dismissed, _ := r.frame(); !dismissed {
		t.Error("pressing the × did not dismiss")
	}
}

func TestDialogCloseRunsTheTransitionOut(t *testing.T) {
	r := newDialogRig()
	r.settle()
	r.open = false
	r.frame()
	r.now = r.now.Add(dialogTransition / 2)
	r.frame()
	r.click(image.Pt(10, 10))
	if _, clicked := r.frame(); clicked {
		t.Error("mid fade-out: the click reached the content, want the scrim still up")
	}
	r.now = r.now.Add(dialogTransition)
	r.frame()
	r.click(image.Pt(10, 10))
	if _, clicked := r.frame(); !clicked {
		t.Error("after the fade-out: the click did not reach the content")
	}
}

// The panel is centered at max-w-md over a scrim that darkens the content by
// bg-black/10 as a browser does: 0xff blue under it reads 0xff*0.9.
func TestDialogRendersCenteredPanelOverScrim(t *testing.T) {
	r := newDialogRig()
	r.settle()
	img := dialogRender(t, &r.ops)
	mid := dialogTestWindow.Div(2)
	left, right := mid.X, mid.X
	for near(img.NRGBAAt(left-1, mid.Y), Dark.Popover) {
		left--
	}
	for near(img.NRGBAAt(right+1, mid.Y), Dark.Popover) {
		right++
	}
	if w := right - left + 1; w != int(DialogWidthMD) || left+right+1 != dialogTestWindow.X {
		t.Errorf("panel spans x %d..%d, want %v wide and centered", left, right, DialogWidthMD)
	}
	if got, want := img.NRGBAAt(5, 5).B, uint8(230); got > want+1 || got+1 < want {
		t.Errorf("content under the scrim: blue %d, want %d", got, want)
	}
}

func dialogRender(t *testing.T, ops *op.Ops) *image.NRGBA {
	t.Helper()
	win, err := headless.NewWindow(dialogTestWindow.X, dialogTestWindow.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	rgba := image.NewRGBA(image.Rectangle{Max: dialogTestWindow})
	if err := win.Screenshot(rgba); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(rgba.Rect)
	for y := range dialogTestWindow.Y {
		for x := range dialogTestWindow.X {
			c := rgba.RGBAAt(x, y)
			img.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, c.A})
		}
	}
	return img
}
