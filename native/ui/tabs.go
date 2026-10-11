package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

// TabsVariant is shadcn's tabs list variant (tabsListVariants in tabs.tsx).
type TabsVariant uint8

const (
	TabsVariantDefault TabsVariant = iota
	TabsVariantLine
	// TabsVariantGhost is the dock header's look: RightDock.tsx overrides the
	// default list with a transparent one, accent fills and text-xs triggers.
	TabsVariantGhost
)

// TabsOrientation is the Tabs root's orientation prop.
type TabsOrientation uint8

const (
	TabsHorizontal TabsOrientation = iota
	TabsVertical
)

// TabsTrigger is one TabsTrigger: an optional leading icon, the label and an
// optional trailing widget (the dock's diff stat).
type TabsTrigger struct {
	Text     string
	Icon     *icons.Icon
	Trailing layout.Widget
	Disabled bool
}

// TabsState is the caller's half of the tabs: the selected index, which the
// caller may set at any time (a controlled value), and the clickables behind
// the triggers.
type TabsState struct {
	Selected int
	clicks   []widget.Clickable
	changed  bool
}

// Changed reports, once, whether the user picked another tab since the last
// call: onValueChange.
func (s *TabsState) Changed() bool {
	c := s.changed
	s.changed = false
	return c
}

func (s *TabsState) tabsUpdate(gtx C, n int) {
	for len(s.clicks) < n {
		s.clicks = append(s.clicks, widget.Clickable{})
	}
	for i := range s.clicks[:n] {
		for s.clicks[i].Clicked(gtx) {
			if s.Selected != i {
				s.Selected, s.changed = i, true
			}
		}
	}
}

// TabsStyle draws shadcn's Tabs: the TabsList with its TabsTriggers and, below
// it (right of it when vertical), the TabsContent of the selected trigger.
type TabsStyle struct {
	State       *TabsState
	Triggers    []TabsTrigger
	Variant     TabsVariant
	Orientation TabsOrientation
	theme       *Theme
}

// Tabs is a default-variant, horizontal tabs list.
func (th *Theme) Tabs(state *TabsState, triggers ...TabsTrigger) TabsStyle {
	return TabsStyle{State: state, Triggers: triggers, theme: th}
}

type tabsGeometry struct {
	listHeight, listPad, listGap, listRadius unit.Dp
	padX, padY, gap, icon                    unit.Dp
	text                                     unit.Sp
	lineHeight                               unit.Dp
}

const (
	// tabsListPad is the list's p-[0.1875rem].
	tabsListPad = unit.Dp(3)
	// tabsBorder is the trigger's border, transparent unless it is the active
	// default one or the focus ring.
	tabsBorder = unit.Dp(1)
	// tabsFocusBand is focus-visible:ring-[0.1875rem], the first pixel of
	// which sits under the 1px outline.
	tabsFocusBand        = unit.Dp(2)
	tabsFocusRingOpacity = 0.5
	// tabsIndicatorOffset is after:bottom-[-5px] (and -right-1 when vertical),
	// measured from the trigger's padding box.
	tabsIndicatorOffset = unit.Dp(5)
	tabsVerticalOffset  = unit.Dp(4)
	// text-sm and text-xs line heights; the dock's h-auto triggers take their
	// height from them.
	tabsLineHeightSM = unit.Dp(20)
	tabsLineHeightXS = unit.Dp(16)
)

// tabsGeometries transcribes tabs.tsx and, for the ghost variant, RightDock's
// TAB_CLASS: h-9 p-[0.1875rem] and px-2 py-1 gap-1.5 text-sm, against
// h-auto p-0.5 gap-1 and px-2 py-0.5 gap-1 text-xs.
var tabsGeometries = map[TabsVariant]tabsGeometry{
	TabsVariantDefault: {
		listHeight: Space(9), listPad: tabsListPad, listRadius: RadiusLG,
		padX: Space(2), padY: Space(1), gap: Space(1.5), icon: Space(4), text: TextSM, lineHeight: tabsLineHeightSM,
	},
	TabsVariantLine: {
		listHeight: Space(9), listPad: tabsListPad, listGap: Space(1),
		padX: Space(2), padY: Space(1), gap: Space(1.5), icon: Space(4), text: TextSM, lineHeight: tabsLineHeightSM,
	},
	TabsVariantGhost: {
		listPad: Space(0.5), listGap: Space(1), listRadius: RadiusLG,
		padX: Space(2), padY: Space(0.5), gap: Space(1), icon: Space(3.5), text: TextXS, lineHeight: tabsLineHeightXS,
	},
}

