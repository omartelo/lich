package ui

import (
	"image"
	"image/color"
	"math"
	"time"

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

// The classes of dialog.tsx's DialogOverlay and DialogContent.
const (
	dialogTransition  = 100 * time.Millisecond // duration-100
	dialogRadius      = unit.Dp(12)            // rounded-xl
	dialogRingOpacity = 0.1                    // ring-foreground/10
	dialogScrim       = 0.1                    // bg-black/10
)

// dialogScrimOpacity is bg-black/10 as Gio draws it. The scrim covers
// whatever the window holds, not a known surface, so it cannot be composited
// with Theme.Over and stays translucent. A browser blends in sRGB, scaling
// every channel of what is under the scrim by 0.9; Gio blends in linear light,
// where the same scaling takes black at 1-0.9^2.4 (sRGB's gamma), exact
// everywhere but the near-black linear toe of the curve.
var dialogScrimOpacity = float32(1 - math.Pow(1-dialogScrim, 2.4))

var black = color.NRGBA{A: 0xff}

// Dialog widths: sm:max-w-md, the DialogContent default, and the
// sm:max-w-xl ConfirmDialog widens it to.
const (
	DialogWidthMD unit.Dp = 448
	DialogWidthXL unit.Dp = 576
)

// DialogState is a dialog's widget state, owned by the caller like a
// widget.Clickable. The open flag is not part of it: the caller owns that
// and passes it to every Layout.
type DialogState struct {
	close widget.Clickable
	scrim popupBackdrop
	panel bool // the panel's event tag
	shown transition
}

// DialogStyle draws shadcn's Dialog: a scrim over the whole window and a
// centered panel holding a header (title, description), an optional body and
// a right-aligned footer of actions, with the × in the top right corner.
// Body and Actions draw on the popover: build them with th.On(th.Popover).
type DialogStyle struct {
	State       *DialogState
	Open        bool
	Title       string
	Description string
	Body        layout.Widget   // optional
	Actions     []layout.Widget // the footer, left to right
	MaxWidth    unit.Dp
	// HideClose drops the corner ×, showCloseButton={false}.
	HideClose bool
	theme     *Theme
}

// Dialog is a DialogStyle at the DialogContent default width.
func (th *Theme) Dialog(state *DialogState, open bool, title, description string) DialogStyle {
	return DialogStyle{State: state, Open: open, Title: title, Description: description, MaxWidth: DialogWidthMD, theme: th}
}

// Layout handles the dialog's input and defers its drawing above everything
// laid out in the frame; it occupies no space where it is called. Call it with
// the window's constraints and transform, and before the content under it, so
// it claims Escape ahead of any key filter there. It reports true when the
// user dismissed the dialog (Escape, a press on the scrim, the ×); the caller
// then closes it by passing Open false from the next frame on.
func (d DialogStyle) Layout(gtx C) (dismissed bool) {
	s := d.State
	dismissed = d.Open && s.dismissed(gtx)
	if s.shown.set(gtx, d.Open && !dismissed) && s.shown.open {
		gtx.Execute(key.FocusCmd{Tag: &s.panel})
	}
	amount, ok := s.shown.progress(gtx, dialogTransition)
	if !ok {
		return dismissed
	}
	macro := op.Record(gtx.Ops)
	d.overlay(gtx, amount)
	op.Defer(gtx.Ops, macro.Stop())
	return dismissed
}

func (s *DialogState) dismissed(gtx C) bool {
	dismissed := s.close.Clicked(gtx)
	if _, ok := gtx.Event(key.Filter{Name: key.NameEscape}); ok {
		dismissed = true
	}
	if s.scrim.pressed(gtx) {
		dismissed = true
	}
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &s.panel}, pointer.Filter{Target: &s.panel, Kinds: pointer.Press}); !ok {
			break
		}
	}
	return dismissed
}

