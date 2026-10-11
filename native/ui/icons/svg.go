package icons

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
)

// segment is one drawing command in viewBox units, every curve reduced to a
// cubic so drawing needs only MoveTo, LineTo, CubeTo and Close.
type segment struct {
	op     byte // 'M', 'L', 'C' or 'Z'
	c1, c2 f32.Point
	to     f32.Point
}

// shape is one SVG element: its outline, and whether it is stroked, filled or
// both.
type shape struct {
	segs   []segment
	stroke bool
	fill   bool
}

// pathBuilder turns SVG geometry into segments, tracking what the relative
// and smooth commands depend on.
type pathBuilder struct {
	segs       []segment
	cur, start f32.Point
	lastCtrl   f32.Point // reflected by S and T
	lastOp     byte
}

func (b *pathBuilder) move(p f32.Point) {
	b.segs = append(b.segs, segment{op: 'M', to: p})
	b.cur, b.start, b.lastCtrl = p, p, p
}

func (b *pathBuilder) line(p f32.Point) {
	b.segs = append(b.segs, segment{op: 'L', to: p})
	b.cur, b.lastCtrl = p, p
}

func (b *pathBuilder) cubic(c1, c2, p f32.Point) {
	b.segs = append(b.segs, segment{op: 'C', c1: c1, c2: c2, to: p})
	b.cur, b.lastCtrl = p, c2
}

// quad is drawn as the cubic it is exactly equal to; lastCtrl keeps the
// quadratic control point, which is what T reflects.
func (b *pathBuilder) quad(q, p f32.Point) {
	from := b.cur
	b.cubic(from.Add(q.Sub(from).Mul(2.0/3)), p.Add(q.Sub(p).Mul(2.0/3)), p)
	b.lastCtrl = q
}

func (b *pathBuilder) close() {
	b.segs = append(b.segs, segment{op: 'Z'})
	b.cur, b.lastCtrl = b.start, b.start
}

// arc appends the SVG elliptical arc from the current point to p, converted
// to center form and drawn as cubics of at most 90° each (SVG 1.1 F.6.5).
func (b *pathBuilder) arc(rx, ry, rotation float64, large, sweep bool, p f32.Point) {
	x1, y1 := float64(b.cur.X), float64(b.cur.Y)
	x2, y2 := float64(p.X), float64(p.Y)
	if x1 == x2 && y1 == y2 {
		return
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		b.line(p)
		return
	}
	phi := rotation * math.Pi / 180
	sin, cos := math.Sincos(phi)
	dx, dy := (x1-x2)/2, (y1-y2)/2
	x1p, y1p := cos*dx+sin*dy, -sin*dx+cos*dy
	if l := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry); l > 1 {
		rx, ry = rx*math.Sqrt(l), ry*math.Sqrt(l)
	}
	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	coef := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		coef = -coef
	}
	cxp, cyp := coef*rx*y1p/ry, -coef*ry*x1p/rx
	cx, cy := cos*cxp-sin*cyp+(x1+x2)/2, sin*cxp+cos*cyp+(y1+y2)/2
	theta := math.Atan2((y1p-cyp)/ry, (x1p-cxp)/rx)
	delta := math.Atan2((-y1p-cyp)/ry, (-x1p-cxp)/rx) - theta
	if sweep && delta < 0 {
		delta += 2 * math.Pi
	} else if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	}
	n := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	step := delta / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	point := func(t float64) (x, y, tx, ty float64) {
		st, ct := math.Sincos(t)
		ex, ey := rx*ct, ry*st
		dex, dey := -rx*st, ry*ct
		return cos*ex - sin*ey + cx, sin*ex + cos*ey + cy, cos*dex - sin*dey, sin*dex + cos*dey
	}
	for i := range n {
		t0, t1 := theta+float64(i)*step, theta+float64(i+1)*step
		ax, ay, atx, aty := point(t0)
		bx, by, btx, bty := point(t1)
		end := f32.Pt(float32(bx), float32(by))
		if i == n-1 {
			end = p
		}
		b.cubic(f32.Pt(float32(ax+k*atx), float32(ay+k*aty)), f32.Pt(float32(bx-k*btx), float32(by-k*bty)), end)
	}
}

