package main

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// row lays its widgets side by side, gap Tailwind units apart.
func row(gap float32, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		kids := make([]layout.FlexChild, 0, 2*len(ws))
		for i, w := range ws {
			if i > 0 {
				kids = append(kids, ui.Gap(gap))
			}
			kids = append(kids, layout.Rigid(w))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	}
}

// stack lays its widgets one under the other, gap Tailwind units apart. The
// section's full-width minimum stops here: a button in a stack keeps its own
// width, and a field that wants the row asks for it.
func stack(gap float32, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		kids := make([]layout.FlexChild, 0, 2*len(ws))
		for i, w := range ws {
			if i > 0 {
				kids = append(kids, ui.VGap(gap))
			}
			kids = append(kids, layout.Rigid(w))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	}
}

// split gives each widget an equal share of the width.
func split(gap float32, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		kids := make([]layout.FlexChild, 0, 2*len(ws))
		for i, w := range ws {
			if i > 0 {
				kids = append(kids, ui.Gap(gap))
			}
			kids = append(kids, layout.Flexed(1, w))
		}
		return layout.Flex{Alignment: layout.Start}.Layout(gtx, kids...)
	}
}

// minWidth pads a grid column out to width, so the header names line up with
// buttons of different natural widths.
func minWidth(width unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx C) D {
		d := w(gtx)
		d.Size.X = max(d.Size.X, min(gtx.Dp(width), gtx.Constraints.Max.X))
		return d
	}
}

// height fixes a widget whose flexed panel would otherwise take the
// scrolling page's unbounded height.
func height(h unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Max.Y = gtx.Dp(h)
		return w(gtx)
	}
}

// caption is the small mono name above a specimen.
func caption(th *ui.Theme, s string) layout.Widget {
	return th.Text(ui.TextXS, s).Muted().Mono().Layout
}

// section is a titled card; body draws on the card, so it gets a theme on it.
func (g *gallery) section(title string, body func(C, *ui.Theme) D) layout.Widget {
	th := g.th
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, title).Weight(font.Medium).Layout),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Background{}.Layout(gtx,
					func(gtx C) D { return ui.Fill(gtx, th.Over(th.Border), ui.RadiusLG) },
					func(gtx C) D {
						return layout.UniformInset(1).Layout(gtx, func(gtx C) D {
							return layout.Background{}.Layout(gtx,
								func(gtx C) D { return ui.Fill(gtx, th.Card, ui.RadiusLG-1) },
								func(gtx C) D {
									return layout.UniformInset(ui.Space(4)).Layout(gtx, func(gtx C) D {
										full := gtx.Constraints.Min.X
										d := body(gtx, th.On(th.Card))
										d.Size.X = max(d.Size.X, full)
										return d
									})
								},
							)
						})
					},
				)
			}),
		)
	}
}

var buttonVariants = []struct {
	name    string
	variant ui.Variant
}{
	{"default", ui.VariantDefault},
	{"outline", ui.VariantOutline},
	{"secondary", ui.VariantSecondary},
	{"ghost", ui.VariantGhost},
	{"destructive", ui.VariantDestructive},
}

// buttonColumns pairs each size with the width of its grid column.
var buttonColumns = []struct {
	name  string
	size  ui.Size
	width unit.Dp
	icon  bool
}{
	{"default", ui.SizeDefault, 84, false},
	{"xs", ui.SizeXS, 64, false},
	{"sm", ui.SizeSM, 76, false},
	{"lg", ui.SizeLG, 88, false},
	{"icon", ui.SizeIcon, 36, true},
	{"icon-xs", ui.SizeIconXS, 24, true},
	{"icon-sm", ui.SizeIconSM, 32, true},
	{"icon-lg", ui.SizeIconLG, 40, true},
}

const buttonLabelWidth unit.Dp = 76

func (g *gallery) buttonsSection(gtx C, th *ui.Theme) D {
	header := []layout.Widget{minWidth(buttonLabelWidth, func(C) D { return D{} })}
	for _, col := range buttonColumns {
		header = append(header, minWidth(col.width, caption(th, col.name)))
	}
	rows := []layout.Widget{row(1.5, header...)}
	for v, variant := range buttonVariants {
		cells := []layout.Widget{minWidth(buttonLabelWidth, th.Text(ui.TextXS, variant.name).Muted().Mono().Layout)}
		for s, col := range buttonColumns {
			b := th.Button(&g.buttons[v][s], "Run")
			b.Variant, b.Size = variant.variant, col.size
			b.Icon = g.icon("play")
			if col.icon {
				b.Text = ""
			}
			cells = append(cells, minWidth(col.width, b.Layout))
		}
		rows = append(rows, row(1.5, cells...))
	}
	return stack(2.5, append(rows, g.disabledButtons(th))...)(gtx)
}

