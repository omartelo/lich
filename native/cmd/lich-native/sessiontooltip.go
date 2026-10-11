package main

import (
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// namedConflicts is base-status.ts NAMED_CONFLICTS: how many conflicting
// files the tooltip names before counting the rest.
const namedConflicts = 3

var (
	icSandboxed = icons.Lucide("shield")
	icConflict  = icons.Lucide("triangle-alert")
	icBranch    = icons.Lucide("git-branch")
	icPR        = icons.Lucide("git-pull-request-arrow")
)

// sessionTooltip is SessionTooltip: everything the card knows, in words. Cut
// to the sections this window keeps: the full path, the open request and its
// ticket, what the shield means, the branch, and the base standing spelled
// out with the files that would conflict.
//
// ponytail: English text until the native window has i18n.
func sessionTooltip(th *ui.Theme, sv *sessionView) layout.Widget {
	th = th.On(th.Popover)
	path := sv.shown
	if sv.host != "" {
		path = unknownCwd(sv.host)
	}
	sections := []layout.Widget{
		th.Text(ui.TextXS, sv.Label).Weight(font.Medium).Layout,
		para(th.Text(ui.TextXS, path).Muted().Mono()).Layout,
	}
	if sv.relay.Direction != "" {
		sections = append(sections, relaySection(th, sv.relay))
	}
	if sv.sandbox.Confined {
		sections = append(sections, sandboxSection(th, sv.sandbox))
	}
	if sv.git.Branch != "" {
		sections = append(sections, branchSection(th, sv))
	}
	sections = append(sections, baseSections(th, sv.base)...)
	return func(gtx C) D { return stack(gtx, 1.5, sections) }
}

// para lets a label wrap instead of truncating.
func para(l ui.LabelStyle) ui.LabelStyle {
	l.MaxLines = 0
	return l
}

// stack lays widgets out top to bottom, gap apart (flex-col gap-N).
func stack(gtx C, gap float32, ws []layout.Widget) D {
	children := make([]layout.FlexChild, 0, 2*len(ws))
	for i, w := range ws {
		if i > 0 {
			children = append(children, ui.VGap(gap))
		}
		children = append(children, layout.Rigid(w))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// iconLine is a glyph and what follows it, gap-1.5 apart.
func iconLine(gtx C, ic *icons.Icon, glyph ui.LabelStyle, rest ...layout.FlexChild) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 12, glyph.Color) }),
		ui.Gap(1.5),
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, append(children, rest...)...)
}

// relaySection names the other end and the ticket the request runs on, the
// one place a person can read it; the side that owes the answer also gets
// the reply command to hand back to a session that lost the instruction.
func relaySection(th *ui.Theme, r sessionRelay) layout.Widget {
	verb, ic := "Waiting on", icRelayOut
	if r.Direction == relayIn {
		verb, ic = "Answering", icRelayIn
	}
	peer := th.Text(ui.TextXS, r.Peer).Weight(font.Medium)
	if r.Peer == "" {
		peer = th.Text(ui.TextXS, "the command line").Muted().Italic()
	}
	muted := th.Text(ui.TextXS, verb).Muted()
	rows := []layout.Widget{func(gtx C) D {
		return iconLine(gtx, ic, muted, layout.Rigid(muted.Layout), ui.Gap(1), layout.Rigid(peer.Layout))
	}}
	if r.Ticket != "" {
		rows = append(rows, th.Text(ui.TextXS, "ticket "+r.Ticket).Mono().Layout)
	}
	if r.Ticket != "" && r.Direction == relayIn {
		rows = append(rows, para(th.Text(ui.TextXS, `lich reply `+r.Ticket+` "…"`).Muted().Mono()).Layout)
	}
	return func(gtx C) D { return stack(gtx, 0.5, rows) }
}

// sandboxSection is what the card's shield means, that it was settled when
// the session opened, and what the sandbox left out for being a symlink.
func sandboxSection(th *ui.Theme, s sandboxEvent) layout.Widget {
	title := th.Text(ui.TextXS, "Sandboxed")
	rows := []layout.Widget{
		func(gtx C) D { return iconLine(gtx, icSandboxed, title, layout.Rigid(title.Layout)) },
		para(th.Text(ui.TextXS, "Empty home, machine read-only, writes only in this checkout. Set when the session opened; reopen it to change.").Muted()).Layout,
	}
	if len(s.SkippedLinks) > 0 {
		rows = append(rows, para(th.Text(ui.TextXS, "Not mounted (symlinks): "+strings.Join(s.SkippedLinks, ", ")).Muted()).Layout)
	}
	return func(gtx C) D { return stack(gtx, 0.5, rows) }
}

func branchSection(th *ui.Theme, sv *sessionView) layout.Widget {
	return func(gtx C) D {
		muted := th.Text(ui.TextXS, sv.git.Branch).Muted()
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return iconLine(gtx, icBranch, muted, layout.Rigid(muted.Layout)) }),
		}
		if sv.pr != nil {
			number := th.Text(ui.TextXS, "#"+strconv.Itoa(sv.pr.Number)).Muted()
			children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D {
				return iconLine(gtx, icPR, number, layout.Rigid(number.Layout))
			}))
		}
		if sv.git.Files > 0 {
			children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return diffStat(gtx, th, sv.git.Added, sv.git.Deleted) }))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// baseSections spell out base-status.ts baseReadout: commits behind the
// named base, then the conflict, then the first files it is in.
func baseSections(th *ui.Theme, b *lichclient.BaseStatus) []layout.Widget {
	if b == nil {
		return nil
	}
	var out []layout.Widget
	if b.Behind > 0 {
		out = append(out, para(th.Text(ui.TextXS, plural(b.Behind, "commit", "commits")+" behind "+b.Base).Muted()).Layout)
	}
	if n := len(b.Conflicts); n > 0 {
		line := th.Text(ui.TextXS, "Conflicts in "+plural(n, "file", "files")).In(th.ToneWait)
		out = append(out,
			func(gtx C) D { return iconLine(gtx, icConflict, line, layout.Flexed(1, line.Layout)) },
			conflictPaths(th, b.Conflicts),
		)
	}
	return out
}

func conflictPaths(th *ui.Theme, paths []string) layout.Widget {
	var rows []layout.Widget
	for _, p := range paths[:min(len(paths), namedConflicts)] {
		rows = append(rows, para(th.Text(ui.TextXS, p).Muted().Mono()).Layout)
	}
	if more := len(paths) - namedConflicts; more > 0 {
		rows = append(rows, th.Text(ui.TextXS, "+"+strconv.Itoa(more)+" more").Muted().Layout)
	}
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(4)}.Layout(gtx, func(gtx C) D { return stack(gtx, 0.5, rows) })
	}
}

func plural(n int, one, other string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + other
}
