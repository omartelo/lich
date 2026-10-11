package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// closeButton is common/CloseButton.tsx: the × a card shows while hovered
// (size-4, rounded, hover:bg-foreground/15, an X at size-3). Hidden, it still
// takes clicks, as the web's opacity-0 span does.
func closeButton(gtx C, th *ui.Theme, click *widget.Clickable, ic *icons.Icon, revealed bool) D {
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(16), gtx.Dp(16)))
		if !revealed && !click.Hovered() {
			return D{Size: gtx.Constraints.Min}
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				if !click.Hovered() {
					return D{Size: gtx.Constraints.Min}
				}
				return ui.Fill(gtx, ui.Alpha(th.Foreground, 0.15), unit.Dp(4))
			},
			func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, 12, th.MutedForeground) })
			},
		)
	})
}