func (d DialogStyle) overlay(gtx C, amount float32) {
	if amount < 1 {
		defer paint.PushOpacity(gtx.Ops, amount).Pop()
	}
	window := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, Alpha(black, dialogScrimOpacity), clip.Rect{Max: window}.Op())
	d.State.scrim.layout(gtx, window)

	panelGtx := gtx
	width := min(gtx.Dp(d.MaxWidth), window.X-gtx.Dp(Space(8)))
	panelGtx.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, window.Y)}
	macro := op.Record(gtx.Ops)
	size := d.panel(panelGtx).Size
	panel := macro.Stop()

	origin := window.Sub(size).Div(2)
	center := f32.Pt(float32(window.X)/2, float32(window.Y)/2)
	zoom := zoomFrom + (1-zoomFrom)*amount
	defer op.Affine(f32.Affine2D{}.Scale(center, f32.Pt(zoom, zoom))).Push(gtx.Ops).Pop()
	defer op.Offset(origin).Push(gtx.Ops).Pop()
	panel.Add(gtx.Ops)
}

// panel draws the ring, the surface and the content at the origin. The ring
// sits over the scrim, composited as if the window under it were the
// surface the dialog was laid out on.
func (d DialogStyle) panel(gtx C) D {
	ring := Over(d.theme.Over(Alpha(black, dialogScrim)), Alpha(d.theme.Foreground, dialogRingOpacity))
	d.theme = d.theme.On(d.theme.Popover)
	macro := op.Record(gtx.Ops)
	content := d.content(gtx)
	call := macro.Stop()
	size := content.Size

	ringWidth := gtx.Dp(1)
	outer := clip.UniformRRect(image.Rectangle{Min: image.Pt(-ringWidth, -ringWidth), Max: size.Add(image.Pt(ringWidth, ringWidth))}, gtx.Dp(dialogRadius)+ringWidth)
	paint.FillShape(gtx.Ops, ring, outer.Op(gtx.Ops))
	surface := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(dialogRadius)).Push(gtx.Ops)
	paint.ColorOp{Color: d.theme.Popover}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	event.Op(gtx.Ops, &d.State.panel)
	call.Add(gtx.Ops)
	if !d.HideClose {
		d.closeButton(gtx, size.X)
	}
	surface.Pop()
	return content
}

// content is the grid of DialogContent: p-6 gap-6 over header, body, footer.
func (d DialogStyle) content(gtx C) D {
	children := []layout.FlexChild{layout.Rigid(d.header)}
	if d.Body != nil {
		children = append(children, VGap(6), layout.Rigid(d.Body))
	}
	if len(d.Actions) > 0 {
		children = append(children, VGap(6), layout.Rigid(d.footer))
	}
	return layout.UniformInset(Space(6)).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// header is DialogHeader (flex-col gap-2) over DialogTitle (leading-none
// font-medium) and DialogDescription (text-sm text-muted-foreground); both
// wrap.
func (d DialogStyle) header(gtx C) D {
	title := d.theme.Text(TextSM, d.Title).Weight(font.Medium).Leading(TextSM)
	description := d.theme.Text(TextSM, d.Description).Muted()
	title.MaxLines, description.MaxLines = 0, 0
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(title.Layout),
		VGap(2),
		layout.Rigid(description.Layout),
	)
}

// footer is DialogFooter at sm and up: flex-row justify-end gap-2.
func (d DialogStyle) footer(gtx C) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	children := make([]layout.FlexChild, 0, 2*len(d.Actions))
	for i, action := range d.Actions {
		if i > 0 {
			children = append(children, Gap(2))
		}
		children = append(children, layout.Rigid(action))
	}
	return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx, children...)
}

// closeButton is the ghost icon-sm × at absolute top-4 right-4.
func (d DialogStyle) closeButton(gtx C, panelWidth int) {
	b := d.theme.Button(&d.State.close, "")
	b.Icon, b.Variant, b.Size = icons.Lucide("x"), VariantGhost, SizeIconSM
	inset := gtx.Dp(Space(4))
	defer op.Offset(image.Pt(panelWidth-inset-gtx.Dp(buttonSizes[SizeIconSM].height), inset)).Push(gtx.Ops).Pop()
	gtx.Constraints.Min = image.Point{}
	b.Layout(gtx)
}
