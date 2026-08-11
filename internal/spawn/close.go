package spawn

import (
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/store"
)

// What a caller may do with the checkout a session was the last one in. The
// window asks the same question with a dialog; a caller outside it has nobody
// to ask, so it has to say up front.
const (
	// KeepWorktree leaves the checkout on disk and parks the session's row, so
	// its conversation can be resumed later — the window's "keep" answer.
	KeepWorktree = "keep"
	// RemoveWorktree deletes the checkout and the session with it.
	RemoveWorktree = "remove"
)

// Closed is what a caller is told about the session it closed.
type Closed struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Project   string `json:"project"`
	Label     string `json:"label"`
	// Worktree is the checkout the session lived in, empty for a session in the
	// project's own directory.
	Worktree string `json:"worktree"`
	// Kept is whether the checkout was left on disk (and the session parked for
	// a later resume) rather than removed. Meaningless without a Worktree.
	Kept bool `json:"kept"`
	// Removed is whether the checkout was deleted.
	Removed bool `json:"removed"`
}

// Close closes a session, and settles what happens to the checkout it was the
// last one in.
//
// The window asks about that in a dialog, twice over when the checkout has
// uncommitted work. Nobody can be asked here, so the answer is an argument:
// closing the last session of a worktree without one is an error naming the
// choice rather than a guess. Removing a checkout that is dirty needs force on
// top, because what it discards exists in no other place — not in a commit, not
// on a remote, not in the session that was working on it.
//
// A session that shares its checkout with another, or lives in the project's own
// directory, has nothing at stake and closes on the spot.
func (s *Service) Close(fromID, target, projectName, worktree string, force bool) (Closed, error) {
	if worktree != "" && worktree != KeepWorktree && worktree != RemoveWorktree {
		return Closed{}, fmt.Errorf(
			"%q is not something to do with a worktree — %q keeps the checkout, %q deletes it",
			worktree, KeepWorktree, RemoveWorktree,
		)
	}

	projects, err := s.sessions.LoadState()
	if err != nil {
		return Closed{}, fmt.Errorf("read the workspace: %w", err)
	}
	found, err := findSession(projects, fromID, target, projectName)
	if err != nil {
		return Closed{}, err
	}

	closed := Closed{
		ID:        found.session.ID,
		ProjectID: found.project.ID,
		Project:   found.project.Name,
		Label:     found.session.Label,
		Worktree:  found.session.Path,
	}
	last := found.session.Path != "" && lastInWorktree(found.project, found.session)
	if last && worktree == "" {
		return Closed{}, fmt.Errorf(
			"%q is the last session in the worktree %s, so closing it decides that checkout's "+
				"fate: say %q to leave it on disk (the session is parked and can be resumed) "+
				"or %q to delete it",
			found.session.Label, found.session.Path, KeepWorktree, RemoveWorktree,
		)
	}

	active := neighborOf(found.project, found.session.ID)
	if last && worktree == KeepWorktree {
		if err := s.sessions.CloseSession(found.project.ID, found.session.ID, active); err != nil {
			return Closed{}, err
		}
		closed.Kept = true
		s.finish(found, active)
		return closed, nil
	}
	if last && worktree == RemoveWorktree {
		if err := s.removeCheckout(found, active, force); err != nil {
			return Closed{}, err
		}
		closed.Removed = true
		s.finish(found, active)
		return closed, nil
	}

	if err := s.sessions.DeleteSession(found.project.ID, found.session.ID, active); err != nil {
		return Closed{}, err
	}
	s.finish(found, active)
	return closed, nil
}

