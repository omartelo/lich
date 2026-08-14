package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
)

// claudeBinKey is the settings key holding the Claude Code binary path. Claude
// keeps this legacy key (rather than the "provider.<id>.bin" scheme the others
// use) so overrides configured before the providers feature keep resolving.
const claudeBinKey = "claude.bin"

// globalScope is the sentinel project_id for settings that apply to every
// project. A concrete project id scopes the setting to that project only.
const globalScope = ""

// GetSetting returns a setting's value for the given scope. An empty projectID
// reads the global value. A missing setting returns "" and no error.
func (s *Service) GetSetting(key, projectID string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM settings WHERE key = ? AND project_id = ?`,
		key, projectID,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

// SetSetting writes a setting's value for the given scope. An empty projectID
// sets the global value.
func (s *Service) SetSetting(key, projectID, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, project_id, value) VALUES (?, ?, ?)
		 ON CONFLICT(key, project_id) DO UPDATE SET value = excluded.value`,
		key, projectID, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// binKey is the settings key holding a provider's custom binary path. Claude
// uses the legacy "claude.bin"; every other provider is namespaced by id.
func binKey(providerID string) string {
	if providerID == providers.Claude {
		return claudeBinKey
	}
	return "provider." + providerID + ".bin"
}

// ProviderBin resolves a provider's binary path for a project: the project
// override wins, then the global value, then "" (letting the terminal fall back
// to the provider's default). It is the single call the terminal service makes
// when spawning a session's PTY.
func (s *Service) ProviderBin(providerID, projectID string) string {
	key := binKey(providerID)
	if projectID != globalScope {
		if bin, err := s.GetSetting(key, projectID); err == nil && bin != "" {
			return bin
		}
	}
	bin, err := s.GetSetting(key, globalScope)
	if err != nil {
		return ""
	}
	return bin
}

// skipPermissionsKey is the settings key that spawns a provider with its
// permission prompts turned off, in one of two scopes: the project's own
// checkout, and a worktree. Global only, and on only for the literal "true" —
// this hands the agent the machine, so anything else (absent, unreadable, a
// value from some other feature) has to mean no.
//
// Two keys rather than one because a worktree is a throwaway checkout: giving an
// agent free rein there while keeping the prompts in the main working tree is
// the answer most users want, and a single flag cannot express it.
func skipPermissionsKey(providerID string, worktree bool) string {
	key := "provider." + providerID + ".skip-permissions"
	if worktree {
		key += ".worktree"
	}
	return key
}

// SkipPermissions reports whether a session of this provider, starting in cwd,
// runs without its permission prompts. cwd picks the scope: anything but the
// project's own directory is a worktree. A project whose path cannot be read
// falls back to the main-checkout setting — a session lich cannot place is not
// one to answer for with the more permissive of two flags.
func (s *Service) SkipPermissions(providerID, projectID, cwd string) bool {
	root := s.ProjectPath(projectID)
	worktree := root != "" && filepath.Clean(cwd) != filepath.Clean(root)
	value, err := s.GetSetting(skipPermissionsKey(providerID, worktree), globalScope)
	if err != nil {
		return false
	}
	return value == "true"
}

// ghAccountKey is the settings key holding the gh account a project's GitHub
// calls run as. Project-scoped only, no global fallback: gh already has a
// global answer (its active account), and this exists precisely to override it
// for one repository.
const ghAccountKey = "vcs.account"

// GHAccountForPath returns the gh account login configured for the project the
// checkout at path belongs to, or "" for none. The path is a project directory
// or one of its worktrees — the project services address a checkout by path,
// never by project id, so the mapping is resolved here.
func (s *Service) GHAccountForPath(path string) string {
	projectID := s.projectIDForPath(path)
	if projectID == "" {
		return ""
	}
	login, err := s.GetSetting(ghAccountKey, projectID)
	if err != nil {
		return ""
	}
	return login
}

// projectIDForPath resolves which project a checkout belongs to: its own
// directory, or a session's (a worktree lives outside the project directory, so
// only its session row ties it back). Returns "" when neither matches.
func (s *Service) projectIDForPath(path string) string {
	if path == "" {
		return ""
	}
	var id string
	err := s.db.QueryRow(
		`SELECT id FROM projects WHERE path = ?
		 UNION ALL
		 SELECT project_id FROM sessions WHERE path = ?
		 LIMIT 1`,
		path, path,
	).Scan(&id)
	if err != nil {
		return ""
	}
	return id
}

// costReadoutKey is the settings key that turns the per-session cost readout
// on. Global only, and off unless it holds "true": the number means nothing on
// a subscription, so it is absent until someone billed per token asks for it.
// A backend setting rather than a UI preference because the flag also gates the
// work — off, no transcript is summed and no price is ever fetched.
const costReadoutKey = "usage.cost"

// CostReadout reports whether the session cost readout is on. Any read failure
// answers off, which is the safe default for a number most users must not see.
func (s *Service) CostReadout() bool {
	value, err := s.GetSetting(costReadoutKey, globalScope)
	if err != nil {
		return false
	}
	return value == "true"
}