// ellipse appends a closed ellipse as two half arcs.
func (b *pathBuilder) ellipse(cx, cy, rx, ry float64) {
	b.move(f32.Pt(float32(cx+rx), float32(cy)))
	b.arc(rx, ry, 0, false, true, f32.Pt(float32(cx-rx), float32(cy)))
	b.arc(rx, ry, 0, false, true, f32.Pt(float32(cx+rx), float32(cy)))
	b.close()
}

// rect appends a rectangle, its corners rounded by rx and ry as SVG resolves
// them: a missing one takes the other's value, and both are capped at half
// the side.
func (b *pathBuilder) rect(x, y, w, h, rx, ry float64) {
	if rx == 0 {
		rx = ry
	}
	if ry == 0 {
		ry = rx
	}
	rx, ry = math.Min(rx, w/2), math.Min(ry, h/2)
	pt := func(px, py float64) f32.Point { return f32.Pt(float32(px), float32(py)) }
	b.move(pt(x+rx, y))
	b.line(pt(x+w-rx, y))
	b.arc(rx, ry, 0, false, true, pt(x+w, y+ry))
	b.line(pt(x+w, y+h-ry))
	b.arc(rx, ry, 0, false, true, pt(x+w-rx, y+h))
	b.line(pt(x+rx, y+h))
	b.arc(rx, ry, 0, false, true, pt(x, y+h-ry))
	b.line(pt(x, y+ry))
	b.arc(rx, ry, 0, false, true, pt(x+rx, y))
	b.close()
}

// points appends a polyline, closed when polygon is set.
func (b *pathBuilder) points(list string, polygon bool) error {
	s := scanner{s: list}
	first := true
	for s.more() {
		x, err := s.number()
		if err != nil {
			return err
		}
		y, err := s.number()
		if err != nil {
			return err
		}
		p := f32.Pt(float32(x), float32(y))
		if first {
			b.move(p)
			first = false
			continue
		}
		b.line(p)
	}
	if polygon {
		b.close()
	}
	return nil
}

// path appends an SVG path's d attribute.
func (b *pathBuilder) path(d string) error {
	s := scanner{s: d}
	var cmd byte
	for s.more() {
		if c, ok := s.command(); ok {
			cmd = c
		} else if cmd == 0 {
			return fmt.Errorf("path %q: number before any command", d)
		}
		if err := b.apply(&s, cmd); err != nil {
			return fmt.Errorf("path %q: %w", d, err)
		}
		// Coordinates after a moveto are implicit linetos.
		if cmd == 'M' {
			cmd = 'L'
		} else if cmd == 'm' {
			cmd = 'l'
		}
	}
	return nil
}

// apply reads one command's arguments and appends its geometry.
func (b *pathBuilder) apply(s *scanner, cmd byte) error {
	rel := cmd >= 'a'
	at := func(x, y float64) f32.Point {
		p := f32.Pt(float32(x), float32(y))
		if rel {
			return p.Add(b.cur)
		}
		return p
	}
	upper := cmd &^ 0x20
	defer func() { b.lastOp = upper }()
	switch upper {
	case 'Z':
		b.close()
		return nil
	case 'M', 'L', 'T':
		v, err := s.numbers(2)
		if err != nil {
			return err
		}
		switch upper {
		case 'M':
			b.move(at(v[0], v[1]))
		case 'L':
			b.line(at(v[0], v[1]))
		default:
			b.quad(b.reflected('Q', 'T'), at(v[0], v[1]))
		}
	case 'H', 'V':
		v, err := s.numbers(1)
		if err != nil {
			return err
		}
		p := b.cur
		switch {
		case upper == 'H' && rel:
			p.X += float32(v[0])
		case upper == 'H':
			p.X = float32(v[0])
		case rel:
			p.Y += float32(v[0])
		default:
			p.Y = float32(v[0])
		}
		b.line(p)
	case 'C':
		v, err := s.numbers(6)
		if err != nil {
			return err
		}
		b.cubic(at(v[0], v[1]), at(v[2], v[3]), at(v[4], v[5]))
	case 'S', 'Q':
		v, err := s.numbers(4)
		if err != nil {
			return err
		}
		if upper == 'S' {
			b.cubic(b.reflected('C', 'S'), at(v[0], v[1]), at(v[2], v[3]))
		} else {
			b.quad(at(v[0], v[1]), at(v[2], v[3]))
		}
	case 'A':
		return b.applyArc(s, at)
	default:
		return fmt.Errorf("unsupported command %q", cmd)
	}
	return nil
}

