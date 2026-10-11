// Package shot writes a PNG of a Gio frame, drawn again off screen, so a run
// can be checked visually without a screen grabber.
package shot

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"gioui.org/gpu/headless"
	"gioui.org/op"
)

// Write renders ops into a headless window of size and saves it as a PNG.
func Write(path string, size image.Point, ops *op.Ops) error {
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		return fmt.Errorf("headless window: %w", err)
	}
	defer win.Release()
	if err := win.Frame(ops); err != nil {
		return fmt.Errorf("headless frame: %w", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		return fmt.Errorf("headless screenshot: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
