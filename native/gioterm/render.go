package gioterm

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/glyphs"
)

// Renderer draws a Terminal's cell grid with Gio.
type Renderer interface {
	// Measure sets the cell metrics for the window's current scale; call it
	// every frame before Cell and Draw.
	Measure(gtx layout.Context) error
	// Cell is the size of one cell in pixels.
	Cell() (w, h int)
	// Draw paints the snapshot at the current offset and returns how many rows
	// it had to redraw.
	Draw(gtx layout.Context, s Snapshot) (int, error)
}

// Renderer kinds for NewRenderer.
const (
	// RenderPaths draws text through Gio's own text stack: one vector path per
	// glyph, which costs the most once the screen is full of text.
	RenderPaths = "paths"
	// RenderAtlas keeps rasterized glyphs in texture pages and draws one
	// textured rectangle per glyph.
	RenderAtlas = "atlas"
	// RenderRows composes each changed row into an image and draws one quad
	// per row.
	RenderRows = "rows"
)

// NewRenderer builds a renderer of the given kind for the font family (a
// fontconfig name) at size.
func NewRenderer(kind, family string, size unit.Sp) (Renderer, error) {
	switch kind {
	case RenderPaths:
		faces, err := loadFamily(family)
		if err != nil {
			return nil, err
		}
		return newGrid(text.NewShaper(text.WithCollection(faces)), faces[0].Font.Typeface, size), nil
	case RenderAtlas, RenderRows:
		r, err := glyphs.New(family)
		if err != nil {
			return nil, err
		}
		if kind == RenderAtlas {
			return newAtlasRenderer(r, size), nil
		}
		return newRowRenderer(r, size), nil
	}
	return nil, fmt.Errorf("unknown renderer %q", kind)
}

// Fit returns how many cells of r fit in size.
func Fit(size image.Point, r Renderer) (cols, rows int) {
	w, h := r.Cell()
	return max(size.X/w, 2), max(size.Y/h, 2)
}

func rgba(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

type cellStyle struct {
	fg, bg     uint32
	bold, ital bool
	faint      bool
}

func styleOf(c cell, sn snap) cellStyle {
	st := cellStyle{fg: sn.fg, bg: sn.bg, bold: c.flags&cellBold != 0, ital: c.flags&cellItalic != 0, faint: c.flags&cellFaint != 0}
	if c.flags&cellHasFg != 0 {
		st.fg = c.fg
	}
	if c.flags&cellHasBg != 0 {
		st.bg = c.bg
	}
	if c.flags&cellInverse != 0 {
		st.fg, st.bg = st.bg, st.fg
	}
	return st
}

// paintBackgrounds fills each run of cells whose background differs from the
// terminal's, one rectangle per run.
func paintBackgrounds(ops *op.Ops, row []cell, sn snap, cellW, cellH int) {
	for x := 0; x < len(row); {
		bg := styleOf(row[x], sn).bg
		end := x + 1
		for end < len(row) && styleOf(row[end], sn).bg == bg {
			end++
		}
		if bg != sn.bg {
			paint.FillShape(ops, rgba(bg), clip.Rect(image.Rect(x*cellW, 0, end*cellW, cellH)).Op())
		}
		x = end
	}
}

// paintBlock fills a block element's rectangles snapped to whole pixels, so
// adjacent cells meet edge to edge.
func paintBlock(ops *op.Ops, col int, rects []cellRect, c color.NRGBA, alpha uint8, cellW, cellH int) {
	c.A = uint8(uint16(c.A) * uint16(alpha) / 255)
	for _, r := range rects {
		paint.FillShape(ops, c, clip.Rect(r.pixels(col*cellW, cellW, cellH)).Op())
	}
}

// Faces loads a font family's four styles from the files fontconfig picks, for
// building a text.Shaper that matches the terminal's font.
func Faces(family string) ([]font.FontFace, error) { return loadFamily(family) }