func (g *gallery) disabledButtons(th *ui.Theme) layout.Widget {
	cells := []layout.Widget{minWidth(buttonLabelWidth, th.Text(ui.TextXS, "disabled").Muted().Mono().Layout)}
	for _, variant := range buttonVariants {
		b := th.Button(&g.inert, "Run")
		b.Variant, b.Icon, b.Disabled = variant.variant, g.icon("play"), true
		cells = append(cells, b.Layout)
	}
	return row(2, cells...)
}

func (g *gallery) inputsSection(gtx C, th *ui.Theme) D {
	field := func(label string, w layout.Widget) layout.Widget {
		return stack(1.5, th.Label(label).Layout, w)
	}
	empty := th.Input(&g.inputs[0], "Branch name")
	filled := th.Input(&g.inputs[1], "Branch name")
	disabled := th.Input(&g.inputs[2], "Branch name")
	disabled.Disabled = true
	invalid := th.Input(&g.inputs[3], "Branch name")
	invalid.Invalid = true
	dense := th.Input(&g.inputs[4], "Filter…")
	dense.Height, dense.TextSize = ui.Space(7), ui.TextXS
	leading := th.Input(&g.inputs[5], "Search sessions")
	leading.Leading = g.icon("search")

	area := th.Textarea(&g.textarea, "Describe what the agent should do…")

	withIcon := th.InputGroup(&g.groups[0], "Search sessions")
	withIcon.Start = ui.InputGroupAddon{Items: []layout.Widget{th.InputGroupIcon(g.icon("search"))}}
	withButton := th.InputGroup(&g.groups[1], "Repository URL")
	inner := withButton.Inner()
	copyButton := inner.InputGroupButton(&g.copyURL, "Copy")
	copyButton.Icon = g.icon("copy")
	withButton.Start = ui.InputGroupAddon{Items: []layout.Widget{inner.InputGroupText("https://")}}
	withButton.End = ui.InputGroupAddon{Items: []layout.Widget{copyButton.Layout}, Button: true}
	groupDisabled := th.InputGroup(&g.groups[2], "Repository")
	groupDisabled.Disabled = true
	groupDisabled.Start = ui.InputGroupAddon{Items: []layout.Widget{th.InputGroupIcon(g.icon("git-branch"))}}

	return stack(4,
		split(4, field("Empty, placeholder", empty.Layout), field("Filled", filled.Layout)),
		split(4, field("Disabled", disabled.Layout), field("Invalid", invalid.Layout)),
		split(4, field("Dense (h-7)", dense.Layout), field("Leading icon", leading.Layout)),
		field("Textarea", area.Layout),
		field("InputGroup, icon addon", withIcon.Layout),
		field("InputGroup, text and button addons", withButton.Layout),
		field("InputGroup, disabled", groupDisabled.Layout),
	)(gtx)
}

func (g *gallery) selectionSection(gtx C, th *ui.Theme) D {
	check := func(i int, label string, disabled bool) layout.Widget {
		c := th.Checkbox(&g.checks[i])
		c.Disabled = disabled
		l := th.Label(label)
		l.Disabled = disabled
		return row(2, c.Layout, l.Layout)
	}
	sw := func(i int, size ui.SwitchSize, label string, disabled bool) layout.Widget {
		s := th.Switch(&g.switches[i])
		s.Size, s.Disabled = size, disabled
		l := th.Label(label)
		l.Disabled = disabled
		return row(2, s.Layout, l.Layout)
	}
	toggle := func(i int, label string, variant ui.ToggleVariant, icon *icons.Icon, disabled bool) layout.Widget {
		t := th.Toggle(&g.toggles[i], label)
		t.Variant, t.Icon, t.Disabled = variant, icon, disabled
		return t.Layout
	}
	bold := th.ToggleGroup(&g.groupBold,
		ui.ToggleItem{Value: "left", Icon: g.icon("text-align-start")},
		ui.ToggleItem{Value: "center", Icon: g.icon("text-align-center")},
		ui.ToggleItem{Value: "right", Icon: g.icon("text-align-end")},
		ui.ToggleItem{Value: "justify", Icon: g.icon("text-align-justify"), Disabled: true},
	)
	grid := th.ToggleGroup(&g.groupGrid,
		ui.ToggleItem{Value: "list", Text: "List", Icon: g.icon("list")},
		ui.ToggleItem{Value: "grid", Text: "Grid", Icon: g.icon("layout-grid")},
		ui.ToggleItem{Value: "board", Text: "Board"},
	)
	grid.Variant, grid.Size = ui.ToggleVariantOutline, ui.ToggleSizeSM

	return stack(4,
		caption(th, "Checkbox"),
		row(6, check(0, "Unchecked", false), check(1, "Checked", false), check(2, "Disabled", true), check(3, "Disabled on", true)),
		caption(th, "Switch, default and sm"),
		row(6, sw(0, ui.SwitchSizeDefault, "Off", false), sw(1, ui.SwitchSizeDefault, "On", false), sw(2, ui.SwitchSizeSM, "Small", false), sw(3, ui.SwitchSizeSM, "Disabled on", true)),
		caption(th, "Toggle: default, outline, with icon, disabled"),
		row(2,
			toggle(0, "Default", ui.ToggleVariantDefault, nil, false),
			toggle(1, "Outline", ui.ToggleVariantOutline, nil, false),
			toggle(2, "Bold", ui.ToggleVariantDefault, g.icon("bold"), false),
			toggle(3, "Disabled", ui.ToggleVariantOutline, g.icon("italic"), true),
		),
		caption(th, "ToggleGroup: default (icons) and outline sm (text)"),
		row(4, bold.Layout, grid.Layout),
	)(gtx)
}

