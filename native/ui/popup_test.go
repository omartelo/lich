package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

func TestPopupPlace(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	window := image.Rect(0, 0, 200, 100)
	size := image.Pt(40, 20)
	cases := []struct {
		name     string
		place    popupPlace
		anchor   image.Rectangle
		want     image.Rectangle
		wantSide Side
	}{
		{"top, centered, sideOffset", popupPlace{side: SideTop, sideOffset: 4}, image.Rect(80, 50, 120, 60), image.Rect(80, 26, 120, 46), SideTop},
		{"bottom, start, alignOffset", popupPlace{side: SideBottom, align: alignStart, sideOffset: 4, alignOffset: -3}, image.Rect(80, 20, 120, 30), image.Rect(77, 34, 117, 54), SideBottom},
		{"top flips below", popupPlace{side: SideTop, sideOffset: 4}, image.Rect(80, 10, 120, 20), image.Rect(80, 24, 120, 44), SideBottom},
		{"right flips left", popupPlace{side: SideRight, align: alignStart}, image.Rect(180, 40, 180, 40), image.Rect(140, 40, 180, 60), SideLeft},
		// Neither side has room: the preferred one stays, shifted inside.
		{"no room either way", popupPlace{side: SideBottom, align: alignStart}, image.Rect(10, 10, 20, 90), image.Rect(10, 75, 50, 95), SideBottom},
		{"clamped on the cross axis", popupPlace{side: SideTop}, image.Rect(0, 50, 10, 60), image.Rect(5, 30, 45, 50), SideTop},
		{"clamped against the far edge", popupPlace{side: SideRight, align: alignStart, alignOffset: 4}, image.Rect(10, 95, 10, 95), image.Rect(10, 75, 50, 95), SideRight},
	}
	for _, tc := range cases {
		box, side := tc.place.box(gtx, tc.anchor, size, window)
		if box != tc.want || side != tc.wantSide {
			t.Errorf("%s: got %v on side %d, want %v on side %d", tc.name, box, side, tc.want, tc.wantSide)
		}
	}
}

// A backdrop laid out over a clickable takes a press outside the popup drawn
// above it, which the clickable never sees; a press on the popup reaches
// neither.
func TestPopupBackdropBlocksAndReportsOutsidePress(t *testing.T) {
	var (
		router   input.Router
		ops      op.Ops
		under    widget.Clickable
		backdrop popupBackdrop
		popup    struct{ _ byte }
	)
	window := image.Pt(100, 100)
	frame := func() (outside, underClicked bool) {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(window)}
		outside, underClicked = backdrop.pressed(gtx), under.Clicked(gtx)
		under.Layout(gtx, func(gtx C) D { return D{Size: window} })
		backdrop.layout(gtx, window)
		area := clip.Rect{Min: image.Pt(40, 40), Max: image.Pt(60, 60)}.Push(gtx.Ops)
		event.Op(gtx.Ops, &popup)
		area.Pop()
		router.Frame(&ops)
		return outside, underClicked
	}
	click := func(at image.Point) {
		pos := f32.Pt(float32(at.X), float32(at.Y))
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos},
		)
	}
	frame()
	click(image.Pt(10, 10))
	if outside, clicked := frame(); !outside || clicked {
		t.Errorf("press outside: reported %v, content clicked %v; want reported, not clicked", outside, clicked)
	}
	click(image.Pt(50, 50))
	if outside, clicked := frame(); outside || clicked {
		t.Errorf("press on the popup: reported %v, content clicked %v; want neither", outside, clicked)
	}
}
