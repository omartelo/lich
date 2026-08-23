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

// providerDefaultKey holds a provider id in either the global scope or a
// project scope. An absent or empty project value means the project inherits
// the global default dynamically.
const providerDefaultKey = "provider.default"

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

// binOffKey is the settings key that parks a binary layer: the path stays
// written, but the layer is skipped when resolving. Scoped exactly like the path
// it parks, and on only for the literal "true" — an absent key is a layer nobody
// switched off, which is every layer configured before this key existed.
func binOffKey(providerID string) string {
	return binKey(providerID) + ".off"
}

// ProviderBin resolves a provider's binary path for a project: the project
// override wins, then the global value, then "" (letting the terminal fall back
// to the provider's default). A parked layer is skipped as if it were unset. It
// is the single call the terminal service makes when spawning a session's PTY.
func (s *Service) ProviderBin(providerID, projectID string) string {
	key, offKey := binKey(providerID), binOffKey(providerID)
	if projectID != globalScope && !s.binParked(offKey, projectID) {
		if bin, err := s.GetSetting(key, projectID); err == nil && bin != "" {
			return bin
		}
	}
	if s.binParked(offKey, globalScope) {
		return ""
	}
	bin, err := s.GetSetting(key, globalScope)
	if err != nil {
		return ""
	}
	return bin
}

// SessionCustomBin reports whether a session runs a binary the user configured
// rather than the provider's own — the question the quota reader asks before
// trusting lich's own login to say what that session spends (internal/quota).
// A session row that cannot be read is not one: the reading it gates is the one
// every session got before this existed.
func (s *Service) SessionCustomBin(sessionID string) bool {
	var kind, projectID string
	if err := s.db.QueryRow(
		`SELECT kind, project_id FROM sessions WHERE id = ?`, sessionID,
	).Scan(&kind, &projectID); err != nil {
		return false
	}
	return s.ProviderBin(kind, projectID) != ""
}

// binParked reports a layer switched off in one scope. An unreadable value is
// not one: a layer the user configured keeps resolving rather than silently
// falling through to a different binary.
func (s *Service) binParked(offKey, projectID string) bool {
	value, err := s.GetSetting(offKey, projectID)
	return err == nil && value == "true"
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

// Sandbox rungs, ordered by how much of the machine a session can reach. They
// are the stored spelling of the control in Settings › Providers, and anything
// this list does not name reads as SandboxOff — an unknown value must never be
// the answer that leaves a session unconfined by accident, but it must also
// never confine one the user never asked to confine.
const (
	// SandboxOff runs sessions straight on the machine. The default.
	SandboxOff = "off"
	// SandboxAsk leaves the answer to the session about to open: the dialog
	// carries the choice, and a session opened any other way is not confined.
	SandboxAsk = "ask"
	// SandboxWorktrees confines sessions in a worktree and leaves the project's
	// own checkout alone — a worktree is the throwaway one, which is the split
	// skipPermissionsKey already makes for the opposite reason.
	SandboxWorktrees = "worktrees"
	// SandboxEverywhere confines every session of the provider.
	SandboxEverywhere = "everywhere"
)

// sandboxKey is the settings key holding a provider's sandbox rung. Scoped like
// ProviderBin — a project value wins over the global one — because confining is
// a decision about a repository as much as about a provider: the checkout full
// of other people's code is the one to confine, and the user's own scratch
// project is not.
func sandboxKey(providerID string) string {
	return "provider." + providerID + ".sandbox"
}

// SandboxLevel returns the rung configured for a provider in a project: the
// project's own value, then the global one, then SandboxOff. An unreadable or
// unrecognised value is SandboxOff for the reason the constants give.
func (s *Service) SandboxLevel(providerID, projectID string) string {
	switch level := s.projectSetting(sandboxKey(providerID), projectID); level {
	case SandboxAsk, SandboxWorktrees, SandboxEverywhere:
		return level
	}
	return SandboxOff
}

// projectSetting reads one setting the way every sandbox control is scoped: the
// project's own value when it has one, the global value otherwise, and "" when
// neither answers. An unreadable value is indistinguishable from an absent one,
// which is what each caller here wants — every one of them treats the unknown
// state as the restrictive answer.
func (s *Service) projectSetting(key, projectID string) string {
	if projectID != globalScope {
		if value, err := s.GetSetting(key, projectID); err == nil && value != "" {
			return value
		}
	}
	value, err := s.GetSetting(key, globalScope)
	if err != nil {
		return ""
	}
	return value
}

// The sandbox grant keys each hand a confined session one of the two credentials
// its private home took away, and they are two keys rather than one because they
// open different doors. The agent signs with every identity loaded into it, for
// any host it can reach; the token is one GitHub account's, limited to the scopes
// that account was granted. Someone who wants gh to work does not necessarily
// want to hand over their ssh keys, and the reverse is just as true.
//
// Unlike sandboxKey they carry no provider. A grant describes what exists inside
// the sandbox, not which agent is running in it — and keying them by provider
// would multiply every future grant by the provider list for a distinction
// nobody makes. Which sessions are confined is the rung's question; what a
// confined session may carry in is this one.
//
// Scoped like sandboxKey all the same — a project value wins over the global one
// — because the repository is what the answer is about. On only for the literal
// "true": each puts a credential in front of an agent running unattended, so
// anything else has to mean no.
const (
	sshAgentKey = "sandbox.ssh-agent"
	ghTokenKey  = "sandbox.gh-token"
)

// SandboxSSHAgent reports whether a confined session in this project is given the
// user's ssh agent, which is what makes `git push` work inside one.
func (s *Service) SandboxSSHAgent(projectID string) bool {
	return s.projectSetting(sshAgentKey, projectID) == "true"
}

// SandboxGHToken reports whether a confined session in this project is given a
// GitHub token in its environment, which is what makes gh work inside one: the
// token lives in the system keyring, and a private home cannot reach it.
func (s *Service) SandboxGHToken(projectID string) bool {
	return s.projectSetting(ghTokenKey, projectID) == "true"
}

// SandboxDefault reports whether a session of this provider, starting in cwd,
// is confined unless something says otherwise. cwd picks the scope the same way
// SkipPermissions does: anything but the project's own directory is a worktree,
// and a project whose path cannot be read falls back to the main checkout.
//
// SandboxAsk answers false here on purpose. It is the rung that hands the
// decision to whoever opens the session, and every caller that cannot ask —
// a respawn, an MCP tool, a delegation — has to be left with the answer the
// user would get by closing the dialog rather than with one nobody chose.
func (s *Service) SandboxDefault(providerID, projectID, cwd string) bool {
	switch s.SandboxLevel(providerID, projectID) {
	case SandboxEverywhere:
		return true
	case SandboxWorktrees:
		root := s.ProjectPath(projectID)
		return root != "" && filepath.Clean(cwd) != filepath.Clean(root)
	}
	return false
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