// removeCheckout takes a worktree off disk together with the session that was
// the last one in it.
//
// The order is the window's, and it is load-bearing: the PTY dies before git is
// asked to remove the directory it is running in, and the parked rows go before
// the checkout does — one left behind would offer a resume into a directory that
// no longer exists.
func (s *Service) removeCheckout(found located, active string, force bool) error {
	dirty, err := s.worktrees.WorktreeDirty(found.session.Path)
	if err == nil && dirty && !force {
		return fmt.Errorf(
			"the worktree %s has uncommitted work, and removing it discards what is in no "+
				"commit and on no remote. Force it only if that is what you mean",
			found.session.Path,
		)
	}
	if err := s.term.Close(found.session.ID); err != nil {
		return fmt.Errorf("close the session's terminal: %w", err)
	}
	if err := s.sessions.DeleteSession(found.project.ID, found.session.ID, active); err != nil {
		return err
	}
	if err := s.sessions.PurgeWorktreeSessions(found.project.ID, found.session.Path); err != nil {
		return err
	}
	return s.worktrees.RemoveWorktree(found.project.Path, found.session.Path, force)
}

// finish takes down the PTY and the card. The terminal close is repeated for
// the paths that already made it (removeCheckout) because it is a no-op the
// second time, and skipping it on the others would leave an agent running in a
// session nobody can reach.
func (s *Service) finish(found located, active string) {
	if err := s.term.Close(found.session.ID); err != nil {
		// The row is already gone, so there is nothing to undo and nothing the
		// caller could do about it. The window's own close ignores this too.
		return
	}
	if s.events != nil {
		s.events.Emit(ClosedEventName, ClosedEvent{
			ID: found.session.ID, ProjectID: found.project.ID, ActiveID: active,
		})
	}
}

// located is a session together with the project holding it.
type located struct {
	project store.Project
	session store.Session
}

// findSession resolves a session by the label on its card or the name it answers
// to in the peer roster, exactly as the relay does — an agent holding either
// should not have to know which one it has.
//
// Unlike the relay it looks past the live ones: a card whose terminal was never
// opened is still a session, and closing it is the one thing you can do with it.
func findSession(projects []store.Project, fromID, target, projectName string) (located, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return located{}, fmt.Errorf("no session given to close")
	}

	var matches []located
	for _, p := range projects {
		if projectName != "" && !strings.EqualFold(p.Name, projectName) {
			continue
		}
		for _, sess := range p.Sessions {
			cwd := sess.Path
			if cwd == "" {
				cwd = p.Path
			}
			if strings.EqualFold(sess.Label, target) ||
				strings.EqualFold(relay.RosterName(cwd, sess.ID), target) {
				matches = append(matches, located{project: p, session: sess})
			}
		}
	}

	switch len(matches) {
	case 1:
		if matches[0].session.ID == fromID {
			return located{}, fmt.Errorf(
				"%q is this session — closing it would take down the agent asking for it",
				target,
			)
		}
		return matches[0], nil
	case 0:
		where := ""
		if projectName != "" {
			where = fmt.Sprintf(" in project %q", projectName)
		}
		return located{}, fmt.Errorf("no session named %q%s", target, where)
	default:
		return located{}, fmt.Errorf(
			"%q names %d sessions — narrow it with the project", target, len(matches),
		)
	}
}

// lastInWorktree reports whether a session is the only one left in its checkout,
// which is what makes closing it a decision about that checkout.
func lastInWorktree(p store.Project, sess store.Session) bool {
	for _, other := range p.Sessions {
		if other.ID != sess.ID && other.Path == sess.Path {
			return false
		}
	}
	return true
}

// neighborOf picks the session that takes the closed one's place as the
// project's active card: the one that fills its slot, else the one before it,
// else none. The same rule the window applies (sessions.ts, neighborId) — the
// row it writes is the one the window reads back on the next launch.
func neighborOf(p store.Project, id string) string {
	index := -1
	for i, sess := range p.Sessions {
		if sess.ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		return p.ActiveSessionID
	}
	left := append([]store.Session{}, p.Sessions[:index]...)
	left = append(left, p.Sessions[index+1:]...)
	if len(left) == 0 {
		return ""
	}
	if index < len(left) {
		return left[index].ID
	}
	return left[index-1].ID
}
