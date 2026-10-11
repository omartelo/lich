// Package ui is the native client's counterpart of frontend/src/components/ui
// and the Tailwind vocabulary around it: the design tokens of
// frontend/src/index.css, a spacing and type scale named after Tailwind's, and
// shadcn-style primitives whose variants carry shadcn's names, so a screen is
// ported by reading its TSX side by side.
//
// Gio is immediate mode: widget state (a widget.Clickable) belongs to the
// caller, and a primitive only draws it, like a controlled React component.
package ui

import (
	"image/color"
	"math"
)

// OKLCH converts a CSS oklch() color to sRGB, so tokens are written with the
// same numbers as index.css. alpha is 0..1.
func OKLCH(l, c, h, alpha float64) color.NRGBA {
	hr := h * math.Pi / 180
	a, b := c*math.Cos(hr), c*math.Sin(hr)
	lc := math.Pow(l+0.3963377774*a+0.2158037573*b, 3)
	mc := math.Pow(l-0.1055613458*a-0.0638541728*b, 3)
	sc := math.Pow(l-0.0894841775*a-1.2914855480*b, 3)
	return color.NRGBA{
		R: srgb(4.0767416621*lc - 3.3077115913*mc + 0.2309699292*sc),
		G: srgb(-1.2684380046*lc + 2.6097574011*mc - 0.3413193965*sc),
		B: srgb(-0.0041960863*lc - 0.7034186147*mc + 1.7076147010*sc),
		A: uint8(math.Round(alpha * 255)),
	}
}

func srgb(linear float64) uint8 {
	v := 12.92 * linear
	if linear > 0.0031308 {
		v = 1.055*math.Pow(linear, 1/2.4) - 0.055
	}
	return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255))
}

// Alpha is Tailwind's color/opacity modifier: bg-accent/60 is
// Alpha(p.Accent, 0.6).
func Alpha(c color.NRGBA, opacity float32) color.NRGBA {
	c.A = uint8(float32(c.A) * opacity)
	return c
}

// MixOKLab is CSS color-mix(in oklab, a, b weight): weight 0 is a, 1 is b.
// Alpha mixes linearly and the color premultiplied, as CSS specifies, so
// mixing toward transparent fades a without darkening it.
func MixOKLab(a, b color.NRGBA, weight float64) color.NRGBA {
	la, lb := toOKLab(a), toOKLab(b)
	alpha := la[3]*(1-weight) + lb[3]*weight
	if alpha == 0 {
		return color.NRGBA{}
	}
	var out [3]float64
	for i := range out {
		out[i] = (la[i]*la[3]*(1-weight) + lb[i]*lb[3]*weight) / alpha
	}
	return fromOKLab(out, alpha)
}

func linear(v uint8) float64 {
	x := float64(v) / 255
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}

// toOKLab returns L, a, b and alpha (0..1).
func toOKLab(c color.NRGBA) [4]float64 {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return [4]float64{
		0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
		float64(c.A) / 255,
	}
}

func fromOKLab(lab [3]float64, alpha float64) color.NRGBA {
	return OKLCH(lab[0], math.Hypot(lab[1], lab[2]), math.Atan2(lab[2], lab[1])*180/math.Pi, alpha)
}

// Over composites c onto the opaque backdrop the way a browser does, in sRGB,
// and returns the opaque result. Gio blends translucent paint in linear light
// instead, which renders a dark UI's /10 borders and /60 hovers about twice
// as bright as the web (white at 10% over #18181b: #5d5d5e against #2f2f31),
// so a translucent token must reach paint already composited.
func Over(backdrop, c color.NRGBA) color.NRGBA {
	a := float32(c.A) / 255
	blend := func(b, f uint8) uint8 { return uint8(float32(b) + (float32(f)-float32(b))*a + 0.5) }
	return color.NRGBA{R: blend(backdrop.R, c.R), G: blend(backdrop.G, c.G), B: blend(backdrop.B, c.B), A: 255}
}
