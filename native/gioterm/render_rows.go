package gioterm

import (
	"image"
	"image/draw"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/glyphs"
)

// rowRenderer composes each changed row into an image on the CPU and hands
// Gio one textured quad per row, so a frame costs one draw per row however
// many glyphs are on screen, and an unchanged row costs no work at all.
type rowRenderer struct {
	r    *glyphs.Rasterizer
	size unit.Sp
	imgs []*image.RGBA
	ops  []paint.ImageOp
	// cursorRow is the row the cursor was composed into last frame, -1 for none.
	cursorRow int
}

func newRowRenderer(r *glyphs.Rasterizer, size unit.Sp) *rowRenderer {
	return &rowRenderer{r: r, size: size, cursorRow: -1}
}

func (rr *rowRenderer) Cell() (w, h int) { return rr.r.CellW(), rr.r.CellH() }

func (rr *rowRenderer) Measure(gtx layout.Context) error {
	changed, err := rr.r.SetSize(float64(gtx.Sp(rr.size)))
	if changed {
		rr.imgs = nil
	}
	return err
}

func (rr *rowRenderer) Draw(gtx layout.Context, s Snapshot) (int, error) {
	return rr.draw(gtx, s.s, s.cells, s.dirty)
}

func (rr *rowRenderer) draw(gtx layout.Context, sn snap, cells []cell, dirty []uint8) (int, error) {
	width := sn.cols * rr.r.CellW()
	if len(rr.imgs) != sn.rows || rr.imgs[0].Bounds().Dx() != width {
		rr.imgs = make([]*image.RGBA, sn.rows)
		rr.ops = make([]paint.ImageOp, sn.rows)
		for y := range rr.imgs {
			rr.imgs[y] = image.NewRGBA(image.Rect(0, 0, width, rr.r.CellH()))
			dirty[y] = 1
		}
	}
	// The cursor lives inside its row's image, so the row it left and the row
	// it is on are both recomposed.
	cursorRow := -1
	if sn.cursorVisible && sn.cursorX < sn.cols && sn.cursorY < sn.rows {
		cursorRow = sn.cursorY
		dirty[cursorRow] = 1
	}
	if rr.cursorRow >= 0 && rr.cursorRow < sn.rows {
		dirty[rr.cursorRow] = 1
	}
	rr.cursorRow = cursorRow

	paint.Fill(gtx.Ops, rgba(sn.bg))
	redrawn := 0
	for y := range sn.rows {
		if dirty[y] != 0 {
			cursorX := -1
			if y == cursorRow {
				cursorX = sn.cursorX
			}
			if err := rr.compose(rr.imgs[y], cells[y*sn.cols:(y+1)*sn.cols], sn, cursorX); err != nil {
				return 0, err
			}
			rr.ops[y] = paint.NewImageOp(rr.imgs[y])
			rr.ops[y].Filter = paint.FilterNearest
			redrawn++
		}
		t := op.Offset(image.Pt(0, y*rr.r.CellH())).Push(gtx.Ops)
		cl := clip.Rect(rr.imgs[y].Bounds()).Push(gtx.Ops)
		rr.ops[y].Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		cl.Pop()
		t.Pop()
	}
	return redrawn, nil
}

// compose draws one row: every background first, then every glyph, so a wide
// glyph overhanging the next cell is not painted over by that cell's fill.
func (rr *rowRenderer) compose(img *image.RGBA, row []cell, sn snap, cursorX int) error {
	cw, ch := rr.r.CellW(), rr.r.CellH()
	styles := make([]cellStyle, len(row))
	draw.Draw(img, img.Bounds(), image.NewUniform(rgba(sn.bg)), image.Point{}, draw.Src)
	for x, c := range row {
		st := styleOf(c, sn)
		if x == cursorX {
			st.fg, st.bg = st.bg, st.fg
		}
		styles[x] = st
		if st.bg != sn.bg {
			draw.Draw(img, image.Rect(x*cw, 0, (x+1)*cw, ch), image.NewUniform(rgba(st.bg)), image.Point{}, draw.Src)
		}
	}
	for x, c := range row {
		if c.cp <= ' ' {
			continue
		}
		st := styles[x]
		col := rgba(st.fg)
		if st.faint {
			col.A = 140
		}
		if rects, alpha, ok := blockElement(rune(c.cp)); ok {
			col.A = uint8(uint16(col.A) * uint16(alpha) / 255)
			for _, r := range rects {
				draw.Draw(img, r.pixels(x*cw, cw, ch), image.NewUniform(col), image.Point{}, draw.Over)
			}
			continue
		}
		m, err := rr.r.Mask(glyphs.Key{R: rune(c.cp), Bold: st.bold, Italic: st.ital})
		if err != nil {
			return err
		}
		if m.Img == nil {
			continue
		}
		dst := m.Img.Bounds().Add(m.Off).Add(image.Pt(x*cw, 0))
		draw.DrawMask(img, dst, image.NewUniform(col), image.Point{}, m.Img, image.Point{}, draw.Over)
	}
	return nil
}
