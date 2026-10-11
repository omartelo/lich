package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

// Variant is shadcn's button variant (components/ui/button.tsx).
type Variant uint8

const (
	VariantDefault Variant = iota
	VariantOutline
	VariantSecondary
	VariantGhost
	VariantDestructive
)

// Size is shadcn's button size; the Icon sizes are square.
type Size uint8

const (
	SizeDefault Size = iota
	SizeXS
	SizeSM
	SizeLG
	SizeIcon
	SizeIconXS
	SizeIconSM
	SizeIconLG
)

type buttonGeometry struct {
	height, padX, gap, icon unit.Dp
	text                    unit.Sp
	square                  bool
}

// buttonSizes transcribes the size classes of button.tsx (h-9 gap-1.5 px-2.5,
// ...). Every size is rounded-md: the min(--radius-md, 8px) of the small ones
// is 6px too.
var buttonSizes = map[Size]buttonGeometry{
	SizeDefault: {height: Space(9), padX: Space(2.5), gap: Space(1.5), icon: Space(4), text: TextSM},
	SizeXS:      {height: Space(6), padX: Space(2), gap: Space(1), icon: Space(3), text: TextXS},
	SizeSM:      {height: Space(8), padX: Space(2.5), gap: Space(1), icon: Space(4), text: TextSM},
	SizeLG:      {height: Space(10), padX: Space(2.5), gap: Space(1.5), icon: Space(4), text: TextSM},
	SizeIcon:    {height: Space(9), icon: Space(4), text: TextSM, square: true},
	SizeIconXS:  {height: Space(6), icon: Space(3), text: TextXS, square: true},
	SizeIconSM:  {height: Space(8), icon: Space(4), text: TextSM, square: true},
	SizeIconLG:  {height: Space(10), icon: Space(4), text: TextSM, square: true},
}

// disabledOpacity is button.tsx's disabled:opacity-50.
const disabledOpacity = 0.5

// fade is the opaque color c as disabled:opacity-50 leaves it over the surface.
// The web fades the finished widget as one layer; fading each opaque color it
// is made of gives the same pixels, since every pixel is the top one's color.
// Transparent stays transparent, so a ghost button at rest paints nothing.
func (th *Theme) fade(c color.NRGBA) color.NRGBA {
	if c.A == 0 {
		return c
	}
	return th.Over(Alpha(c, disabledOpacity))
}

// ButtonStyle draws a shadcn Button around a caller-owned Clickable. It takes
// its natural width, or the minimum constraint when wider (w-full), and
// centers its content either way.
type ButtonStyle struct {
	Click    *widget.Clickable
	Text     string
	Icon     *icons.Icon // leading, optional
	Variant  Variant
	Size     Size
	Disabled bool
	theme    *Theme
}

// Button is a default-variant, default-size button.
func (th *Theme) Button(click *widget.Clickable, text string) ButtonStyle {
	return ButtonStyle{Click: click, Text: text, theme: th}
}

type buttonColors struct{ bg, fg, border color.NRGBA }

// colors are the dark-mode classes of each variant, at rest or hovered, every
// translucent one composited onto the surface. The border of the outline
// variant lands on the surface too: the button is bg-clip-padding, so its fill
// stops short of it.
func (b ButtonStyle) colors(hovered bool) buttonColors {
	th := b.theme
	p := th.Palette
	switch b.Variant {
	case VariantOutline:
		bg := th.Over(Alpha(p.Input, 0.3))
		if hovered {
			bg = th.Over(Alpha(p.Input, 0.5))
		}
		return buttonColors{bg: bg, fg: p.Foreground, border: th.Over(p.Input)}
	case VariantSecondary:
		bg := p.Secondary
		if hovered {
			// color-mix(in oklch, ...): foreground is achromatic, so its hue is
			// powerless and oklch carries secondary's own; that is oklab's mix.
			bg = MixOKLab(p.Secondary, p.Foreground, 0.05)
		}
		return buttonColors{bg: bg, fg: p.SecondaryForeground}
	case VariantGhost:
		var bg color.NRGBA
		if hovered {
			bg = th.Over(Alpha(p.Muted, 0.5))
		}
		return buttonColors{bg: bg, fg: p.Foreground}
	case VariantDestructive:
		bg := th.Over(Alpha(p.Destructive, 0.2))
		if hovered {
			bg = th.Over(Alpha(p.Destructive, 0.3))
		}
		return buttonColors{bg: bg, fg: p.Destructive}
	}
	bg := p.Primary
	if hovered {
		bg = th.Over(Alpha(p.Primary, 0.8))
	}
	return buttonColors{bg: bg, fg: p.PrimaryForeground}
}

func (b ButtonStyle) Layout(gtx C) D {
	if b.Disabled {
		return b.draw(gtx, false)
	}
	return b.Click.Layout(gtx, func(gtx C) D {
		if b.Click.Pressed() {
			// active:translate-y-px
			defer op.Offset(image.Pt(0, gtx.Dp(1))).Push(gtx.Ops).Pop()
		}
		return b.draw(gtx, b.Click.Hovered())
	})
}

func (b ButtonStyle) draw(gtx C, hovered bool) D {
	geo := buttonSizes[b.Size]
	col := b.colors(hovered)
	if b.Disabled {
		th := b.theme
		col = buttonColors{th.fade(col.bg), th.fade(col.fg), th.fade(col.border)}
	}
	h := gtx.Dp(geo.height)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	if geo.square {
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = h, h
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return b.surface(gtx, col) },
		func(gtx C) D {
			return layout.Inset{Left: geo.padX, Right: geo.padX}.Layout(gtx, func(gtx C) D {
				// Inset keeps the minimum whole; w-full counts the padding.
				gtx.Constraints.Min.X = max(gtx.Constraints.Min.X-2*gtx.Dp(geo.padX), 0)
				return layout.Center.Layout(gtx, func(gtx C) D { return b.content(gtx, geo, col.fg) })
			})
		},
	)
}

func (b ButtonStyle) surface(gtx C, col buttonColors) D {
	d := Fill(gtx, col.bg, RadiusMD)
	if col.border.A != 0 {
		// Inside the box, on whole pixels: a stroke centered on the edge would
		// cover half of each of two rows and render the border at half strength.
		toggleStroke(gtx, d.Size, -gtx.Dp(1), gtx.Dp(RadiusMD), col.border)
	}
	return d
}

func (b ButtonStyle) content(gtx C, geo buttonGeometry, fg color.NRGBA) D {
	var children []layout.FlexChild
	if b.Icon != nil {
		children = append(children, layout.Rigid(func(gtx C) D { return b.Icon.Layout(gtx, geo.icon, fg) }))
	}
	if b.Icon != nil && b.Text != "" {
		children = append(children, layout.Rigid(layout.Spacer{Width: geo.gap}.Layout))
	}
	if b.Text != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return b.theme.Text(geo.text, b.Text).In(fg).Weight(font.Medium).Layout(gtx)
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}
