package ui

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

const (
	// tabsPulsePeriod and tabsPulseTrough are Tailwind's animate-pulse:
	// `pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite`, where the keyframe
	// at 50% is `opacity: .5`.
	tabsPulsePeriod = 2 * time.Second
	tabsPulseTrough = 0.5
	// tabsBisections pin the bezier's parameter to well under a pixel of
	// opacity.
	tabsBisections = 24
)

// SkeletonStyle is shadcn's Skeleton: a pulsing bg-foreground/20 rounded-md
// block, the pulse an opacity over the whole element. Width and Height are the w-* and h-* of the TSX; a zero Width is a
// block div's auto width, the whole row. A width past the constraint is
// clamped to it, as max-w-full does.
type SkeletonStyle struct {
	Width, Height unit.Dp
	theme         *Theme
}

// Skeleton is a placeholder of width by height.
func (th *Theme) Skeleton(width, height unit.Dp) SkeletonStyle {
	return SkeletonStyle{Width: width, Height: height, theme: th}
}

// Layout draws the block and asks for the next frame. Every skeleton pulses
// off the same clock, so a stack laid out together stays in step, as the CSS
// animations of elements mounted together do.
func (s SkeletonStyle) Layout(gtx C) D {
	w := gtx.Constraints.Max.X
	if s.Width > 0 {
		w = min(w, gtx.Dp(s.Width))
	}
	// An explicit size wins over Min, as w-* and h-* do on a block.
	size := image.Pt(w, min(gtx.Dp(s.Height), gtx.Constraints.Max.Y))
	gtx.Constraints.Min = size
	phase := float64(gtx.Now.UnixNano()%int64(tabsPulsePeriod)) / float64(tabsPulsePeriod)
	fill := Alpha(Alpha(s.theme.Foreground, 0.2), float32(tabsPulseOpacity(phase)))
	Fill(gtx, s.theme.Over(fill), RadiusMD)
	gtx.Execute(op.InvalidateCmd{})
	return layout.Dimensions{Size: size}
}

// tabsPulseOpacity is the keyframes at phase 0..1 of the period: full at
// both ends, the trough at the middle, each half eased on its own.
func tabsPulseOpacity(phase float64) float64 {
	if phase < 0.5 {
		return 1 - (1-tabsPulseTrough)*tabsEase(phase*2)
	}
	return tabsPulseTrough + (1-tabsPulseTrough)*tabsEase((phase-0.5)*2)
}

// tabsEase is cubic-bezier(0.4, 0, 0.6, 1): the curve's x is solved for the
// parameter by bisection, then read off at y.
func tabsEase(progress float64) float64 {
	const x1, y1, x2, y2 = 0.4, 0, 0.6, 1
	at := func(s, a, b float64) float64 {
		u := 1 - s
		return 3*u*u*s*a + 3*u*s*s*b + s*s*s
	}
	lo, hi := 0.0, 1.0
	for range tabsBisections {
		mid := (lo + hi) / 2
		if at(mid, x1, x2) < progress {
			lo = mid
		} else {
			hi = mid
		}
	}
	return at((lo+hi)/2, y1, y2)
}
