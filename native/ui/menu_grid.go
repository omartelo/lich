package ui

import (
	"image"
	"image/color"

	"gioui.org/io/key"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// The swatch grid of CardColorMenu.tsx: grid gap-0.5 of items p-1.5, each a
// size-4 rounded-full ring-1 ring-foreground/15 swatch, the current one with
// outline-2 outline-offset-2 outline-foreground.
const menuSwatchRing = 0.15

var (
	menuGridGap       = Space(0.5)
	menuSwatchPad     = Space(1.5)
	menuSwatchSize    = Space(4)
	menuSwatchOutline = Space(0.5)
)

// levelColumns is how many cells wide level k's grid is, 0 for a column of
// rows.
func (m menuCore) levelColumns(k int) int {
	if k == 0 {
		return 0
	}
	parent := m.s.levels[k-1]
	return m.levelEntries(k - 1)[parent.highlight].Columns
}

// gridKey moves a grid's highlight in two dimensions, stopping at its edges:
// Up and Down by a row, Left and Right within one. It leaves every other key,
// and ArrowLeft off the first column (closing the submenu), to key.
func (m menuCore) gridKey(k, cols int, name key.Name) bool {
	lv, entries := m.s.levels[k], m.levelEntries(k)
	h := lv.highlight
	if h < 0 {
		return false
	}
	to := h
	switch name {
	case key.NameDownArrow:
		to = h + cols
	case key.NameUpArrow:
		to = h - cols
	case key.NameRightArrow:
		if (h+1)%cols != 0 {
			to = h + 1
		}
	case key.NameLeftArrow:
		if h%cols == 0 {
			return false
		}
		to = h - 1
	default:
		return false
	}
	if to >= 0 && to < len(entries) && entries[to].actionable() {
		m.highlight(k, to)
	}
	return true
}

// layoutGrid is layoutPanel for a grid of cols columns: every cell as large as
// the largest, recorded in lv.rows like a row.
func (m menuCore) layoutGrid(gtx C, lv *menuLevel, entries []MenuEntry, cols int) image.Point {
	pad, gap := gtx.Dp(menuPanelPad), gtx.Dp(menuGridGap)
	if len(lv.hints) != len(entries) {
		lv.hints = make([]Tooltip, len(entries))
	}
	cells := make([]menuRow, len(entries))
	var cell image.Point
	for i, e := range entries {
		cells[i] = m.measureSwatch(gtx, e, i == lv.highlight, &lv.hints[i])
		cell = image.Pt(max(cell.X, cells[i].natural), max(cell.Y, cells[i].height))
	}
	lines := (len(entries) + cols - 1) / cols
	size := image.Pt(2*pad+cols*cell.X+(cols-1)*gap, 2*pad+lines*cell.Y+(lines-1)*gap)
	m.drawSurface(gtx, size)
	lv.rows = lv.rows[:0]
	for i, c := range cells {
		at := image.Pt(pad+i%cols*(cell.X+gap), pad+i/cols*(cell.Y+gap))
		rect := image.Rectangle{Min: at, Max: at.Add(cell)}
		lv.rows = append(lv.rows, rect)
		c.draw(gtx, rect)
	}
	return size
}

// measureSwatch is a swatch cell, its Hint on the swatch as the web wraps it.
func (m menuCore) measureSwatch(gtx C, e MenuEntry, highlighted bool, hint *Tooltip) menuRow {
	bg := m.rowColors(e, highlighted).bg
	on := m.panel
	if highlighted {
		on = m.panel.On(bg)
	}
	swatch, w := menuRecord(gtx, func(gtx C) D {
		return m.panel.Tooltip(m.root, hint, e.Label).Layout(gtx, func(gtx C) D { return menuDrawSwatch(gtx, e, on) })
	})
	pad := gtx.Dp(menuSwatchPad)
	return menuRow{left: swatch, leftW: w, bg: bg, padL: pad, padY: pad, rightInset: pad, height: w + 2*pad, natural: w + 2*pad}
}

// menuDrawSwatch draws e's swatch on on.Surface. The ring is a disc 1px wider
// than the swatch under it, a box-shadow ring's footprint.
func menuDrawSwatch(gtx C, e MenuEntry, on *Theme) D {
	d, px := gtx.Dp(menuSwatchSize), gtx.Dp(1)
	disc := func(grow int) image.Rectangle { return image.Rect(-grow, -grow, d+grow, d+grow) }
	if e.Checked {
		// outline-2 and outline-offset-2 share the one width.
		w := gtx.Dp(menuSwatchOutline)
		path := clip.Ellipse(disc(w + w/2)).Path(gtx.Ops)
		paint.FillShape(gtx.Ops, on.Foreground, clip.Stroke{Path: path, Width: float32(w)}.Op())
	}
	paint.FillShape(gtx.Ops, on.Over(Alpha(on.Foreground, menuSwatchRing)), clip.Ellipse(disc(px)).Op(gtx.Ops))
	if e.Swatch != (color.NRGBA{}) {
		paint.FillShape(gtx.Ops, e.Swatch, clip.Ellipse(disc(0)).Op(gtx.Ops))
		return D{Size: image.Pt(d, d)}
	}
	// conic-gradient(accent 0 50%, background 0): accent clockwise from
	// the top to the bottom, so the right half.
	paint.FillShape(gtx.Ops, on.Background, clip.Ellipse(disc(0)).Op(gtx.Ops))
	half := clip.Rect{Min: image.Pt(d/2, 0), Max: image.Pt(d, d)}.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, on.Accent, clip.Ellipse(disc(0)).Op(gtx.Ops))
	half.Pop()
	return D{Size: image.Pt(d, d)}
}
