package main

import (
	"strings"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

var (
	icMenuRename = icons.Lucide("pencil")
	icMenuColor  = icons.Lucide("palette")
	icMenuPin    = icons.Lucide("pin")
	icMenuUnpin  = icons.Lucide("pin-off")
	icOpenIn     = icons.Lucide("folder-open")
	icInTerminal = icons.Lucide("terminal")
	icInEditor   = icons.Lucide("folder-code")
	icMenuPR     = icons.Lucide("git-pull-request-arrow")
	icMenuClose  = icons.Lucide("x")
)

// cardMenu is the session card's right-click menu, cut to what this window
// keeps; a pick lands in act, but Rename only turns the label into a field. A pinned card offers no close: closing is what
// the pin withholds.
//
// ponytail: English labels until the native window has i18n.
func cardMenu(sv *sessionView, act *sidebarAction, startRename func()) []ui.MenuEntry {
	pin := ui.MenuEntry{Label: "Pin", Icon: icMenuPin, OnClick: func() { act.pin, act.pinTo = sv.ID, true }}
	if sv.Pinned {
		pin = ui.MenuEntry{Label: "Unpin", Icon: icMenuUnpin, OnClick: func() { act.pin, act.pinTo = sv.ID, false }}
	}
	entries := []ui.MenuEntry{
		{Label: "Rename", Icon: icMenuRename, OnClick: startRename},
		pin,
		{Kind: ui.MenuSubTrigger, Label: "Color", Icon: icMenuColor, Columns: 3, Sub: colorSwatches(sv, act)},
		{Kind: ui.MenuSubTrigger, Label: "Open in", Icon: icOpenIn, Sub: openInMenu(sv, act)},
		pullRequestItem(sv, act),
	}
	if !sv.Pinned {
		entries = append(entries,
			ui.MenuEntry{Kind: ui.MenuSeparator},
			ui.MenuEntry{Label: "Close session", Icon: icMenuClose, Variant: ui.VariantDestructive, OnClick: func() { act.close = sv.ID }},
		)
	}
	return entries
}

// colorSwatches is CardColorMenu's grid: the theme first, then the palette.
func colorSwatches(sv *sessionView, act *sidebarAction) []ui.MenuEntry {
	pick := func(name string) func() {
		return func() { act.color, act.colorTo = sv.ID, name }
	}
	swatches := []ui.MenuEntry{{Kind: ui.MenuSwatch, Label: "Theme", Checked: sv.Color == "", OnClick: pick("")}}
	for _, name := range cardColorNames {
		swatches = append(swatches, ui.MenuEntry{
			Kind: ui.MenuSwatch, Label: strings.ToUpper(name[:1]) + name[1:], Swatch: cardColors[name],
			Checked: sv.Color == name, OnClick: pick(name),
		})
	}
	return swatches
}

// openInMenu opens the directory the card shows. A shell session has no use
// for another terminal there.
func openInMenu(sv *sessionView, act *sidebarAction) []ui.MenuEntry {
	var sub []ui.MenuEntry
	if sv.Kind != "shell" {
		sub = append(sub, ui.MenuEntry{Label: "Terminal", Icon: icInTerminal, OnClick: func() { act.terminalAt = sv.shown }})
	}
	return append(sub,
		ui.MenuEntry{Label: "Editor", Icon: icInEditor, OnClick: func() { act.editorAt = sv.shown }},
		ui.MenuEntry{Label: "File manager", Icon: icOpenIn, OnClick: func() { act.folderAt = sv.shown }},
	)
}

// pullRequestItem opens the branch's PR in the browser, this window having no
// Pulls screen to open it in; disabled while the branch has none.
func pullRequestItem(sv *sessionView, act *sidebarAction) ui.MenuEntry {
	item := ui.MenuEntry{Label: "Pull request", Icon: icMenuPR, Disabled: sv.pr == nil}
	if sv.pr != nil {
		url := sv.pr.URL
		item.OnClick = func() { act.openURL = url }
	}
	return item
}
