package ui

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// Root is laid out once, around the whole window, and every popup is placed
// against it. Gio does not expose a widget's position in the window; but the
// root's pointer area is the ancestor of every other, so it sees each pointer
// event in window coordinates while a widget sees the same event in its own,
// and the difference is that widget's origin.
type Root struct {
	window  image.Point
	pointer f32.Point
}

func (r *Root) Layout(gtx C, w layout.Widget) D {
	r.window = gtx.Constraints.Max
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: r, Kinds: pointer.Move | pointer.Drag | pointer.Press | pointer.Release | pointer.Enter | pointer.Leave})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			r.pointer = e.Position
		}
	}
	defer clip.Rect{Max: r.window}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, r)
	return w(gtx)
}

// origin is the window position of the top-left of a widget that saw this
// frame's pointer event at local.
func (r *Root) origin(local f32.Point) image.Point { return r.pointer.Sub(local).Round() }

// Side is base-ui's side: where a popup sits around its anchor. Only
// the tooltip takes it as a prop; the menus fix theirs.
type Side uint8

const (
	SideTop Side = iota
	SideBottom
	SideLeft
	SideRight
)

func (s Side) vertical() bool { return s == SideTop || s == SideBottom }

func (s Side) opposite() Side {
	return [...]Side{SideBottom, SideTop, SideRight, SideLeft}[s]
}

// align is base-ui's align along the side.
type align uint8

const (
	alignCenter align = iota
	alignStart
)

// collisionPadding is base-ui Positioner's default.
const collisionPadding unit.Dp = 5

// popupPlace is base-ui's Positioner: side, align, sideOffset, alignOffset.
type popupPlace struct {
	side                    Side
	align                   align
	sideOffset, alignOffset unit.Dp
}

// box places a popup of size around anchor, both in window's coordinates,
// and returns the side it landed on: base-ui's flip to the opposite side when
// the preferred one overflows and the opposite does not, then a shift to stay
// collisionPadding inside the window, the start winning when it cannot fit.
func (p popupPlace) box(gtx C, anchor image.Rectangle, size image.Point, window image.Rectangle) (image.Rectangle, Side) {
	pad := image.Pt(gtx.Dp(collisionPadding), gtx.Dp(collisionPadding))
	inside := image.Rectangle{Min: window.Min.Add(pad), Max: window.Max.Sub(pad)}
	side := p.side
	box := p.at(gtx, side, anchor, size)
	if flipped := p.at(gtx, side.opposite(), anchor, size); !fits(side, box, inside) && fits(side.opposite(), flipped, inside) {
		side, box = side.opposite(), flipped
	}
	return box.Add(image.Pt(shift(box.Min.X, size.X, inside.Min.X, inside.Max.X), shift(box.Min.Y, size.Y, inside.Min.Y, inside.Max.Y))), side
}

func (p popupPlace) at(gtx C, side Side, anchor image.Rectangle, size image.Point) image.Rectangle {
	offset, alignBy := gtx.Dp(p.sideOffset), gtx.Dp(p.alignOffset)
	start := anchor.Min
	if p.align == alignCenter {
		start = start.Add(anchor.Size().Sub(size).Div(2))
	}
	var at image.Point
	switch side {
	case SideTop:
		at = image.Pt(start.X+alignBy, anchor.Min.Y-offset-size.Y)
	case SideBottom:
		at = image.Pt(start.X+alignBy, anchor.Max.Y+offset)
	case SideLeft:
		at = image.Pt(anchor.Min.X-offset-size.X, start.Y+alignBy)
	case SideRight:
		at = image.Pt(anchor.Max.X+offset, start.Y+alignBy)
	}
	return image.Rectangle{Min: at, Max: at.Add(size)}
}

func fits(side Side, box, inside image.Rectangle) bool {
	switch side {
	case SideTop:
		return box.Min.Y >= inside.Min.Y
	case SideBottom:
		return box.Max.Y <= inside.Max.Y
	case SideLeft:
		return box.Min.X >= inside.Min.X
	}
	return box.Max.X <= inside.Max.X
}

// shift is how far a span at start, length long, moves to sit inside
// [lo, hi]; lo wins when it cannot fit.
func shift(start, length, lo, hi int) int {
	return max(min(start, hi-length), lo) - start
}

// popupBackdrop is a modal layer over the whole window. Laid out above the
// content and below the popup, it takes every press the popup does not, so
// nothing under it sees one. Its address is its event tag.
type popupBackdrop struct{ _ byte }

// pressed reports whether a press landed on the backdrop, outside the popup,
// since the last frame.
func (b *popupBackdrop) pressed(gtx C) bool {
	pressed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: b, Kinds: pointer.Press})
		if !ok {
			return pressed
		}
		if _, ok := ev.(pointer.Event); ok {
			pressed = true
		}
	}
}

// layout covers window, in the current transform, with the backdrop.
func (b *popupBackdrop) layout(gtx C, window image.Point) {
	defer clip.Rect{Max: window}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, b)
}

// zoomFrom is zoom-in-95 / zoom-out-95.
const zoomFrom = 0.95

// transition is tw-animate-css's animate-in and animate-out, in their default
// "ease": how far a popup is in, running backwards once closed.
type transition struct {
	open bool
	at   time.Time
}

// set follows open and reports whether it just changed.
func (t *transition) set(gtx C, open bool) bool {
	if t.open == open {
		return false
	}
	t.open, t.at = open, gtx.Now
	return true
}

// progress is how far in the popup is, 0..1, after running for duration, and
// whether to draw it at all.
func (t *transition) progress(gtx C, duration time.Duration) (float32, bool) {
	elapsed := gtx.Now.Sub(t.at)
	if t.at.IsZero() || (!t.open && elapsed >= duration) {
		return 0, false
	}
	p := float32(1)
	if elapsed < duration {
		gtx.Execute(op.InvalidateCmd{})
		p = ease(float32(elapsed) / float32(duration))
	}
	if !t.open {
		p = 1 - p
	}
	return p, true
}

// ease is CSS's "ease", cubic-bezier(0.25, 0.1, 0.25, 1), solved for x by
// bisection.
func ease(x float32) float32 {
	bezier := func(p1, p2, s float32) float32 {
		return 3*(1-s)*(1-s)*s*p1 + 3*(1-s)*s*s*p2 + s*s*s
	}
	lo, hi := float32(0), float32(1)
	for range 24 {
		mid := (lo + hi) / 2
		if bezier(0.25, 0.25, mid) < x {
			lo = mid
		} else {
			hi = mid
		}
	}
	return bezier(0.1, 1, (lo+hi)/2)
}
