package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// swatchMenu is a "Color" submenu of five swatches in three columns,
//
//	a b c
//	d e
//
// then a Rename item.
func swatchMenu(c *chosen) []MenuEntry {
	cells := make([]MenuEntry, 0, 5)
	for _, label := range []string{"a", "b", "c", "d", "e"} {
		cell := c.item(label)
		cell.Kind = MenuSwatch
		cells = append(cells, cell)
	}
	cells[1].Swatch = Dark.Destructive
	return []MenuEntry{{Kind: MenuSubTrigger, Label: "Color", Sub: cells, Columns: 3}, c.item("Rename")}
}

// swatchGridAt is the first cell's top-left: the grid opens against the
// Color row's right edge, inside the root's p-1, 3px above the row
// (alignOffset -3), with p-1 of its own around 28px cells (p-1.5 about
// size-4) 2px apart (gap-0.5).
var swatchGridAt = image.Pt(triggerAt.X+dropdownWidth-4+4, dropdownTop+4-3+4)

func TestSwatchGridKeyboardMovesInTwoDimensions(t *testing.T) {
	cases := []struct {
		keys []key.Name
		want string
	}{
		{nil, "a"},
		{[]key.Name{key.NameRightArrow}, "b"},
		{[]key.Name{key.NameDownArrow}, "d"},
		{[]key.Name{key.NameRightArrow, key.NameDownArrow}, "e"},
		{[]key.Name{key.NameRightArrow, key.NameRightArrow, key.NameRightArrow}, "c"},
		{[]key.Name{key.NameDownArrow, key.NameDownArrow}, "d"},
		{[]key.Name{key.NameRightArrow, key.NameRightArrow, key.NameDownArrow}, "c"},
		{[]key.Name{key.NameDownArrow, key.NameUpArrow}, "a"},
		{[]key.Name{key.NameRightArrow, key.NameLeftArrow}, "a"},
		// ArrowLeft off the first column closes the grid, back to Color.
		{[]key.Name{key.NameLeftArrow, key.NameDownArrow}, "Rename"},
	}
	for _, tc := range cases {
		var c chosen
		g, s, frame := openDropdown(t, swatchMenu(&c))
		g.press(key.NameDownArrow, key.NameRightArrow)
		g.press(append(tc.keys, key.NameReturn)...)
		frame()
		if len(c) != 1 || c[0] != tc.want {
			t.Errorf("%v: chosen %q, want only %q", tc.keys, []string(c), tc.want)
		}
		if s.Opened() {
			t.Errorf("%v: menu still open after Enter", tc.keys)
		}
	}
}

func TestSwatchGridClickChoosesTheCellAndCloses(t *testing.T) {
	var c chosen
	g, s, frame := openDropdown(t, swatchMenu(&c))
	g.hover(dropdownRowCenter(0))
	frame()
	cell := func(col, row int) image.Point { return swatchGridAt.Add(image.Pt(col*30+14, row*30+14)) }
	g.hover(cell(1, 1))
	frame()
	g.click(pointer.ButtonPrimary, cell(1, 1))
	frame()
	c.only(t, "e")
	if s.Opened() {
		t.Error("menu still open after choosing a swatch")
	}
}

func TestSwatchGridGapChoosesNothing(t *testing.T) {
	var c chosen
	g, s, frame := openDropdown(t, swatchMenu(&c))
	g.hover(dropdownRowCenter(0))
	frame()
	gap := swatchGridAt.Add(image.Pt(28+1, 14))
	g.click(pointer.ButtonPrimary, gap)
	frame()
	if len(c) > 0 || !s.Opened() {
		t.Errorf("open %v, chosen %q: want open, nothing chosen", s.Opened(), []string(c))
	}
}

// The cells are drawn deferred, outside the root's pointer area, yet their
// Hint still finds its place in the window: on top, where there is room.
func TestSwatchHintOpensAboveTheCell(t *testing.T) {
	size := image.Pt(400, 400)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	t.Cleanup(win.Release)
	var c chosen
	g, s, th, now := &menuRig{size: size}, new(MenuState), testTheme(), time.Unix(1000, 0)
	entries := swatchMenu(&c)
	frame := func() *image.RGBA {
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
		return img
	}
	frame()
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	frame()
	g.hover(dropdownRowCenter(0))
	frame()
	// The submenu animates in for menuDuration, its hit areas moving with
	// it; the pointer reaches a cell once it has settled.
	now = now.Add(menuDuration)
	frame()
	cell := swatchGridAt.Add(image.Pt(30+14, 30+14))
	g.hover(cell)
	frame()
	now = now.Add(tooltipOpenDelay)
	frame()
	now = now.Add(tooltipDuration)
	img := frame()
	// The swatch's edge is 8px off the cell's center and the bubble sits
	// sideOffset 4px beyond it, so 6px further is inside the bubble.
	above, below := img.At(cell.X, cell.Y-8-4-6), img.At(cell.X, cell.Y+8+4+6)
	if !near(color.NRGBAModel.Convert(above).(color.NRGBA), th.Foreground) {
		t.Errorf("above the swatch is %v, want the bubble's %v", above, th.Foreground)
	}
	if near(color.NRGBAModel.Convert(below).(color.NRGBA), th.Foreground) {
		t.Errorf("below the swatch is the bubble's %v, want it above", below)
	}
}
