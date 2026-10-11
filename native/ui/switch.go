package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// SwitchSize is shadcn's switch size.
type SwitchSize uint8

const (
	SwitchSizeDefault SwitchSize = iota
	SwitchSizeSM
)

// toggleRem is the root font size the switch's rem dimensions are written in.
const toggleRem unit.Dp = 16

type toggleSwitchGeometry struct{ width, height, thumb unit.Dp }

// toggleSwitchSizes transcribes data-[size=*] of switch.tsx. The default
// track is 1.15rem, 18.4px, around a 16px thumb.
var toggleSwitchSizes = map[SwitchSize]toggleSwitchGeometry{
	SwitchSizeDefault: {width: 2 * toggleRem, height: 1.15 * toggleRem, thumb: Space(4)},
	SwitchSizeSM:      {width: 1.5 * toggleRem, height: 0.875 * toggleRem, thumb: Space(3)},
}

const (
	// toggleSlideDuration and the curve below are Tailwind's default
	// transition (150ms, cubic-bezier(0.4, 0, 0.2, 1)); index.css overrides
	// neither.
	toggleSlideDuration = 150 * time.Millisecond
	toggleSwitchOffFill = 0.8 // dark:data-unchecked:bg-input/80
)

// toggleSwitchTravel is how far short of its own width the thumb travels
// (translate-x-[calc(100%-2px)]): the track's 1px transparent border on both
// sides.
const toggleSwitchTravel unit.Dp = 2

// SwitchState is a Switch's caller-owned state: a widget.Bool, plus the
// position of the thumb, which slides when Value changes. Gio has no CSS
// transition to hand that to, so the state carries it.
type SwitchState struct {
	widget.Bool
	slide toggleSlide
}

// toggleSlide is a transition-transform in flight: the thumb's position, 0 at
// the unchecked end and 1 at the checked one.
type toggleSlide struct {
	from, to float32
	start    time.Time
	seeded   bool
}

// at advances the slide to now, headed for target, and reports whether it is
// still moving, i.e. whether another frame is due. The first call lands on
// target without moving: a switch drawn for the first time is not a change.
func (s *toggleSlide) at(now time.Time, target float32) (pos float32, moving bool) {
	if !s.seeded {
		s.seeded, s.from, s.to = true, target, target
	}
	if target != s.to {
		s.from, s.to, s.start = s.position(now), target, now
	}
	return s.position(now), s.from != s.to && now.Sub(s.start) < toggleSlideDuration
}

func (s *toggleSlide) position(now time.Time) float32 {
	if s.from == s.to {
		return s.to
	}
	t := float32(now.Sub(s.start)) / float32(toggleSlideDuration)
	return s.from + (s.to-s.from)*toggleEase(min(max(t, 0), 1))
}

// toggleEase is cubic-bezier(0.4, 0, 0.2, 1) of progress x: it solves the
// curve's x(u) = x for u by bisection, then returns y(u).
func toggleEase(x float32) float32 {
	const x1, x2, y1, y2 = 0.4, 0.2, 0.0, 1.0
	axis := func(u, c1, c2 float32) float32 {
		return 3*(1-u)*(1-u)*u*c1 + 3*(1-u)*u*u*c2 + u*u*u
	}
	lo, hi := float32(0), float32(1)
	for range 20 {
		mid := (lo + hi) / 2
		if axis(mid, x1, x2) < x {
			lo = mid
		} else {
			hi = mid
		}
	}
	return axis((lo+hi)/2, y1, y2)
}

// SwitchStyle draws a shadcn Switch around a caller-owned SwitchState.
type SwitchStyle struct {
	Checked  *SwitchState
	Size     SwitchSize
	Disabled bool
	theme    *Theme
}

// Switch is an enabled, default-size switch.
func (th *Theme) Switch(checked *SwitchState) SwitchStyle {
	return SwitchStyle{Checked: checked, theme: th}
}

func (s SwitchStyle) Layout(gtx C) D {
	click := s.Checked.Bool.Layout
	if s.Disabled {
		click = toggleInert
	}
	return toggleHit(gtx, click, s.draw)
}

func (s SwitchStyle) draw(gtx C) D {
	th := s.theme
	p := th.Palette
	geo := toggleSwitchSizes[s.Size]
	size := image.Pt(gtx.Dp(geo.width), gtx.Dp(geo.height))
	thumb := gtx.Dp(geo.thumb)
	checked := s.Checked.Value

	target := float32(0)
	if checked {
		target = 1
	}
	pos, moving := s.Checked.slide.at(gtx.Now, target)
	if moving {
		gtx.Execute(op.InvalidateCmd{})
	}

	// The transition runs between two non-legacy colors, which CSS Color 4
	// interpolates in oklab with premultiplied alpha, then the browser
	// composites the result onto the page.
	track := th.Over(MixOKLab(Alpha(p.Input, toggleSwitchOffFill), p.Primary, float64(pos)))
	knob := p.Foreground
	if checked {
		knob = p.PrimaryForeground
	}
	// Opaque layers faded one by one are the faded whole, thumb included: the
	// thumb covers the track under it, as it does in the one faded layer.
	if s.Disabled {
		track, knob = th.fade(track), th.fade(knob)
	}

	inset := gtx.Dp(toggleBorderWidth)
	travel := thumb - gtx.Dp(toggleSwitchTravel)
	at := image.Pt(inset+int(math.Round(float64(pos)*float64(travel))), (size.Y-thumb)/2)

	focused := !s.Disabled && gtx.Focused(&s.Checked.Bool)
	radius := size.Y / 2
	if focused {
		toggleStroke(gtx, size, gtx.Dp(toggleRingWidth), radius, th.Over(Alpha(p.Ring, toggleRingOpacity)))
	}
	paint.FillShape(gtx.Ops, track, clip.UniformRRect(image.Rectangle{Max: size}, radius).Op(gtx.Ops))
	if focused {
		toggleStroke(gtx, size, -gtx.Dp(toggleBorderWidth), radius, p.Ring)
	}
	thumbArea := image.Rectangle{Min: at, Max: at.Add(image.Pt(thumb, thumb))}
	paint.FillShape(gtx.Ops, knob, clip.Ellipse(thumbArea).Op(gtx.Ops))
	return D{Size: size}
}
