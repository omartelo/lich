// Package spawn opens new lich sessions for a caller that is not the window:
// the `lich` command line and the MCP tools beside it (internal/cli). An agent
// could already talk to the sessions around it (internal/relay); this is what
// lets it create one — with a git worktree under it when the work needs its own
// checkout.
//
// It exists as its own service because opening a session is four things at once
// and no single package owns them: a worktree (internal/project), a row
// (internal/store), a PTY (internal/terminal) and a card in the window
// (internal/events). The window does the same four in its own order, in
// providers/projects.tsx.
//
// The PTY is started here rather than left to the window, and that is the whole
// point of the feature. The window spawns a session's terminal only once
// somebody has looked at the card, so a session opened for an agent and never
// clicked would have no process in it — invisible to `lich sessions`,
// unreachable by `lich send`, useless to whoever asked for it. Starting it here
// makes it addressable the moment Open returns; the card's first view attaches
// to the PTY that is already running, exactly as it does after a page reload.
package spawn

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/terminal"
)

// OpenedEventName carries a session opened outside the window, so the card
// appears without a reload. Global, like every other session event: its consumer
// is the workspace state, which outlives any one card.
const OpenedEventName = "session-opened"

// ClosedEventName carries a session closed outside the window, so the card goes
// away without a reload. Its payload is ClosedEvent.
const ClosedEventName = "session-closed"

// ClosedEvent is the payload of ClosedEventName: which session left, the project
// it left, and which of that project's sessions is active now — the window has
// no say in the choice, because the row that records it is already written.
type ClosedEvent struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	ActiveID  string `json:"activeId"`
}

// The size a session opened here starts at: none of its own, which the terminal
// service reads as "whatever size the window last reported" (terminal.sizeFor).
//
// There is no terminal to measure at this end — that is what opening a session
// for an agent means — and naming a shape here was wrong rather than arbitrary.
// The provider drew its whole conversation into a grid the window did not have;
// the first view of the card resized the PTY, the TUI repainted, and everything
// it had written was gone from the screen. Copying the window's size costs
// nothing and leaves the card readable.
const (
	startCols = 0
	startRows = 0
)

// Sessions is the persistence this needs: the workspace to resolve a project
// in, and the writes that add a session to one, take one away, and park one for
// a later resume. The store implements it.
type Sessions interface {
	LoadState() ([]store.Project, error)
	AddSession(projectID, sessionID, label, kind, path string, nextSeq int) error
	SetSessionModel(sessionID, model string) error
	DeleteSession(projectID, sessionID, activeID string) error
	CloseSession(projectID, sessionID, activeID string) error
	PurgeWorktreeSessions(projectID, path string) error
}

// Worktrees is the git side. The project service implements it.
type Worktrees interface {
	Branch(path string) string
	ListBranches(path string) (project.Branches, error)
	ListCheckouts(path string) ([]project.Worktree, error)
	CreateWorktree(projectPath, projectID, name, base string, baseIsRemote bool) (*project.Worktree, error)
	RemoveWorktree(projectPath, wtPath string, force bool) error
	WorktreeDirty(wtPath string) (bool, error)
}

// Terminal is the PTY side: the same spawn the window asks for when a card is
// first viewed, and the same close it asks for when a card goes away. The
// terminal service implements it.
type Terminal interface {
	Start(id, projectID, cwd, kind, resume, name string, setup bool, cols, rows int) error
	Close(id string) error
}

// Events is where an opened session is announced to the window. A nil one leaves
// the feature working and silent — the session runs, its card appears on the
// next reload.
type Events interface {
	Emit(name string, data any)
}

