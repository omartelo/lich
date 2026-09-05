package spawn

import (
	"fmt"
	"log/slog"
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
	// Kept is the answer given to the worktree question: the checkout was left on
	// disk rather than removed. Meaningless unless the close had that question to
	// answer — a session sharing its checkout, or living in the project's own
	// directory, reports false while leaving everything where it was.
	//
	// Nothing here reports the park, because every close but a removal parks and
	// a field that is true on all of them tells a caller nothing.
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
	found, err := findSession(projects, target, projectName)
	if err != nil {
		return Closed{}, err
	}
	if found.session.ID == fromID {
		return Closed{}, fmt.Errorf(
			"%q is this session — closing it would take down the agent asking for it", target,
		)
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

	// Parked, not deleted: nothing about this close is a decision to destroy the
	// session. The row carries the conversation, the cost ledgers and the name
	// the user gave it, and it is what puts the session in the history a resume
	// reads back — the same close the window performs (projects.tsx,
	// closeSession). Only a checkout's removal takes a row with it.
	if err := s.sessions.CloseSession(found.project.ID, found.session.ID, active); err != nil {
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
	// Asked before anything is taken apart, not after: the removal itself is
	// refused for an adopted checkout (project.RemoveWorktree), but by then the
	// terminal is closed and the rows are gone, and the session would be lost to
	// keep a directory that was never at risk.
	if s.worktrees.WorktreeAdopted(found.session.Path) {
		return fmt.Errorf(
			"the worktree %s was not created by lich, so closing this session cannot delete it: "+
				"say %q instead, which parks the session and leaves the checkout where it is",
			found.session.Path, KeepWorktree,
		)
	}
	// A check that cannot answer costs the message, never the work: `git worktree
	// remove` without --force refuses a dirty checkout on its own
	// (project.RemoveWorktree), so an unreadable status ends in git's own refusal
	// instead of the sentence written for this case.
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
		// The row is already written — parked, or deleted with its checkout — so
		// there is nothing to undo and nothing the caller could do about it. The
		// window's own close ignores this too, but the card still has to come
		// down, or a failed PTY close leaves one on screen for a session the
		// workspace no longer lists.
		slog.Warn("spawn: close the session's terminal", "session", found.session.ID, "err", err)
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
// should not have to know which one it has. The label is tried first and wins a
// tie against another session's roster name, because that is how `lich send`
// reads the same name (relay.resolve); one product cannot disagree with itself
// about which session a name addresses.
//
// Unlike the relay it looks past the live ones: a card whose terminal was never
// opened is still a session, and closing it is the one thing you can do with it.
//
// Whether the caller's own session is a legal answer is the caller's rule, not
// this one's: a close refuses it, a rename is the main thing it is asked for.
func findSession(projects []store.Project, target, projectName string) (located, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return located{}, fmt.Errorf("no session given to close")
	}

	// Resolved once rather than per project: a path argument stats the
	// directory it names, and this loop runs over every project open.
	inProject, err := projectFilter(projectName)
	if err != nil {
		return located{}, err
	}

	var byLabel, byName []located
	for _, p := range projects {
		if !inProject(p) {
			continue
		}
		for _, sess := range p.Sessions {
			cwd := sess.Path
			if cwd == "" {
				cwd = p.Path
			}
			switch {
			case strings.EqualFold(sess.Label, target):
				byLabel = append(byLabel, located{project: p, session: sess})
			case strings.EqualFold(relay.RosterName(cwd, sess.ID), target):
				byName = append(byName, located{project: p, session: sess})
			}
		}
	}
	matches := byLabel
	if len(matches) == 0 {
		matches = byName
	}

	switch len(matches) {
	case 1:
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
	// Only the active card's close moves the focus. The window applies whatever
	// this returns without a second thought (sessions.ts, dropClosedSession), so
	// naming a neighbour for a card nobody was looking at takes the user off the
	// session they were reading.
	if p.ActiveSessionID != id {
		return p.ActiveSessionID
	}
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
