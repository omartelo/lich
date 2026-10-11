package main

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// sidebarWidth matches the web sidebar's default 18rem.
const sidebarWidth = 288

type sidebar struct {
	root    *ui.Root // where card tooltips float
	tips    map[string]*ui.Tooltip
	menus   map[string]*ui.MenuState
	renames map[string]*ui.InlineEdit
	// renamed is the label a rename committed during this frame's layout.
	renamed   sessionRename
	newShell  widget.Clickable
	newClaude widget.Clickable
	list      widget.List
	cards     map[string]*widget.Clickable
	closes    map[string]*widget.Clickable
	pins      map[string]*widget.Clickable
	prChips   map[string]*widget.Clickable
	folds     map[string]*widget.Clickable
	collapsed map[string]bool
	icPlus    *icons.Icon
	icChev    *icons.Icon
	icOpen    *icons.Icon
	icBranch  *icons.Icon
	icClose   *icons.Icon
	icPin     *icons.Icon
	icShield  *icons.Icon
	icBehind  *icons.Icon
	icClash   *icons.Icon
	icPR      *icons.Icon
	started   time.Time
}

func newSidebar(root *ui.Root) *sidebar {
	s := &sidebar{
		root:      root,
		cards:     map[string]*widget.Clickable{},
		closes:    map[string]*widget.Clickable{},
		pins:      map[string]*widget.Clickable{},
		tips:      map[string]*ui.Tooltip{},
		menus:     map[string]*ui.MenuState{},
		renames:   map[string]*ui.InlineEdit{},
		prChips:   map[string]*widget.Clickable{},
		folds:     map[string]*widget.Clickable{},
		collapsed: map[string]bool{},
		icPlus:    icons.Lucide("plus"),
		icChev:    icons.Lucide("chevron-right"),
		icOpen:    icons.Lucide("chevron-down"),
		icBranch:  icons.Lucide("git-branch"),
		icClose:   icons.Lucide("x"),
		icPin:     icons.Lucide("pin"),
		icShield:  icons.Lucide("shield"),
		icBehind:  icons.Lucide("arrow-down"),
		icClash:   icons.Lucide("triangle-alert"),
		icPR:      icons.Lucide("git-pull-request-arrow"),
		started:   time.Now(),
	}
	s.list.Axis = layout.Vertical
	return s
}

// holdsKeys reports whether a card is being renamed or has its menu open,
// either of which wants the keyboard the terminal otherwise takes.
func (s *sidebar) holdsKeys() bool {
	for _, r := range s.renames {
		if r.Active() {
			return true
		}
	}
	for _, m := range s.menus {
		if m.Opened() {
			return true
		}
	}
	return false
}

// sidebarAction is what a frame's clicks asked for.
type sidebarAction struct {
	activate string // session id
	close    string // session id
	pin      string // session id
	pinTo    bool   // the pin the click asked for
	openURL  string // a pull request to show in the browser
	newKind  string // "shell" or "claude"
	// Open in, from the card menu: a directory for a shell session, the
	// editor or the file manager.
	terminalAt, editorAt, folderAt string
	rename                         sessionRename
	color, colorTo                 string // session id, and its new tint ("" for the theme)
}

// sessionRename is a label typed into a card.
type sessionRename struct{ id, label string }

func (s *sidebar) layout(gtx C, th *ui.Theme, groups []group, active string) (D, sidebarAction) {
	var act sidebarAction
	if s.newShell.Clicked(gtx) {
		act.newKind = "shell"
	}
	if s.newClaude.Clicked(gtx) {
		act.newKind = "claude"
	}
	gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(sidebarWidth), gtx.Constraints.Max.Y))
	dims := layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusLG) },
		func(gtx C) D {
			th := th.On(th.Sidebar)
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return s.toolbar(gtx, th) }),
					ui.VGap(2),
					layout.Flexed(1, func(gtx C) D {
						d, a := s.items(gtx, th, groups, active)
						a.newKind = act.newKind
						act = a
						return d
					}),
				)
			})
		},
	)
	return dims, act
}

