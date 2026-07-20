package terminal

import (
	"time"

	"github.com/omartelo/lich/internal/events"
)

// cwdEventName carries a session's live working directory ({id, cwd}), emitted
// once with the directory the PTY starts in and again whenever the child
// process moves (the user runs `cd`). Global like the other session events:
// its consumer (the session card's path line) may be unmounted when it fires.
const cwdEventName = "session-cwd"

// cwdEvent is the payload of cwdEventName.
type cwdEvent struct {
	ID  string `json:"id"`
	Cwd string `json:"cwd"`
}

// cwdPollInterval is how often a session's child working directory is read.
// Matches the spirit of the frontend's ~3s git-status poll: cheap enough to
// never matter, fresh enough to feel live.
const cwdPollInterval = 2 * time.Second

// watchCwd reports the session child's working directory to the frontend
// whenever it changes, until done closes. Each platform brings its own read
// (see the cwd_* files); on one without any, and when the PTY reports no
// child PID, this is a no-op.
func watchCwd(id string, pid int, initial string, done <-chan struct{}, hub *events.Hub) {
	if !cwdTracked || pid <= 0 {
		return
	}
	ticker := time.NewTicker(cwdPollInterval)
	defer ticker.Stop()
	pollCwd(pid, initial, ticker.C, done, func(cwd string) {
		hub.Emit(cwdEventName, cwdEvent{ID: id, Cwd: cwd})
	})
}

// pollCwd reads pid's working directory on every tick and calls emit when it
// differs from the last seen value, returning when done closes. An unreadable
// directory (the process just died, done is about to close) is skipped rather
// than reported. Split from watchCwd so tests drive the ticks.
func pollCwd(pid int, last string, tick <-chan time.Time, done <-chan struct{}, emit func(string)) {
	for {
		select {
		case <-done:
			return
		case <-tick:
			cwd := processCwd(pid)
			if cwd == "" || cwd == last {
				continue
			}
			last = cwd
			emit(cwd)
		}
	}
}
