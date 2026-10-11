package main

import (
	"strconv"

	"gioui.org/font"

	"gioui.org/layout"

	"github.com/omartelo/lich/native/ui"
)

// The dark half of DiffStat.tsx's text-emerald-600 dark:text-emerald-400 and
// text-red-600 dark:text-red-400: Tailwind colors, not theme tokens.
var (
	colAdded   = ui.OKLCH(0.765, 0.177, 163.223, 1)
	colDeleted = ui.OKLCH(0.704, 0.191, 22.216, 1)
)

// diffStat is the +added -deleted pair every diff surface shows, spaced by
// the gap-1.5 of the clusters it sits in.
func diffStat(gtx C, th *ui.Theme, added, deleted int) D {
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextXS, "+"+strconv.Itoa(added)).In(colAdded).Weight(font.Medium).Layout),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextXS, "-"+strconv.Itoa(deleted)).In(colDeleted).Weight(font.Medium).Layout),
	)
}
