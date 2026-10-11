package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

// ToggleVariant is shadcn's toggle variant (components/ui/toggle.tsx).
type ToggleVariant uint8

const (
	ToggleVariantDefault ToggleVariant = iota
	ToggleVariantOutline
)

// ToggleSize is shadcn's toggle size.
type ToggleSize uint8

const (
	ToggleSizeDefault ToggleSize = iota
	ToggleSizeSM
	ToggleSizeLG
)

type toggleGeometry struct {
	height, padX, padIcon unit.Dp
}

// toggleSizes transcribes the size classes of toggle.tsx. min-w-N repeats the
// height of every size, so the minimum width is the table's own height.
var toggleSizes = map[ToggleSize]toggleGeometry{
	ToggleSizeDefault: {height: Space(9), padX: Space(2.5), padIcon: Space(2)},
	ToggleSizeSM:      {height: Space(8), padX: Space(2.5), padIcon: Space(1.5)},
	ToggleSizeLG:      {height: Space(10), padX: Space(2.5), padIcon: Space(2)},
}

var (
	toggleGap      = Space(1) // gap-1
	toggleIconSize = Space(4) // [&_svg]:size-4
)

const (
	toggleRingWidth   = 3   // focus-visible:ring-3
	toggleRingOpacity = 0.5 // focus-visible:ring-ring/50
	toggleBorderWidth = 1
	toggleHoverFill   = 0.5 // hover:bg-accent/50
)

// ToggleStyle draws a shadcn Toggle around a caller-owned widget.Bool: Pressed
// is aria-pressed, a click flips its Value.
type ToggleStyle struct {
	Pressed  *widget.Bool
	Text     string
	Icon     *icons.Icon // leading, optional
	Variant  ToggleVariant
	Size     ToggleSize
	Disabled bool
	theme    *Theme
}

// Toggle is a default-variant, default-size toggle.
func (th *Theme) Toggle(pressed *widget.Bool, text string) ToggleStyle {
	return ToggleStyle{Pressed: pressed, Text: text, theme: th}
}

func (t ToggleStyle) Layout(gtx C) D {
	spec := toggleSpec{
		text:     t.Text,
		icon:     t.Icon,
		variant:  t.Variant,
		size:     t.Size,
		pressed:  t.Pressed.Value,
		disabled: t.Disabled,
	}
	if t.Disabled {
		return t.theme.toggleItem(gtx, spec)
	}
	// The ring is drawn outside the clickable: its pointer area clips what
	// it draws to the toggle's own box.
	macro := op.Record(gtx.Ops)
	dims := t.Pressed.Layout(gtx, func(gtx C) D {
		spec.pressed = t.Pressed.Value
		spec.hovered = t.Pressed.Hovered()
		spec.focused = gtx.Focused(t.Pressed)
		return t.theme.toggleItem(gtx, spec)
	})
	toggle := macro.Stop()
	if gtx.Focused(t.Pressed) {
		toggleStroke(gtx, dims.Size, gtx.Dp(toggleRingWidth), gtx.Dp(RadiusMD), t.theme.Over(Alpha(t.theme.Ring, toggleRingOpacity)))
	}
	toggle.Add(gtx.Ops)
	return dims
}

// toggleSpec is everything a toggle's look depends on, so a group can draw
// its options through the same code as a lone Toggle.
type toggleSpec struct {
	text    string
	icon    *icons.Icon
	variant ToggleVariant
	size    ToggleSize
	// height, padX and textSize stand for the h-*, px-* and text-* a call
	// site puts in className; zero keeps the size's own.
	height   unit.Dp
	padX     unit.Dp
	textSize unit.Sp

	pressed, hovered, focused, disabled bool
}

type toggleColorSet struct{ bg, fg, border color.NRGBA }

// colors are the dark-mode classes of toggle.tsx: aria-pressed beats hover.
// The outline's border lands on whatever fill runs under it.
func (th *Theme) toggleColorsOf(s toggleSpec) toggleColorSet {
	p := th.Palette
	c := toggleColorSet{fg: p.MutedForeground}
	switch {
	case s.pressed:
		c.bg, c.fg = p.Accent, p.AccentForeground
	case s.hovered:
		c.bg, c.fg = th.Over(Alpha(p.Accent, toggleHoverFill)), p.Foreground
	}
	if s.variant == ToggleVariantOutline {
		under := th.Surface
		if c.bg.A != 0 {
			under = c.bg
		}
		c.border = th.On(under).Over(p.Input)
		if s.focused {
			c.border = p.Ring
		}
	}
	if s.disabled {
		c = toggleColorSet{th.fade(c.bg), th.fade(c.fg), th.fade(c.border)}
	}
	return c
}

