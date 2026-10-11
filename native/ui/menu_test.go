package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// menuRig drives a window of size through an input.Router, one frame per
// call, with a trigger box at triggerAt.
type menuRig struct {
	router input.Router
	ops    op.Ops
	root   Root
	size   image.Point
}

var (
	triggerAt   = image.Pt(40, 30)
	triggerSize = image.Pt(80, 30)
)

func (g *menuRig) frame(menu func(gtx C, trigger layout.Widget) D) {
	g.ops.Reset()
	gtx := layout.Context{Ops: &g.ops, Source: g.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(g.size)}
	g.root.Layout(gtx, func(gtx C) D {
		defer op.Offset(triggerAt).Push(gtx.Ops).Pop()
		return menu(gtx, func(gtx C) D { return D{Size: triggerSize} })
	})
	g.router.Frame(&g.ops)
}

func (g *menuRig) click(buttons pointer.Buttons, at image.Point) {
	pos := f32.Pt(float32(at.X), float32(at.Y))
	g.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pos},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: buttons, Position: pos},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos},
	)
}

func (g *menuRig) hover(at image.Point) {
	g.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(at.X), float32(at.Y))})
}

func (g *menuRig) press(names ...key.Name) {
	for _, n := range names {
		g.router.Queue(key.Event{Name: n, State: key.Press})
	}
}

// chosen records which entries ran their OnClick.
type chosen []string

func (c *chosen) item(label string) MenuEntry {
	return MenuEntry{Label: label, OnClick: func() { *c = append(*c, label) }}
}

func (c chosen) only(t *testing.T, want string) {
	t.Helper()
	if len(c) != 1 || c[0] != want {
		t.Errorf("chosen %q, want only %q", []string(c), want)
	}
}

func TestMenuStepSkipsInertRowsAndLoops(t *testing.T) {
	entries := []MenuEntry{{Label: "a"}, {Kind: MenuSeparator}, {Label: "b", Disabled: true}, {Label: "c"}, {Kind: MenuLabel}}
	cases := []struct{ from, dir, want int }{{-1, 1, 0}, {0, 1, 3}, {3, 1, 0}, {-1, -1, 3}, {0, -1, 3}, {5, -1, 3}}
	for _, tc := range cases {
		if got := menuStep(entries, tc.from, tc.dir); got != tc.want {
			t.Errorf("menuStep(from %d, dir %d) = %d, want %d", tc.from, tc.dir, got, tc.want)
		}
	}
}
