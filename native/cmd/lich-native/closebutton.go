package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// cornerButton is common/CloseButton.tsx, the shape of both controls in a
// card's corner: size-4, rounded, hover:bg-foreground/15, a glyph at size-3.
// On is the set pin: the glyph in the foreground and filled.
func cornerButton(gtx C, th *ui.Theme, click *widget.Clickable, ic *icons.Icon, on bool) D {
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(16), gtx.Dp(16)))
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				if !click.Hovered() {
					return D{Size: gtx.Constraints.Min}
				}
				return ui.Fill(gtx, th.Over(ui.Alpha(th.Foreground, 0.15)), unit.Dp(4))
			},
			func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					if on {
						return ic.LayoutFilled(gtx, 12, th.Foreground)
					}
					return ic.Layout(gtx, 12, th.MutedForeground)
				})
			},
		)
	})
}
