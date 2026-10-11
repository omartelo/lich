package gioterm

import (
	"fmt"
	"image"
	"os"
	"os/exec"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/glyphs"
	"golang.org/x/image/math/fixed"
)

// grid draws the terminal cells. Each row is recorded once into its own ops
// list and replayed until libghostty reports that row dirty again.
type grid struct {
	shaper  *text.Shaper
	size    unit.Sp
	font    font.Font
	pxPerEm fixed.Int26_6
	cellW   int
	cellH   int
	ascent  int
	rowOps  []*op.Ops
	rowCall []op.CallOp
	cache   map[glyphs.Key]cachedGlyph
}

func newGrid(shaper *text.Shaper, typeface font.Typeface, size unit.Sp) *grid {
	return &grid{shaper: shaper, font: font.Font{Typeface: typeface}, size: size}
}

// loadFamily reads the family's four styles from the files fontconfig picks.
// Gio's own lookup by name misses some families (Nerd Fonts among them) and
// its fallback never reaches their private-use icons, so the grid loads the
// files itself.
func loadFamily(family string) ([]font.FontFace, error) {
	var faces []font.FontFace
	for _, style := range []string{"regular", "bold", "italic", "bold:italic"} {
		path, err := exec.Command("fc-match", "-f", "%{file}", family+":"+style).Output()
		if err != nil {
			return nil, fmt.Errorf("fc-match %s:%s: %w", family, style, err)
		}
		data, err := os.ReadFile(string(path))
		if err != nil {
			return nil, err
		}
		parsed, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		faces = append(faces, parsed...)
	}
	return faces, nil
}

func (g *grid) Cell() (w, h int) { return g.cellW, g.cellH }

func (g *grid) Measure(gtx layout.Context) error {
	px := fixed.I(gtx.Sp(g.size))
	if px == g.pxPerEm {
		return nil
	}
	g.pxPerEm = px
	g.shaper.LayoutString(text.Parameters{Font: g.font, PxPerEm: px, MaxLines: 1, MaxWidth: 1 << 20}, "M")
	glyph, _ := g.shaper.NextGlyph()
	for _, ok := g.shaper.NextGlyph(); ok; _, ok = g.shaper.NextGlyph() {
	}
	g.cellW = glyph.Advance.Ceil()
	g.ascent = glyph.Ascent.Ceil()
	// The font's own line height, no extra leading: block and box-drawing
	// glyphs span exactly ascent to descent, and any gap splits them per row.
	g.cellH = (glyph.Ascent + glyph.Descent).Ceil()
	g.rowOps = nil
	g.cache = map[glyphs.Key]cachedGlyph{}
	return nil
}

func (g *grid) Draw(gtx layout.Context, s Snapshot) (int, error) {
	return g.draw(gtx, s.s, s.cells, s.dirty)
}

func (g *grid) draw(gtx layout.Context, sn snap, cells []cell, dirty []uint8) (int, error) {
	if len(g.rowOps) != sn.rows {
		g.rowOps = make([]*op.Ops, sn.rows)
		g.rowCall = make([]op.CallOp, sn.rows)
		for i := range g.rowOps {
			g.rowOps[i] = new(op.Ops)
		}
		for i := range dirty {
			dirty[i] = 1
		}
	}
	paint.Fill(gtx.Ops, rgba(sn.bg))
	repainted := 0
	for y := range sn.rows {
		if dirty[y] != 0 {
			g.recordRow(cells[y*sn.cols:(y+1)*sn.cols], sn, y)
			repainted++
		}
		t := op.Offset(image.Pt(0, y*g.cellH)).Push(gtx.Ops)
		g.rowCall[y].Add(gtx.Ops)
		t.Pop()
	}
	if sn.cursorVisible && sn.cursorX < sn.cols && sn.cursorY < sn.rows {
		g.drawCursor(gtx.Ops, cells[sn.cursorY*sn.cols+sn.cursorX], sn)
	}
	return repainted, nil
}

// drawCursor paints a block cursor that inverts the cell under it, so the
// character stays readable.
func (g *grid) drawCursor(ops *op.Ops, c cell, sn snap) {
	st := styleOf(c, sn)
	r := image.Rect(sn.cursorX*g.cellW, sn.cursorY*g.cellH, (sn.cursorX+1)*g.cellW, (sn.cursorY+1)*g.cellH)
	paint.FillShape(ops, rgba(st.fg), clip.Rect(r).Op())
	if c.cp <= ' ' {
		return
	}
	if _, _, ok := blockElement(rune(c.cp)); ok {
		return
	}
	gl := g.glyph(glyphs.Key{R: rune(c.cp), Bold: st.bold, Italic: st.ital})
	t := op.Offset(image.Pt(r.Min.X, r.Min.Y+g.ascent)).Push(ops)
	shape := clip.Outline{Path: gl.path}.Op().Push(ops)
	paint.ColorOp{Color: rgba(st.bg)}.Add(ops)
	paint.PaintOp{}.Add(ops)
	shape.Pop()
	t.Pop()
}

func (g *grid) recordRow(row []cell, sn snap, y int) {
	ops := g.rowOps[y]
	ops.Reset()
	macro := op.Record(ops)
	paintBackgrounds(ops, row, sn, g.cellW, g.cellH)
	for x, c := range row {
		if c.cp <= ' ' {
			continue
		}
		st := styleOf(c, sn)
		col := rgba(st.fg)
		if st.faint {
			col.A = 140
		}
		if rects, alpha, ok := blockElement(rune(c.cp)); ok {
			paintBlock(ops, x, rects, col, alpha, g.cellW, g.cellH)
			continue
		}
		gl := g.glyph(glyphs.Key{R: rune(c.cp), Bold: st.bold, Italic: st.ital})
		t := op.Offset(image.Pt(x*g.cellW, g.ascent)).Push(ops)
		shape := clip.Outline{Path: gl.path}.Op().Push(ops)
		paint.ColorOp{Color: col}.Add(ops)
		paint.PaintOp{}.Add(ops)
		shape.Pop()
		gl.bitmaps.Add(ops)
		t.Pop()
	}
	g.rowCall[y] = macro.Stop()
}

type cachedGlyph struct {
	path    clip.PathSpec
	bitmaps op.CallOp
}

// glyph shapes one character once and keeps its outline, origin on the
// baseline. Every cell then replays the same path under a translation, which
// is also the key Gio's GPU path cache matches on, so a glyph is tessellated
// once rather than once per row it appears in. ponytail: no ligatures, and a
// grapheme is its first codepoint only.
func (g *grid) glyph(k glyphs.Key) cachedGlyph {
	if c, ok := g.cache[k]; ok {
		return c
	}
	f := g.font
	if k.Bold {
		f.Weight = font.Bold
	}
	if k.Italic {
		f.Style = font.Italic
	}
	g.shaper.LayoutString(text.Parameters{Font: f, PxPerEm: g.pxPerEm, MaxLines: 1, MaxWidth: 1 << 20}, string(k.R))
	var gs []text.Glyph
	for gl, ok := g.shaper.NextGlyph(); ok; gl, ok = g.shaper.NextGlyph() {
		gs = append(gs, gl)
	}
	c := cachedGlyph{path: g.shaper.Shape(gs), bitmaps: g.shaper.Bitmaps(gs)}
	g.cache[k] = c
	return c
}