func (g *gallery) tabsSection(gtx C, th *ui.Theme) D {
	triggers := []ui.TabsTrigger{
		{Text: "Overview", Icon: g.icon("layout-grid")},
		{Text: "Activity", Icon: g.icon("activity")},
		{Text: "Archive", Disabled: true},
	}
	panel := func(s string) layout.Widget { return th.Text(ui.TextSM, s).Muted().Layout }
	panels := []layout.Widget{panel("Overview panel"), panel("Activity panel"), panel("Archive panel")}
	variants := []struct {
		name    string
		variant ui.TabsVariant
	}{{"default", ui.TabsVariantDefault}, {"line", ui.TabsVariantLine}, {"ghost", ui.TabsVariantGhost}}

	var horizontal, vertical []layout.Widget
	for i, v := range variants {
		h := th.Tabs(&g.tabsH[i], triggers...)
		h.Variant = v.variant
		horizontal = append(horizontal, stack(1.5, caption(th, v.name+", horizontal"),
			height(88, func(gtx C) D { return h.Layout(gtx, panels...) })))
		vt := th.Tabs(&g.tabsV[i], triggers...)
		vt.Variant, vt.Orientation = v.variant, ui.TabsVertical
		vertical = append(vertical, stack(1.5, caption(th, v.name+", vertical"),
			height(110, func(gtx C) D { return vt.Layout(gtx, panels...) })))
	}
	return stack(4, append(horizontal, split(4, vertical...))...)(gtx)
}

func (g *gallery) typographySection(gtx C, th *ui.Theme) D {
	lucideNames := []string{"settings", "search", "git-branch", "folder", "trash-2", "plus", "check", "x", "copy", "terminal", "star", "bell", "house", "mail"}
	var lucide []layout.Widget
	for _, name := range lucideNames {
		ic := g.icon(name)
		lucide = append(lucide, func(gtx C) D { return ic.Layout(gtx, ui.Space(5), th.Foreground) })
	}
	var lobe []layout.Widget
	for _, name := range []string{"claude", "codex", "antigravity", "opencode", "pi", "cursor"} {
		ic := icons.Lobe(name)
		lobe = append(lobe, func(gtx C) D { return ic.Layout(gtx, ui.Space(6), th.Foreground) })
	}
	disabled := th.Label("Label, disabled")
	disabled.Disabled = true
	upper := th.Label("Label, uppercase")
	upper.Uppercase = true

	return stack(4,
		caption(th, "Text xs / sm / base"),
		stack(1,
			th.Text(ui.TextXS, "text-xs: the quick brown fox jumps over the lazy dog").Layout,
			th.Text(ui.TextSM, "text-sm: the quick brown fox jumps over the lazy dog").Layout,
			th.Text(ui.TextBase, "text-base: the quick brown fox jumps over the lazy dog").Layout,
			th.Text(ui.TextSM, "text-sm muted, medium weight").Muted().Weight(font.Medium).Layout,
			th.Text(ui.TextSM, "text-sm mono: git rebase --onto main").Mono().Layout,
		),
		caption(th, "Label"),
		row(6, th.Label("Label").Layout, disabled.Layout, upper.Layout),
		caption(th, "Separator"),
		th.Separator,
		caption(th, "Skeleton"),
		row(3,
			th.Skeleton(40, 40).Layout,
			stack(2, th.Skeleton(220, 14).Layout, th.Skeleton(160, 14).Layout),
		),
		caption(th, "Lucide icons"),
		row(3, lucide...),
		caption(th, "Lobe marks"),
		row(4, lobe...),
	)(gtx)
}

