package terminal

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/sandbox"
	"github.com/omartelo/lich/internal/store"
)

// wrapSandbox rewrites a session's spawn so its whole process tree runs
// confined: a private empty home holding only the provider's own state, the
// machine read-only, and write access to the checkout alone (internal/sandbox).
//
// It is the outermost wrapper, applied after wrapEntrypoint and wrapSetup, so
// everything those two put in front of the provider — the worktree setup script
// above all — is confined too. A setup script is the first thing to run in a
// new checkout and the last thing anyone reads before it does.
//
// home is the user's home directory, passed in rather than resolved here so the
// decision stays testable; an empty one returns the spawn untouched, because a
// sandbox whose private home has no path is one that would confine a session to
// nothing it can name.
//
// dropDir is where this session's dropped-file copies live (sessionDropDir),
// bound read-only so a file dragged onto a confined terminal is a file the
// agent can actually open. Empty leaves it out.
func wrapSandbox(spec ptySpec, kind, home, dropDir string, confined bool, creds sandboxCreds) ptySpec {
	if !confined || home == "" || !sandbox.Available() {
		return spec
	}
	read := append(executables(spec.bin), dropDir)
	sb := sandbox.Describe(kind, home, spec.dir, project.GitCommonDir(spec.dir), read, creds.sshAgent)
	spec.bin, spec.args = sandbox.Wrap(sb, spec.bin, spec.args)
	// The token goes in the child's environment rather than through bubblewrap's
	// --setenv, which would write it into an argument list any process on the
	// machine can read out of /proc.
	if creds.ghToken != "" {
		spec.env = append(spec.env, "GH_TOKEN="+creds.ghToken)
	}
	return spec
}

// sandboxCreds is what a confined session is handed to act on the network as the
// user: the ssh agent a push authenticates through, and the GitHub token gh
// works through. The zero value is the sandbox as it first shipped — a session
// that commits locally and hands the push back to whoever is watching.
//
// They are two answers rather than one because they open different doors. The
// agent signs with every identity loaded into it, for any host; the token is one
// account's, carrying the scopes that account was granted. Wanting gh to work is
// not wanting to hand over your ssh keys.
type sandboxCreds struct {
	// sshAgent binds the host's ssh-agent socket into the session.
	sshAgent bool
	// ghToken is the GitHub token to put in the session's environment, or "" for
	// none — which is also what a token lookup that failed leaves behind.
	ghToken string
}

// sandboxCredentials reads what this session's project allows a confined spawn to
// carry in. An unconfined session gets nothing: it already runs with the user's
// whole environment, and asking gh for a token it does not need would put one in
// an environment that never lacked it.
//
// The grants carry no provider — they describe what is inside the sandbox, not
// who runs in it (internal/store/settings.go) — which is why the provider is not
// a parameter here even though the rung above it is keyed by one.
//
// The account is the project's own gh account (the store's "vcs.account"), so a
// confined session answers as the account the rest of lich answers as for this
// repository rather than as whichever one gh has active.
//
// Ceiling: one `gh auth token` subprocess in front of every confined spawn that
// asks for it, capped by the project package's own timeout. gh's tokens are
// short-lived and gh owns their refresh, so caching one here would mean owning
// the invalidation too — the same trade internal/project already priced.
func (s *Service) sandboxCredentials(projectID, cwd string, confined bool) sandboxCreds {
	if !confined {
		return sandboxCreds{}
	}
	creds := sandboxCreds{sshAgent: s.store.SandboxSSHAgent(projectID)}
	if s.store.SandboxGHToken(projectID) {
		creds.ghToken = project.GHToken(s.store.GHAccountForPath(cwd))
	}
	return creds
}

// sessionDropDir is the directory holding the copies of the files dropped into
// one session (internal/drop), created here because a bind mount of a source
// that does not exist yet is dropped from the spec — and the first drop of the
// session comes long after the spawn.
//
// One session's directory rather than the whole copies tree, so a confined
// session reads what was dropped into it and not what was dropped into the
// session beside it. Empty when there is nothing to mount, which costs the
// session its drops and never its spawn — and for an unconfined session, which
// reads the copy at its real path like any other file and would otherwise leave
// an empty directory behind for every spawn.
func sessionDropDir(dropDir, sessionID string, confined bool) string {
	if !confined || dropDir == "" || sessionID == "" {
		return ""
	}
	dir := filepath.Join(dropDir, sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("session drop dir", "dir", dir, "err", err)
		return ""
	}
	return dir
}

// executables are the directories holding the binaries a confined spawn has to
// reach: the one it runs, and the lich binary a session calls back through
// (LICH_BIN — the `lich` CLI and the MCP server the provider is handed both run
// it). Both are regularly installed inside the home the sandbox is about to
// empty, and a `task dev` build is under /tmp, which the sandbox replaces too.
//
// Directories rather than the executables themselves, and every hop of their
// symlink chains: see sandbox.BinaryDirs, which is where that decision and the
// spawn failure behind it are written down.
func executables(bin string) []string {
	var dirs []string
	for _, name := range []string{bin, lichBin()} {
		if name == "" {
			continue
		}
		dirs = append(dirs, sandbox.BinaryDirs(name)...)
	}
	return dirs
}

// confined answers whether one session runs in the sandbox, and records the
// answer on its own row: the row's own value when it has one, otherwise the
// provider's rung for this project and checkout.
//
// The row wins outright, in both directions. A session the user opened confined
// stays confined through every later spawn — a reload, a respawn, the resume of
// a parked worktree session — and one they deliberately opened on the machine is
// not quietly confined later because the rung moved under it.
//
// Writing the verdict back is what makes that true for a session nobody was
// asked about, and it is also what the window reads to mark a confined card: the
// decision takes a rung, a checkout and an override to reach, and the card is
// not the place to reach it a second time. A failed write costs the mark on the
// card, never the confinement — the spawn already has its answer.
func confined(s sandboxStore, sessionID, kind, projectID, cwd string) bool {
	answer := false
	switch s.SessionSandbox(sessionID) {
	case store.SessionConfined:
		answer = true
	case store.SessionUnconfined:
		answer = false
	default:
		answer = s.SandboxDefault(kind, projectID, cwd)
	}
	record := store.SessionUnconfined
	if answer {
		record = store.SessionConfined
	}
	if err := s.SetSessionSandbox(sessionID, record); err != nil {
		slog.Warn("record sandbox verdict", "session", sessionID, "err", err)
	}
	return answer
}

// sandboxStore is the store as this file uses it, named separately so the
// decision above can be tested against a stub rather than a database.
type sandboxStore interface {
	SessionSandbox(sessionID string) string
	SandboxDefault(providerID, projectID, cwd string) bool
	SetSessionSandbox(sessionID, sandbox string) error
}

// userHome is the home directory the sandbox replaces, or "" when it cannot be
// resolved — which wrapSandbox reads as "do not confine", because a sandbox
// built around the wrong home is one that hides the checkout from the session.
func userHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
