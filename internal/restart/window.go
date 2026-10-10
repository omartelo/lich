package restart

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

// closeWait bounds how long quitting waits for a window it asked to close. The
// wait is for Chromium to flush the profile (terminateProcess says why); a
// window that outstays it goes with this process anyway, through the stdin pipe
// it reads for EOF (internal/chromium).
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
	// process is the open window's process, nil until it has started.
	process *os.Process
	// exited is closed when the open window's run returns.
	exited chan struct{}
	opened bool
	// closing is set once Close has run: lich is quitting, so no window opens
	// again and the one closing is not reported as having ended.
	closing bool
	// run opens a window and blocks until it ends, handing its process to
	// onStart once it is up (chromium.Run).
	run func(onStart func(*os.Process)) error
	// focus brings the open window to the front.
	focus func()
	// ended hears how each window ended.
	ended     func(WindowEnd)
	terminate func(*os.Process) error
}

// NewWindow returns a keeper with no window open. run opens one and blocks
// until it ends, focus brings an open one forward, and ended is called, on the
// window's own goroutine, each time one ends.
func NewWindow(run func(onStart func(*os.Process)) error, focus func(), ended func(WindowEnd)) *Window {
	return &Window{run: run, focus: focus, ended: ended, terminate: terminateProcess}
}

// Show opens a window when none is open, and brings the open one forward
// otherwise. A window still starting is left to finish: focusing it then would
// race its own launch for the profile.
func (w *Window) Show() {
	w.mu.Lock()
	if w.closing {
		w.mu.Unlock()
		return
	}
	if w.open {
		started := w.process != nil
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
	go w.keep(first)
}

func (w *Window) keep(first bool) {
	started := time.Now()
	err := w.run(w.started)
	w.mu.Lock()
	w.open = false
	w.process = nil
	close(w.exited)
	closing := w.closing
	w.mu.Unlock()
	if closing {
		return
	}
	w.ended(WindowEnd{Err: err, First: first, Uptime: time.Since(started)})
}

func (w *Window) started(p *os.Process) {
	w.mu.Lock()
	w.process = p
	w.mu.Unlock()
}

// Close asks the open window to close and waits, up to closeWait, for it to
// go. It is the way out of a quitting lich: the window is closed rather than
// left to die with the process, so Chromium flushes the profile first.
func (w *Window) Close() {
	w.mu.Lock()
	w.closing = true
	process, exited := w.process, w.exited
	w.mu.Unlock()
	if process == nil {
		return
	}
	if err := w.terminate(process); err != nil {
		slog.Warn("close window", "err", err)
		return
	}
	select {
	case <-exited:
	case <-time.After(closeWait):
		slog.Warn("window did not close in time, leaving it to the exit", "wait", closeWait)
	}
}
