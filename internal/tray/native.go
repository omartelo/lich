//go:build linux || windows

package tray

import (
	"log/slog"
	"runtime"
	"time"

	"fyne.io/systray"
)

// startNative runs the platform's tray loop on a thread of its own: Windows
// pumps its notification-area messages on the thread that created the icon,
// and Linux talks StatusNotifierItem over D-Bus. Neither needs main's thread.
func startNative(t *Tray) {
	go func() {
		runtime.LockOSThread()
		systray.Run(func() { t.ready() }, func() {})
	}()
}

func (t *Tray) ready() {
	systray.SetIcon(t.icon())
	systray.SetTooltip("lich")
	labels := t.currentLabels()
	show := systray.AddMenuItem(labels.Show, "")
	running := systray.AddMenuItem(labels.running(t.live()), "")
	running.Disable()
	systray.AddSeparator()
	quit := systray.AddMenuItem(labels.Quit, "")
	t.setRelabel(func(l Labels) {
		show.SetTitle(l.Show)
		running.SetTitle(l.running(t.live()))
		quit.SetTitle(l.Quit)
	})
	go t.serve(show, running, quit)
}

func (t *Tray) serve(show, running, quit *systray.MenuItem) {
	ticker := time.NewTicker(refreshEvery)
	defer ticker.Stop()
	for {
		select {
		case <-show.ClickedCh:
			t.show()
		case <-quit.ClickedCh:
			if err := t.quit(); err != nil {
				slog.Warn("tray: quit", "err", err)
			}
		case <-ticker.C:
			running.SetTitle(t.currentLabels().running(t.live()))
		}
	}
}
