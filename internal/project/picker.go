package project

import (
	"errors"

	"github.com/ncruces/zenity"
)

// ZenityPicker shells out to zenity/qarma (github.com/ncruces/zenity) — the
// Chromium shell has no native dialog of its own. Cancel maps to ("", nil).
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
