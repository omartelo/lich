package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/omartelo/lich/internal/fonts"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/terminal"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

// main is the application's entry point. It creates the application, opens the
// main window and blocks until the app exits.
func main() {
	// WebKitGTK under Wayland fractional scaling renders every damage frame
	// at 2x and downsamples it on the CPU — typing in a full-size window cost
	// ~40ms/frame of engine time regardless of raster backend (measured
	// 2026-07-10; GPU policy, DMABUF, Skia threads and canvas alpha all made
	// no difference). Under X11/Xwayland the app sees an integer scale and the
	// same workload runs stall-free at full frame rate. Respect an explicit
	// GDK_BACKEND so this stays overridable.
	if runtime.GOOS == "linux" && os.Getenv("GDK_BACKEND") == "" {
		if err := os.Setenv("GDK_BACKEND", "x11"); err != nil {
			log.Printf("failed to set GDK_BACKEND: %v", err)
		}
	}

	db, err := store.New()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := application.New(application.Options{
		Name:        "lich",
		Description: "Personal harness",
		Services: []application.Service{
			application.NewService(terminal.New(db)),
			application.NewService(fonts.New()),
			application.NewService(project.New()),
			application.NewService(db),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Window sized to the golden ratio (1000 / 618 ≈ 1.618).
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "lich",
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
