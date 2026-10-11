package ui

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"
)

// Palette is the color tokens of frontend/src/index.css, by the same names.
type Palette struct {
	Background, Foreground                 color.NRGBA
	Card, CardForeground                   color.NRGBA
	Popover, PopoverForeground             color.NRGBA
	Primary, PrimaryForeground             color.NRGBA
	Secondary, SecondaryForeground         color.NRGBA
	Muted, MutedForeground                 color.NRGBA
	Accent, AccentForeground               color.NRGBA
	Destructive                            color.NRGBA
	TonePass, ToneWait                     color.NRGBA
	Border, Input, Ring                    color.NRGBA
	Sidebar, SidebarForeground             color.NRGBA
	SidebarAccent, SidebarAccentForeground color.NRGBA
	SidebarBorder                          color.NRGBA
}

// ponytail: dark only; the light block of index.css lands with the first
// screen that switches themes.
var Dark = Palette{
	Background:              OKLCH(0.141, 0.005, 285.823, 1),
	Foreground:              OKLCH(0.985, 0, 0, 1),
	Card:                    OKLCH(0.21, 0.006, 285.885, 1),
	CardForeground:          OKLCH(0.985, 0, 0, 1),
	Popover:                 OKLCH(0.21, 0.006, 285.885, 1),
	PopoverForeground:       OKLCH(0.985, 0, 0, 1),
	Primary:                 OKLCH(0.92, 0.004, 286.32, 1),
	PrimaryForeground:       OKLCH(0.21, 0.006, 285.885, 1),
	Secondary:               OKLCH(0.274, 0.006, 286.033, 1),
	SecondaryForeground:     OKLCH(0.985, 0, 0, 1),
	Muted:                   OKLCH(0.274, 0.006, 286.033, 1),
	MutedForeground:         OKLCH(0.705, 0.015, 286.067, 1),
	Accent:                  OKLCH(0.274, 0.006, 286.033, 1),
	AccentForeground:        OKLCH(0.985, 0, 0, 1),
	Destructive:             OKLCH(0.704, 0.191, 22.216, 1),
	TonePass:                OKLCH(0.696, 0.17, 162.48, 1),
	ToneWait:                OKLCH(0.769, 0.188, 70.08, 1),
	Border:                  OKLCH(1, 0, 0, 0.10),
	Input:                   OKLCH(1, 0, 0, 0.15),
	Ring:                    OKLCH(0.552, 0.016, 285.938, 1),
	Sidebar:                 OKLCH(0.21, 0.006, 285.885, 1),
	SidebarForeground:       OKLCH(0.985, 0, 0, 1),
	SidebarAccent:           OKLCH(0.274, 0.006, 286.033, 1),
	SidebarAccentForeground: OKLCH(0.985, 0, 0, 1),
	SidebarBorder:           OKLCH(1, 0, 0, 0.10),
}

// Theme is what every primitive draws with: the palette, the shaper, the
// font families (font-sans and font-mono) and the surface being drawn on.
type Theme struct {
	Palette
	Shaper *text.Shaper
	Sans   font.Typeface
	Mono   font.Typeface
	// Surface is the opaque color under what is being drawn, which Over
	// composites translucent tokens onto. A widget that paints a background
	// hands its children On(that background).
	Surface color.NRGBA
}

// On returns a copy of th drawing on surface, for the children of a widget
// that painted it.
func (th *Theme) On(surface color.NRGBA) *Theme {
	on := *th
	on.Surface = surface
	return &on
}

// Over is c composited onto th.Surface as a browser would: every translucent
// token (bg-accent/60, border-border, ...) goes through it before reaching
// paint. It panics when no surface was set, since compositing onto nothing
// would draw the linear-light blend Over exists to avoid.
func (th *Theme) Over(c color.NRGBA) color.NRGBA {
	if th.Surface.A != 255 {
		panic("ui: Theme.Over without an opaque Surface; set it, or draw under th.On(background)")
	}
	return Over(th.Surface, c)
}

// Space is Tailwind's spacing scale: p-2.5 is Space(2.5).
func Space(n float32) unit.Dp { return unit.Dp(4 * n) }

// Tailwind's type scale (text-xs, text-sm, text-base).
const (
	TextXS   unit.Sp = 12
	TextSM   unit.Sp = 14
	TextBase unit.Sp = 16
)

// Tailwind's radius scale (rounded-sm, rounded-md, rounded-lg). index.css
// declares a --radius that no rounded-* class reads, so the web app renders
// Tailwind's own values, and so do these.
const (
	RadiusSM unit.Dp = 4
	RadiusMD unit.Dp = 6
	RadiusLG unit.Dp = 8
)
