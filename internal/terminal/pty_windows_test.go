//go:build windows

package terminal

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestWindowsPTYCloseRunsExitHandler proves a session close reaches the child
// as a console Ctrl+C: it leaves through its own interrupt handler, which a
// kill would never let it run. The child does nothing to make itself hear that
// signal — startPTY clearing the inherited "ignore Ctrl+C" (see heedCtrlC) is
// what delivers it, and this test is red without that call, because a CI
// runner is exactly the service whose children inherit the attribute.
func TestWindowsPTYCloseRunsExitHandler(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved")
	p, transcript := startWindowsCloseChild(t, path)

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if saved, err := os.ReadFile(path); err != nil || string(saved) != "saved on interrupt" {
		t.Fatalf("exit handler's file after Close = %q, %v; the child said %q", saved, err, transcript())
	}
}

// startWindowsCloseChild runs this test binary as a child that saves a file
// when it is interrupted, and returns once that child says its handler is
// armed — a Ctrl+C delivered before then would be answered by the default
// disposition, and the test would be measuring the runtime instead of the
// child. The second return reads back everything the child has said, which is
// the only account of a close that went wrong on a machine nobody can attach
// a debugger to.
func startWindowsCloseChild(t *testing.T, path string) (ptyHandle, func() string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p, err := startPTY(ptySpec{
		bin: exe, args: []string{"-test.run=^TestWindowsPTYCloseChild$"},
		// Not t.TempDir(): the child holds its working directory open for as
		// long as it takes Windows to tear it down after the close, and the
		// cleanup that cannot delete it fails the test for the wrong reason.
		dir: os.TempDir(), env: append(os.Environ(), "LICH_TEST_CLOSE_FILE="+path),
		cols: 80, rows: 24,
	})
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	var mu sync.Mutex
	var seen bytes.Buffer
	transcript := func() string {
		mu.Lock()
		defer mu.Unlock()
		return seen.String()
	}
	armed := make(chan struct{})
	go func() {
		said := false
		buf := make([]byte, 256)
		for {
			n, err := p.Read(buf)
			mu.Lock()
			seen.Write(buf[:n])
			hit := bytes.Contains(seen.Bytes(), []byte("armed"))
			mu.Unlock()
			if hit && !said {
				said = true
				close(armed)
			}
			if err != nil {
				return
			}
		}
	}()
	select {
	case <-armed:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for the child to arm its Ctrl+C handler; it said %q", transcript())
	}
	return p, transcript
}

// TestWindowsPTYCloseChild is the child startWindowsCloseChild runs, and a
// no-op in every other run of this binary.
func TestWindowsPTYCloseChild(t *testing.T) {
	path := os.Getenv("LICH_TEST_CLOSE_FILE")
	if path == "" {
		return
	}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	fmt.Println("armed")

	select {
	case <-interrupt:
	case <-time.After(30 * time.Second):
		t.Fatal("no interrupt arrived")
	}
	if err := os.WriteFile(path, []byte("saved on interrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestWindowsPTYCloseIsSingleShot pins the guard windowsPTY puts over conpty's
// Close, which the service relies on: stream reaps a PTY the user-driven Close
// already released. A second call reaching ConPty.Close would run
// ClosePseudoConsole on a freed HPCON and CloseHandle on six values this
// process may have reissued since — the witness file is opened between the two
// closes for that reason, holding a fresh handle of the kind a stale one gets
// reused as.
func TestWindowsPTYCloseIsSingleShot(t *testing.T) {
	p, err := startPTY(ptySpec{
		bin:  "cmd.exe",
		args: []string{"/c", "pause"},
		// Not t.TempDir(): the child holds its working directory open for as
		// long as it takes Windows to tear it down after the close, and the
		// cleanup that cannot delete it fails the test for the wrong reason.
		dir:  os.TempDir(),
		cols: 80,
		rows: 24,
	})
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	witness, err := os.Create(filepath.Join(t.TempDir(), "witness"))
	if err != nil {
		t.Fatalf("open witness: %v", err)
	}
	defer witness.Close()

	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := witness.WriteString("still open"); err != nil {
		t.Fatalf("second Close took another handle: %v", err)
	}
	if _, err := p.Wait(); err != nil {
		t.Fatalf("Wait after Close: %v", err)
	}
}
