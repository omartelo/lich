package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestMenuOriginFacesTheAnchor(t *testing.T) {
	size := image.Pt(100, 60)
	cases := []struct {
		side           Side
		anchor         image.Point
		origin, toward f32.Point
	}{
		{SideRight, image.Pt(-4, 20), f32.Pt(0, 20), f32.Pt(-1, 0)},
		{SideLeft, image.Pt(104, 90), f32.Pt(100, 60), f32.Pt(1, 0)},
		{SideBottom, image.Pt(30, -4), f32.Pt(30, 0), f32.Pt(0, -1)},
		{SideTop, image.Pt(-10, 64), f32.Pt(0, 60), f32.Pt(0, 1)},
	}
	for _, c := range cases {
		origin, toward := menuOrigin(c.side, c.anchor, size)
		if origin != c.origin || toward != c.toward {
			t.Errorf("menuOrigin(%v, %v) = %v, %v; want %v, %v", c.side, c.anchor, origin, toward, c.origin, c.toward)
		}
	}
}

func TestMenuPanelFadesIn(t *testing.T) {
	size := image.Pt(300, 300)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	t.Cleanup(win.Release)
	var c chosen
	g, s, th, now := &menuRig{size: size}, new(MenuState), testTheme(), time.Unix(1000, 0)
	entries := []MenuEntry{c.item("One"), c.item("Two")}
	frame := func() color.NRGBA {
		g.ops.Reset()
		gtx := layout.Context{Ops: &g.ops, Now: now, Source: g.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(size)}
		g.root.Layout(gtx, func(gtx C) D {
			defer op.Offset(triggerAt).Push(gtx.Ops).Pop()
			return th.DropdownMenu(&g.root, s, entries).Layout(gtx, func(gtx C) D { return D{Size: triggerSize} })
		})
		g.router.Frame(&g.ops)
		if err := win.Frame(&g.ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rectangle{Max: size})
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		// Inside the panel, right of the row text.
		return color.NRGBAModel.Convert(img.At(triggerAt.X+110, dropdownRowCenter(1).Y)).(color.NRGBA)
	}
	frame()
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	if first := frame(); near(first, th.Popover) {
		t.Errorf("first frame already shows the panel's %v, want it fading in", first)
	}
	now = now.Add(menuDuration)
	if settled := frame(); !near(settled, th.Popover) {
		t.Errorf("after menuDuration the panel reads %v, want %v", settled, th.Popover)
	}
}
