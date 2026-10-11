package gioterm

import (
	"image"
	"image/color"
	"image/draw"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/glyphs"
)

// atlasSize is the side of one atlas page in pixels: 4 MiB of RGBA, room for
// a few thousand cell-sized glyphs per page.
const atlasSize = 1024

// atlasRenderer keeps every glyph it has drawn, already in its color, in
// texture pages, and draws a cell as that page clipped to a rectangle. A
// rectangle clip takes Gio's fast path, with no stencil or path packing, but
// it is still one draw per glyph.
type atlasRenderer struct {
	r       *glyphs.Rasterizer
	size    unit.Sp
	pages   []*atlasPage
	tinted  map[tintKey]tintEntry
	rowOps  []*op.Ops
	rowCall []op.CallOp
}

type tintKey struct {
	g glyphs.Key
	c color.NRGBA
}

// tintEntry locates a colored glyph in its page; page is nil for a glyph that
// draws nothing.
type tintEntry struct {
	page *atlasPage
	rect image.Rectangle
	off  image.Point
}

// atlasPage packs glyphs in shelves: left to right, then a new shelf below.
type atlasPage struct {
	img    *image.RGBA
	op     paint.ImageOp
	stale  bool
	x, y   int
	shelfH int
}

func newAtlasRenderer(r *glyphs.Rasterizer, size unit.Sp) *atlasRenderer {
	return &atlasRenderer{r: r, size: size}
}

func (a *atlasRenderer) Cell() (w, h int) { return a.r.CellW(), a.r.CellH() }

func (a *atlasRenderer) Measure(gtx layout.Context) error {
	changed, err := a.r.SetSize(float64(gtx.Sp(a.size)))
	if changed {
		a.pages, a.tinted, a.rowOps = nil, map[tintKey]tintEntry{}, nil
	}
	return err
}

func (a *atlasRenderer) Draw(gtx layout.Context, s Snapshot) (int, error) {
	return a.draw(gtx, s.s, s.cells, s.dirty)
}

func (a *atlasRenderer) draw(gtx layout.Context, sn snap, cells []cell, dirty []uint8) (int, error) {
	if len(a.rowOps) != sn.rows {
		a.rowOps = make([]*op.Ops, sn.rows)
		a.rowCall = make([]op.CallOp, sn.rows)
		for y := range a.rowOps {
			a.rowOps[y] = new(op.Ops)
			dirty[y] = 1
		}
	}
	cursor, err := a.prepare(sn, cells, dirty)
	if err != nil {
		return 0, err
	}
	if a.uploadStalePages() {
		// A recorded row holds the page texture it was recorded with; re-record
		// them all so none keeps an outdated copy of the page alive.
		for y := range dirty {
			dirty[y] = 1
		}
	}
	paint.Fill(gtx.Ops, rgba(sn.bg))
	redrawn := 0
	for y := range sn.rows {
		if dirty[y] != 0 {
			a.recordRow(cells[y*sn.cols:(y+1)*sn.cols], sn, y)
			redrawn++
		}
		t := op.Offset(image.Pt(0, y*a.r.CellH())).Push(gtx.Ops)
		a.rowCall[y].Add(gtx.Ops)
		t.Pop()
	}
	if cursor != nil {
		a.drawCursor(gtx.Ops, sn, cursor)
	}
	return redrawn, nil
}

// cursorCell is the cell under the cursor, drawn inverted on top of the grid.
type cursorCell struct {
	st    cellStyle
	glyph tintEntry
}

// prepare puts every glyph the dirty rows and the cursor need into the atlas
// before anything is recorded, so a page that gains glyphs is uploaded once
// per frame rather than once per glyph.
func (a *atlasRenderer) prepare(sn snap, cells []cell, dirty []uint8) (*cursorCell, error) {
	for y, d := range dirty {
		if d == 0 {
			continue
		}
		for _, c := range cells[y*sn.cols : (y+1)*sn.cols] {
			if _, err := a.entry(c, styleOf(c, sn)); err != nil {
				return nil, err
			}
		}
	}
	if !sn.cursorVisible || sn.cursorX >= sn.cols || sn.cursorY >= sn.rows {
		return nil, nil
	}
	c := cells[sn.cursorY*sn.cols+sn.cursorX]
	st := styleOf(c, sn)
	st.fg, st.bg = st.bg, st.fg
	e, err := a.entry(c, st)
	if err != nil {
		return nil, err
	}
	return &cursorCell{st: st, glyph: e}, nil
}

func glyphColor(st cellStyle) color.NRGBA {
	c := rgba(st.fg)
	if st.faint {
		c.A = 140
	}
	return c
}

