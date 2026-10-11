package ui

import (
	"image"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

var (
	toggleCheckboxSize = Space(4)   // size-4
	toggleCheckSize    = Space(3.5) // [&>svg]:size-3.5
	toggleCheckIcon    = icons.Lucide("check")
)

// toggleCheckboxFill is dark:bg-input/30.
const toggleCheckboxFill = 0.3

// CheckboxStyle draws a shadcn Checkbox around a caller-owned widget.Bool.
// shadow-xs is not drawn: a 5% black shadow is below what the dark canvas
// can show.
type CheckboxStyle struct {
	Checked  *widget.Bool
	Disabled bool
	theme    *Theme
}

// Checkbox is an unchecked, enabled checkbox.
func (th *Theme) Checkbox(checked *widget.Bool) CheckboxStyle {
	return CheckboxStyle{Checked: checked, theme: th}
}

func (c CheckboxStyle) Layout(gtx C) D {
	click := c.Checked.Layout
	if c.Disabled {
		click = toggleInert
	}
	return toggleHit(gtx, click, c.draw)
}

func (c CheckboxStyle) draw(gtx C) D {
	th := c.theme
	p := th.Palette
	px := gtx.Dp(toggleCheckboxSize)
	size := image.Pt(px, px)
	checked := c.Checked.Value
	focused := !c.Disabled && gtx.Focused(c.Checked)

	// The fill runs under the border, which lands on it.
	bg := th.Over(Alpha(p.Input, toggleCheckboxFill))
	border, fg := th.On(bg).Over(p.Input), p.PrimaryForeground
	switch {
	case checked:
		bg = p.Primary
	case focused:
		border = p.Ring
	}
	if c.Disabled {
		bg, border, fg = th.fade(bg), th.fade(border), th.fade(fg)
	}

	radius := gtx.Dp(RadiusSM)
	if focused {
		toggleStroke(gtx, size, gtx.Dp(toggleRingWidth), radius, th.Over(Alpha(p.Ring, toggleRingOpacity)))
	}
	paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: size}, radius).Op(gtx.Ops))
	if !checked {
		// Checked, the border is the fill's own color.
		toggleStroke(gtx, size, -gtx.Dp(toggleBorderWidth), radius, border)
	}
	if checked {
		inset := (px - gtx.Dp(toggleCheckSize)) / 2
		defer op.Offset(image.Pt(inset, inset)).Push(gtx.Ops).Pop()
		toggleCheckIcon.Layout(gtx, toggleCheckSize, fg)
	}
	return D{Size: size}
}
