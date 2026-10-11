package ui

import (
	"image"
	"testing"

	"gioui.org/io/pointer"
	"gioui.org/layout"
)

// A context item is py-1.5 around a 20px line.
const contextRow = 32

func contextRig(entries []MenuEntry) (*menuRig, *MenuState, func()) {
	g, s, th := &menuRig{size: image.Pt(400, 300)}, new(MenuState), testTheme()
	frame := func() {
		g.frame(func(gtx C, trigger layout.Widget) D { return th.ContextMenu(&g.root, s, entries).Layout(gtx, trigger) })
	}
	frame()
	return g, s, frame
}

func TestContextMenuOpensAtThePointerOnRightPress(t *testing.T) {
	var c chosen
	g, s, frame := contextRig([]MenuEntry{c.item("Rename"), c.item("Pin")})
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(30, 10)))
	frame()
	if s.Opened() {
		t.Fatal("a left press opened the context menu")
	}
	at := triggerAt.Add(image.Pt(30, 10))
	g.click(pointer.ButtonSecondary, at)
	frame()
	// side right, alignOffset 4: the panel's corner is 4px below the pointer.
	g.click(pointer.ButtonPrimary, image.Pt(at.X+10, at.Y+4+4+contextRow+contextRow/2))
	frame()
	c.only(t, "Pin")
}

func TestContextMenuFlipsAndClampsInsideTheWindow(t *testing.T) {
	var c chosen
	g, _, frame := contextRig([]MenuEntry{c.item("Rename"), c.item("Pin")})
	at := triggerAt.Add(triggerSize).Sub(image.Pt(1, 1))
	g.size = at.Add(image.Pt(10, 10))
	frame()
	g.click(pointer.ButtonSecondary, at)
	frame()
	// No room right of the pointer: the panel flips to its left. No room
	// below: it shifts up to end 5px (collisionPadding) above the bottom.
	g.click(pointer.ButtonPrimary, image.Pt(at.X-10, g.size.Y-5-4-contextRow/2))
	frame()
	c.only(t, "Pin")
}
