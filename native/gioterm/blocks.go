package gioterm

import (
	"image"
	"math"
)

// cellRect is a part of a cell in fractions of its width and height.
type cellRect struct{ x0, y0, x1, y1 float32 }

// pixels places r in the cell whose left edge is at x, rounded to whole
// pixels so that neighbouring cells share their edges exactly.
func (r cellRect) pixels(x, cellW, cellH int) image.Rectangle {
	round := func(f float32, n int) int { return int(math.Round(float64(f) * float64(n))) }
	return image.Rect(x+round(r.x0, cellW), round(r.y0, cellH), x+round(r.x1, cellW), round(r.y1, cellH))
}

// Quadrant bits of the U+2596..U+259F block elements, in codepoint order.
const (
	quadUL = 1 << iota
	quadUR
	quadLL
	quadLR
)

var quadrants = [...]int{
	quadLL,                   // ▖
	quadLR,                   // ▗
	quadUL,                   // ▘
	quadUL | quadLL | quadLR, // ▙
	quadUL | quadLR,          // ▚
	quadUL | quadUR | quadLL, // ▛
	quadUL | quadUR | quadLR, // ▜
	quadUR,                   // ▝
	quadUR | quadLL,          // ▞
	quadUR | quadLL | quadLR, // ▟
}

// blockElement returns the rectangles and coverage of a Unicode block element
// (U+2580..U+259F). Terminals draw these themselves rather than with the
// font: font glyphs are antialiased at the edges, so neighbouring cells meet
// at half coverage and a solid figure shows a grid of seams.
func blockElement(r rune) (rects []cellRect, alpha uint8, ok bool) {
	switch {
	case r == 0x2580:
		return []cellRect{{0, 0, 1, .5}}, 255, true
	case r >= 0x2581 && r <= 0x2588:
		h := float32(r-0x2580) / 8
		return []cellRect{{0, 1 - h, 1, 1}}, 255, true
	case r >= 0x2589 && r <= 0x258F:
		w := float32(0x2590-r) / 8
		return []cellRect{{0, 0, w, 1}}, 255, true
	case r == 0x2590:
		return []cellRect{{.5, 0, 1, 1}}, 255, true
	case r >= 0x2591 && r <= 0x2593:
		return []cellRect{{0, 0, 1, 1}}, uint8(64 * (r - 0x2590)), true
	case r == 0x2594:
		return []cellRect{{0, 0, 1, .125}}, 255, true
	case r == 0x2595:
		return []cellRect{{.875, 0, 1, 1}}, 255, true
	case r >= 0x2596 && r <= 0x259F:
		q := quadrants[r-0x2596]
		for bit, rect := range map[int]cellRect{
			quadUL: {0, 0, .5, .5}, quadUR: {.5, 0, 1, .5},
			quadLL: {0, .5, .5, 1}, quadLR: {.5, .5, 1, 1},
		} {
			if q&bit != 0 {
				rects = append(rects, rect)
			}
		}
		return rects, 255, true
	}
	return nil, 0, false
}
