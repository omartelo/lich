package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

// The geometry of input.tsx: h-9, px-2.5, py-1, and the pl-8 + left-2.5 of an
// icon tucked into the field (common/SearchInput.tsx).
var (
	inputHeight        = Space(9)
	inputPadX          = Space(2.5)
	inputPadY          = Space(1)
	inputLeadingPadX   = Space(8)
	inputLeadingIconX  = Space(2.5)
	inputLeadingIconSz = Space(4)
	inputBorderWidth   = unit.Dp(1)
	inputRingWidth     = unit.Dp(3) // ring-3
)

// The dark-mode alphas of input.tsx: dark:bg-input/30, focus-visible:ring-ring/50,
// dark:aria-invalid:border-destructive/50 and ring-destructive/40.
const (
	inputFillAlpha          = 0.3
	inputFocusRingAlpha     = 0.5
	inputInvalidBorderAlpha = 0.5
	inputInvalidRingAlpha   = 0.4
	// inputSelectionAlpha is index.css's ::selection, primary at 25%.
	inputSelectionAlpha = 0.25
)

// inputState is what the aria and :focus-visible classes of an input select on.
type inputState struct{ focused, invalid, disabled bool }

// inputPaint is the box's three layers, opaque, or transparent where a layer
// is not drawn.
type inputPaint struct{ fill, border, ring color.NRGBA }

// inputFill is dark:bg-input/30 composited onto the surface. The fill runs under
// the border (background-clip is border-box), so the border lands on it.
func inputFill(th *Theme) color.NRGBA { return th.Over(Alpha(th.Palette.Input, inputFillAlpha)) }

// inputPaintFor resolves the box colors of a state. An invalid field keeps its
// destructive border while focused: aria-invalid sits after focus-visible in
// the class list, which is how shadcn means it to win. The ring is a
// box-shadow, so it lands on the surface, not on the fill.
func inputPaintFor(th *Theme, st inputState, borderless bool) inputPaint {
	p := th.Palette
	fill := inputFill(th)
	box := th.On(fill)
	c := inputPaint{fill: fill, border: box.Over(p.Input)}
	switch {
	case st.invalid:
		c.border, c.ring = box.Over(Alpha(p.Destructive, inputInvalidBorderAlpha)), th.Over(Alpha(p.Destructive, inputInvalidRingAlpha))
	case st.focused:
		c.border, c.ring = p.Ring, th.Over(Alpha(p.Ring, inputFocusRingAlpha))
	}
	if borderless {
		c.border = color.NRGBA{}
		if !st.invalid {
			c.ring = color.NRGBA{}
		}
	}
	if st.disabled {
		c = inputPaint{th.fade(c.fill), th.fade(c.border), th.fade(c.ring)}
	}
	return c
}

// InputStyle draws shadcn's Input around a caller-owned Editor. It is one
// line (Layout sets Editor.SingleLine), fills the maximum width unless Width
// says otherwise, and draws its focus ring outside its bounds like a
// box-shadow does, so the parent must leave room for the 3px.
type InputStyle struct {
	Editor      *widget.Editor
	Placeholder string
	// Leading is an icon inside the left edge, like common/SearchInput.
	Leading     *icons.Icon
	LeadingSize unit.Dp // 0 is size-4
	Height      unit.Dp // 0 is h-9; the dense fields are Space(7) and Space(8)
	Width       unit.Dp // 0 is w-full
	TextSize    unit.Sp // 0 is text-sm; the dense fields use TextXS
	Mono        bool
	// Borderless is border-0 focus-visible:ring-0, for a field that sits in a
	// panel that is already a box.
	Borderless bool
	Disabled   bool
	Invalid    bool
	theme      *Theme
}

// Input is a default input over editor.
func (th *Theme) Input(editor *widget.Editor, placeholder string) InputStyle {
	return InputStyle{Editor: editor, Placeholder: placeholder, theme: th}
}

func (s InputStyle) Layout(gtx C) D {
	if s.Disabled {
		gtx = gtx.Disabled()
	}
	s.Editor.SingleLine = true
	height := s.Height
	if height == 0 {
		height = inputHeight
	}
	gtx.Constraints.Max.X = inputWidth(gtx, s.Width)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	h := gtx.Dp(height)
	left := inputPadX
	if s.Leading != nil {
		left = inputLeadingPadX
	}
	field := s.field(left)
	st := inputState{invalid: s.Invalid, disabled: s.Disabled}
	return inputFrame(gtx, s.theme, s.Editor, st, s.Borderless, func(gtx C) D {
		d := field.layout(gtx, h)
		s.layoutLeading(gtx, h)
		return d
	})
}

func (s InputStyle) field(left unit.Dp) inputText {
	f := inputText{theme: s.theme, editor: s.Editor, placeholder: s.Placeholder, size: s.TextSize,
		font: font.Font{Typeface: s.theme.Sans}, padLeft: left, padRight: inputPadX, padY: inputPadY, single: true}
	if s.Mono {
		f.font.Typeface = s.theme.Mono
	}
	if f.size == 0 {
		f.size = TextSM
	}
	return f
}

func (s InputStyle) layoutLeading(gtx C, h int) {
	if s.Leading == nil {
		return
	}
	size := s.LeadingSize
	if size == 0 {
		size = inputLeadingIconSz
	}
	col := s.theme.MutedForeground
	if s.Disabled {
		col = s.theme.fade(col)
	}
	px := gtx.Dp(size)
	defer op.Offset(image.Pt(gtx.Dp(inputLeadingIconX), (h-px)/2)).Push(gtx.Ops).Pop()
	s.Leading.Layout(gtx, size, col)
}

