package ui

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// The rename field of SessionCard.tsx: ring-1 ring-accent-foreground/30 on a
// rounded-sm, transparent box.
var inlineEditRingWidth = unit.Dp(1)

const inlineEditRingAlpha = 0.3

// InlineEdit is the caller-owned state of one in-row rename. It must not be
// copied once it has been laid out: the editor's identity is its address.
type InlineEdit struct {
	editor   widget.Editor
	original string
	active   bool
	// focused is set once the editor has been seen holding focus; losing it
	// afterwards is the blur that commits.
	focused bool
}

// Start begins editing text with all of it selected.
func (e *InlineEdit) Start(text string) {
	e.editor.SetText(text)
	e.editor.SetCaret(e.editor.Len(), 0)
	e.original = text
	e.active = true
	e.focused = false
}

// Active reports whether the field is being edited, which is when the caller
// lays it out in place of the label.
func (e *InlineEdit) Active() bool { return e.active }

type InlineEditOutcome uint8

const (
	// InlineEditPending is a frame in which the user is still typing.
	InlineEditPending InlineEditOutcome = iota
	// InlineEditCommitted carries a trimmed value that is non-empty and
	// differs from the text Start was given.
	InlineEditCommitted
	// InlineEditCancelled is Escape, and also Enter or blur on an empty or
	// unchanged value: the web card leaves editing without renaming then.
	InlineEditCancelled
)

type InlineEditResult struct {
	Outcome InlineEditOutcome
	Value   string
}

// InlineEditStyle draws the borderless field that takes a label's place. It is
// laid out exactly like the LabelStyle of the same size and weight, so the row
// does not jump when editing starts.
type InlineEditStyle struct {
	State    *InlineEdit
	Size     unit.Sp
	Font     font.Font
	PadRight unit.Dp // pr-*: inside the ring, as the web input's padding is
	theme    *Theme
}

// InlineEdit is a field of size over state, in the foreground color.
func (th *Theme) InlineEdit(state *InlineEdit, size unit.Sp) InlineEditStyle {
	return InlineEditStyle{State: state, Size: size, Font: font.Font{Typeface: th.Sans}, theme: th}
}

// Weight sets the font weight (font-medium is font.Medium).
func (s InlineEditStyle) Weight(w font.Weight) InlineEditStyle { s.Font.Weight = w; return s }

// Layout draws the field and reports how the edit ended, if it did. The frame
// that ends it is still drawn, so the caller swaps the label back in on the
// next one without a gap.
func (s InlineEditStyle) Layout(gtx C) (D, InlineEditResult) {
	e := s.State
	if !e.active {
		return D{}, InlineEditResult{}
	}
	e.editor.SingleLine, e.editor.Submit = true, true
	// Editor.Layout drains its own events, so the Enter has to be read first.
	res := e.poll(gtx)
	d := s.draw(gtx)
	if res.Outcome == InlineEditPending && e.lostFocus(gtx) {
		res = e.finish(true)
	}
	return d, res
}

func (e *InlineEdit) poll(gtx C) InlineEditResult {
	for {
		ev, ok := e.editor.Update(gtx)
		if !ok {
			break
		}
		if _, submit := ev.(widget.SubmitEvent); submit {
			return e.finish(true)
		}
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &e.editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if k, isKey := ev.(key.Event); isKey && k.State == key.Press {
			return e.finish(false)
		}
	}
	return InlineEditResult{}
}

// lostFocus asks for focus until the editor has it, then reports the frame it
// loses it.
func (e *InlineEdit) lostFocus(gtx C) bool {
	if gtx.Focused(&e.editor) {
		e.focused = true
		return false
	}
	if !e.focused {
		gtx.Execute(key.FocusCmd{Tag: &e.editor})
		return false
	}
	return true
}

func (e *InlineEdit) finish(submit bool) InlineEditResult {
	e.active = false
	value := strings.TrimSpace(e.editor.Text())
	if !submit || value == "" || value == e.original {
		return InlineEditResult{Outcome: InlineEditCancelled}
	}
	return InlineEditResult{Outcome: InlineEditCommitted, Value: value}
}

// draw lays the editor in the same CSS line box LabelStyle uses: Gio sizes the
// editor to its glyphs, so the half-leading above and below is added by hand.
func (s InlineEditStyle) draw(gtx C) D {
	th, ed := s.theme, &s.State.editor
	width := gtx.Constraints.Max.X
	inner := gtx
	inner.Constraints.Min = image.Point{}
	inner.Constraints.Max.X = max(width-gtx.Dp(s.PadRight), 0)

	textMat := inputMaterial(gtx, th.Foreground)
	selMat := inputMaterial(gtx, th.Over(Alpha(th.Primary, inputSelectionAlpha)))
	macro := op.Record(gtx.Ops)
	d := ed.Layout(inner, th.Shaper, s.Font, s.Size, textMat, selMat)
	call := macro.Stop()

	line := max(gtx.Sp(tailwindLeading[s.Size]), d.Size.Y)
	leading := line - d.Size.Y
	top := leading / 2
	size := image.Pt(width, line)
	ring := th.Over(Alpha(th.AccentForeground, inlineEditRingAlpha))
	inputStroke(gtx, size, gtx.Dp(RadiusSM), float32(gtx.Dp(inlineEditRingWidth)), true, ring)
	inputFocusOnPress(gtx, ed, size)
	defer op.Offset(image.Pt(0, top)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return D{Size: size, Baseline: d.Baseline + leading - top}
}
