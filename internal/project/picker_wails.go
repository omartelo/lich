//go:build !nowails

package project

import "github.com/wailsapp/wails/v3/pkg/application"

// WailsPicker asks through the Wails dialog API — only valid while the Wails
// application shell is running. Excluded from nowails builds, which carry no
// Wails linkage at all (docs/chromium-shell.md phase 4).
type WailsPicker struct{}

func (WailsPicker) PickDirectory(title string) (string, error) {
	return application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle(title).
		PromptForSingleSelection()
}

func (WailsPicker) PickFile(title string) (string, error) {
	return application.Get().Dialog.OpenFile().
		CanChooseFiles(true).
		CanChooseDirectories(false).
		SetTitle(title).
		PromptForSingleSelection()
}
