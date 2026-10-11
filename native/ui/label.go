package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// FormLabelStyle is shadcn's Label: the caption of a form control, text-sm
// font-medium leading-none. It is not LabelStyle, which is a run of text.
//
// The TSX's htmlFor, which makes a click on the caption act on its control, is
// the caller's to wire: wrap this in the control's own Clickable.
// ponytail: tracking-wide is not ported; the Gio shaper has no letter spacing.
type FormLabelStyle struct {
	Text      string
	Size      unit.Sp
	Weight    font.Weight
	Uppercase bool
	Disabled  bool
	theme     *Theme
}

// Label is a text-sm font-medium caption.
func (th *Theme) Label(text string) FormLabelStyle {
	return FormLabelStyle{Text: text, Size: TextSM, Weight: font.Medium, theme: th}
}

func (l FormLabelStyle) Layout(gtx C) D {
	c := l.theme.Foreground
	if l.Disabled {
		c = l.theme.fade(c)
	}
	// leading-none: the line box is as tall as the font size and the glyphs
	// overflow it evenly, where Gio's label keeps the font's own line height.
	oneLine := gtx
	oneLine.Constraints = layout.Constraints{Max: image.Pt(unboundedWidth, gtx.Constraints.Max.Y)}
	probe, _ := l.run(oneLine, c, 1)
	excess := max(0, probe.Size.Y-gtx.Sp(l.Size))
	d, call := l.run(gtx, c, 0)
	defer op.Offset(image.Pt(0, -excess/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	d.Size.Y -= excess
	d.Baseline -= excess / 2
	return d
}

func (l FormLabelStyle) run(gtx C, c color.NRGBA, maxLines int) (D, op.CallOp) {
	text := l.Text
	if l.Uppercase {
		text = strings.ToUpper(text)
	}
	colorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := colorMacro.Stop()
	w := widget.Label{MaxLines: maxLines, LineHeight: l.Size, LineHeightScale: 1}
	macro := op.Record(gtx.Ops)
	d := w.Layout(gtx, l.theme.Shaper, font.Font{Typeface: l.theme.Sans, Weight: l.Weight}, l.Size, text, material)
	return d, macro.Stop()
}
