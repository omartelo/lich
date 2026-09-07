//go:build darwin || linux

package awake

import (
	"errors"
	"os/exec"
	"testing"
	"time"
)

// The pipe is the release: closing it has to bring the inhibitor down on its
// own, with nothing signalled. A release that hangs here is one that would
// hang a session-state report.
func TestHoldReleasesByClosingThePipe(t *testing.T) {
	release, err := hold()
	if errors.Is(err, exec.ErrNotFound) {
		t.Skipf("no inhibitor on this machine: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { release(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("release did not return: the inhibitor outlived its pipe")
	}
}
