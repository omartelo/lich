// stubBins, the Store every session test builds on, lives in the Unix-only
// suite (terminal_test.go), so these ride the same tag. Windows exercises its
// environment reader directly in env_self_test.go.
//go:build !windows

package terminal

import (
	"testing"

	"github.com/omartelo/lich/internal/events"
)

// A card whose PTY is not up has no environment to read, and reporting it as
// read is what would serve it the machine-wide login's quota.
func TestSessionAccountWithoutALiveProcess(t *testing.T) {
	svc := New(stubBins{bin: "/opt/claude-work.sh"}, nil, events.New())

	env, read := svc.SessionAccount("s1")

	if read || env != nil {
		t.Errorf("read = %v, env = %v, want nothing: the session has no process", read, env)
	}
}
