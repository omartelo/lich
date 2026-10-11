package ui

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui/icons"
)

// The inline addon classes of input-group.tsx: pl-2 / pr-2, gap-2, and the
// 1.5 the control gives up beside an addon (has-[>input]:pl-1.5).
const inputGroupAddonGap = 2

var (
	inputGroupAddonPad    = Space(2)
	inputGroupButtonPull  = Space(1) // has-[>button]:-ml-1
	inputGroupControlNear = Space(1.5)
	inputGroupIconSize    = Space(4)
)

// InputGroupAddon is an inline-start or inline-end addon: its items in a row,
// gap-2 apart. Button is has-[>button]: the addon holds a button, so it sits
// 4px closer to the edge.
type InputGroupAddon struct {
	Items  []layout.Widget
	Button bool
}

// InputGroupStyle draws shadcn's InputGroup: one box, ring and border around
// an editor and the addons on its sides. The control sheds its own box, so
// the group's focus ring and invalid colors are the only ones. Pressing an
// addon focuses the editor; a button inside it takes its own press.
// Disabled lays the whole group out disabled, so the buttons inside go inert
// too; the items are the caller's and keep their own colors.
type InputGroupStyle struct {
	Editor      *widget.Editor
	Placeholder string
	Start, End  InputGroupAddon
	Width       unit.Dp // 0 is w-full
	Disabled    bool
	Invalid     bool
	multiline   bool
	theme       *Theme
}

// InputGroup is an InputGroupInput: a one-line control, h-9.
func (th *Theme) InputGroup(editor *widget.Editor, placeholder string) InputGroupStyle {
	return InputGroupStyle{Editor: editor, Placeholder: placeholder, theme: th}
}

// InputGroupTextarea is an InputGroupTextarea: the group grows with the text
// (has-[>textarea]:h-auto) and its addons center against it.
func (th *Theme) InputGroupTextarea(editor *widget.Editor, placeholder string) InputGroupStyle {
	return InputGroupStyle{Editor: editor, Placeholder: placeholder, multiline: true, theme: th}
}

// Inner is the theme an addon's items are built with: it draws on the group's
// fill, so a hover or a border an item composites lands on what is under it.
// Call it after setting Disabled, which fades the fill.
func (s InputGroupStyle) Inner() *Theme {
	return s.theme.On(inputPaintFor(s.theme, inputState{disabled: s.Disabled}, false).fill)
}

func (s InputGroupStyle) Layout(gtx C) D {
	if s.Disabled {
		gtx = gtx.Disabled()
	}
	s.Editor.SingleLine = !s.multiline
	gtx.Constraints.Max.X = inputWidth(gtx, s.Width)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	st := inputState{invalid: s.Invalid, disabled: s.Disabled}
	return inputFrame(gtx, s.theme, s.Editor, st, false, s.row)
}

func (s InputGroupStyle) row(gtx C) D {
	gtx.Constraints.Min.Y = 0
	h := gtx.Dp(inputHeight)
	field := inputText{theme: s.theme, editor: s.Editor, placeholder: s.Placeholder, size: TextSM,
		font: font.Font{Typeface: s.theme.Sans}, padLeft: inputPadX, padRight: inputPadX, padY: inputPadY, single: !s.multiline}
	if s.multiline {
		field.padY, field.minHeight = textareaPadY, textareaMinHeight
	}
	if len(s.Start.Items) > 0 {
		field.padLeft = inputGroupControlNear
	}
	if len(s.End.Items) > 0 {
		field.padRight = inputGroupControlNear
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(s.Start.layout(true)),
		layout.Flexed(1, func(gtx C) D { return field.layout(gtx, h) }),
		layout.Rigid(s.End.layout(false)),
	)
}

func (a InputGroupAddon) layout(start bool) layout.Widget {
	return func(gtx C) D {
		if len(a.Items) == 0 {
			return D{}
		}
		pad := inputGroupAddonPad
		if a.Button {
			pad -= inputGroupButtonPull
		}
		in := layout.Inset{Right: pad}
		if start {
			in = layout.Inset{Left: pad}
		}
		return in.Layout(gtx, func(gtx C) D {
			var kids []layout.FlexChild
			for i, item := range a.Items {
				if i > 0 {
					kids = append(kids, Gap(inputGroupAddonGap))
				}
				kids = append(kids, layout.Rigid(item))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
	}
}

// InputGroupText is an InputGroupText inside an addon: text-sm
// text-muted-foreground, medium weight from the addon.
func (th *Theme) InputGroupText(s string) layout.Widget {
	return th.Text(TextSM, s).Muted().Weight(font.Medium).Layout
}

// InputGroupIcon is an icon inside an addon: size-4, muted.
func (th *Theme) InputGroupIcon(ic *icons.Icon) layout.Widget {
	return func(gtx C) D { return ic.Layout(gtx, inputGroupIconSize, th.MutedForeground) }
}

// InputGroupButton is an InputGroupButton: a ghost, xs button. Set Size to
// SizeIconXS or SizeIconSM for a bare icon, and Icon for its glyph.
func (th *Theme) InputGroupButton(click *widget.Clickable, text string) ButtonStyle {
	b := th.Button(click, text)
	b.Variant, b.Size = VariantGhost, SizeXS
	return b
}