func (th *Theme) toggleItem(gtx C, s toggleSpec) D {
	geo := toggleSizes[s.size]
	col := th.toggleColorsOf(s)
	height, padLeft, padRight := geo.height, geo.padX, geo.padX
	if s.height != 0 {
		height = s.height
	}
	if s.padX != 0 {
		padLeft, padRight = s.padX, s.padX
	}
	if s.icon != nil {
		padLeft = geo.padIcon
	}
	h := gtx.Dp(height)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	gtx.Constraints.Min.X = min(max(gtx.Constraints.Min.X, gtx.Dp(geo.height)), gtx.Constraints.Max.X)
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return toggleSurface(gtx, col) },
		func(gtx C) D {
			return layout.Inset{Left: padLeft, Right: padRight}.Layout(gtx, func(gtx C) D {
				// Inset keeps the minimum whole; min-w counts the padding.
				gtx.Constraints.Min.X = max(gtx.Constraints.Min.X-gtx.Dp(padLeft)-gtx.Dp(padRight), 0)
				return layout.Center.Layout(gtx, func(gtx C) D { return th.toggleContent(gtx, s, col.fg) })
			})
		},
	)
}

func toggleSurface(gtx C, col toggleColorSet) D {
	size := gtx.Constraints.Min
	d := Fill(gtx, col.bg, RadiusMD)
	if col.border.A != 0 {
		toggleStroke(gtx, size, -gtx.Dp(toggleBorderWidth), gtx.Dp(RadiusMD), col.border)
	}
	return d
}

func (th *Theme) toggleContent(gtx C, s toggleSpec, fg color.NRGBA) D {
	size := TextSM
	if s.textSize != 0 {
		size = s.textSize
	}
	text := th.Text(size, s.text)
	var children []layout.FlexChild
	if s.icon != nil {
		children = append(children, layout.Rigid(func(gtx C) D { return s.icon.Layout(gtx, toggleIconSize, fg) }))
	}
	if s.icon != nil && s.text != "" {
		children = append(children, layout.Rigid(layout.Spacer{Width: toggleGap}.Layout))
	}
	if s.text != "" {
		children = append(children, layout.Rigid(text.In(fg).Weight(font.Medium).Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// toggleStroke strokes a rounded rectangle of size along its edge: grow > 0
// is a band of that many px outside it (a box-shadow ring), grow < 0 a band
// inside (a CSS border). The band sits on whole pixels, so it stays crisp
// where a stroke centered on the edge would smear over two rows.
func toggleStroke(gtx C, size image.Point, grow, radius int, c color.NRGBA) {
	width := max(grow, -grow)
	rect := image.Rectangle{Max: size.Add(image.Pt(grow, grow))}
	shift := -float32(grow) / 2
	defer op.Affine(f32.Affine2D{}.Offset(f32.Pt(shift, shift))).Push(gtx.Ops).Pop()
	rr := clip.UniformRRect(rect, max(radius+grow/2, 0))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(width)}.Op())
}

// Checkbox and Switch both carry after:-inset-x-3 after:-inset-y-2: the
// pointer area reaches past the drawn box.
var (
	toggleHitX = Space(3)
	toggleHitY = Space(2)
)

// toggleInert is a clickable that is not: the Layout of a disabled control.
func toggleInert(gtx C, w layout.Widget) D { return w(gtx) }

// toggleHit lays box out at the origin and runs it inside clickable's
// pointer area, which is the box grown by toggleHitX/Y on every side. It
// reports the box's own size, so the larger area costs the layout nothing.
func toggleHit(gtx C, clickable func(C, layout.Widget) D, box layout.Widget) D {
	grow := image.Pt(gtx.Dp(toggleHitX), gtx.Dp(toggleHitY))
	defer op.Offset(image.Pt(-grow.X, -grow.Y)).Push(gtx.Ops).Pop()
	var inner D
	clickable(gtx, func(gtx C) D {
		defer op.Offset(grow).Push(gtx.Ops).Pop()
		inner = box(gtx)
		return D{Size: inner.Size.Add(grow.Mul(2))}
	})
	return inner
}