func (s *sidebar) toolbar(gtx C, th *ui.Theme) D {
	button := func(c *widget.Clickable, text string) layout.FlexChild {
		return layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			b := th.Button(c, text)
			b.Icon, b.Variant, b.Size = s.icPlus, ui.VariantGhost, ui.SizeSM
			return b.Layout(gtx)
		})
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		button(&s.newShell, "Shell"),
		ui.Gap(1),
		button(&s.newClaude, "Claude"),
	)
}

type row struct {
	group   *group
	session *sessionView
}

func (s *sidebar) items(gtx C, th *ui.Theme, groups []group, active string) (D, sidebarAction) {
	var rows []row
	for gi := range groups {
		g := &groups[gi]
		rows = append(rows, row{group: g})
		if s.collapsed[g.path] {
			continue
		}
		for si := range g.sessions {
			rows = append(rows, row{group: g, session: &g.sessions[si]})
		}
	}
	var act sidebarAction
	d := th.ScrollArea(&s.list).Layout(gtx, len(rows), func(gtx C, i int) D {
		r := rows[i]
		top := unit.Dp(6)
		switch {
		case i == 0:
			top = 0
		case r.session == nil:
			top = 12
		}
		return layout.Inset{Top: top}.Layout(gtx, func(gtx C) D {
			if r.session == nil {
				return s.groupHeader(gtx, th, r.group)
			}
			d, a := s.card(gtx, th, r.session, r.session.ID == active)
			// One card is clicked per frame at most.
			if a != (sidebarAction{}) {
				act = a
			}
			return d
		})
	})
	return d, act
}

func clickable(m map[string]*widget.Clickable, key string) *widget.Clickable {
	c, ok := m[key]
	if !ok {
		c = new(widget.Clickable)
		m[key] = c
	}
	return c
}

func (s *sidebar) groupHeader(gtx C, th *ui.Theme, g *group) D {
	click := clickable(s.folds, g.path)
	if click.Clicked(gtx) {
		s.collapsed[g.path] = !s.collapsed[g.path]
	}
	nameColor := th.Over(ui.Alpha(th.MutedForeground, 0.7))
	if click.Hovered() {
		nameColor = th.MutedForeground
	}
	return click.Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: 2, Bottom: 2, Left: 4, Right: 4}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					ic := s.icOpen
					if s.collapsed[g.path] {
						ic = s.icChev
					}
					return ic.Layout(gtx, 12, nameColor)
				}),
				ui.Gap(1.5),
				layout.Rigid(func(gtx C) D {
					return th.Text(11, strings.ToUpper(g.name)).In(nameColor).Weight(font.SemiBold).Layout(gtx)
				}),
				ui.Gap(1.5),
				layout.Flexed(1, th.Separator),
			)
		})
	})
}

func (s *sidebar) card(gtx C, th *ui.Theme, sv *sessionView, active bool) (D, sidebarAction) {
	var act sidebarAction
	click, closeClick, pinClick := clickable(s.cards, sv.ID), clickable(s.closes, sv.ID), clickable(s.pins, sv.ID)
	if click.Clicked(gtx) {
		act.activate = sv.ID
	}
	if closeClick.Clicked(gtx) {
		act.close = sv.ID
	}
	if pinClick.Clicked(gtx) {
		act.pin, act.pinTo = sv.ID, !sv.Pinned
	}
	prChip := clickable(s.prChips, sv.ID)
	if prChip.Clicked(gtx) && sv.pr != nil {
		act.openURL = sv.pr.URL
	}
	fill := cardFill(th, sv.Color, active, click.Hovered())
	th = th.On(fill)
	tip, ok := s.tips[sv.ID]
	if !ok {
		tip = new(ui.Tooltip)
		s.tips[sv.ID] = tip
	}
	menu, ok := s.menus[sv.ID]
	if !ok {
		menu = new(ui.MenuState)
		s.menus[sv.ID] = menu
	}
	tooltip := th.TooltipCard(s.root, tip, sessionTooltip(th, sv))
	tooltip.Side = ui.SideRight
	rename, ok := s.renames[sv.ID]
	if !ok {
		rename = new(ui.InlineEdit)
		s.renames[sv.ID] = rename
	}
	entries := cardMenu(sv, &act, func() { rename.Start(sv.Label) })
	d := th.ContextMenu(s.root, menu, entries).Layout(gtx, func(gtx C) D {
		return tooltip.Layout(gtx, func(gtx C) D { return s.cardFace(gtx, th, sv, click, pinClick, closeClick, prChip, fill) })
	})
	if s.renamed.id == sv.ID {
		act.rename, s.renamed = s.renamed, sessionRename{}
	}
	return d, act
}

