package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/widget"
)

func TestCheckboxIsSixteenSquare(t *testing.T) {
	cb := testTheme().Checkbox(new(widget.Bool))
	if d := layoutAt(cb.Layout, 0, 500); d.Size != image.Pt(16, 16) {
		t.Errorf("got %v, want 16x16 (size-4)", d.Size)
	}
}

func TestCheckboxClickFlipsChecked(t *testing.T) {
	checked := new(widget.Bool)
	rig := newToggleRig(image.Pt(20, 20), testTheme().Checkbox(checked).Layout)
	rig.click(image.Pt(8, 8))
	if !checked.Value {
		t.Fatal("click did not check the box")
	}
	rig.click(image.Pt(8, 8))
	if checked.Value {
		t.Error("second click did not uncheck it")
	}
}

// after:-inset-x-3 after:-inset-y-2.
func TestCheckboxPointerAreaReachesPastTheBox(t *testing.T) {
	cases := []struct {
		name string
		at   image.Point
		hit  bool
	}{
		{"12px left of the box", image.Pt(-11, 8), true},
		{"13px left of the box", image.Pt(-13, 8), false},
		{"8px below the box", image.Pt(8, 16+7), true},
		{"9px below the box", image.Pt(8, 16+9), false},
	}
	for _, tc := range cases {
		checked := new(widget.Bool)
		rig := newToggleRig(image.Pt(30, 30), testTheme().Checkbox(checked).Layout)
		rig.click(tc.at)
		if checked.Value != tc.hit {
			t.Errorf("%s: checked = %v, want %v", tc.name, checked.Value, tc.hit)
		}
	}
}

func TestCheckboxPointerAreaLeavesTheLayoutAlone(t *testing.T) {
	cb := testTheme().Checkbox(new(widget.Bool))
	rig := newToggleRig(image.Pt(30, 30), cb.Layout)
	if d := rig.frame(); d.Size != image.Pt(16, 16) {
		t.Errorf("got %v, want the box's own 16x16", d.Size)
	}
}

func TestCheckboxDisabledIgnoresClicks(t *testing.T) {
	checked := new(widget.Bool)
	cb := testTheme().Checkbox(checked)
	cb.Disabled = true
	rig := newToggleRig(image.Pt(20, 20), cb.Layout)
	rig.click(image.Pt(8, 8))
	if checked.Value {
		t.Error("a disabled checkbox was checked")
	}
}

func TestCheckboxCheckedFillsWithPrimary(t *testing.T) {
	th := testTheme()
	// Left of the tick's short stroke and inside the border: bare fill.
	inside := image.Pt(3, 13)
	draw := func(checked bool) *image.RGBA {
		cb := th.Checkbox(&widget.Bool{Value: checked})
		return toggleRender(t, image.Pt(16, 16), time.Time{}, cb.Layout)
	}
	if img := draw(true); !toggleIs(img, inside, th.Primary) {
		t.Errorf("checked: pixel %v, want primary %v", img.RGBAAt(inside.X, inside.Y), th.Primary)
	}
	// input/30 (white at 15% x 30%) over the page, composited in sRGB as the
	// browser does: #09090b + 4.5% of the way to white.
	if img, want := draw(false), (color.NRGBA{R: 20, G: 20, B: 22, A: 255}); !toggleIs(img, inside, want) {
		t.Errorf("unchecked: pixel %v, want input/30 over the page %v", img.RGBAAt(inside.X, inside.Y), want)
	}
}

// opacity-50 fades the finished box over the page: primary at half, not at the
// alpha Gio would blend in linear light.
func TestCheckboxDisabledFadesOntoThePage(t *testing.T) {
	th := testTheme()
	cb := th.Checkbox(&widget.Bool{Value: true})
	cb.Disabled = true
	img := toggleRender(t, image.Pt(16, 16), time.Time{}, cb.Layout)
	if want := (color.NRGBA{R: 119, G: 119, B: 121, A: 255}); !toggleIs(img, image.Pt(3, 13), want) {
		t.Errorf("disabled checked: pixel %v, want primary at 50%% over the page %v", img.RGBAAt(3, 13), want)
	}
}
