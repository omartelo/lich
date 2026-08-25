package terminal

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/omartelo/lich/internal/project"
)

// snapQueueDepth bounds the work waiting on the snapshot worker. A full queue
// drops the job rather than blocking the hook that raised it: losing a turn's
// diff is a panel that says so, where a stalled hook is an agent that stops.
const snapQueueDepth = 32

// The three answers LastTurnDiff can give, kept apart on the wire because the
// panel must never dress one as another: a turn that changed nothing is a real
// answer, and no turn on record is the absence of one.
const (
	turnDiffOK          = "ok"
	turnDiffEmpty       = "empty"
	turnDiffUnavailable = "unavailable"
)

// LastTurn is what the Review panel's "Last turn" mode reads. Diff is empty for
// every state but "ok", and EndedAt (unix ms) is set only when there is a window
// to date — its absence is the second signal that nothing is on record.
type LastTurn struct {
	State   string `json:"state"`
	Diff    string `json:"diff,omitempty"`
	EndedAt int64  `json:"endedAt,omitempty"`
}

// turnPair is one closed turn: the trees on either side of the window it ran in,
// and when that window shut.
type turnPair struct {
	before  string
	after   string
	endedAt time.Time
}

// snapState is one session's snapshot accounting. seq numbers the turns so a
// snapshot landing late is filed against the turn it was taken for and no other;
// before is the opening tree of turn seq, still empty while its job is queued.
type snapState struct {
	dir    string
	index  string
	seq    uint64
	open   bool
	before string
	last   *turnPair
}

// turnSnaps records what each session's last finished turn changed on disk, by
// bracketing the turn with two tree snapshots of its checkout
// (project.SnapshotTree). The turn boundary is the session-state contract's own
// `busy` and `done` (docs/hooks/session-state.md), which is why a provider that
// reports no state has no last turn here and never will.
//
// Every snapshot runs on ONE worker, for two reasons that happen to have the
// same fix. git refuses a second `add` against an index another one holds
// (`<index>.lock: File exists`), and a turn's two snapshots must be taken in the
// order they were asked for. A single FIFO worker gives both, at the cost of one
// session's cold snapshot delaying another's — acceptable while the cold case is
// paid once per checkout, at spawn. Per-session workers are the way out if it
// ever stops being.
//
// The zero value is ready to use.
type turnSnaps struct {
	mu       sync.Mutex
	sessions map[string]*snapState
	jobs     chan func()
	// root holds the index files. Empty means "resolve it from the config
	// directory on first use"; tests set it to a directory of their own.
	root string
	// filed is called once a session's last-turn record has changed, which is
	// the only moment the panel can act on: a turn is filed on the worker, well
	// after the `done` that closed it. Nil in a test that does not care.
	filed func(id string)
}

// track binds a session to the checkout its PTY was spawned at and warms the
// index, so the first turn's opening snapshot is not the one paying for hashing
// the whole worktree. Any previous accounting under this id belongs to the
// provider that left: a respawn is a new session's turns, and the last one it
// recorded is not this one's.
func (t *turnSnaps) track(id, dir string) {
	if id == "" || dir == "" {
		return
	}
	index, err := t.indexPath(id)
	if err != nil {
		slog.Warn("turnsnap: no place to keep the index", "session", id, "err", err)
		return
	}
	t.mu.Lock()
	if t.sessions == nil {
		t.sessions = make(map[string]*snapState)
	}
	t.sessions[id] = &snapState{dir: dir, index: index}
	t.mu.Unlock()
	// Warming discards the tree it computes: what is being bought is git's stat
	// cache in the index file, not the oid.
	t.submit(func() {
		if _, err := project.SnapshotTree(dir, index); err != nil {
			// A checkout git cannot snapshot has no turns to bracket, and the
			// ordinary reason is a session opened outside a repository at all.
			// Dropped once here rather than warned about twice per turn for the
			// rest of the session; the panel then reads "unavailable", which is
			// the true answer either way.
			slog.Debug("turnsnap: no snapshots for this checkout", "session", id, "err", err)
			t.forget(id)
		}
	})
}

// note reads one session-state report as a turn boundary. Repeat `busy` reports
// are what every provider sends between tool calls, and Antigravity sends one
// before each model call, so only the first of a run opens anything — otherwise
// the "before" would creep forward through the turn it is supposed to precede.
func (t *turnSnaps) note(id, state string) {
	switch state {
	case statusBusy:
		t.openTurn(id)
	case statusDone:
		t.closeTurn(id)
	case statusIdle:
		t.dropTurn(id)
	}
}