func (g *gallery) overlaysSection(gtx C, th *ui.Theme) D {
	if g.dialogOpener.Clicked(gtx) {
		g.dialogOpen = true
	}
	act := func(s string) func() { return func() { g.lastAction = s } }

	tipTrigger := th.Button(&g.tipLabelTrigger, "Hover for a tooltip")
	tipTrigger.Variant, tipTrigger.Icon = ui.VariantOutline, g.icon("info")
	tip := th.Tooltip(&g.root, &g.tipLabel, "Opens after a short delay")

	pop := th.On(th.Popover)
	cardTrigger := th.Button(&g.tipCardTrigger, "Hover for a card")
	cardTrigger.Variant, cardTrigger.Icon = ui.VariantOutline, g.icon("info")
	card := th.TooltipCard(&g.root, &g.tipCard, stack(1,
		pop.Text(ui.TextXS, "claude · opus").Weight(font.Medium).Layout,
		pop.Text(ui.TextXS, "12 turns, 48k tokens").Muted().Layout,
	))
	card.Side = ui.SideBottom

	menuTrigger := th.Button(&g.menuTrigger, "Session menu")
	menuTrigger.Variant, menuTrigger.Icon = ui.VariantOutline, g.icon("ellipsis")
	entries := []ui.MenuEntry{
		{Kind: ui.MenuLabel, Label: "Session"},
		{Label: "Rename", Icon: g.icon("pencil"), Shortcut: "F2", OnClick: act("Rename")},
		{Label: "Duplicate", Icon: g.icon("copy"), OnClick: act("Duplicate")},
		{Kind: ui.MenuCheckboxItem, Label: "Pin to top", Checked: g.pinned, OnCheckedChange: func(c bool) { g.pinned = c }},
		{Kind: ui.MenuSubTrigger, Label: "Move to", Icon: g.icon("folder"), Sub: []ui.MenuEntry{
			{Label: "Inbox", OnClick: act("Move to Inbox")},
			{Label: "Archive", OnClick: act("Move to Archive")},
		}},
		{Kind: ui.MenuSeparator},
		{Label: "Close session", Icon: g.icon("trash-2"), Shortcut: "Ctrl+W", Variant: ui.VariantDestructive, OnClick: act("Close session")},
	}
	dropdown := th.DropdownMenu(&g.root, &g.menu, entries)
	context := th.ContextMenu(&g.root, &g.contextMenu, entries)

	opener := th.Button(&g.dialogOpener, "Delete session…")
	opener.Variant, opener.Icon = ui.VariantDestructive, g.icon("trash-2")

	status := "Pinned: no. Last action: " + g.lastAction
	if g.pinned {
		status = "Pinned: yes. Last action: " + g.lastAction
	}
	return stack(4,
		caption(th, "Tooltip (label and card)"),
		row(2,
			func(gtx C) D { return tip.Layout(gtx, tipTrigger.Layout) },
			func(gtx C) D { return card.Layout(gtx, cardTrigger.Layout) },
		),
		caption(th, "DropdownMenu, click the trigger"),
		func(gtx C) D { return dropdown.Layout(gtx, menuTrigger.Layout) },
		caption(th, "ContextMenu, right-click the area"),
		func(gtx C) D {
			return context.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				gtx.Constraints.Min.Y = gtx.Dp(ui.Space(16))
				return layout.Background{}.Layout(gtx,
					func(gtx C) D { return ui.Fill(gtx, th.Over(ui.Alpha(th.Muted, 0.5)), ui.RadiusMD) },
					func(gtx C) D {
						return layout.Center.Layout(gtx, th.Text(ui.TextSM, "Right-click anywhere here").Muted().Layout)
					},
				)
			})
		},
		th.Text(ui.TextXS, status).Muted().Layout,
		caption(th, "Dialog, confirm"),
		opener.Layout,
	)(gtx)
}

// layoutDialog runs before the page, so the dialog claims Escape ahead of
// anything under it, and draws above everything the frame lays out.
func (g *gallery) layoutDialog(gtx C) {
	if g.dialogCancel.Clicked(gtx) || g.dialogDelete.Clicked(gtx) {
		g.dialogOpen = false
	}
	th := g.th.On(g.th.Popover)
	cancel := th.Button(&g.dialogCancel, "Cancel")
	cancel.Variant = ui.VariantOutline
	del := th.Button(&g.dialogDelete, "Delete")
	del.Variant = ui.VariantDestructive
	d := th.Dialog(&g.dialog, g.dialogOpen, "Delete this session?",
		"Its terminal closes and its worktree is removed. This cannot be undone.")
	d.Actions = []layout.Widget{cancel.Layout, del.Layout}
	if d.Layout(gtx) {
		g.dialogOpen = false
	}
}