// Layout draws the list and, when panels are given, the panel of the selected
// tab under it. Only that panel is laid out, as base-ui unmounts the others.
func (t TabsStyle) Layout(gtx C, panels ...layout.Widget) D {
	t.State.tabsUpdate(gtx, len(t.Triggers))
	if t.State.Selected < 0 || t.State.Selected >= len(panels) {
		return t.tabsList(gtx)
	}
	// flex gap-2, column when horizontal.
	if t.Orientation == TabsVertical {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(t.tabsList), Gap(2), layout.Flexed(1, panels[t.State.Selected]))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(t.tabsList), VGap(2), layout.Flexed(1, panels[t.State.Selected]))
}

func (t TabsStyle) tabsList(gtx C) D {
	geo := tabsGeometries[t.Variant]
	vertical := t.Orientation == TabsVertical
	n := len(t.Triggers)
	pad, gap := gtx.Dp(geo.listPad), gtx.Dp(geo.listGap)
	trig := t.tabsTriggerSize(gtx, geo)

	size := image.Pt(2*pad+n*trig.X+(n-1)*gap, 2*pad+trig.Y)
	step, lead := image.Pt(trig.X+gap, 0), image.Pt(pad, pad)
	if vertical {
		size = image.Pt(2*pad+trig.X, 2*pad+n*trig.Y+(n-1)*gap)
		step = image.Pt(0, trig.Y+gap)
	} else if geo.listHeight > 0 {
		size.Y = gtx.Dp(geo.listHeight)
		lead.Y = pad + (size.Y-2*pad-trig.Y)/2
	}
	if t.Variant == TabsVariantDefault {
		gtx.Constraints.Min = size
		Fill(gtx, t.theme.Muted, geo.listRadius)
		// The triggers sit on the list's bg-muted, not on the page.
		t.theme = t.theme.On(t.theme.Muted)
	}
	for i := range t.Triggers {
		off := op.Offset(lead.Add(step.Mul(i))).Push(gtx.Ops)
		t.tabsTrigger(gtx, i, trig, geo)
		off.Pop()
	}
	return D{Size: size}
}

// tabsTriggerSize is one trigger's size. flex-1 in a w-fit list makes every
// trigger as wide as the widest; a vertical list is w-full over the same.
func (t TabsStyle) tabsTriggerSize(gtx C, geo tabsGeometry) image.Point {
	frame := gtx.Dp(tabsBorder)
	var w, h int
	for i := range t.Triggers {
		unbounded := gtx
		unbounded.Constraints = layout.Constraints{Max: image.Pt(unboundedWidth, gtx.Constraints.Max.Y)}
		macro := op.Record(gtx.Ops)
		d := t.tabsContent(unbounded, i, geo, color.NRGBA{})
		macro.Stop()
		w, h = max(w, d.Size.X), max(h, d.Size.Y)
	}
	size := image.Pt(w+2*(gtx.Dp(geo.padX)+frame), h+2*(gtx.Dp(geo.padY)+frame))
	if geo.listHeight > 0 && t.Orientation == TabsHorizontal {
		// h-[calc(100%-1px)] of the list's inner height.
		size.Y = gtx.Dp(geo.listHeight) - 2*gtx.Dp(geo.listPad) - frame
	}
	return size
}

type tabsColors struct{ bg, fg, border color.NRGBA }

// tabsColorsFor are the dark-mode classes of a trigger: the dark: overrides
// of text-foreground/60 and data-active:bg-background win. The border lands on
// the fill, which runs under it.
func (t TabsStyle) tabsColorsFor(active, hovered bool) tabsColors {
	th := t.theme
	p := th.Palette
	c := tabsColors{fg: p.MutedForeground}
	if hovered {
		c.fg = p.Foreground
	}
	switch t.Variant {
	case TabsVariantDefault:
		if active {
			c.bg = th.Over(Alpha(p.Input, 0.3))
			c.border, c.fg = th.On(c.bg).Over(p.Input), p.Foreground
		}
	case TabsVariantGhost:
		if hovered {
			c.bg = th.Over(Alpha(p.Accent, 0.5))
		}
		if active {
			c.bg, c.fg = p.Accent, p.AccentForeground
		}
	default:
		if active {
			c.fg = p.Foreground
		}
	}
	return c
}

