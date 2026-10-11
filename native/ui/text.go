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
)

// LabelStyle is one run of text. Text returns it set to one line, truncated
// with "…", in the foreground color, at its size's Tailwind line height; set
// MaxLines to 0 to wrap.
type LabelStyle struct {
	Text     string
	Size     unit.Sp
	Color    color.NRGBA
	Font     font.Font
	MaxLines int
	// LineHeight is CSS line-height: each line box is this tall and the text
	// sits centered in it (half-leading). 0 keeps the font's own height.
	LineHeight unit.Sp
	theme      *Theme
}

// tailwindLeading is the line-height Tailwind pairs with each text size
// (text-xs/1rem, text-sm/1.25rem, text-base/1.5rem).
var tailwindLeading = map[unit.Sp]unit.Sp{TextXS: 16, TextSM: 20, TextBase: 24}

// Text is a sans label of size in the foreground color.
func (th *Theme) Text(size unit.Sp, s string) LabelStyle {
	return LabelStyle{Text: s, Size: size, Color: th.Foreground, Font: font.Font{Typeface: th.Sans}, MaxLines: 1, LineHeight: tailwindLeading[size], theme: th}
}

// Muted sets text-muted-foreground.
func (l LabelStyle) Muted() LabelStyle { l.Color = l.theme.MutedForeground; return l }

// Weight sets the font weight (font-medium is font.Medium).
func (l LabelStyle) Weight(w font.Weight) LabelStyle { l.Font.Weight = w; return l }

// Mono sets font-mono.
func (l LabelStyle) Mono() LabelStyle { l.Font.Typeface = l.theme.Mono; return l }

// Italic sets italic.
func (l LabelStyle) Italic() LabelStyle { l.Font.Style = font.Italic; return l }

// In sets the color.
func (l LabelStyle) In(c color.NRGBA) LabelStyle { l.Color = c; return l }

// Leading sets the line height (leading-none is Leading(l.Size)).
func (l LabelStyle) Leading(h unit.Sp) LabelStyle { l.LineHeight = h; return l }

func (l LabelStyle) Layout(gtx layout.Context) layout.Dimensions {
	if l.LineHeight == 0 {
		return l.draw(gtx)
	}
	// Gio spaces baselines LineHeight apart but sizes the box to the glyphs,
	// so the half-leading CSS puts above the first line and below the last
	// is added here: the gap between one line's glyph height and its box.
	gtx.Constraints.Min.Y = 0
	macro := op.Record(gtx.Ops)
	d := l.draw(gtx)
	text := macro.Stop()
	oneLine := d.Size.Y
	if l.MaxLines != 1 {
		single := l
		single.MaxLines = 1
		probe := op.Record(gtx.Ops)
		oneLine = single.draw(gtx).Size.Y
		probe.Stop()
	}
	leading := gtx.Sp(l.LineHeight) - oneLine
	top := leading / 2
	off := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	text.Add(gtx.Ops)
	off.Pop()
	d.Size.Y += leading
	d.Baseline += leading - top
	return d
}

func (l LabelStyle) draw(gtx layout.Context) layout.Dimensions {
	colorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: l.Color}.Add(gtx.Ops)
	w := widget.Label{MaxLines: l.MaxLines, Truncator: "…", LineHeight: l.LineHeight}
	if l.LineHeight != 0 {
		// Gio scales an explicit LineHeight by 1.2 unless told otherwise.
		w.LineHeightScale = 1
	}
	return w.Layout(gtx, l.theme.Shaper, l.Font, l.Size, l.Text, colorMacro.Stop())
}

// unboundedWidth measures a line without wrapping it. The shaper holds widths
// in 26.6 fixed point, so math.MaxInt32 overflows; 1<<24 px does not.
const unboundedWidth = 1 << 24

// LayoutTail draws one line that keeps its end when it is too wide, behind a
// leading "…": the card's path readout, where the folder at the end is the
// part worth reading.
func (l LabelStyle) LayoutTail(gtx C) D {
	gtx.Constraints.Min = image.Point{}
	l.MaxLines = 0
	unbounded := gtx
	unbounded.Constraints = layout.Constraints{Max: image.Pt(unboundedWidth, gtx.Constraints.Max.Y)}
	macro := op.Record(gtx.Ops)
	full := l.Layout(unbounded)
	text := macro.Stop()
	maxX := gtx.Constraints.Max.X
	if full.Size.X <= maxX {
		text.Add(gtx.Ops)
		return full
	}
	l.Text = "…"
	ellipsis := l.Layout(gtx)
	room := maxX - ellipsis.Size.X
	defer op.Offset(image.Pt(ellipsis.Size.X, 0)).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: image.Pt(room, full.Size.Y)}.Push(gtx.Ops).Pop()
	shift := op.Offset(image.Pt(room-full.Size.X, 0)).Push(gtx.Ops)
	text.Add(gtx.Ops)
	shift.Pop()
	return D{Size: image.Pt(maxX, full.Size.Y), Baseline: full.Baseline}
}
