package ui

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/layout"
)

// dropdownGeometry transcribes dropdown-menu.tsx: min-w-32 content below the
// trigger (sideOffset 4), items px-2.5 py-2, submenus min-w-[6rem] to the
// right (alignOffset -3).
var dropdownGeometry = menuGeometry{
	minWidth: Space(32), subMinWidth: Space(24),
	itemPadX: Space(2.5), itemPadY: Space(2),
	root: popupPlace{side: SideBottom, align: alignStart, sideOffset: 4},
	sub:  popupPlace{side: SideRight, align: alignStart, alignOffset: -3},
}

// DropdownMenuStyle draws a shadcn DropdownMenu: a trigger that opens Entries
// on press, as base-ui's Menu.Trigger does, in a popover over everything.
type DropdownMenuStyle struct {
	Root    *Root
	State   *MenuState
	Entries []MenuEntry
	theme   *Theme
}

func (th *Theme) DropdownMenu(root *Root, state *MenuState, entries []MenuEntry) DropdownMenuStyle {
	return DropdownMenuStyle{Root: root, State: state, Entries: entries, theme: th}
}

// Layout lays out trigger, typically a Button, and the menu when open.
// ponytail: opens from the pointer only; a focused trigger's Enter and
// ArrowDown land with keyboard focus traversal.
func (d DropdownMenuStyle) Layout(gtx C, trigger layout.Widget) D {
	s := d.State
	dims := menuLayoutTrigger(gtx, &s.trigger, trigger)
	if at, ok := menuTriggerPress(gtx, &s.trigger, pointer.ButtonPrimary); ok && !s.open {
		origin := d.Root.origin(at)
		s.menuOpen(image.Rectangle{Min: origin, Max: origin.Add(dims.Size)}, origin)
	}
	menuCore{th: d.theme, root: d.Root, s: s, entries: d.Entries, geo: &dropdownGeometry}.layout(gtx)
	return dims
}
