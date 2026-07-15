//go:build !nowails

package main

import (
	"log"
	"os"

	"github.com/omartelo/lich/internal/claudeplugin"
	"github.com/omartelo/lich/internal/fonts"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/terminal"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// wailsShellAvailable gates the shell choice in main: this build links the
// Wails webview shell. The nowails build (docs/chromium-shell.md phase 4)
// carries no Wails — and with it no GTK/WebKitGTK — linkage.
const wailsShellAvailable = true

// eventFallback is the hub's second channel: the Wails event bridge.
// application.Get() resolves lazily at emit time, after the app is created.
func eventFallback() func(name string, data any) {
	return func(name string, data any) {
		application.Get().Event.Emit(name, data)
	}
}

// shellPicker picks the native dialog implementation for the active shell.
func shellPicker(chromiumShell bool) project.Picker {
	if chromiumShell {
		return project.ZenityPicker{}
	}
	return project.WailsPicker{}
}

// runWails is the webview shell: the Wails application owns the window, the
// dialogs and the event bridge, exactly as before the Chromium migration.
func runWails(
	term *terminal.Service,
	proj *project.Service,
	fontSvc *fonts.Service,
	plugin *claudeplugin.Service,
	db *store.Service,
) {
	// The Name becomes the GTK application ID (org.wails.<name> on D-Bus),
	// which is single-instance: a second process with the same ID is treated
	// as a remote instance and never gets a window. A distinct dev name lets
	// `task dev` (LICH_DEV=1, see Taskfile.yml) open alongside an installed
	// lich.
	name := "lich"
	if os.Getenv("LICH_DEV") != "" {
		name = "lichdev"
	}

	app := application.New(application.Options{
		Name:        name,
		Description: "Personal harness",
		Services: []application.Service{
			application.NewService(term),
			application.NewService(fontSvc),
			application.NewService(proj),
			application.NewService(plugin),
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