// Session is one opened session: what the caller is told, and what the window is
// handed to draw the card. Both read the same struct because both need the same
// facts — a divergence between them is a card that does not match the session
// the agent was given.
//
// Name is what the session answers to in Claude Code's peer roster; Label is the
// name on its card. Both address it (see internal/relay). Path is the worktree
// the session lives in, empty for a session in the project's own directory —
// the store's own spelling. NextSeq is the project's label counter after this
// session took its number.
type Session struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Project   string `json:"project"`
	Label     string `json:"label"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	NextSeq   int    `json:"nextSeq"`
}

// Service opens sessions on behalf of a caller outside the window.
type Service struct {
	sessions  Sessions
	worktrees Worktrees
	term      Terminal
	events    Events
}

func New(sessions Sessions, worktrees Worktrees, term Terminal, events Events) *Service {
	return &Service{sessions: sessions, worktrees: worktrees, term: term, events: events}
}

// Open creates a session and starts its PTY.
//
// fromID is the session the caller runs in, empty for the command line run
// outside one. It decides two defaults: the project the session lands in and the
// provider it runs, both taken from the caller unless named. A caller with no
// session of its own must name the project — there is nothing to inherit.
//
// worktree, when given, is the branch name of a new git worktree created off
// base (the project's current branch when base is empty); the session is rooted
// there, labelled after it, and runs the project's worktree setup script before
// its provider, exactly as the window's own worktree flow does.
//
// model, when given, is the model the provider is spawned on, in that provider's
// own spelling. It is recorded on the row so every later spawn repeats it.
func (s *Service) Open(fromID, projectName, kind, worktree, base, model string) (Session, error) {
	projects, err := s.sessions.LoadState()
	if err != nil {
		return Session{}, fmt.Errorf("read the workspace: %w", err)
	}
	target, err := resolveProject(projects, fromID, projectName)
	if err != nil {
		return Session{}, err
	}
	kind, err = resolveKind(projects, fromID, kind)
	if err != nil {
		return Session{}, err
	}
	model = strings.TrimSpace(model)
	if model != "" && !terminal.SupportsModel(kind) {
		return Session{}, fmt.Errorf(
			"%s cannot be told which model to run when lich starts it — open the session without a model and pick one inside it",
			kindName(kind),
		)
	}

	// cwd is where the PTY starts; stored is what the row records, which is
	// empty for a session in the project's own directory (the store's spelling,
	// and what the window's grouping reads).
	cwd, stored := target.Path, ""
	label := fmt.Sprintf("Session %d", target.NextSeq)
	setup := false
	if worktree != "" {
		// An existing checkout is opened, not created: asking for a worktree by
		// the branch it holds is asking to work on that branch, and a caller
		// that cannot see the repository has no way to know which of the two it
		// is asking for. Only a fresh one runs the setup script — the checkout
		// that is already there ran it when it was made.
		if path, ok := s.checkoutAt(target, worktree); ok {
			cwd, stored, setup = path, path, false
		} else {
			wt, err := s.addWorktree(target, worktree, base)
			if err != nil {
				return Session{}, err
			}
			cwd, stored, setup = wt.Path, wt.Path, true
		}
		// The card is named after the checkout, as the window names it — unless
		// a session there already took that name. Two live sessions answering to
		// one label is the one thing `lich send` cannot resolve, so the second
		// falls back to the project's own counter.
		if !labelTaken(target, worktree) {
			label = worktree
		}
	}

	id, err := newSessionID()
	if err != nil {
		return Session{}, err
	}
	opened := Session{
		ID:        id,
		ProjectID: target.ID,
		Project:   target.Name,
		Label:     label,
		Name:      relay.RosterName(cwd, id),
		Kind:      kind,
		Path:      stored,
		NextSeq:   target.NextSeq + 1,
	}
	if err := s.sessions.AddSession(target.ID, id, label, kind, stored, opened.NextSeq); err != nil {
		return Session{}, err
	}
	// Announced before the spawn, and regardless of how it goes: the row exists
	// either way, so the card has to exist either way too — a session only the
	// database knows about is one the user cannot reach to see what went wrong.
	if s.events != nil {
		s.events.Emit(OpenedEventName, opened)
	}
	// After the card, for that same reason: the model is an override on a row
	// that already exists, so a write that fails must not be what hides the
	// session. The spawn below reads the model back from the row, so a failure
	// here costs the provider's default and nothing else.
	if model != "" {
		if err := s.sessions.SetSessionModel(id, model); err != nil {
			return Session{}, err
		}
	}
	if err := s.term.Start(id, target.ID, cwd, kind, "", opened.Name, setup, startCols, startRows); err != nil {
		return Session{}, fmt.Errorf("session %q was created but its terminal did not start: %w", label, err)
	}
	return opened, nil
}

// addWorktree creates the checkout a worktree session lives in.
func (s *Service) addWorktree(target store.Project, name, base string) (*project.Worktree, error) {
	base, remote, err := s.resolveBase(target, base)
	if err != nil {
		return nil, err
	}
	wt, err := s.worktrees.CreateWorktree(target.Path, target.ID, name, base, remote)
	if err != nil {
		return nil, err
	}
	return wt, nil
}

// resolveBase settles what a new worktree branches off, and whether that base is
// a remote branch — which decides whether it is fetched first and tracked.
//
// An empty base is the project's current branch, the one thing a caller with no
// view of the repository can be assumed to have meant. A named base is looked up
// rather than trusted: git would happily branch off a typo that happens to be a
// valid revision, and the worktree it left behind is one nobody asked for.
func (s *Service) resolveBase(target store.Project, base string) (string, bool, error) {
	if base == "" {
		current := s.worktrees.Branch(target.Path)
		if current == "" {
			return "", false, fmt.Errorf(
				"%s has no current branch to start from (detached HEAD) — name one with the base",
				target.Name,
			)
		}
		return current, false, nil
	}

	branches, err := s.worktrees.ListBranches(target.Path)
	if err != nil {
		return "", false, err
	}
	for _, name := range branches.Local {
		if strings.EqualFold(name, base) {
			return name, false, nil
		}
	}
	// A branch already checked out in another worktree is reported there rather
	// than among the local ones, and is still a perfectly good base to branch off.
	for _, wt := range branches.Worktrees {
		if strings.EqualFold(wt.Name, base) {
			return wt.Name, false, nil
		}
	}
	for _, name := range branches.Remote {
		if strings.EqualFold(name, base) {
			return name, true, nil
		}
	}
	return "", false, fmt.Errorf("no branch named %q in %s", base, target.Name)
}

// resolveProject picks the project the session lands in: the one named, or the
// caller's own when nothing is named.
func resolveProject(projects []store.Project, fromID, name string) (store.Project, error) {
	if name == "" {
		for _, p := range projects {
			for _, sess := range p.Sessions {
				if sess.ID == fromID {
					return p, nil
				}
			}
		}
		return store.Project{}, fmt.Errorf(
			"no project given, and this is not running in a lich session — name one. %s",
			knownProjects(projects),
		)
	}

	var matches []store.Project
	for _, p := range projects {
		if strings.EqualFold(p.Name, name) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return store.Project{}, fmt.Errorf("no open project named %q. %s", name, knownProjects(projects))
	default:
		return store.Project{}, fmt.Errorf(
			"%q names %d open projects — they cannot be told apart by name", name, len(matches),
		)
	}
}

// resolveKind settles what the new session runs: the provider named, or the
// caller's own. A caller that is not a session at all falls back to the default
// provider, which is also what the window opens a session with.
func resolveKind(projects []store.Project, fromID, kind string) (string, error) {
	if kind != "" {
		if !providers.Known(kind) && kind != terminal.KindShell {
			return "", fmt.Errorf("%q is not a provider lich runs. %s", kind, knownKinds())
		}
		return kind, nil
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == fromID {
				return sess.Kind, nil
			}
		}
	}
	return providers.Claude, nil
}

// labelTaken reports whether a project already holds a session under a label.
func labelTaken(p store.Project, label string) bool {
	for _, sess := range p.Sessions {
		if strings.EqualFold(sess.Label, label) {
			return true
		}
	}
	return false
}

func knownProjects(projects []store.Project) string {
	if len(projects) == 0 {
		return "No project is open."
	}
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	return "Open projects: " + strings.Join(names, ", ") + "."
}

// kindName is what a provider is called in an error the user reads, falling
// back to the kind itself — which is what a shell session has.
func kindName(kind string) string {
	for _, p := range providers.Registry {
		if p.ID == kind {
			return p.Name
		}
	}
	return kind
}

func knownKinds() string {
	kinds := make([]string, 0, len(providers.Registry)+1)
	for _, p := range providers.Registry {
		kinds = append(kinds, p.ID)
	}
	kinds = append(kinds, terminal.KindShell)
	return "Known kinds: " + strings.Join(kinds, ", ") + "."
}

// sessionIDBytes is the length of a session id before hex encoding. The window
// mints a UUID for the same column; both are opaque to everything that reads
// them, so this only has to be as unguessable and as unique.
const sessionIDBytes = 16

func newSessionID() (string, error) {
	raw := make([]byte, sessionIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
