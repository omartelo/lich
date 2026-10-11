package restart

import (
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// fakeWindow stands in for chromium.Run: each run hands over its close and
// then blocks until the test, or that close, ends the window it opened.
type fakeWindow struct {
	runs    chan chan error
	ends    chan WindowEnd
	focuses chan struct{}
	// focused is the card each run was opened on, read only after its run
	// has been received.
	focused []string
	// closes counts the closes asked of the open window.
	closes atomic.Int32
}

func newFakeWindow() (*fakeWindow, *Window) {
	f := &fakeWindow{runs: make(chan chan error, 4), ends: make(chan WindowEnd, 4), focuses: make(chan struct{}, 4)}
	w := NewWindow(
		func(focus string, onStart func(close func() error)) error {
			f.focused = append(f.focused, focus)
			closed := make(chan error, 1)
			onStart(func() error {
				f.closes.Add(1)
				closed <- nil
				return nil
			})
			f.runs <- closed
			return <-closed
		},
		func() { f.focuses <- struct{}{} },
		func(end WindowEnd) { f.ends <- end },
	)
	return f, w
}

func (f *fakeWindow) opened(t *testing.T) chan error {
	t.Helper()
	select {
	case closed := <-f.runs:
		return closed
	case <-time.After(time.Second):
		t.Fatal("no window opened")
		return nil
	}
}

func (f *fakeWindow) ended(t *testing.T) WindowEnd {
	t.Helper()
	select {
	case end := <-f.ends:
		return end
	case <-time.After(time.Second):
		t.Fatal("the window's end was not reported")
		return WindowEnd{}
	}
}

func TestWindowReopensAfterClose(t *testing.T) {
	f, w := newFakeWindow()

	w.Show()
	f.opened(t) <- nil
	if end := f.ended(t); end.Err != nil || !end.First {
		t.Fatalf("first end = %+v, want a clean close of the first window", end)
	}

	w.Show()
	crash := errors.New("exit status 139")
	f.opened(t) <- crash
	if end := f.ended(t); !errors.Is(end.Err, crash) || end.First {
		t.Fatalf("second end = %+v, want the crash of a window that was not the first", end)
	}
}

func TestWindowShowFocusesTheOpenOne(t *testing.T) {
	f, w := newFakeWindow()

	w.Show()
	closed := f.opened(t)
	w.Show()
	select {
	case <-f.focuses:
	case <-time.After(time.Second):
		t.Fatal("Show on an open window did not focus it")
	}
	select {
	case <-f.runs:
		t.Fatal("Show on an open window opened a second one")
	default:
	}
	closed <- nil
	f.ended(t)
}

func TestWindowShowWhileStartingDoesNothing(t *testing.T) {
	release := make(chan struct{})
	focused := 0
	w := NewWindow(
		func(string, func(func() error)) error { <-release; return nil },
		func() { focused++ },
		func(WindowEnd) {},
	)
	w.Show()
	w.Show()
	close(release)
	if focused != 0 {
		t.Fatal("focused a window that had not started")
	}
}

func TestWindowClose(t *testing.T) {
	t.Run("closes the open window and reports no end", func(t *testing.T) {
		f, w := newFakeWindow()
		w.Show()
		f.opened(t)
		w.Close()
		if got := f.closes.Load(); got != 1 {
			t.Fatalf("closes = %d, want the open window closed once", got)
		}
		select {
		case end := <-f.ends:
			t.Fatalf("a window closed by quitting was reported as ended: %+v", end)
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("no window open is nothing to close", func(t *testing.T) {
		f, w := newFakeWindow()
		w.Close()
		if got := f.closes.Load(); got != 0 {
			t.Fatalf("closes = %d with no window, want none", got)
		}
	})

	t.Run("no window opens once quitting", func(t *testing.T) {
		f, w := newFakeWindow()
		w.Close()
		w.Show()
		select {
		case <-f.runs:
			t.Fatal("a quitting lich opened a window")
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("a failed close does not wait", func(t *testing.T) {
		release := make(chan struct{})
		w := NewWindow(
			func(_ string, onStart func(func() error)) error {
				onStart(func() error { return errors.New("broken pipe") })
				<-release
				return nil
			},
			func() {}, func(WindowEnd) {},
		)
		w.Show()
		deadline := time.Now().Add(time.Second)
		for !w.hasStarted() {
			if time.Now().After(deadline) {
				t.Fatal("the window never started")
			}
			time.Sleep(time.Millisecond)
		}
		done := make(chan struct{})
		go func() { w.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Close waited on a window it could not close")
		}
		close(release)
	})
}

// Dismiss is the page's "keep running": the window closes, lich does not, and
// the end is reported as a close, so a later Show opens a window again.
func TestWindowDismiss(t *testing.T) {
	f, w := newFakeWindow()
	if err := w.Dismiss(); err == nil {
		t.Fatal("Dismiss with no window = nil, want an error")
	}
	w.Show()
	f.opened(t)
	if err := w.Dismiss(); err != nil {
		t.Fatalf("Dismiss = %v", err)
	}
	if end := f.ended(t); end.Err != nil || !end.Dismissed {
		t.Fatalf("end = %+v, want a clean close marked dismissed", end)
	}
	w.Show()
	f.opened(t) <- nil
	if end := f.ended(t); end.Dismissed {
		t.Fatalf("end = %+v, a window closed on its own was reported dismissed", end)
	}
}

func TestWindowShowSessionOpensOnTheCard(t *testing.T) {
	f, w := newFakeWindow()

	w.ShowSession("s2")
	closed := f.opened(t)
	if !slices.Equal(f.focused, []string{"s2"}) {
		t.Fatalf("opened on %q, want the window opened on s2's card", f.focused)
	}
	// An open window is brought forward and not opened again on the card.
	w.ShowSession("s3")
	select {
	case <-f.focuses:
	case <-time.After(time.Second):
		t.Fatal("ShowSession on an open window did not focus it")
	}
	closed <- nil
	f.ended(t)

	w.Show()
	f.opened(t) <- nil
	f.ended(t)
	if !slices.Equal(f.focused, []string{"s2", ""}) {
		t.Fatalf("opened on %q, want a plain Show to open on no card", f.focused)
	}
}

func (w *Window) hasStarted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeWindow != nil
}
