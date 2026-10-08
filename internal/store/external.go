package store

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/omartelo/lich/internal/providers"
)

// Conversation is one conversation a provider keeps on disk, as its own store
// describes it: whoever started it, lich or not. internal/terminal reads them
// (terminal.Conversations); Cwd is the directory the agent ran in, which is
// what ties the conversation to a project.
type Conversation struct {
	Kind  string
	ID    string
	Title string
	Cwd   string
	// Unix seconds of the last activity the provider recorded.
	UpdatedAt int64
}

// ExternalSession is a conversation started outside lich that one of its
// projects can adopt: it ran in that project's directory or in one of its
// checkouts. Path is what the adopted row carries, "" for the project's own
// directory, the same as a session lich opened there.
type ExternalSession struct {
	Kind              string `json:"kind"`
	ProviderSessionID string `json:"providerSessionId"`
	Title             string `json:"title"`
	Path              string `json:"path"`
	ProjectID         string `json:"projectId"`
	ProjectName       string `json:"projectName"`
	ProjectPath       string `json:"projectPath"`
	UpdatedAt         int64  `json:"updatedAt"`
}

// SetConversationsOf registers how to list the conversations on disk
// (terminal.Conversations). Startup wiring; without it there are no external
// sessions, which is what every test that does not wire it gets.
func (s *Service) SetConversationsOf(fn func() []Conversation) {
	s.conversationsOf = fn
}

// SetCheckoutsOf registers how to list a repository's checkouts, its own
// directory included (project.Service.ListCheckouts). Startup wiring; without it
// a conversation matches a project only by the project's own directory.
func (s *Service) SetCheckoutsOf(fn func(projectPath string) ([]string, error)) {
	s.checkoutsOf = fn
}

// ExternalSessions lists the conversations on disk that lich holds no row for
// and never deleted one for, and that ran in a checkout of a project lich knows,
// open or closed. Newest first.
//
// A conversation whose directory is gone is left out: there is nowhere to
// resume it, the same reason a history row without its checkout cannot be.
func (s *Service) ExternalSessions() ([]ExternalSession, error) {
	external := []ExternalSession{}
	if s.conversationsOf == nil {
		return external, nil
	}
	conversations := s.conversationsOf()
	if len(conversations) == 0 {
		return external, nil
	}
	held, err := s.heldProviderSessions()
	if err != nil {
		return nil, err
	}
	forgotten, err := s.ForgottenProviderSessions()
	if err != nil {
		return nil, err
	}
	owners, err := s.checkoutOwners()
	if err != nil {
		return nil, err
	}
	for _, c := range conversations {
		if held[c.ID] || forgotten[c.ID] {
			continue
		}
		key, ok := checkoutKey(c.Cwd)
		if !ok {
			continue
		}
		owner, ok := owners[key]
		if !ok {
			continue
		}
		external = append(external, ExternalSession{
			Kind: c.Kind, ProviderSessionID: c.ID, Title: c.Title, Path: owner.path,
			ProjectID: owner.id, ProjectName: owner.name, ProjectPath: owner.projectPath,
			UpdatedAt: c.UpdatedAt,
		})
	}
	slices.SortStableFunc(external, func(a, b ExternalSession) int {
		return cmp.Compare(b.UpdatedAt, a.UpdatedAt)
	})
	return external, nil
}

// checkoutOwner is the project a checkout belongs to, and the path a session
// running in it is stored under.
type checkoutOwner struct {
	id, name, projectPath, path string
}

// checkoutOwners maps every checkout of every project to its project. A project
// whose checkouts git cannot list (not a repository, a directory that moved)
// still owns its own directory, which is all a session opened there needs.
func (s *Service) checkoutOwners() (map[string]checkoutOwner, error) {
	projects, err := s.allProjects()
	if err != nil {
		return nil, err
	}
	owners := map[string]checkoutOwner{}
	for _, p := range projects {
		if key, ok := checkoutKey(p.Path); ok {
			owners[key] = checkoutOwner{p.ID, p.Name, p.Path, ""}
		}
		if s.checkoutsOf == nil {
			continue
		}
		checkouts, err := s.checkoutsOf(p.Path)
		if err != nil {
			continue
		}
		for _, c := range checkouts {
			key, ok := checkoutKey(c)
			if !ok {
				continue
			}
			if _, taken := owners[key]; taken {
				continue
			}
			owners[key] = checkoutOwner{p.ID, p.Name, p.Path, c}
		}
	}
	return owners, nil
}

// checkoutKey is the one spelling two paths are compared by. git reports a
// checkout resolved and a provider records the cwd its process saw, so the same
// directory arrives under two names whenever a symlink is in the way. False for
// a path that does not exist.
func checkoutKey(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}
	return resolved, true
}

func (s *Service) allProjects() ([]Recent, error) {
	rows, err := s.db.Query(`SELECT id, name, path FROM projects`)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()
	projects := []Recent{}
	for rows.Next() {
		var p Recent
		if err := rows.Scan(&p.ID, &p.Name, &p.Path); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return projects, nil
}

func (s *Service) heldProviderSessions() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT provider_session_id FROM sessions WHERE provider_session_id <> ''`)
	if err != nil {
		return nil, fmt.Errorf("query provider sessions: %w", err)
	}
	defer rows.Close()
	held := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan provider session: %w", err)
		}
		held[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider sessions: %w", err)
	}
	return held, nil
}

// ErrConversationHeld is an adoption refused because a session row already
// holds the conversation: another window adopted it first, or lich opened it.
var ErrConversationHeld = errors.New("this conversation already has a session in lich")

// AdoptExternalSession files a conversation started outside lich as a parked
// session of projectID under sessionID, so the history's own resume
// (ReopenSession) brings it back like any session lich closed. path is the
// ExternalSession's, "" for the project's own directory.
func (s *Service) AdoptExternalSession(projectID, sessionID, kind, path, providerSessionID, label string) error {
	if !providers.Known(kind) {
		return fmt.Errorf("adopt session: unknown provider %q", kind)
	}
	if providerSessionID == "" {
		return errors.New("adopt session: no provider conversation id")
	}
	held, err := s.heldProviderSessions()
	if err != nil {
		return err
	}
	if held[providerSessionID] {
		return ErrConversationHeld
	}
	if _, err := s.db.Exec(
		`INSERT INTO sessions (id, project_id, label, kind, path, provider_session_id, is_open, closed_at, position)
		 VALUES (?, ?, ?, ?, ?, ?, 0, ?, `+nextSessionPosition+`)`,
		sessionID, projectID, label, kind, path, providerSessionID, now().Unix(), projectID,
	); err != nil {
		return fmt.Errorf("adopt session %q: %w", providerSessionID, err)
	}
	return nil
}
