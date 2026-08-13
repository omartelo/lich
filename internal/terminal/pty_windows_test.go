//go:build windows

package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait after Close: %v", err)
	}
}