// cardFace is the card itself, under its tooltip.
func (s *sidebar) cardFace(gtx C, th *ui.Theme, sv *sessionView, click, pinClick, closeClick, prChip *widget.Clickable, fill color.NRGBA) D {
	return click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, fill, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Stack{Alignment: layout.NE}.Layout(gtx,
					layout.Stacked(func(gtx C) D { return s.cardBody(gtx, th, sv, prChip) }),
					layout.Stacked(func(gtx C) D {
						return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
							return s.cardControls(gtx, th, sv, pinClick, closeClick, click.Hovered())
						})
					}),
				)
			},
		)
	})
}

func (s *sidebar) cardBody(gtx C, th *ui.Theme, sv *sessionView, prChip *widget.Clickable) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	path := displayPath(sv.shown)
	if sv.host != "" {
		path = unknownCwd(sv.host)
	}
	rows := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, s.titleRow(th, sv)...)
		}),
	}
	if rung, ok := statusRung(th, sv); ok {
		rows = append(rows, ui.VGap(2), layout.Rigid(rung))
	}
	rows = append(rows, ui.VGap(2), layout.Rigid(th.Text(ui.TextXS, path).Muted().Mono().LayoutTail))
	if sv.git.Branch != "" {
		rows = append(rows, ui.VGap(2), layout.Rigid(func(gtx C) D { return s.branchRow(gtx, th, sv, prChip) }))
	}
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// titleRow is the card's first line: the status ring around the provider
// mark, a shield on a session that runs sandboxed, and the label, which stops
// short of the corner controls (pr-11, pr-6 for a pinned card's lone pin).
func (s *sidebar) titleRow(th *ui.Theme, sv *sessionView) []layout.FlexChild {
	row := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return s.statusRing(gtx, th, sv) }),
		ui.Gap(1.5),
	}
	if sv.sandbox.Confined {
		row = append(row, layout.Rigid(func(gtx C) D { return s.icShield.Layout(gtx, 12, th.MutedForeground) }), ui.Gap(1.5))
	}
	controls := ui.Space(11)
	if sv.Pinned {
		controls = ui.Space(6)
	}
	if rename := s.renames[sv.ID]; rename != nil && rename.Active() {
		return append(row, layout.Flexed(1, func(gtx C) D { return s.renameField(gtx, th, sv.ID, rename, controls) }))
	}
	return append(row, layout.Flexed(1, func(gtx C) D {
		return layout.Inset{Right: controls}.Layout(gtx, th.Text(ui.TextSM, sv.Label).Weight(font.Medium).Layout)
	}))
}

// renameField is the label as a field while the card is being renamed; the
// padding sits inside its ring, as the web input's does.
func (s *sidebar) renameField(gtx C, th *ui.Theme, id string, rename *ui.InlineEdit, controls unit.Dp) D {
	field := th.InlineEdit(rename, ui.TextSM).Weight(font.Medium)
	field.PadRight = controls
	d, res := field.Layout(gtx)
	if res.Outcome == ui.InlineEditCommitted {
		s.renamed = sessionRename{id: id, label: res.Value}
	}
	return d
}

