package ui

import (
	"image"
	"image/color"
	"testing"
	"time"
)

func TestSwitchSizes(t *testing.T) {
	th := testTheme()
	cases := []struct {
		size SwitchSize
		want image.Point
	}{
		{SwitchSizeDefault, image.Pt(32, 18)}, // 2rem x 1.15rem, 18.4 rounded
		{SwitchSizeSM, image.Pt(24, 14)},
	}
	for _, tc := range cases {
		s := th.Switch(new(SwitchState))
		s.Size = tc.size
		if d := layoutAt(s.Layout, 0, 500); d.Size != tc.want {
			t.Errorf("size %d: got %v, want %v", tc.size, d.Size, tc.want)
		}
	}
}

func TestSwitchClickFlipsChecked(t *testing.T) {
	state := new(SwitchState)
	rig := newToggleRig(image.Pt(20, 20), testTheme().Switch(state).Layout)
	rig.click(image.Pt(16, 9))
	if !state.Value {
		t.Fatal("click did not switch it on")
	}
	rig.click(image.Pt(16, 9))
	if state.Value {
		t.Error("second click did not switch it off")
	}
}

func TestSwitchPointerAreaReachesPastTheTrack(t *testing.T) {
	state := new(SwitchState)
	rig := newToggleRig(image.Pt(30, 30), testTheme().Switch(state).Layout)
	rig.click(image.Pt(-11, 9))
	if !state.Value {
		t.Error("a click 11px left of the track missed it, want after:-inset-x-3")
	}
}

func TestSwitchDisabledIgnoresClicks(t *testing.T) {
	state := new(SwitchState)
	s := testTheme().Switch(state)
	s.Disabled = true
	rig := newToggleRig(image.Pt(20, 20), s.Layout)
	rig.click(image.Pt(16, 9))
	if state.Value {
		t.Error("a disabled switch was switched on")
	}
}

// The thumb of the default switch spans x 1..17 off and 15..31 on, and takes
// 150ms to get there. Checked, it is primary-foreground, which no track color
// of the slide comes near.
func TestSwitchThumbSlidesOverTheTransition(t *testing.T) {
	th, state := testTheme(), new(SwitchState)
	s := th.Switch(state)
	t0 := time.Unix(1000, 0)
	dark := func(now time.Time, x int) bool {
		return toggleIs(toggleRender(t, image.Pt(32, 18), now, s.Layout), image.Pt(x, 9), th.PrimaryForeground)
	}
	dark(t0, 0) // seats the state off, before the flip
	state.Value = true

	if !dark(t0.Add(time.Second), 8) || dark(t0.Add(time.Second), 24) {
		t.Error("the frame of the flip: want the thumb still at the off end")
	}
	mid := t0.Add(time.Second + toggleSlideDuration/2)
	if dark(mid, 4) || !dark(mid, 24) {
		t.Error("halfway: want the thumb off the off end and short of the on end")
	}
	end := t0.Add(time.Second + 2*toggleSlideDuration)
	if dark(end, 4) || !dark(end, 29) {
		t.Error("after the transition: want the thumb at the on end")
	}
}

func TestSwitchFirstFrameDoesNotSlide(t *testing.T) {
	th, state := testTheme(), &SwitchState{}
	state.Value = true
	s := th.Switch(state)
	img := toggleRender(t, image.Pt(32, 18), time.Unix(1000, 0), s.Layout)
	if !toggleIs(img, image.Pt(29, 9), th.PrimaryForeground) {
		t.Errorf("pixel %v, want the thumb already at the on end", img.RGBAAt(29, 9))
	}
}

// A disabled switch fades as one layer: the thumb is half primary-foreground
// over the page, not over the half-faded track, which would leave it
// three-quarters opaque. Both are composited in sRGB, as the browser does.
func TestSwitchDisabledThumbSitsOnThePageNotTheTrack(t *testing.T) {
	state := new(SwitchState)
	state.Value = true
	s := testTheme().Switch(state)
	s.Disabled = true
	img := toggleRender(t, image.Pt(32, 18), time.Unix(1000, 0), s.Layout)
	thumb, track := color.NRGBA{R: 17, G: 17, B: 19, A: 255}, color.NRGBA{R: 119, G: 119, B: 121, A: 255}
	if !toggleIs(img, image.Pt(23, 9), thumb) {
		t.Errorf("thumb pixel %v, want primary-foreground at half over the page %v", img.RGBAAt(23, 9), thumb)
	}
	if !toggleIs(img, image.Pt(4, 9), track) {
		t.Errorf("track pixel %v, want primary at half over the page %v", img.RGBAAt(4, 9), track)
	}
}

// dark:data-unchecked:bg-input/80 is white at 12% over the page.
func TestSwitchOffTrackIsInputOverThePage(t *testing.T) {
	img := toggleRender(t, image.Pt(32, 18), time.Unix(1000, 0), testTheme().Switch(new(SwitchState)).Layout)
	if want := (color.NRGBA{R: 38, G: 38, B: 40, A: 255}); !toggleIs(img, image.Pt(28, 9), want) {
		t.Errorf("off track pixel %v, want input/80 over the page %v", img.RGBAAt(28, 9), want)
	}
}
