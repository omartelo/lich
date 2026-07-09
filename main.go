package main

import (
	"embed"
	"log"

	"github.com/skipodotdev/skipo/internal/fonts"
	"github.com/skipodotdev/skipo/internal/project"
	"github.com/skipodotdev/skipo/internal/store"
	"github.com/skipodotdev/skipo/internal/terminal"

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
	db, err := store.New()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := application.New(application.Options{
		Name:        "skipo",
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
		Title:  "skipo",
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
