// stubBins, the Store every session test builds on, lives in the Unix-only
// suite (terminal_test.go), so these ride the same tag. What Windows would
// prove here it proves by construction instead: env_other.go's envReadable is
// a false constant, and the platform CI run covers the spawn path around it.
//go:build !windows

package terminal

import (
	"testing"

	"github.com/omartelo/lich/internal/events"
)

func TestSessionAccountWithoutALiveProcess(t *testing.T) {
	svc := New(stubBins{bin: "/opt/claude-work.sh"}, nil, events.New())

	env, custom, read := svc.SessionAccount("s1")

	if read || env != nil {
		t.Errorf("read = %v, env = %v, want nothing: the session has no process", read, env)
	}
	// Which binary a session runs is a settings question, answerable with no
	// process at all — and it is what keeps a card whose PTY is not up from
	// being served another account's quota.
	if !custom {
		t.Error("custom = false, want the configured binary reported")
	}
}

func TestSessionAccountOfADefaultSession(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())

	if _, custom, _ := svc.SessionAccount("s1"); custom {
		t.Error("custom = true for a session running the provider's own binary")
	}
}
