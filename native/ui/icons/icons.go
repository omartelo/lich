// Package icons draws the two icon sets the web app uses, from their SVG
// geometry:
//
//   - Lucide (lucide.dev, ISC; some icons MIT from Feather): the whole set of
//     lucide-static 1.23.0, the version frontend/package.json pins for
//     lucide-react, vendored as its icon-nodes.json. Names are Lucide's
//     canonical kebab-case ones ("git-branch"); the deprecated aliases
//     lucide-react still exports (Home, MoreHorizontal, ...) are not included,
//     so a port uses the name they alias ("house", "ellipsis").
//   - Lobe (github.com/lobehub/lobe-icons, MIT): the AI provider marks lich
//     uses, the same paths frontend/src/components/ProviderIcon.tsx vendors.
//
// Each set's license sits beside it (lucide/LICENSE, lobe/LICENSE) and must
// travel with any copy, the binary's third-party notices included.
package icons

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"sync"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// viewBox is the side of both sets' square viewBox.
const viewBox = 24

// lucideStroke is Lucide's stroke-width, in viewBox units; it scales with the
// icon, as lucide-react draws it by default.
const lucideStroke = 2

// Icon is a parsed icon, drawn at any size in any color.
type Icon struct {
	shapes []shape
}

//go:embed lucide/icon-nodes.json
var lucideNodes []byte

var lucideSet = sync.OnceValues(func() (map[string][][2]json.RawMessage, error) {
	var set map[string][][2]json.RawMessage
	if err := json.Unmarshal(lucideNodes, &set); err != nil {
		return nil, fmt.Errorf("lucide icon-nodes.json: %w", err)
	}
	return set, nil
})

// Lucide returns the Lucide icon of that name. It panics on a name the set
// does not have: icons are package variables, so a typo fails at startup.
func Lucide(name string) *Icon {
	set, err := lucideSet()
	if err != nil {
		panic(err)
	}
	nodes, ok := set[name]
	if !ok {
		panic(fmt.Sprintf("icons: no Lucide icon %q", name))
	}
	ic, err := parseLucide(nodes)
	if err != nil {
		panic(fmt.Sprintf("icons: Lucide %q: %v", name, err))
	}
	return ic
}

// parseLucide reads one icon's nodes: [element, attributes] pairs. Every
// element is stroked; one with fill="currentColor" is filled as well.
func parseLucide(nodes [][2]json.RawMessage) (*Icon, error) {
	ic := &Icon{}
	for _, n := range nodes {
		var el string
		var attrs map[string]string
		if err := json.Unmarshal(n[0], &el); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(n[1], &attrs); err != nil {
			return nil, err
		}
		segs, err := elementSegments(el, attrs)
		if err != nil {
			return nil, err
		}
		fill := attrs["fill"] != "" && attrs["fill"] != "none"
		ic.shapes = append(ic.shapes, shape{segs: segs, stroke: true, fill: fill})
	}
	return ic, nil
}

// elementSegments reads one element. An attribute it leaves out is 0, as in
// SVG (a rect's x and y, a corner radius).
func elementSegments(el string, attrs map[string]string) ([]segment, error) {
	var b pathBuilder
	var bad error
	num := func(key string) float64 {
		raw, ok := attrs[key]
		if !ok {
			return 0
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil && bad == nil {
			bad = fmt.Errorf("%s %s=%q: %w", el, key, raw, err)
		}
		return v
	}
	switch el {
	case "path":
		if err := b.path(attrs["d"]); err != nil {
			return nil, err
		}
	case "circle":
		b.ellipse(num("cx"), num("cy"), num("r"), num("r"))
	case "ellipse":
		b.ellipse(num("cx"), num("cy"), num("rx"), num("ry"))
	case "rect":
		b.rect(num("x"), num("y"), num("width"), num("height"), num("rx"), num("ry"))
	case "line":
		b.move(f32.Pt(float32(num("x1")), float32(num("y1"))))
		b.line(f32.Pt(float32(num("x2")), float32(num("y2"))))
	case "polyline", "polygon":
		if err := b.points(attrs["points"], el == "polygon"); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported element %q", el)
	}
	return b.segs, bad
}

// fillOnly builds a filled icon from path data, the form of the Lobe marks.
func fillOnly(d string) (*Icon, error) {
	var b pathBuilder
	if err := b.path(d); err != nil {
		return nil, err
	}
	return &Icon{shapes: []shape{{segs: b.segs, fill: true}}}, nil
}

// Layout draws the icon in a size×size square (size-4 is 16) in color c.
func (ic *Icon) Layout(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	scale := float32(px) / viewBox
	for _, sh := range ic.shapes {
		if sh.fill {
			paint.FillShape(gtx.Ops, c, clip.Outline{Path: sh.spec(gtx, scale)}.Op())
		}
		if sh.stroke {
			paint.FillShape(gtx.Ops, c, clip.Stroke{Path: sh.spec(gtx, scale), Width: lucideStroke * scale}.Op())
		}
	}
	return layout.Dimensions{Size: image.Pt(px, px)}
}

func (sh shape) spec(gtx layout.Context, scale float32) clip.PathSpec {
	var p clip.Path
	p.Begin(gtx.Ops)
	for _, s := range sh.segs {
		switch s.op {
		case 'M':
			p.MoveTo(s.to.Mul(scale))
		case 'L':
			p.LineTo(s.to.Mul(scale))
		case 'C':
			p.CubeTo(s.c1.Mul(scale), s.c2.Mul(scale), s.to.Mul(scale))
		case 'Z':
			p.Close()
		}
	}
	return p.End()
}
