package project

import (
	"errors"

	"github.com/ncruces/zenity"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// WailsPicker asks through the Wails dialog API — only valid while the Wails
// application shell is running.
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

// ZenityPicker shells out to zenity/qarma (github.com/ncruces/zenity) — the
// Chromium shell path, where no Wails application exists. Cancel maps to
// ("", nil), matching the Wails dialog contract.
type ZenityPicker struct{}

func (ZenityPicker) PickDirectory(title string) (string, error) {
	return mapZenity(zenity.SelectFile(zenity.Title(title), zenity.Directory()))
}

func (ZenityPicker) PickFile(title string) (string, error) {
	return mapZenity(zenity.SelectFile(zenity.Title(title)))
}

func mapZenity(path string, err error) (string, error) {
	if errors.Is(err, zenity.ErrCanceled) {
		return "", nil
	}
	return path, err
}
