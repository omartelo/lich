package main

import (
	"image"
	"image/color"
	"math"
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
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// sidebarWidth matches the web sidebar's default 18rem.
const sidebarWidth = 288

type sidebar struct {
	newShell  widget.Clickable
	newClaude widget.Clickable
	list      widget.List
	cards     map[string]*widget.Clickable
	closes    map[string]*widget.Clickable
	folds     map[string]*widget.Clickable
	collapsed map[string]bool
	icPlus    *icons.Icon
	icChev    *icons.Icon
	icOpen    *icons.Icon
	icBranch  *icons.Icon
	icClose   *icons.Icon
	started   time.Time
}

func newSidebar() *sidebar {
	s := &sidebar{
		cards:     map[string]*widget.Clickable{},
		closes:    map[string]*widget.Clickable{},
		folds:     map[string]*widget.Clickable{},
		collapsed: map[string]bool{},
		icPlus:    icons.Lucide("plus"),
		icChev:    icons.Lucide("chevron-right"),
		icOpen:    icons.Lucide("chevron-down"),
		icBranch:  icons.Lucide("git-branch"),
		icClose:   icons.Lucide("x"),
		started:   time.Now(),
	}
	s.list.Axis = layout.Vertical
	return s
}

// sidebarAction is what a frame's clicks asked for.
type sidebarAction struct {
	activate string // session id
	close    string // session id
	newKind  string // "shell" or "claude"
}

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
						act.activate, act.close = a.activate, a.close
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
			if a.activate != "" {
				act.activate = a.activate
			}
			if a.close != "" {
				act.close = a.close
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
	nameColor := ui.Alpha(th.MutedForeground, 0.7)
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
	click, closeClick := clickable(s.cards, sv.ID), clickable(s.closes, sv.ID)
	if click.Clicked(gtx) {
		act.activate = sv.ID
	}
	if closeClick.Clicked(gtx) {
		act.close = sv.ID
	}
	d := click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				return ui.Fill(gtx, cardFill(th, sv.Color, active, click.Hovered()), ui.RadiusMD)
			},
			func(gtx C) D {
				return layout.Stack{Alignment: layout.NE}.Layout(gtx,
					layout.Stacked(func(gtx C) D { return s.cardBody(gtx, th, sv) }),
					layout.Stacked(func(gtx C) D {
						return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
							return closeButton(gtx, th, closeClick, s.icClose, click.Hovered())
						})
					}),
				)
			},
		)
	})
	return d, act
}

func (s *sidebar) cardBody(gtx C, th *ui.Theme, sv *sessionView) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	path := displayPath(sv.shown)
	if sv.host != "" {
		path = unknownCwd(sv.host)
	}
	rows := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return s.statusRing(gtx, th, sv) }),
				ui.Gap(1.5),
				// pr-11: the title stops short of the controls in the corner.
				layout.Flexed(1, func(gtx C) D {
					return layout.Inset{Right: ui.Space(11)}.Layout(gtx, th.Text(ui.TextSM, sv.Label).Weight(font.Medium).Layout)
				}),
			)
		}),
		ui.VGap(2),
		layout.Rigid(th.Text(ui.TextXS, path).Muted().Mono().LayoutTail),
	}
	if sv.git.Branch != "" {
		rows = append(rows, ui.VGap(2), layout.Rigid(func(gtx C) D { return s.branchRow(gtx, th, sv.git) }))
	}
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// branchRow is the card's last line: the branch, and on the right the
// checkout's changes while it has any.
func (s *sidebar) branchRow(gtx C, th *ui.Theme, git lichclient.DiffStats) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return s.icBranch.Layout(gtx, 12, th.MutedForeground) }),
		ui.Gap(1),
		layout.Flexed(1, th.Text(ui.TextXS, git.Branch).Muted().Layout),
	}
	if git.Files > 0 {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return diffStat(gtx, th, git.Added, git.Deleted) }))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// statusRing draws the ring SessionStatusIcon draws around the provider mark:
// spinning while busy, amber when waiting on the user, emerald when done.
func (s *sidebar) statusRing(gtx C, th *ui.Theme, sv *sessionView) D {
	size := gtx.Dp(22)
	stroke := float32(gtx.Dp(1.5))
	center := f32.Pt(float32(size)/2, float32(size)/2)
	radius := float32(size)/2 - stroke/2
	switch sv.status {
	case "busy", "compacting":
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, ui.Alpha(th.MutedForeground, 0.25))
		turn := float32(time.Since(s.started).Seconds()) * 2 * math.Pi
		strokeArc(gtx, center, radius, stroke, turn, math.Pi/2, th.MutedForeground)
		gtx.Execute(op.InvalidateCmd{})
	case "waiting":
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.ToneWait)
	case "done":
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.TonePass)
	}
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	return layout.Center.Layout(gtx, func(gtx C) D {
		return th.Text(10, providerMark(sv.Kind)).Muted().Weight(font.Bold).Layout(gtx)
	})
}

// ponytail: provider logos are a letter; the real port needs ProviderIcon's SVGs.
func providerMark(kind string) string {
	if kind == "shell" || kind == "" {
		return ">_"
	}
	return strings.ToUpper(kind[:1])
}

func strokeArc(gtx C, center f32.Point, radius, width, start, sweep float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	from := f32.Pt(center.X+radius*float32(math.Cos(float64(start))), center.Y+radius*float32(math.Sin(float64(start))))
	p.MoveTo(from)
	p.ArcTo(center, center, sweep)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}
