package restart

import (
	"errors"
	"os"
	"testing"
	"time"
)

// fakeWindow stands in for chromium.Run: each run reports a process and then
// blocks until the test closes the window it opened.
type fakeWindow struct {
	runs    chan chan error
	ends    chan WindowEnd
	focuses chan struct{}
}

func newFakeWindow() (*fakeWindow, *Window) {
	f := &fakeWindow{runs: make(chan chan error, 4), ends: make(chan WindowEnd, 4), focuses: make(chan struct{}, 4)}
	w := NewWindow(
		func(onStart func(*os.Process)) error {
			onStart(&os.Process{Pid: 42})
			closed := make(chan error)
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
		func(func(*os.Process)) error { <-release; return nil },
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
	t.Run("terminates the open window and reports no end", func(t *testing.T) {
		f, w := newFakeWindow()
		w.Show()
		closed := f.opened(t)
		var terminated *os.Process
		w.terminate = func(p *os.Process) error {
			terminated = p
			closed <- nil
			return nil
		}
		w.Close()
		if terminated == nil || terminated.Pid != 42 {
			t.Fatalf("terminated %v, want the window's process", terminated)
		}
		select {
		case end := <-f.ends:
			t.Fatalf("a window closed by quitting was reported as ended: %+v", end)
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("no window open is nothing to close", func(t *testing.T) {
		_, w := newFakeWindow()
		w.terminate = func(*os.Process) error { t.Fatal("terminated with no window"); return nil }
		w.Close()
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
		f, w := newFakeWindow()
		w.Show()
		closed := f.opened(t)
		w.terminate = func(*os.Process) error { return errors.New("no such process") }
		done := make(chan struct{})
		go func() { w.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Close waited on a window it could not close")
		}
		closed <- nil
	})
}
