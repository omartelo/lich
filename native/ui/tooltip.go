package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// TooltipVariant is tooltip.tsx's variant: Label names a control in a few
// words, Card carries several lines about the thing under the pointer.
type TooltipVariant uint8

const (
	TooltipLabel TooltipVariant = iota
	TooltipCard
)

const (
	// base-ui's OPEN_DELAY and closeDelay: lich mounts no TooltipProvider,
	// so every trigger waits the full delay and closes at once.
	tooltipOpenDelay = 600 * time.Millisecond
	// tw-animate-css's animate-in/animate-out default.
	tooltipDuration = 150 * time.Millisecond
	// base-ui Positioner's default.
	tooltipArrowPadding unit.Dp = 5
	// rounded-[0.125rem].
	tooltipArrowRadius unit.Dp = 2
	// The arrow's translate-y calc(-50%-2px) sinks its center this far into
	// the bubble on the top and bottom sides.
	tooltipArrowSink unit.Dp = 2
)

var (
	tooltipSideOffset         = Space(1)   // sideOffset = 4
	tooltipMaxWidth           = Space(80)  // max-w-xs
	tooltipPadX               = Space(3)   // px-3
	tooltipPadY               = Space(1.5) // py-1.5
	tooltipSlide              = Space(2)   // slide-in-from-*-2
	tooltipArrowSize          = Space(2.5) // size-2.5
	tooltipArrowSide          = Space(1)   // -left-1 / -right-1 on the left and right sides
	tooltipBorder     unit.Dp = 1          // card's border
)

// Tooltip is the caller-owned state of one trigger's tooltip.
type Tooltip struct {
	hovered bool
	// pressed holds the tooltip shut after a press until the pointer leaves,
	// as base-ui's trigger does.
	pressed bool
	hoverAt time.Time
	shown   transition
	// origin is the trigger's top-left in window space, read off events
	// that carry a position: a Cancel's is zero.
	origin image.Point
}

func (t *Tooltip) update(gtx C, root *Root) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: t, Kinds: pointer.Enter | pointer.Leave | pointer.Move | pointer.Press | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Enter, pointer.Move:
			t.origin = root.origin(e.Position)
			if !t.hovered {
				t.hovered, t.hoverAt = true, gtx.Now
			}
		case pointer.Leave, pointer.Cancel:
			t.hovered, t.pressed = false, false
			t.shown.set(gtx, false)
		case pointer.Press:
			t.origin = root.origin(e.Position)
			t.pressed = true
			t.shown.set(gtx, false)
		}
	}
	if !t.hovered || t.pressed || t.shown.open {
		return
	}
	if due := t.hoverAt.Add(tooltipOpenDelay); gtx.Now.Before(due) {
		gtx.Execute(op.InvalidateCmd{At: due})
		return
	}
	t.shown.set(gtx, true)
}

// TooltipStyle draws a shadcn Tooltip over a trigger widget.
type TooltipStyle struct {
	State   *Tooltip
	Root    *Root
	Variant TooltipVariant
	Side    Side
	// Content is the bubble's body, at most max-w-xs wide inside the
	// padding; nil draws the trigger bare, as Hint does for an empty label.
	Content layout.Widget
	theme   *Theme
}

// Tooltip is Hint: a label-variant tooltip on top of the trigger.
func (th *Theme) Tooltip(root *Root, state *Tooltip, label string) TooltipStyle {
	t := TooltipStyle{State: state, Root: root, theme: th}
	if label != "" {
		text := th.On(th.Foreground).Text(TextXS, label).In(th.Background)
		text.MaxLines = 0
		t.Content = text.Layout
	}
	return t
}

// TooltipCard is a card-variant tooltip; content draws its own text, in
// PopoverForeground at TextXS to match, with th.On(th.Popover).
func (th *Theme) TooltipCard(root *Root, state *Tooltip, content layout.Widget) TooltipStyle {
	return TooltipStyle{State: state, Root: root, Variant: TooltipCard, Content: content, theme: th}
}

// Layout draws trigger, then the bubble deferred above everything else.
func (t TooltipStyle) Layout(gtx C, trigger layout.Widget) D {
	if t.Content == nil {
		return trigger(gtx)
	}
	t.State.update(gtx, t.Root)
	dims := trigger(gtx)
	area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	// Passes so the trigger's own Clickable, laid out below, still gets
	// its presses.
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, t.State)
	pass.Pop()
	area.Pop()
	if p, ok := t.State.shown.progress(gtx, tooltipDuration); ok {
		t.drawBubble(gtx, dims.Size, p)
	}
	return dims
}

func (t TooltipStyle) colors() (bg, border color.NRGBA) {
	if t.Variant == TooltipCard {
		return t.theme.Popover, t.theme.On(t.theme.Popover).Over(t.theme.Border)
	}
	return t.theme.Foreground, color.NRGBA{}
}

