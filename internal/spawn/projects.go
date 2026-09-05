package spawn

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/store"
)

// ensureProject resolves the project a session is being opened in, and is the
// one caller allowed to put a new tab on screen. A name reaches only what the
// window is already holding; a path reaches a directory that is not open yet —
// closed into the workspace's history, or never opened at all — which is what
// opening a project from outside the window means.
//
// It answers with the project list the rest of the open reads, reloaded when a
// row was written: a project reopened from the history comes back with the
// sessions it was closed with, and its label counter is the one the new card
// takes its number from.
func (s *Service) ensureProject(
	projects []store.Project, fromID, name string,
) ([]store.Project, store.Project, error) {
	if !looksLikePath(name) {
		target, err := resolveProject(projects, fromID, name)
		return projects, target, err
	}
	identity, err := project.Identify(name)
	if err != nil {
		return nil, store.Project{}, err
	}
	for _, p := range projects {
		if p.Path == identity.Path {
			return projects, p, nil
		}
	}
	return s.adoptProject(identity)
}

// adoptProject opens a directory the window is not holding: the row it already
// has in the history, or a new one.
//
// The id is matched through the history by path rather than derived from it,
// because the two only agree until a project is relocated — a checkout that
// moved keeps the id its sessions and its worktree directory hang off, and
// deriving a fresh one from the new path would open a second project on the same
// directory. That is the duplicate the window's own relocate refuses, and it has
// to be refused from this side too: every path-addressed lookup lich makes —
// which project a checkout belongs to, which gh account its calls run as —
// answers with whichever of the two rows the query reached first.
func (s *Service) adoptProject(identity *project.Project) ([]store.Project, store.Project, error) {
	id, name := identity.ID, identity.Name
	recents, err := s.sessions.RecentProjects()
	if err != nil {
		return nil, store.Project{}, fmt.Errorf("read the closed projects: %w", err)
	}
	for _, r := range recents {
		if r.Path == identity.Path {
			id, name = r.ID, r.Name
			break
		}
	}
	if err := s.sessions.AddProject(id, name, identity.Path); err != nil {
		return nil, store.Project{}, err
	}
	reloaded, err := s.sessions.LoadState()
	if err != nil {
		return nil, store.Project{}, fmt.Errorf("read the workspace: %w", err)
	}
	for _, p := range reloaded {
		if p.ID != id {
			continue
		}
		// Announced before the session that follows it, and read by the window
		// in that order: a card is dropped when its project is not on screen
		// yet, so the tab has to land first.
		if s.events != nil {
			s.events.Emit(ProjectOpenedEventName, p)
		}
		return reloaded, p, nil
	}
	return nil, store.Project{}, fmt.Errorf("project %q was opened but is not in the workspace", name)
}

// looksLikePath says whether a --project argument names a directory rather than
// a project on screen. A project's name is the base name of its directory, so it
// holds no separator; a path that holds none — a bare "repo" — is a name, and is
// resolved as one.
//
// A relative path that carries a separator is a path here, and refused by
// project.Identify rather than falling through to be looked up as a name: what
// "./repo" is wrong about is where it would resolve, and that is the answer the
// caller needs. A bare "repo" is a name either way, and is answered as one — the
// two readings are genuinely ambiguous there, and the project on screen is the
// one the argument has always meant.
func looksLikePath(name string) bool {
	return strings.ContainsRune(name, '/') ||
		strings.ContainsRune(name, filepath.Separator) ||
		name == "~" || strings.HasPrefix(name, "~/")
}

// projectFilter turns a --project argument into the test that narrows a search
// to it: a directory path matches the project rooted there, anything else
// matches by name, and an empty argument narrows nothing.
//
// One reading of the argument for every command that takes it. A path resolves
// the same everywhere — it just cannot open a project anywhere but an open, and
// nothing else here writes.
func projectFilter(name string) (func(store.Project) bool, error) {
	if name == "" {
		return func(store.Project) bool { return true }, nil
	}
	if !looksLikePath(name) {
		return func(p store.Project) bool { return strings.EqualFold(p.Name, name) }, nil
	}
	identity, err := project.Identify(name)
	if err != nil {
		return nil, err
	}
	return func(p store.Project) bool { return p.Path == identity.Path }, nil
}
