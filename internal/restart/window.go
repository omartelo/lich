package restart

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// closeWait bounds how long quitting waits for a window it asked to close. The
// wait is for Chromium to flush the profile, where the page's prefs live (a
// window killed rather than closed loses what was written in its last seconds,
// #199); a window that outstays it goes with this process anyway, through the
// stdin pipe it reads for EOF (internal/chromium).
const closeWait = 5 * time.Second

// WindowEnd is how one window ended: what its run returned, whether it was the
// first window this lich opened, and how long it ran.
type WindowEnd struct {
	Err    error
	First  bool
	Uptime time.Duration
}

// Window is lich's window over a backend that outlives it. Closing the window
// leaves the backend and its sessions running; Show opens a new one on them, or
// brings forward the one already open. At most one is open at a time.
type Window struct {
	mu sync.Mutex
	// open is set from the moment a window is asked for until its run returns,
	// so a second Show while one is still starting does not open a second.
	open bool
	// closeWindow closes the open window, nil until it has started.
	closeWindow func() error
	// exited is closed when the open window's run returns.
	exited chan struct{}
	opened bool
	// closing is set once Close has run: lich is quitting, so no window opens
	// again and the one closing is not reported as having ended.
	closing bool
	// run opens a window on focus's card (none when empty) and blocks until it
	// ends, handing onStart the way to close it once it is up (chromium.Run).
	run func(focus string, onStart func(close func() error)) error
	// focus brings the open window to the front.
	focus func()
	// ended hears how each window ended.
	ended func(WindowEnd)
}

// NewWindow returns a keeper with no window open. run opens one, on a session's
// card when it names one, and blocks until it ends; focus brings an open one
// forward, and ended is called, on the window's own goroutine, each time one
// ends.
func NewWindow(run func(focus string, onStart func(close func() error)) error, focus func(), ended func(WindowEnd)) *Window {
	return &Window{run: run, focus: focus, ended: ended}
}

// Show opens a window when none is open, and brings the open one forward
// otherwise. A window still starting is left to finish: focusing it then would
// race its own launch for the profile.
func (w *Window) Show() {
	w.ShowSession("")
}

// ShowSession is Show for someone pointing at one session's card. A window it
// opens starts on that card: the event that opens a card in an open window is
// sent before a new page is there to hear it. An open window is only brought
// forward; the event is what moves it to the card.
func (w *Window) ShowSession(id string) {
	w.mu.Lock()
	if w.closing {
		w.mu.Unlock()
		return
	}
	if w.open {
		started := w.closeWindow != nil
		w.mu.Unlock()
		if started {
			w.focus()
		}
		return
	}
	w.open = true
	first := !w.opened
	w.opened = true
	w.exited = make(chan struct{})
	w.mu.Unlock()
	go w.keep(first, id)
}

func (w *Window) keep(first bool, focus string) {
	started := time.Now()
	err := w.run(focus, w.started)
	w.mu.Lock()
	w.open = false
	w.closeWindow = nil
	close(w.exited)
	closing := w.closing
	w.mu.Unlock()
	if closing {
		return
	}
	w.ended(WindowEnd{Err: err, First: first, Uptime: time.Since(started)})
}

func (w *Window) started(closeWindow func() error) {
	w.mu.Lock()
	w.closeWindow = closeWindow
	w.mu.Unlock()
}

// errNoWindow is Dismiss with no window up to close.
var errNoWindow = errors.New("no lich window is open")

// Dismiss closes the open window and leaves lich running, which is what the
// page asks for when the user answers that lich keeps running in the
// background. The window's end is reported like any other close.
func (w *Window) Dismiss() error {
	w.mu.Lock()
	closeWindow := w.closeWindow
	w.mu.Unlock()
	if closeWindow == nil {
		return errNoWindow
	}
	return closeWindow()
}

// Close asks the open window to close and waits, up to closeWait, for it to
// go. It is the way out of a quitting lich: the window is closed rather than
// left to die with the process, so Chromium flushes the profile first.
func (w *Window) Close() {
	w.mu.Lock()
	w.closing = true
	closeWindow, exited := w.closeWindow, w.exited
	w.mu.Unlock()
	if closeWindow == nil {
		return
	}
	if err := closeWindow(); err != nil {
		slog.Warn("close window", "err", err)
		return
	}
	select {
	case <-exited:
	case <-time.After(closeWait):
		slog.Warn("window did not close in time, leaving it to the exit", "wait", closeWait)
	}
}
