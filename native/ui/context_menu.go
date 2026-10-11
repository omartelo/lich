package ui

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/layout"
)

// contextGeometry transcribes context-menu.tsx: content at the pointer (side
// right, alignOffset 4), items px-2 py-1.5, shortcuts pl-5, and submenus that
// are the same content again. The web's min-w-36 read cramped beside the
// card it opens on, so the native menu is min-w-44.
var contextGeometry = menuGeometry{
	minWidth: Space(44), subMinWidth: Space(44),
	itemPadX: Space(2), itemPadY: Space(1.5), shortcutPL: Space(5),
	root: popupPlace{side: SideRight, align: alignStart, alignOffset: 4},
	sub:  popupPlace{side: SideRight, align: alignStart, alignOffset: 4},
}

// ContextMenuStyle draws a shadcn ContextMenu: Entries open at the pointer
// on a right press inside the trigger area.
type ContextMenuStyle struct {
	Root    *Root
	State   *MenuState
	Entries []MenuEntry
	theme   *Theme
}

func (th *Theme) ContextMenu(root *Root, state *MenuState, entries []MenuEntry) ContextMenuStyle {
	return ContextMenuStyle{Root: root, State: state, Entries: entries, theme: th}
}

// Layout lays out trigger, the area that answers a right press, and the
// menu when open.
func (c ContextMenuStyle) Layout(gtx C, trigger layout.Widget) D {
	s := c.State
	dims := menuLayoutTrigger(gtx, &s.trigger, trigger)
	if at, ok := menuTriggerPress(gtx, &s.trigger, pointer.ButtonSecondary); ok && !s.open {
		pointerAt := c.Root.pointer.Round()
		s.menuOpen(image.Rectangle{Min: pointerAt, Max: pointerAt}, c.Root.origin(at))
	}
	menuCore{th: c.theme, root: c.Root, s: s, entries: c.Entries, geo: &contextGeometry}.layout(gtx)
	return dims
}
