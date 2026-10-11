package ui

import (
	"gioui.org/font"
	"gioui.org/unit"
	"gioui.org/widget"
)

// textareaPadY is textarea.tsx's py-2; its px is the Input's.
var (
	textareaPadY      = Space(2)
	textareaMinHeight = Space(16) // min-h-16
)

// TextareaStyle draws shadcn's Textarea around a caller-owned Editor. It is
// field-sizing-content: as tall as its text, at least min-h-16, and at most
// the maximum height constraint, which is where a max-h-* class goes; past it
// the editor scrolls.
type TextareaStyle struct {
	Editor      *widget.Editor
	Placeholder string
	Width       unit.Dp // 0 is w-full
	Disabled    bool
	Invalid     bool
	theme       *Theme
}

// Textarea is a default textarea over editor.
func (th *Theme) Textarea(editor *widget.Editor, placeholder string) TextareaStyle {
	return TextareaStyle{Editor: editor, Placeholder: placeholder, theme: th}
}

func (s TextareaStyle) Layout(gtx C) D {
	if s.Disabled {
		gtx = gtx.Disabled()
	}
	s.Editor.SingleLine = false
	gtx.Constraints.Max.X = inputWidth(gtx, s.Width)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	field := inputText{theme: s.theme, editor: s.Editor, placeholder: s.Placeholder, size: TextSM,
		font: font.Font{Typeface: s.theme.Sans}, padLeft: inputPadX, padRight: inputPadX, padY: textareaPadY,
		minHeight: textareaMinHeight}
	st := inputState{invalid: s.Invalid, disabled: s.Disabled}
	return inputFrame(gtx, s.theme, s.Editor, st, false, func(gtx C) D { return field.layout(gtx, 0) })
}
