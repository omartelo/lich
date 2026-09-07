// Package awake keeps the machine from idling into sleep while a session is
// working, the way a media player does while it plays. Lock the screen and
// the agents go on; leave the machine at an idle prompt and it sleeps as it
// always did. What is held is the OS's own idle-sleep assertion, so `powercfg
// /requests`, `pmset -g assertions` and `systemd-inhibit --list` all name lich
// as the holder, and the user's own sleep policy is what decides the rest: a
// closed lid, a chosen Sleep and a critical battery all still sleep.
package awake

import (
	"log/slog"
	"sync"
)

// Keeper turns a count of working sessions into one held assertion.
type Keeper struct {
	mu      sync.Mutex
	release func()
	// hold takes the assertion and returns what lets go of it. Set per OS.
	hold func() (release func(), err error)
}

// New returns a Keeper backed by this OS's idle-sleep assertion.
func New() *Keeper {
	return &Keeper{hold: hold}
}

// Set reports how many sessions are working right now. The assertion is
// taken on the first and let go on the last; the numbers in between change
// nothing. A hold that fails is logged and tried again on the next rise from
// zero, so a machine without the tool (a Linux without systemd) costs one log
// line per burst of work, never a stuck session.
//
// The lock is held across the hold itself, which spawns a process on macOS and
// Linux. That costs milliseconds and buys the ordering: a release can never
// overtake the hold it undoes.
func (k *Keeper) Set(working int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	switch {
	case working > 0 && k.release == nil:
		release, err := k.hold()
		if err != nil {
			slog.Warn("keep awake", "err", err)
			return
		}
		k.release = release
	case working == 0 && k.release != nil:
		k.release()
		k.release = nil
	}
}
