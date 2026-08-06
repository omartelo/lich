package project

import (
	"errors"

	"github.com/ncruces/zenity"
)

// ZenityPicker opens the OS file chooser through github.com/ncruces/zenity —
// the Chromium shell has no native dialog of its own. Only Linux shells out to
// zenity/qarma; Windows and macOS run the dialog in-process, so there it is a
// window of lich's own and wears the executable's icon. Cancel maps to ("", nil).
type ZenityPicker struct{}

func (ZenityPicker) PickDirectory(title string) (string, error) {
	return mapZenity(zenity.SelectFile(zenity.Title(title), zenity.Directory()))
}

func (ZenityPicker) PickFile(title string) (string, error) {
	return mapZenity(zenity.SelectFile(zenity.Title(title)))
}

// ConfirmOverwrite is the dialog's own guard: the caller writes whatever path
// comes back, so replacing an existing file is agreed to here or nowhere.
func (ZenityPicker) PickSaveFile(title, defaultName string) (string, error) {
	return mapZenity(zenity.SelectFileSave(
		zenity.Title(title),
		zenity.Filename(defaultName),
		zenity.ConfirmOverwrite(),
	))
}

func mapZenity(path string, err error) (string, error) {
	if errors.Is(err, zenity.ErrCanceled) {
		return "", nil
	}
	return path, err
}
