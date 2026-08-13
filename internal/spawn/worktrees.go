package spawn

import (
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/store"
)

// Checkout is one of a project's git worktrees, as a caller outside the window
// needs to see it: what it is called, where it is, whether anything in it is
// uncommitted, and which sessions are open in it.
//
// Sessions is what makes the list actionable rather than informational. A
// checkout with none is one nobody is working in; the last session in one is the
// thing whose closing decides whether the checkout stays (see Close).
type Checkout struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Dirty    bool     `json:"dirty"`
	Sessions []string `json:"sessions"`
}

// Worktrees lists a project's checkouts. project names it; empty takes the
// caller's own, exactly as Open does.
//
// The project's own directory is not one of them: it is the checkout every
// project has, it cannot be removed, and offering it in a list whose point is
// "what can be opened or closed" would invite exactly that.
func (s *Service) Worktrees(fromID, projectName string) ([]Checkout, error) {
	projects, err := s.sessions.LoadState()
	if err != nil {
		return nil, fmt.Errorf("read the workspace: %w", err)
	}
	target, err := resolveProject(projects, fromID, projectName)
	if err != nil {
		return nil, err
	}

	found, err := s.worktrees.ListCheckouts(target.Path)
	if err != nil {
		return nil, err
	}
	out := make([]Checkout, 0, len(found))
	for _, wt := range found {
		// git lists the repository itself among the worktrees. It is the one
		// checkout that cannot be removed and the one no session records a path
		// for, so offering it in a list whose point is "what can be opened or
		// closed" would only invite trying.
		if wt.Path == target.Path {
			continue
		}
		// A failed read reports clean rather than refusing the whole list: the
		// answer is advisory here, and removing a checkout asks git again.
		dirty, _ := s.worktrees.WorktreeDirty(wt.Path)
		out = append(out, Checkout{
			Name:     wt.Name,
			Path:     wt.Path,
			Dirty:    dirty,
			Sessions: sessionsIn(target, wt.Path),
		})
	}
	return out, nil
}

// sessionsIn names the open sessions rooted at a checkout, in the order the
// sidebar shows them.
func sessionsIn(p store.Project, path string) []string {
	labels := []string{}
	for _, sess := range p.Sessions {
		if sess.Path == path {
			labels = append(labels, sess.Label)
		}
	}
	return labels
}

// checkoutAt finds an existing worktree by branch name, so opening a session in
// one is not the same operation as creating it.
func (s *Service) checkoutAt(target store.Project, name string) (string, bool) {
	found, err := s.worktrees.ListCheckouts(target.Path)
	if err != nil {
		return "", false
	}
	// Case-insensitively, as every other name lookup here reads a branch
	// (resolveBase, labelTaken, findSession): an exact match would tell an
	// existing checkout apart from the name the caller typed for it, and answer
	// by creating a second branch beside it.
	for _, wt := range found {
		if strings.EqualFold(wt.Name, name) {
			return wt.Path, true
		}
	}
	return "", false
}
