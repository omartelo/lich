package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// toolGlyphs is lib/session/tool-glyph.ts: the glyph a tool family wears,
// keyed by the names the harnesses report. An unknown name draws none.
var toolGlyphs = map[string]*icons.Icon{
	"Bash":         icons.Lucide("terminal"),
	"Read":         icons.Lucide("file-text"),
	"Edit":         icons.Lucide("pencil"),
	"Write":        icons.Lucide("pencil"),
	"NotebookEdit": icons.Lucide("pencil"),
	"apply_patch":  icons.Lucide("pencil"),
	"Grep":         icons.Lucide("search"),
	"Glob":         icons.Lucide("search"),
	"WebFetch":     icons.Lucide("globe"),
	"WebSearch":    icons.Lucide("globe"),
	"TodoWrite":    icons.Lucide("list-todo"),
}

var (
	icRelayOut   = icons.Lucide("arrow-right")
	icRelayIn    = icons.Lucide("arrow-left")
	icWaiting    = icons.Lucide("circle-question-mark")
	icCompacting = icons.Lucide("fold-vertical")
)

// minDetail is the width under which the tool's detail is not drawn at all,
// so its separator never dangles after a truncated name.
const minDetail = 24

// statusRung is SessionStatusRung, cut to the rungs this window keeps: an
// open request with another session, a block on the user, a compaction, and
// the tool the turn is running, the first that holds. It reports false when
// none does, and the card stays its usual size.
//
// ponytail: English text until the native window has i18n.
func statusRung(th *ui.Theme, sv *sessionView) (layout.Widget, bool) {
	switch {
	case sv.relay.Direction != "":
		return relayRung(th, sv.relay), true
	case sv.status == statusWaiting:
		reason := sv.reason
		if reason == "" {
			reason = "Waiting on you"
		}
		return rungLine(th, icWaiting, th.ToneWait, th.Text(ui.TextXS, reason).In(th.ToneWait).Weight(font.Medium)), true
	case sv.status == statusCompacting:
		return rungLine(th, icCompacting, th.MutedForeground, th.Text(ui.TextXS, "Compacting…").Weight(font.Medium)), true
	case sv.tool.Name != "":
		return toolRung(th, sv), true
	}
	return nil, false
}

func relayRung(th *ui.Theme, r sessionRelay) layout.Widget {
	ic := icRelayOut
	if r.Direction == relayIn {
		ic = icRelayIn
	}
	peer := th.Text(ui.TextXS, r.Peer).Weight(font.Medium)
	if r.Peer == "" {
		peer = th.Text(ui.TextXS, "command line").Muted().Italic()
	}
	return rungLine(th, ic, th.MutedForeground, peer)
}

// rungLine is a glyph and one truncating line of text-xs.
func rungLine(th *ui.Theme, ic *icons.Icon, glyph color.NRGBA, text ui.LabelStyle) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 12, glyph) }),
			ui.Gap(1),
			layout.Flexed(1, text.Layout),
		)
	}
}

// toolRung is the tool's glyph, its label and the detail the harness sent.
// The detail gives its width up first, and the label only once the detail
// has none left to give.
func toolRung(th *ui.Theme, sv *sessionView) layout.Widget {
	label, detail := toolLine(sv.tool, sv.MCPServers)
	return func(gtx C) D {
		var children []layout.FlexChild
		if ic, ok := toolGlyphs[sv.tool.Name]; ok {
			children = append(children, layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 12, th.MutedForeground) }), ui.Gap(1))
		}
		children = append(children, layout.Rigid(th.Text(ui.TextXS, label).Weight(font.Medium).Layout))
		if detail != "" {
			children = append(children, layout.Flexed(1, func(gtx C) D {
				if gtx.Constraints.Max.X < gtx.Dp(minDetail) {
					return D{}
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					ui.Gap(1),
					layout.Rigid(th.Text(ui.TextXS, "·").In(th.Over(ui.Alpha(th.MutedForeground, 0.5))).Layout),
					ui.Gap(1),
					layout.Flexed(1, th.Text(ui.TextXS, detail).Muted().Mono().Layout),
				)
			}))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
}