// openTurn takes the tree the turn starts from. The job is queued rather than
// run here: this is the hook's own goroutine, and it holds the agent's next step.
func (t *turnSnaps) openTurn(id string) {
	t.mu.Lock()
	state, ok := t.sessions[id]
	if !ok || state.open {
		t.mu.Unlock()
		return
	}
	state.seq++
	state.open = true
	state.before = ""
	seq, dir, index := state.seq, state.dir, state.index
	t.mu.Unlock()

	t.submit(func() {
		oid, err := project.SnapshotTree(dir, index)
		if err != nil {
			slog.Warn("turnsnap: opening snapshot failed", "session", id, "err", err)
			return
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		if state, ok := t.sessions[id]; ok && state.seq == seq {
			state.before = oid
		}
	})
}

// closeTurn takes the tree the turn ends on and files the pair. A turn whose
// opening snapshot never landed clears the record instead of keeping the one
// before it: this answers for the LAST turn, and offering an older one in its
// place would quietly answer a question nobody asked.
func (t *turnSnaps) closeTurn(id string) {
	t.mu.Lock()
	state, ok := t.sessions[id]
	if !ok || !state.open {
		t.mu.Unlock()
		return
	}
	state.open = false
	// Cleared here, not in the job: a job the queue drops never runs, and the
	// turn before it would otherwise stay on screen wearing this turn's name.
	// Between here and the snapshot landing the panel reads "unavailable",
	// which is what "the last turn is not recorded yet" honestly looks like.
	state.last = nil
	seq, dir, index := state.seq, state.dir, state.index
	t.mu.Unlock()

	t.submit(func() {
		oid, err := project.SnapshotTree(dir, index)
		if err != nil {
			slog.Warn("turnsnap: closing snapshot failed", "session", id, "err", err)
		}
		t.mu.Lock()
		state, ok := t.sessions[id]
		if ok && state.seq == seq && err == nil && state.before != "" {
			state.last = &turnPair{before: state.before, after: oid, endedAt: time.Now()}
		}
		filed := t.filed
		t.mu.Unlock()
		// Outside the lock, and whatever the outcome: a turn that lost its
		// snapshot changed the answer too, from "the turn before" to "none".
		if filed != nil {
			filed(id)
		}
	})
}

// dropTurn abandons a turn nothing will close — the CLI left mid-run. The last
// closed turn survives it: the session ending does not un-happen what its last
// turn did.
func (t *turnSnaps) dropTurn(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if state, ok := t.sessions[id]; ok {
		state.open = false
		state.before = ""
	}
}

// forget drops a closed session's accounting and its index file. A queued job
// for it finds no state and files nothing.
func (t *turnSnaps) forget(id string) {
	t.mu.Lock()
	state, ok := t.sessions[id]
	delete(t.sessions, id)
	t.mu.Unlock()
	if ok && state.index != "" {
		_ = os.Remove(state.index)
	}
}

// pair answers with the session's last closed turn and the checkout to render it
// against. ok is false when there is nothing on record — no turn has finished
// here, or the one that did lost a snapshot.
func (t *turnSnaps) pair(id string) (turnPair, string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state, ok := t.sessions[id]
	if !ok || state.last == nil {
		return turnPair{}, "", false
	}
	return *state.last, state.dir, true
}

// submit hands one snapshot to the worker, starting it on first use. A full
// queue drops the job: the turn it belonged to then reads as unavailable, which
// is the honest answer and not the one that says nothing changed.
func (t *turnSnaps) submit(job func()) {
	t.mu.Lock()
	if t.jobs == nil {
		t.jobs = make(chan func(), snapQueueDepth)
		go func(jobs <-chan func()) {
			for job := range jobs {
				job()
			}
		}(t.jobs)
	}
	jobs := t.jobs
	t.mu.Unlock()

	select {
	case jobs <- job:
	default:
		slog.Warn("turnsnap: snapshot queue full, dropping a turn's snapshot")
	}
}

// indexPath resolves where a session's index lives. LICH_DEV separates a
// development rig's indexes from the installed lich's, the same split the
// workspace database makes.
func (t *turnSnaps) indexPath(id string) (string, error) {
	t.mu.Lock()
	root := t.root
	t.mu.Unlock()

	if root == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve config directory: %w", err)
		}
		name := "turns"
		if os.Getenv("LICH_DEV") != "" {
			name = "turns-dev"
		}
		root = filepath.Join(dir, "lich", name)
	}
	return filepath.Join(root, id+".idx"), nil
}

// LastTurnDiff returns what changed on disk in the window this session's last
// finished turn ran in, as a unified diff.
//
// It is a window and not an attribution: everything that touched the checkout
// while the turn ran is in it — a formatter, an editor open beside lich, the
// user's own hands — and nothing here can tell them apart. The panel says as
// much, which is why it names the window rather than the agent.
func (s *Service) LastTurnDiff(id string) (LastTurn, error) {
	pair, dir, ok := s.snaps.pair(id)
	if !ok {
		return LastTurn{State: turnDiffUnavailable}, nil
	}
	ended := pair.endedAt.UnixMilli()
	// Equal trees is the whole answer: the turn ran and left the checkout as it
	// found it. No diff is computed, and none is needed.
	if pair.before == pair.after {
		return LastTurn{State: turnDiffEmpty, EndedAt: ended}, nil
	}
	text, err := project.TreeDiff(dir, pair.before, pair.after)
	if err != nil {
		return LastTurn{}, err
	}
	return LastTurn{State: turnDiffOK, Diff: text, EndedAt: ended}, nil
}
