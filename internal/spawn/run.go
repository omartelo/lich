package spawn

import (
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/terminal"
)

// Run opens a checkout's run card: a terminal session in the checkout at cwd
// whose entrypoint is the project's .lich/run-worktree.sh, so the worktree ends
// up running the project's own app — on the port lich reserved for it, when the
// script spells $LICH_WORKTREE_PORT.
//
// It lives in this package, and not in the window that asks for it, for the
// reason the package exists: the window starts a session's PTY only once
// somebody has looked at the card, and a dev server that starts when you click
// it is not a checkout running anything. Started here, the app is up the moment
// Run returns, whether or not the card is ever opened.
//
// The card is a plain terminal, so what it runs is what the script says and
// nothing supervises it. When the command exits — a crash, a Ctrl-C, a port
// already taken — the entrypoint wrapper leaves the user's own shell in the
// same card, with the error still on screen (internal/terminal.wrapEntrypoint).
//
// cwd is any checkout of the project: a worktree, or the project's own
// directory. One checkout gets one Run card: asking again hands back the card it
// already has, since a second one could only report the port it cannot bind. A
// card whose command has exited still holds that slot: the wrapper left the
// user's shell in it and that shell is the retry, so only closing the card frees
// it, which is what the loaded state already says by listing open rows alone
// (store.sessionsOf).
func (s *Service) Run(projectID, cwd string) (Session, error) {
	if projectID == "" || cwd == "" {
		return Session{}, fmt.Errorf("a run card needs a project and a checkout to open in")
	}

	// The same lock Open takes, for the same reason: the label counter is read
	// and written back, and two callers reading the same number hand two cards
	// the same name.
	s.open.Lock()
	defer s.open.Unlock()

	projects, err := s.sessions.LoadState()
	if err != nil {
		return Session{}, fmt.Errorf("read the workspace: %w", err)
	}
	target, ok := projectByID(projects, projectID)
	if !ok {
		return Session{}, fmt.Errorf("no open project with id %q", projectID)
	}
	// Before the script is even read: the card this checkout already has is the
	// answer whatever .lich/run-worktree.sh says now.
	if live, ok := runCardIn(target, storedPath(target.Path, cwd)); ok {
		return live, nil
	}
	script := project.RunScript(target.Path)
	if script == "" {
		return Session{}, fmt.Errorf(
			"%s ships no .lich/run-worktree.sh, so there is no command to run — write one "+
				"in the New worktree dialog, or add the file to the repository",
			target.Name,
		)
	}

	id, err := newSessionID()
	if err != nil {
		return Session{}, err
	}
	opened := Session{
		ID:        id,
		ProjectID: target.ID,
		Project:   target.Name,
		// The command names the card, the way setting an entrypoint by hand
		// does (frontend/src/lib/session/sessions.ts, setSessionEntrypoint).
		Label:   firstLine(script),
		Kind:    terminal.KindShell,
		Path:    storedPath(target.Path, cwd),
		NextSeq: target.NextSeq + 1,
		Run:     true,
	}
	if err := s.sessions.AddSessionFrom(
		target.ID, id, opened.Label, opened.Kind, opened.Path, opened.NextSeq, "", "",
	); err != nil {
		return Session{}, err
	}
	// Before the spawn rather than after it: terminal.Start reads the entrypoint
	// off the row, so a write that lands later opens a bare shell instead of the
	// app. Both writes are this goroutine's, in order, which is what the window
	// could not have promised had it made them over two RPC calls. It is also
	// what marks the row a Run card, so the lookup above can find it.
	if err := s.sessions.SetRunEntrypoint(id, script); err != nil {
		return Session{}, fmt.Errorf("record the run command on session %q: %w", opened.Label, err)
	}
	// Announced before the spawn, and regardless of how it goes, for Open's
	// reason: a session only the database knows about is one the user cannot
	// reach to see what went wrong.
	if s.events != nil {
		s.events.Emit(OpenedEventName, opened)
	}
	// fork and setup are both false: the run card opens a shell in a checkout
	// that already exists — it has no conversation to branch, and the setup
	// script belongs to the session that created the checkout.
	if err := s.term.Start(
		id, target.ID, cwd, opened.Kind, "", "", false, false, startCols, startRows,
	); err != nil {
		return Session{}, fmt.Errorf("the run card was created but its terminal did not start: %w", err)
	}
	return opened, nil
}

// runCardIn finds a checkout's live Run card among a project's loaded sessions,
// matched on the row's own mark rather than on its entrypoint: the script on
// disk moves, and a card renamed or re-aimed by hand is still the run seat this
// checkout spent.
//
// path is the store's spelling of the checkout (storedPath), which is what the
// row holds and what the window groups by.
func runCardIn(target store.Project, path string) (Session, bool) {
	for _, sess := range target.Sessions {
		if !sess.Run || sess.Path != path {
			continue
		}
		return Session{
			ID:        sess.ID,
			ProjectID: target.ID,
			Project:   target.Name,
			Label:     sess.Label,
			Kind:      sess.Kind,
			Path:      sess.Path,
			// The counter stands where it is: no label was taken this time.
			NextSeq: target.NextSeq,
			Run:     true,
		}, true
	}
	return Session{}, false
}

// projectByID finds an open project by its id — what the window addresses a
// project by, having one on screen already, where every other caller here
// arrives with a name or a path to resolve.
func projectByID(projects []store.Project, id string) (store.Project, bool) {
	for _, p := range projects {
		if p.ID == id {
			return p, true
		}
	}
	return store.Project{}, false
}

// firstLine names a card after a script whose first line is its command — the
// shape every run script has, and the one a multi-line one is truncated to
// rather than wrapped into a label nothing can read.
func firstLine(script string) string {
	line, _, _ := strings.Cut(script, "\n")
	return strings.TrimSpace(line)
}

// storedPath is what the row records for a session rooted at cwd: empty for the
// project's own directory, the checkout itself for a worktree. The store's own
// spelling, and what the window's grouping reads.
func storedPath(projectPath, cwd string) string {
	if cwd == projectPath {
		return ""
	}
	return cwd
}