// cardControls is the card's top-right corner. At rest it holds how long the
// status has lasted, and the pin of a pinned card; hovered, the pin and the ×
// take its place. A pinned card has no ×: closing is what the pin withholds.
func (s *sidebar) cardControls(gtx C, th *ui.Theme, sv *sessionView, pin, close *widget.Clickable, hovered bool) D {
	var children []layout.FlexChild
	if !hovered && sv.aged {
		children = append(children, layout.Rigid(th.Text(ui.TextXS, formatAge(sv.age)).Muted().Layout))
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(nextAgeChange(sv.age))})
	}
	if hovered || sv.Pinned {
		if len(children) > 0 {
			children = append(children, ui.Gap(1))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return cornerButton(gtx, th, pin, s.icPin, sv.Pinned) }))
	}
	if hovered && !sv.Pinned {
		children = append(children, ui.Gap(1), layout.Rigid(func(gtx C) D { return cornerButton(gtx, th, close, s.icClose, false) }))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// branchRow is the card's last line: the branch, and on the right a cluster
// that reads outward from it: where it stands against its base, its open
// pull request, and the checkout's changes.
func (s *sidebar) branchRow(gtx C, th *ui.Theme, sv *sessionView, prChip *widget.Clickable) D {
	git := sv.git
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return s.icBranch.Layout(gtx, 12, th.MutedForeground) }),
		ui.Gap(1),
		layout.Flexed(1, th.Text(ui.TextXS, git.Branch).Muted().Layout),
	}
	if standing := standingOf(sv.base); standing.count > 0 {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return s.baseReadout(gtx, th, standing) }))
	}
	if sv.pr != nil {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return s.prBadge(gtx, th, prChip, sv.pr.Number) }))
	}
	if git.Files > 0 {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return diffStat(gtx, th, git.Added, git.Deleted) }))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// baseReadout is ↓N commits behind the base, or ⚠N files that would conflict
// with it, in the waiting tone.
func (s *sidebar) baseReadout(gtx C, th *ui.Theme, b baseStanding) D {
	ic, c := s.icBehind, th.MutedForeground
	if b.conflict {
		ic, c = s.icClash, th.ToneWait
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 12, c) }),
		ui.Gap(0.5),
		layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(b.count)).In(c).Layout),
	)
}

// prBadge is the open pull request's number, a button that brightens on
// hover.
func (s *sidebar) prBadge(gtx C, th *ui.Theme, click *widget.Clickable, number int) D {
	c := th.MutedForeground
	if click.Hovered() {
		c = th.Foreground
	}
	return click.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return s.icPR.Layout(gtx, 12, c) }),
			ui.Gap(1),
			layout.Rigid(th.Text(ui.TextXS, "#"+strconv.Itoa(number)).In(c).Layout),
		)
	})
}

// statusRing is SessionStatusIcon: the provider mark (the agent live in the
// PTY, else the session's kind) in a ring that spins while busy, is amber
// when waiting on the user and emerald when done, faded once that turn is
// read.
func (s *sidebar) statusRing(gtx C, th *ui.Theme, sv *sessionView) D {
	size := gtx.Dp(22)
	stroke := float32(gtx.Dp(1.5))
	center := f32.Pt(float32(size)/2, float32(size)/2)
	radius := float32(size)/2 - stroke/2
	switch sv.status {
	case "busy", "compacting":
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.Over(ui.Alpha(th.MutedForeground, 0.25)))
		turn := float32(time.Since(s.started).Seconds()) * 2 * math.Pi
		strokeArc(gtx, center, radius, stroke, turn, math.Pi/2, th.MutedForeground)
		gtx.Execute(op.InvalidateCmd{})
	case "waiting":
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.ToneWait)
	case "done":
		ring := th.TonePass
		if !sv.unread {
			ring = th.Over(ui.Alpha(ring, 0.3))
		}
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, ring)
	}
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	kind := sv.Kind
	if sv.agent != "" {
		kind = sv.agent
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return providerIcon(kind).Layout(gtx, 14, th.MutedForeground)
	})
}

func strokeArc(gtx C, center f32.Point, radius, width, start, sweep float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	from := f32.Pt(center.X+radius*float32(math.Cos(float64(start))), center.Y+radius*float32(math.Sin(float64(start))))
	p.MoveTo(from)
	p.ArcTo(center, center, sweep)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}