// entry returns the atlas location of c drawn in st, adding it on first use.
// Blank cells and block elements, which are drawn as rectangles, have none.
func (a *atlasRenderer) entry(c cell, st cellStyle) (tintEntry, error) {
	if c.cp <= ' ' {
		return tintEntry{}, nil
	}
	if _, _, ok := blockElement(rune(c.cp)); ok {
		return tintEntry{}, nil
	}
	k := tintKey{g: glyphs.Key{R: rune(c.cp), Bold: st.bold, Italic: st.ital}, c: glyphColor(st)}
	if e, ok := a.tinted[k]; ok {
		return e, nil
	}
	m, err := a.r.Mask(k.g)
	if err != nil {
		return tintEntry{}, err
	}
	var e tintEntry
	if m.Img != nil {
		e = a.insert(m, k.c)
	}
	a.tinted[k] = e
	return e, nil
}

// insert copies a mask, tinted, into the first page with room for it.
func (a *atlasRenderer) insert(m *glyphs.Mask, c color.NRGBA) tintEntry {
	size := m.Img.Bounds().Size()
	var page *atlasPage
	var at image.Point
	for _, p := range a.pages {
		if pt, ok := p.reserve(size); ok {
			page, at = p, pt
			break
		}
	}
	if page == nil {
		page = &atlasPage{img: image.NewRGBA(image.Rect(0, 0, atlasSize, atlasSize))}
		a.pages = append(a.pages, page)
		at, _ = page.reserve(size)
	}
	rect := image.Rectangle{Min: at, Max: at.Add(size)}
	draw.DrawMask(page.img, rect, image.NewUniform(c), image.Point{}, m.Img, image.Point{}, draw.Src)
	page.stale = true
	return tintEntry{page: page, rect: rect, off: m.Off}
}

// reserve finds room for size on the page. ponytail: a glyph larger than a
// page, or a page that fills up, just opens another page; nothing is evicted.
func (p *atlasPage) reserve(size image.Point) (image.Point, bool) {
	if p.x+size.X > atlasSize {
		p.x, p.y, p.shelfH = 0, p.y+p.shelfH, 0
	}
	if p.y+size.Y > atlasSize {
		return image.Point{}, false
	}
	at := image.Pt(p.x, p.y)
	p.x += size.X
	p.shelfH = max(p.shelfH, size.Y)
	return at, true
}

// uploadStalePages gives every page that gained glyphs a new image op, which
// Gio uploads as a new texture. It reports whether any page changed.
func (a *atlasRenderer) uploadStalePages() bool {
	changed := false
	for _, p := range a.pages {
		if !p.stale {
			continue
		}
		p.op = paint.NewImageOp(p.img)
		p.op.Filter = paint.FilterNearest
		p.stale = false
		changed = true
	}
	return changed
}

func (a *atlasRenderer) recordRow(row []cell, sn snap, y int) {
	ops := a.rowOps[y]
	ops.Reset()
	macro := op.Record(ops)
	paintBackgrounds(ops, row, sn, a.r.CellW(), a.r.CellH())
	for x, c := range row {
		if c.cp <= ' ' {
			continue
		}
		st := styleOf(c, sn)
		if rects, alpha, ok := blockElement(rune(c.cp)); ok {
			paintBlock(ops, x, rects, glyphColor(st), alpha, a.r.CellW(), a.r.CellH())
			continue
		}
		a.drawGlyph(ops, a.tinted[tintKey{g: glyphs.Key{R: rune(c.cp), Bold: st.bold, Italic: st.ital}, c: glyphColor(st)}], x*a.r.CellW(), 0)
	}
	a.rowCall[y] = macro.Stop()
}

func (a *atlasRenderer) drawGlyph(ops *op.Ops, e tintEntry, x, y int) {
	if e.page == nil {
		return
	}
	t := op.Offset(image.Pt(x+e.off.X-e.rect.Min.X, y+e.off.Y-e.rect.Min.Y)).Push(ops)
	cl := clip.Rect(e.rect).Push(ops)
	e.page.op.Add(ops)
	paint.PaintOp{}.Add(ops)
	cl.Pop()
	t.Pop()
}

func (a *atlasRenderer) drawCursor(ops *op.Ops, sn snap, cur *cursorCell) {
	x, y := sn.cursorX*a.r.CellW(), sn.cursorY*a.r.CellH()
	paint.FillShape(ops, rgba(cur.st.bg), clip.Rect(image.Rect(x, y, x+a.r.CellW(), y+a.r.CellH())).Op())
	a.drawGlyph(ops, cur.glyph, x, y)
}
