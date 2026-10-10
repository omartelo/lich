package chromium

import (
	"errors"
	"time"
)

// Ending is what a window that has ended means for the lich behind it. The
// backend outlives its window, so most endings leave it serving; only a launch
// that never put a window on screen has nobody to serve.
type Ending int

const (
	// WindowClosed: the user closed it. The sessions keep running and the next
	// launch of lich opens a window on them.
	WindowClosed Ending = iota
	// WindowTabInstead: macOS's last rung (TabFallback). There is no window of
	// lich's own here, so the page opens in the default browser.
	WindowTabInstead
	// WindowFailed: the window died with an error, after this lich already had
	// one on screen or after its startup. Report it and keep serving: the
	// sessions behind it are alive and the next launch opens a window on them.
	WindowFailed
	// WindowNeverOpened: this launch's first window failed to come up. Report it
	// and exit, as a launch that shows nothing must not leave a lich nobody can
	// see holding the port.
	WindowNeverOpened
)

// EndingOf reads a window's end: err is what Run returned, first is whether it
// was the first window this lich opened, uptime how long it ran.
func EndingOf(err error, first bool, uptime time.Duration) Ending {
	return endingOf(err, first, uptime, TabFallback)
}

func endingOf(err error, first bool, uptime time.Duration, tabFallback bool) Ending {
	switch {
	case err == nil:
		return WindowClosed
	case tabFallback && errors.Is(err, ErrNoShell):
		return WindowTabInstead
	case first && uptime < startupGrace:
		return WindowNeverOpened
	default:
		return WindowFailed
	}
}
