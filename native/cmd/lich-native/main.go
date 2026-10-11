// gio-lich is a vertical slice of lich as a native Gio app: the session
// sidebar, the active session's terminal and its checkout's diff, all driven
// by a real (isolated) lich backend. It exists to judge whether a native
// lich feels native, not to replace the web window.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/omartelo/lich/native/gioterm"
	"github.com/omartelo/lich/native/internal/shot"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

var (
	flagRuntime  = flag.String("runtime", "", "the backend's runtime file (runtime.json / runtime-dev.json); without it, LICH_PORT and LICH_TOKEN as `lich native` sets them")
	flagProject  = flag.String("project", "", "checkout to open when the backend has no project yet")
	flagRenderer = flag.String("renderer", gioterm.RenderRows, "terminal renderer: rows, atlas or paths")
	flagFont     = flag.String("font", "JetBrainsMono Nerd Font", "terminal and diff font family (fontconfig name)")
	flagSize     = flag.Float64("size", 13, "terminal font size in sp")
	flagShot     = flag.String("shot", "", "write a PNG of the frame drawn -shot-after into the run")
	flagAfter    = flag.Duration("shot-after", 3*time.Second, "when to take -shot")
	flagTab      = flag.String("tab", "terminal", "tab shown at start: terminal or diff")
)

func main() {
	flag.Parse()
	rt, err := backendRuntime()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("lich (native slice)"), app.Size(unit.Dp(1400), unit.Dp(900)))
		code := 0
		if err := run(w, rt); err != nil {
			log.Print(err)
			code = 1
		}
		os.Exit(code)
	}()
	app.Main()
}

type tab int

const (
	tabTerminal tab = iota
	tabDiff
)

type window struct {
	ctx     context.Context
	client  *lichclient.Client
	model   *model
	terms   *terminals
	diff    *diffView
	sidebar *sidebar
	th      *material.Theme
	theme   *ui.Theme
	root    ui.Root
	tab     tab
	tabTerm widget.Clickable
	tabDiff widget.Clickable
}

// backendRuntime is the backend to talk to: the -runtime file when given,
// else the one a launching `lich native` put in the environment.
func backendRuntime() (lichclient.Runtime, error) {
	if *flagRuntime != "" {
		return lichclient.ReadRuntime(*flagRuntime)
	}
	return lichclient.RuntimeFromEnv(os.Getenv)
}

func run(w *app.Window, rt lichclient.Runtime) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := lichclient.New(rt)
	faces, err := gioterm.Faces(*flagFont)
	if err != nil {
		return err
	}
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(append(gofont.Collection(), faces...)))
	th.Fg, th.Bg = colForeground, colBackground

	u := &window{
		ctx: ctx, client: client, th: th, sidebar: newSidebar(),
		theme: &ui.Theme{Palette: ui.Dark, Shaper: th.Shaper, Mono: faces[0].Font.Typeface, Surface: ui.Dark.Background},
		model: newModel(client, w.Invalidate),
		terms: &terminals{client: client, invalidate: w.Invalidate, renderer: *flagRenderer, family: *flagFont,
			size: unit.Sp(*flagSize), byID: map[string]*termView{}},
	}
	if u.diff, err = newDiffView(client, w.Invalidate, *flagFont, unit.Sp(*flagSize), th); err != nil {
		return err
	}
	if *flagTab == "diff" {
		u.tab = tabDiff
	}
	if err := u.connect(); err != nil {
		return err
	}
	started := time.Now()
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.ConfigEvent:
			u.model.setFocused(u.ctx, e.Config.Focused)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if err := u.layout(gtx); err != nil {
				return err
			}
			if *flagShot != "" {
				gtx.Execute(op.InvalidateCmd{At: started.Add(*flagAfter)})
				if time.Since(started) > *flagAfter {
					if err := shot.Write(*flagShot, e.Size, &ops); err != nil {
						return err
					}
					*flagShot = ""
				}
			}
			e.Frame(gtx.Ops)
		}
	}
}

// connect opens the event and terminal sockets before loading state, so no
// change made in between is missed.
func (u *window) connect() error {
	events, err := u.client.Events(u.ctx)
	if err != nil {
		return err
	}
	stream, _, err := u.client.Terminal(u.ctx, func(id string, data []byte) { u.terms.onOutput(id, data) })
	if err != nil {
		return err
	}
	u.terms.stream = stream
	if err := u.model.reload(u.ctx); err != nil {
		return err
	}
	if _, ok := u.model.project(); !ok && *flagProject != "" {
		path, err := filepath.Abs(*flagProject)
		if err != nil {
			return err
		}
		if err := u.client.AddProject(u.ctx, "native", filepath.Base(path), path); err != nil {
			return err
		}
		if err := u.model.reload(u.ctx); err != nil {
			return err
		}
	}
	go u.model.follow(u.ctx, events)
	go u.model.pollGit(u.ctx)
	return nil
}