// inputWidth is w-full, or the fixed width of a w-* class when it fits.
func inputWidth(gtx C, width unit.Dp) int {
	if width == 0 {
		return gtx.Constraints.Max.X
	}
	return min(gtx.Dp(width), gtx.Constraints.Max.X)
}

// inputText is the editor and its placeholder inside a frame's padding.
type inputText struct {
	theme                   *Theme
	editor                  *widget.Editor
	placeholder             string
	font                    font.Font
	size                    unit.Sp
	padLeft, padRight, padY unit.Dp
	single                  bool
	// minHeight is the frame's floor (min-h-16) for a field that grows.
	minHeight unit.Dp
}

// layout draws the text and returns the frame's size. A single-line field is
// exactly height tall with its line centered; a growing one is as tall as its
// text plus padding, between minHeight and the maximum constraint.
func (f inputText) layout(gtx C, height int) D {
	width := gtx.Constraints.Max.X
	left, right, padY := gtx.Dp(f.padLeft), gtx.Dp(f.padRight), gtx.Dp(f.padY)
	inner := image.Pt(max(width-left-right, 0), 0)
	growMin, growMax := max(gtx.Dp(f.minHeight)-2*padY, 0), max(gtx.Constraints.Max.Y-2*padY, 0)
	gtx.Constraints.Min = image.Pt(inner.X, 0)
	gtx.Constraints.Max = image.Pt(inner.X, growMax)
	if f.single {
		gtx.Constraints.Max.Y = max(height-2*padY, 0)
	} else {
		gtx.Constraints.Min.Y = growMin
	}
	macro := op.Record(gtx.Ops)
	d := f.layoutEditor(gtx)
	call := macro.Stop()
	h, y := height, padY
	if f.single {
		y = (height - d.Size.Y) / 2
	} else {
		h = d.Size.Y + 2*padY
	}
	defer op.Offset(image.Pt(left, y)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return D{Size: image.Pt(width, h)}
}

// layoutEditor is material.EditorStyle's recipe: the placeholder is measured
// first so the editor is at least as wide and tall as it, then drawn where an
// empty editor's text would be.
func (f inputText) layoutEditor(gtx C) D {
	th := f.theme
	p := th.Palette
	// The highlight lands on the fill, under the glyphs.
	text, hint, selection := p.Foreground, p.MutedForeground, th.On(inputFill(th)).Over(Alpha(p.Primary, inputSelectionAlpha))
	if !gtx.Enabled() {
		text, hint, selection = th.fade(text), th.fade(hint), th.fade(selection)
	}
	textMat, hintMat, selMat := inputMaterial(gtx, text), inputMaterial(gtx, hint), inputMaterial(gtx, selection)
	maxLines := 0
	if f.single {
		maxLines = 1
	}
	hintMacro := op.Record(gtx.Ops)
	hd := widget.Label{MaxLines: maxLines, Truncator: "…"}.Layout(gtx, f.theme.Shaper, f.font, f.size, f.placeholder, hintMat)
	hintCall := hintMacro.Stop()
	gtx.Constraints.Min.Y = max(gtx.Constraints.Min.Y, hd.Size.Y)
	d := f.editor.Layout(gtx, f.theme.Shaper, f.font, f.size, textMat, selMat)
	if f.editor.Len() == 0 {
		hintCall.Add(gtx.Ops)
	}
	return d
}

func inputMaterial(gtx C, c color.NRGBA) op.CallOp {
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	return m.Stop()
}

// inputFrame draws the box around content: the focus ring outside it, the
// translucent fill, and the 1px border over the fill. Pressing anywhere in it,
// padding included, focuses the editor, as a click on an <input> does.
func inputFrame(gtx C, th *Theme, editor *widget.Editor, st inputState, borderless bool, content layout.Widget) D {
	macro := op.Record(gtx.Ops)
	d := content(gtx)
	call := macro.Stop()
	// Read after the editor ran: a focus change reaches it as an event of this
	// frame, and the ring must not trail the caret by one.
	st.focused = gtx.Focused(editor)
	col := inputPaintFor(th, st, borderless)
	radius := gtx.Dp(RadiusMD)
	if col.ring.A != 0 {
		inputStroke(gtx, d.Size, radius, float32(gtx.Dp(inputRingWidth)), true, col.ring)
	}
	paint.FillShape(gtx.Ops, col.fill, clip.UniformRRect(image.Rectangle{Max: d.Size}, radius).Op(gtx.Ops))
	if col.border.A != 0 {
		inputStroke(gtx, d.Size, radius, float32(gtx.Dp(inputBorderWidth)), false, col.border)
	}
	inputFocusOnPress(gtx, editor, d.Size)
	call.Add(gtx.Ops)
	return d
}

// inputStroke paints a rounded-rect band of width px that lies fully outside
// size (a box-shadow ring) or fully inside it (a border).
func inputStroke(gtx C, size image.Point, radius int, width float32, outside bool, c color.NRGBA) {
	grow := width / 2
	if !outside {
		grow = -grow
	}
	rect := image.Rectangle{Max: size.Add(image.Pt(int(2*grow), int(2*grow)))}
	rr := clip.UniformRRect(rect, radius+int(math.Round(float64(grow))))
	defer op.Affine(f32.Affine2D{}.Offset(f32.Pt(-grow, -grow))).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: width}.Op())
}

// inputFocusTag keys the press handler of one editor's frame.
type inputFocusTag struct{ editor *widget.Editor }

func inputFocusOnPress(gtx C, editor *widget.Editor, size image.Point) {
	tag := inputFocusTag{editor}
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press}); !ok {
			break
		}
		gtx.Execute(key.FocusCmd{Tag: editor})
	}
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	pointer.CursorText.Add(gtx.Ops)
	event.Op(gtx.Ops, tag)
}
