// ui-gallery is a standalone window that shows every primitive of package ui
// in each of its variants and states, fully interactive, so the kit can be
// judged without the lich backend.
package main

import (
	"flag"
	"log"
	"os"
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

	"github.com/omartelo/lich/native/internal/shot"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

var (
	flagShot       = flag.String("shot", "", "write a PNG of the page drawn -shot-after into the run, then exit")
	flagAfter      = flag.Duration("shot-after", time.Second, "when to take -shot")
	flagShotHeight = flag.Int("shot-height", 0, "height in px of the -shot render, 0 for the window's; the page scrolls, so a shot of all of it is taller")
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// pagePad is the margin around the two columns of sections.
const pagePad unit.Dp = 24

func main() {
	flag.Parse()
	go func() {
		w := new(app.Window)
		w.Option(app.Title("lich ui gallery"), app.Size(unit.Dp(1400), unit.Dp(900)))
		if err := run(w); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	shaper := text.NewShaper(text.WithCollection(gofont.Collection()))
	g := newGallery(&ui.Theme{
		Palette: ui.Dark,
		Shaper:  shaper,
		Sans:    "Go",
		Mono:    "Go Mono",
		Surface: ui.Dark.Background,
	})
	started := time.Now()
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			g.layout(gtx)
			if *flagShot != "" {
				gtx.Execute(op.InvalidateCmd{At: started.Add(*flagAfter)})
				if time.Since(started) > *flagAfter {
					return g.shoot(e)
				}
			}
			e.Frame(gtx.Ops)
		}
	}
}

// shoot lays the page out once more at the shot's own height, off screen, so
// a page taller than the window is captured whole.
func (g *gallery) shoot(e app.FrameEvent) error {
	if *flagShotHeight > 0 {
		e.Size.Y = *flagShotHeight
	}
	var ops op.Ops
	g.layout(app.NewContext(&ops, e))
	return shot.Write(*flagShot, e.Size, &ops)
}

// gallery owns the state of every widget on the page: the kit's primitives
// keep none of their own.
type gallery struct {
	th   *ui.Theme
	root ui.Root
	page widget.List

	icons map[string]*icons.Icon

	buttons  [5][8]widget.Clickable
	inert    widget.Clickable
	inputs   [6]widget.Editor
	textarea widget.Editor
	groups   [3]widget.Editor
	copyURL  widget.Clickable

	checks    [4]widget.Bool
	switches  [4]ui.SwitchState
	toggles   [4]widget.Bool
	groupBold widget.Enum
	groupGrid widget.Enum
	tabsH     [3]ui.TabsState
	tabsV     [3]ui.TabsState

	tipLabelTrigger widget.Clickable
	tipLabel        ui.Tooltip
	tipCardTrigger  widget.Clickable
	tipCard         ui.Tooltip
	menuTrigger     widget.Clickable
	menu            ui.MenuState
	contextMenu     ui.MenuState
	pinned          bool
	lastAction      string

	dialog       ui.DialogState
	dialogOpen   bool
	dialogOpener widget.Clickable
	dialogCancel widget.Clickable
	dialogDelete widget.Clickable
}

func newGallery(th *ui.Theme) *gallery {
	g := &gallery{th: th, icons: map[string]*icons.Icon{}, lastAction: "none yet"}
	g.page.Axis = layout.Vertical
	g.inputs[1].SetText("lich/native-ui")
	g.inputs[2].SetText("locked value")
	g.inputs[3].SetText("not a branch name")
	g.groups[2].SetText("lichdotdev/lich")
	g.checks[1].Value = true
	g.checks[3].Value = true
	g.switches[1].Value = true
	g.switches[3].Value = true
	g.toggles[1].Value = true
	g.groupBold.Value = "left"
	g.groupGrid.Value = "list"
	return g
}

// icon is a Lucide icon parsed once: Lucide re-reads the set on every call.
func (g *gallery) icon(name string) *icons.Icon {
	ic, ok := g.icons[name]
	if !ok {
		ic = icons.Lucide(name)
		g.icons[name] = ic
	}
	return ic
}

func (g *gallery) layout(gtx C) D {
	paint.Fill(gtx.Ops, ui.Dark.Background)
	return g.root.Layout(gtx, func(gtx C) D {
		g.layoutDialog(gtx)
		return g.th.ScrollArea(&g.page).Layout(gtx, 1, func(gtx C, _ int) D {
			return layout.UniformInset(pagePad).Layout(gtx, g.content)
		})
	})
}

func (g *gallery) content(gtx C) D {
	th := g.th
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextBase, "lich ui").Weight(font.Medium).Layout),
		layout.Rigid(th.Text(ui.TextSM, "Every primitive of the native kit, in each variant and state. All of it is live.").Muted().Layout),
		ui.VGap(6),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Start}.Layout(gtx,
				layout.Flexed(1, stack(6,
					g.section("Button", g.buttonsSection),
					g.section("Input, Textarea, InputGroup", g.inputsSection),
					g.section("Tooltip, menus and Dialog", g.overlaysSection),
				)),
				ui.Gap(6),
				layout.Flexed(1, stack(6,
					g.section("Checkbox, Switch, Toggle, ToggleGroup", g.selectionSection),
					g.section("Tabs", g.tabsSection),
					g.section("Label, Text, Skeleton, Separator, icons", g.typographySection),
				)),
			)
		}),
	)
}
