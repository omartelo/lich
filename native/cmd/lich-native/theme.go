package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/omartelo/lich/native/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// The tokens and helpers diff.go and the tab bar still draw with; they go
// once those move to package ui.
var (
	colBackground = ui.Dark.Background
	colAccent     = ui.Dark.Accent
	colForeground = ui.Dark.Foreground
	colMuted      = ui.Dark.MutedForeground
	colBorder     = ui.Dark.Border
	colPass       = ui.Dark.TonePass
	colWait       = ui.Dark.ToneWait
	colDel        = ui.Dark.Destructive
)

func label(gtx C, th *material.Theme, s string, size unit.Sp, c color.NRGBA, weight font.Weight) D {
	l := material.Label(th, size, s)
	l.Color = c
	l.Font.Weight = weight
	l.MaxLines = 1
	l.Truncator = "…"
	return l.Layout(gtx)
}

func fillRRect(gtx C, c color.NRGBA, radius unit.Dp) D { return ui.Fill(gtx, c, radius) }