func (u *window) layout(gtx C) error {
	paint.Fill(gtx.Ops, colBackground)
	if u.model.connectionLost() {
		u.lostNotice(gtx)
		return nil
	}
	var act sidebarAction
	var contentErr error
	active, hasActive := u.model.activeSession()
	u.root.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx C) D {
					d, a := u.sidebar.layout(gtx, u.theme, u.model.groups(), active.ID)
					act = a
					return d
				})
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Top: 6, Bottom: 6, Right: 6}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return u.tabBar(gtx, active, hasActive) }),
						layout.Flexed(1, func(gtx C) D {
							gtx.Constraints.Min = gtx.Constraints.Max
							if !hasActive {
								return layout.Center.Layout(gtx, func(gtx C) D {
									return label(gtx, u.th, "Open a session from the sidebar.", 14, colMuted, font.Normal)
								})
							}
							contentErr = u.content(gtx, active)
							return D{Size: gtx.Constraints.Max}
						}),
					)
				})
			}),
		)
	})
	u.apply(act)
	return contentErr
}

// lostNotice replaces the whole window once the backend dropped it: what it
// would draw is stale, and a terminal there would type into nothing.
func (u *window) lostNotice(gtx C) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(u.theme.Text(ui.TextSM, "Another lich window took over, or lich quit.").Weight(font.Medium).Layout),
			ui.VGap(1.5),
			layout.Rigid(u.theme.Text(ui.TextSM, "Run `lich native` to reopen here.").Muted().Layout),
		)
	})
}

func (u *window) content(gtx C, active sessionView) error {
	if u.tab == tabDiff {
		u.diff.watch(active.path)
		return u.diff.layout(gtx, u.th)
	}
	u.diff.close()
	t, err := u.terms.show(u.ctx, gtx, active)
	if err != nil {
		return err
	}
	return u.terms.layout(gtx, t)
}

func (u *window) tabBar(gtx C, active sessionView, hasActive bool) D {
	if u.tabTerm.Clicked(gtx) {
		u.tab = tabTerminal
	}
	if u.tabDiff.Clicked(gtx) {
		u.tab = tabDiff
	}
	tabButton := func(c *widget.Clickable, text string, on bool) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return c.Layout(gtx, func(gtx C) D {
				return layout.Background{}.Layout(gtx,
					func(gtx C) D {
						if on {
							return fillRRect(gtx, colAccent, 6)
						}
						return D{Size: gtx.Constraints.Min}
					},
					func(gtx C) D {
						col := colMuted
						if on {
							col = colForeground
						}
						return layout.Inset{Top: 6, Bottom: 6, Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
							return label(gtx, u.th, text, 13, col, font.Medium)
						})
					},
				)
			})
		})
	}
	title := ""
	if hasActive {
		title = active.Label
	}
	return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			tabButton(&u.tabTerm, "Terminal", u.tab == tabTerminal),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			tabButton(&u.tabDiff, "Diff", u.tab == tabDiff),
			layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
			layout.Flexed(1, func(gtx C) D { return label(gtx, u.th, title, 13, colMuted, font.Normal) }),
		)
	})
}

func (u *window) apply(act sidebarAction) {
	if act.activate != "" {
		u.model.setActive(u.ctx, act.activate)
		if p, ok := u.model.project(); ok {
			id := act.activate
			go func() { u.logErr("set active session", u.client.SetActiveSession(u.ctx, p.ID, id)) }()
		}
		u.model.invalidate()
	}
	if act.newKind != "" {
		go u.newSession(act.newKind)
	}
	if act.close != "" {
		u.closeSession(act.close)
	}
	if act.openURL != "" {
		url := act.openURL
		go func() { u.logErr("open pull request", u.client.OpenExternal(u.ctx, url)) }()
	}
	if act.pin != "" && u.model.setPinned(act.pin, act.pinTo) {
		u.model.invalidate()
		id, pinned := act.pin, act.pinTo
		go func() { u.logErr("pin session", u.client.SetSessionPinned(u.ctx, id, pinned)) }()
	}
}

// closeSession parks the session in the backend and stops its process, the
// two halves of the web card's ×.
func (u *window) closeSession(id string) {
	projectID, active, ok := u.model.closeSession(id)
	if !ok {
		return
	}
	u.terms.drop(id)
	u.model.invalidate()
	go func() {
		u.logErr("close session", u.client.CloseSession(u.ctx, projectID, id, active))
		u.logErr("close terminal", u.client.CloseTerminal(u.ctx, id))
	}()
}

func (u *window) logErr(what string, err error) {
	if err != nil {
		log.Printf("%s: %v", what, err)
	}
}

func (u *window) newSession(kind string) {
	p, ok := u.model.project()
	if !ok {
		log.Print("new session: the backend has no project open")
		return
	}
	id := strings.ToLower(rand.Text())
	label := fmt.Sprintf("%s %d", strings.ToUpper(kind[:1])+kind[1:], p.NextSeq+1)
	if err := u.client.AddSession(u.ctx, p.ID, id, label, kind, p.Path, p.NextSeq+1); err != nil {
		u.logErr("new session", err)
		return
	}
	if err := u.model.reload(u.ctx); err != nil {
		u.logErr("reload", err)
		return
	}
	u.model.setActive(u.ctx, id)
	u.logErr("set active session", u.client.SetActiveSession(u.ctx, p.ID, id))
	u.tab = tabTerminal
	u.model.invalidate()
}