func (t TabsStyle) tabsTrigger(gtx C, i int, size image.Point, geo tabsGeometry) {
	tr, click := t.Triggers[i], &t.State.clicks[i]
	active := t.State.Selected == i
	draw := func(gtx C) D {
		col := t.tabsColorsFor(active, click.Hovered() && !tr.Disabled)
		if tr.Disabled {
			th := t.theme
			col = tabsColors{th.fade(col.bg), th.fade(col.fg), th.fade(col.border)}
		}
		return t.tabsSurface(gtx, i, size, geo, col)
	}
	var d D
	if tr.Disabled {
		d = draw(gtx)
	} else {
		d = click.Layout(gtx, draw)
	}
	// Outside the clickable, whose clip would cut them off.
	if !tr.Disabled && gtx.Focused(click) {
		t.tabsFocusRing(gtx, d.Size)
	}
	if active && t.Variant == TabsVariantLine {
		t.tabsIndicator(gtx, d.Size, tr.Disabled)
	}
}

func (t TabsStyle) tabsSurface(gtx C, i int, size image.Point, geo tabsGeometry, col tabsColors) D {
	gtx.Constraints.Min, gtx.Constraints.Max = size, size
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			d := Fill(gtx, col.bg, RadiusMD)
			if col.border.A != 0 {
				toggleStroke(gtx, d.Size, -gtx.Dp(tabsBorder), gtx.Dp(RadiusMD), col.border)
			}
			return d
		},
		func(gtx C) D {
			pos := layout.Center
			if t.Orientation == TabsVertical {
				// justify-start
				pos = layout.W
			}
			return layout.Inset{Left: geo.padX, Right: geo.padX}.Layout(gtx, func(gtx C) D {
				return pos.Layout(gtx, func(gtx C) D { return t.tabsContent(gtx, i, geo, col.fg) })
			})
		},
	)
}

func (t TabsStyle) tabsContent(gtx C, i int, geo tabsGeometry, fg color.NRGBA) D {
	tr := t.Triggers[i]
	gtx.Constraints.Min.X = 0
	gtx.Constraints.Min.Y = gtx.Dp(geo.lineHeight)
	var children []layout.FlexChild
	if tr.Icon != nil {
		children = append(children, layout.Rigid(func(gtx C) D { return tr.Icon.Layout(gtx, geo.icon, fg) }))
	}
	if tr.Text != "" {
		if tr.Icon != nil {
			children = append(children, layout.Rigid(layout.Spacer{Width: geo.gap}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			return t.theme.Text(geo.text, tr.Text).In(fg).Weight(font.Medium).Layout(gtx)
		}))
	}
	if tr.Trailing != nil {
		children = append(children, layout.Rigid(layout.Spacer{Width: geo.gap}.Layout), layout.Rigid(tr.Trailing))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// tabsFocusRing is focus-visible: border-ring, a 1px outline-ring and the
// ring-ring/50 band, drawn as strokes straddling the border box.
func (t TabsStyle) tabsFocusRing(gtx C, size image.Point) {
	band := func(offset unit.Dp, c color.NRGBA) {
		off := gtx.Dp(offset)
		rect := clip.UniformRRect(image.Rectangle{Max: size}.Inset(-off), gtx.Dp(RadiusMD)+off)
		paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rect.Path(gtx.Ops), Width: float32(gtx.Dp(tabsFocusBand))}.Op())
	}
	band(tabsFocusBand, t.theme.Over(Alpha(t.theme.Ring, tabsFocusRingOpacity)))
	band(0, t.theme.Ring)
}

// tabsIndicator is the line variant's after: a 2px bg-foreground bar, h-0.5
// under a horizontal trigger and w-0.5 beside a vertical one.
func (t TabsStyle) tabsIndicator(gtx C, size image.Point, disabled bool) {
	frame, thick := gtx.Dp(tabsBorder), gtx.Dp(Space(0.5))
	var bar image.Rectangle
	if t.Orientation == TabsVertical {
		right := size.X - frame + gtx.Dp(tabsVerticalOffset)
		bar = image.Rect(right-thick, frame, right, size.Y-frame)
	} else {
		bottom := size.Y - frame + gtx.Dp(tabsIndicatorOffset)
		bar = image.Rect(frame, bottom-thick, size.X-frame, bottom)
	}
	c := t.theme.Foreground
	if disabled {
		c = t.theme.fade(c)
	}
	paint.FillShape(gtx.Ops, c, clip.Rect(bar).Op())
}