func (b *pathBuilder) applyArc(s *scanner, at func(x, y float64) f32.Point) error {
	v, err := s.numbers(3)
	if err != nil {
		return err
	}
	large, err := s.flag()
	if err != nil {
		return err
	}
	sweep, err := s.flag()
	if err != nil {
		return err
	}
	end, err := s.numbers(2)
	if err != nil {
		return err
	}
	b.arc(v[0], v[1], v[2], large, sweep, at(end[0], end[1]))
	return nil
}

// reflected is the control point a smooth command implies: the previous
// control mirrored through the current point when the previous command was
// of the same family, else the current point itself.
func (b *pathBuilder) reflected(curve, smooth byte) f32.Point {
	if b.lastOp != curve && b.lastOp != smooth {
		return b.cur
	}
	return b.cur.Mul(2).Sub(b.lastCtrl)
}

// scanner reads the tokens of path data and point lists, where numbers may
// run together ("1-2", ".5.5") and arc flags may too ("011").
type scanner struct {
	s string
	i int
}

func (s *scanner) skip() {
	for s.i < len(s.s) && strings.IndexByte(" \t\n\r,", s.s[s.i]) >= 0 {
		s.i++
	}
}

func (s *scanner) more() bool {
	s.skip()
	return s.i < len(s.s)
}

func (s *scanner) command() (byte, bool) {
	s.skip()
	if s.i < len(s.s) && strings.IndexByte("MmLlHhVvCcSsQqTtAaZz", s.s[s.i]) >= 0 {
		s.i++
		return s.s[s.i-1], true
	}
	return 0, false
}

func (s *scanner) flag() (bool, error) {
	s.skip()
	if s.i < len(s.s) && (s.s[s.i] == '0' || s.s[s.i] == '1') {
		s.i++
		return s.s[s.i-1] == '1', nil
	}
	return false, fmt.Errorf("want an arc flag at offset %d", s.i)
}

func (s *scanner) numbers(n int) ([]float64, error) {
	v := make([]float64, n)
	for i := range v {
		x, err := s.number()
		if err != nil {
			return nil, err
		}
		v[i] = x
	}
	return v, nil
}

func (s *scanner) number() (float64, error) {
	s.skip()
	start := s.i
	if s.i < len(s.s) && (s.s[s.i] == '-' || s.s[s.i] == '+') {
		s.i++
	}
	dot := false
	for s.i < len(s.s) {
		c := s.s[s.i]
		switch {
		case c >= '0' && c <= '9':
		case c == '.' && !dot:
			dot = true
		case (c == 'e' || c == 'E') && s.i > start:
			s.i++
			if s.i < len(s.s) && (s.s[s.i] == '-' || s.s[s.i] == '+') {
				s.i++
			}
			continue
		default:
			return s.parse(start)
		}
		s.i++
	}
	return s.parse(start)
}

func (s *scanner) parse(start int) (float64, error) {
	v, err := strconv.ParseFloat(s.s[start:s.i], 64)
	if err != nil {
		return 0, fmt.Errorf("want a number at offset %d: %w", start, err)
	}
	return v, nil
}