func (t TooltipStyle) drawBubble(gtx C, triggerSize image.Point, progress float32) {
	m := tooltipMetricsFor(gtx, t.Variant)
	inset := image.Pt(m.border+gtx.Dp(tooltipPadX), m.border+gtx.Dp(tooltipPadY))
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(tooltipMaxWidth)-2*inset.X, t.Root.window.Y)}
	macro := op.Record(gtx.Ops)
	content := t.Content(cgtx)
	body := macro.Stop()

	size := content.Size.Add(inset.Mul(2))
	window := image.Rectangle{Max: t.Root.window}.Sub(t.State.origin)
	place := popupPlace{side: t.Side, sideOffset: tooltipSideOffset}
	box, side := place.box(gtx, image.Rectangle{Max: triggerSize}, size, window)
	pl := m.anchor(side, box, triggerSize)
	bg, border := t.colors()

	macro = op.Record(gtx.Ops)
	op.Offset(pl.box.Min).Add(gtx.Ops)
	slide := pl.towardTrigger.Mul(float32(gtx.Dp(tooltipSlide)) * (1 - progress))
	scale := zoomFrom + (1-zoomFrom)*progress
	op.Affine(f32.Affine2D{}.Scale(pl.origin, f32.Pt(scale, scale)).Offset(slide)).Add(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, progress)
	rect := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(RadiusMD))
	paint.FillShape(gtx.Ops, bg, rect.Op(gtx.Ops))
	if m.border > 0 {
		paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rect.Path(gtx.Ops), Width: float32(m.border)}.Op())
	}
	tooltipArrow(gtx, pl.arrow, bg)
	offset := op.Offset(inset).Push(gtx.Ops)
	body.Add(gtx.Ops)
	offset.Pop()
	opacity.Pop()
	op.Defer(gtx.Ops, macro.Stop())
}

// tooltipArrow is the size-2.5 rotate-45 square centered at c.
func tooltipArrow(gtx C, c f32.Point, bg color.NRGBA) {
	half := gtx.Dp(tooltipArrowSize) / 2
	defer op.Affine(f32.Affine2D{}.Rotate(f32.Point{}, math.Pi/4).Offset(c)).Push(gtx.Ops).Pop()
	square := clip.UniformRRect(image.Rect(-half, -half, half, half), gtx.Dp(tooltipArrowRadius))
	paint.FillShape(gtx.Ops, bg, square.Op(gtx.Ops))
}

// tooltipMetrics are the positioner's distances in pixels.
type tooltipMetrics struct {
	offset, arrowPad, arrowHalf, arrowSink, arrowSide, border int
}

func tooltipMetricsFor(gtx C, v TooltipVariant) tooltipMetrics {
	m := tooltipMetrics{
		offset:    gtx.Dp(tooltipSideOffset),
		arrowPad:  gtx.Dp(tooltipArrowPadding),
		arrowHalf: gtx.Dp(tooltipArrowSize) / 2,
		arrowSink: gtx.Dp(tooltipArrowSink),
	}
	m.arrowSide = m.arrowHalf - gtx.Dp(tooltipArrowSide)
	if v == TooltipCard {
		m.border = gtx.Dp(tooltipBorder)
	}
	return m
}

type tooltipPlacement struct {
	box image.Rectangle // trigger-local
	// arrow and origin (base-ui's --transform-origin, the anchor point the
	// zoom grows from) are bubble-local.
	arrow, origin f32.Point
	towardTrigger f32.Point
}

// anchor places the arrow as tooltip.tsx's Arrow classes do: on the top and
// bottom sides it points at the trigger's center, kept arrow-padding inside
// the bubble; on the left and right ones top-1/2! pins it to the middle.
func (m tooltipMetrics) anchor(side Side, box image.Rectangle, trigger image.Point) tooltipPlacement {
	size := box.Size()
	pl := tooltipPlacement{box: box}
	if side.vertical() {
		lo, hi := m.arrowPad+m.arrowHalf, size.X-m.arrowPad-m.arrowHalf
		x := float32(max(min(trigger.X/2-box.Min.X, hi), lo))
		if side == SideTop {
			pl.arrow = f32.Pt(x, float32(size.Y-m.border-m.arrowSink))
			pl.origin = f32.Pt(x, float32(size.Y+m.offset))
			pl.towardTrigger = f32.Pt(0, 1)
		} else {
			pl.arrow = f32.Pt(x, float32(m.border+m.arrowSink))
			pl.origin = f32.Pt(x, float32(-m.offset))
			pl.towardTrigger = f32.Pt(0, -1)
		}
		return pl
	}
	y := float32(size.Y) / 2
	if side == SideLeft {
		pl.arrow = f32.Pt(float32(size.X-m.border-m.arrowSide), y)
		pl.origin = f32.Pt(float32(size.X+m.offset), y)
		pl.towardTrigger = f32.Pt(1, 0)
	} else {
		pl.arrow = f32.Pt(float32(m.border+m.arrowSide), y)
		pl.origin = f32.Pt(float32(-m.offset), y)
		pl.towardTrigger = f32.Pt(-1, 0)
	}
	return pl
}
