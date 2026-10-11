package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// Fill paints the minimum constraint as a rounded rectangle, the bg-* and
// rounded-* of a Tailwind box; it is the background half of a
// layout.Background. A translucent token goes through th.Over first: Gio
// would blend it in linear light.
func Fill(gtx C, c color.NRGBA, radius unit.Dp) D {
	size := gtx.Constraints.Min
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radius)).Op(gtx.Ops))
	return D{Size: size}
}

// Gap is the flex gap-* between two children of a row; VGap of a column.
func Gap(n float32) layout.FlexChild  { return layout.Rigid(layout.Spacer{Width: Space(n)}.Layout) }
func VGap(n float32) layout.FlexChild { return layout.Rigid(layout.Spacer{Height: Space(n)}.Layout) }

// Separator is shadcn's horizontal Separator: a 1px border-colored line the
// width of the row.
func (th *Theme) Separator(gtx C) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: size}.Op())
	return D{Size: size}
}

// Scrollbar thumb opacities, index.css's ::-webkit-scrollbar-thumb: currentColor
// at 25%, 40% hovered. Mixing with transparent keeps the hue, so oklch and
// plain alpha agree.
const (
	scrollThumbOpacity      = 0.25
	scrollThumbHoverOpacity = 0.4
)

// ScrollArea is shadcn's ScrollArea over a widget.List: the list with a thin
// scrollbar that shows while its content overflows.
func (th *Theme) ScrollArea(state *widget.List) material.ListStyle {
	mt := material.Theme{Shaper: th.Shaper}
	mt.Fg = th.Foreground
	list := material.List(&mt, state)
	list.Indicator.Color = th.Over(Alpha(th.Foreground, scrollThumbOpacity))
	list.Indicator.HoverColor = th.Over(Alpha(th.Foreground, scrollThumbHoverOpacity))
	return list
}
