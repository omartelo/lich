package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

// toggleTrackPad is p-[0.1875rem], the padding of the track every lich
// ToggleGroup sits in.
const toggleTrackPad = 0.1875 * toggleRem

var toggleItemGap = Space(1) // spacing={1}

// ToggleItem is one option of a ToggleGroup, a ToggleGroupItem.
type ToggleItem struct {
	Value    string
	Text     string
	Icon     *icons.Icon // leading, optional
	Disabled bool
}

// ToggleGroupStyle draws a shadcn ToggleGroup of one selected value around a
// caller-owned widget.Enum, in the form lich uses it: the options spaced by 1
// inside a bordered track (border border-border p-[0.1875rem], the segmented
// control of DESIGN.md). Pressing the chosen option again keeps it chosen, as
// every call site in the web app does.
type ToggleGroupStyle struct {
	Value   *widget.Enum
	Items   []ToggleItem
	Variant ToggleVariant
	Size    ToggleSize
	// ItemHeight, ItemPadX and ItemText are the h-*, px-* and text-* that
	// call sites put on each ToggleGroupItem; zero keeps the size's own.
	ItemHeight unit.Dp
	ItemPadX   unit.Dp
	ItemText   unit.Sp
	theme      *Theme
}

// ToggleGroup is a default-variant group of items; Value.Value is the chosen
// one.
func (th *Theme) ToggleGroup(value *widget.Enum, items ...ToggleItem) ToggleGroupStyle {
	return ToggleGroupStyle{Value: value, Items: items, theme: th}
}

func (g ToggleGroupStyle) Layout(gtx C) D {
	gtx.Constraints.Min = image.Point{}
	pad := unit.Dp(toggleBorderWidth) + toggleTrackPad
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			size := gtx.Constraints.Min
			toggleStroke(gtx, size, -gtx.Dp(toggleBorderWidth), gtx.Dp(RadiusMD), g.theme.Over(g.theme.Border))
			return D{Size: size}
		},
		func(gtx C) D { return layout.UniformInset(pad).Layout(gtx, g.row) },
	)
}

func (g ToggleGroupStyle) row(gtx C) D {
	var children []layout.FlexChild
	for i, item := range g.Items {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: toggleItemGap}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return g.item(gtx, item) }))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// item has no focus ring: widget.Enum takes focus on a mouse press, so a ring
// drawn for focus would show after every click, which focus-visible does not.
func (g ToggleGroupStyle) item(gtx C, item ToggleItem) D {
	spec := toggleSpec{
		text:     item.Text,
		icon:     item.Icon,
		variant:  g.Variant,
		size:     g.Size,
		height:   g.ItemHeight,
		padX:     g.ItemPadX,
		textSize: g.ItemText,
		pressed:  g.Value.Value == item.Value,
		disabled: item.Disabled,
	}
	if item.Disabled {
		return g.theme.toggleItem(gtx, spec)
	}
	return g.Value.Layout(gtx, item.Value, func(gtx C) D {
		hovered, ok := g.Value.Hovered()
		spec.hovered = ok && hovered == item.Value
		return g.theme.toggleItem(gtx, spec)
	})
}
