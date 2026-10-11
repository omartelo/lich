package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/widget"
)

var toggleGroupItems = []ToggleItem{
	{Value: "worktree", Text: "Working tree"},
	{Value: "turn", Text: "Last turn"},
}

func TestToggleGroupClickChoosesAnItem(t *testing.T) {
	th, enum := testTheme(), &widget.Enum{Value: "worktree"}
	group := th.ToggleGroup(enum, toggleGroupItems...)
	rig := newToggleRig(image.Pt(20, 20), group.Layout)
	d := rig.frame()

	rig.click(image.Pt(d.Size.X-12, d.Size.Y/2))
	if enum.Value != "turn" {
		t.Errorf("clicked the second item: value %q, want turn", enum.Value)
	}
	rig.click(image.Pt(d.Size.X-12, d.Size.Y/2))
	if enum.Value != "turn" {
		t.Errorf("clicked the chosen item again: value %q, want it kept", enum.Value)
	}
}

func TestToggleGroupSkipsADisabledItem(t *testing.T) {
	th, enum := testTheme(), &widget.Enum{Value: "worktree"}
	items := []ToggleItem{toggleGroupItems[0], {Value: "turn", Text: "Last turn", Disabled: true}}
	rig := newToggleRig(image.Pt(20, 20), th.ToggleGroup(enum, items...).Layout)
	d := rig.frame()
	rig.click(image.Pt(d.Size.X-12, d.Size.Y/2))
	if enum.Value != "worktree" {
		t.Errorf("value %q, want the disabled item left unchosen", enum.Value)
	}
}

// border-border is white at 10% over the page.
func TestToggleGroupTrackBorderIsBorderOverThePage(t *testing.T) {
	group := testTheme().ToggleGroup(new(widget.Enum), toggleGroupItems...)
	size := layoutAt(group.Layout, 0, 500).Size
	img := toggleRender(t, size, time.Time{}, group.Layout)
	if want := (color.NRGBA{R: 33, G: 33, B: 35, A: 255}); !toggleIs(img, image.Pt(size.X/2, 0), want) {
		t.Errorf("top edge %v, want border over the page %v", img.RGBAAt(size.X/2, 0), want)
	}
}

// A track of border 1 and padding 3 around the options, 4 apart.
func TestToggleGroupSizesTheTrackAroundItsItems(t *testing.T) {
	th := testTheme()
	group := th.ToggleGroup(new(widget.Enum), toggleGroupItems...)
	group.Size = ToggleSizeSM
	first, second := th.Toggle(new(widget.Bool), "Working tree"), th.Toggle(new(widget.Bool), "Last turn")
	first.Size, second.Size = ToggleSizeSM, ToggleSizeSM
	w1, w2 := layoutAt(first.Layout, 0, 500).Size.X, layoutAt(second.Layout, 0, 500).Size.X

	want := image.Pt(w1+w2+4+2*4, 32+2*4)
	if d := layoutAt(group.Layout, 0, 500); d.Size != want {
		t.Errorf("got %v, want %v", d.Size, want)
	}
}

// The group is w-fit: a parent's minimum width does not stretch it.
func TestToggleGroupIsAsWideAsItsItems(t *testing.T) {
	group := testTheme().ToggleGroup(new(widget.Enum), toggleGroupItems...)
	natural := layoutAt(group.Layout, 0, 500).Size.X
	if d := layoutAt(group.Layout, 400, 500); d.Size.X != natural {
		t.Errorf("got width %d under a 400 minimum, want %d", d.Size.X, natural)
	}
}

// className h-6 px-2.5 on every item: the height changes, the size's min-w-8
// does not.
func TestToggleGroupItemOverrides(t *testing.T) {
	th := testTheme()
	group := th.ToggleGroup(new(widget.Enum), ToggleItem{Value: "a", Text: "i"})
	group.Size = ToggleSizeSM
	group.ItemHeight = Space(6)
	if d := layoutAt(group.Layout, 0, 500); d.Size != image.Pt(32+2*4, 24+2*4) {
		t.Errorf("got %v, want a 32 wide, 24 tall item inside the track", d.Size)
	}
}
