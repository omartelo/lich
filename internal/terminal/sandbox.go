package terminal

import (
	"log/slog"
	"os"

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
func wrapSandbox(spec ptySpec, kind, home string, confined bool) ptySpec {
	if !confined || home == "" || !sandbox.Available() {
		return spec
	}
	sb := sandbox.Describe(kind, home, spec.dir, project.GitCommonDir(spec.dir), executables(spec.bin))
	spec.bin, spec.args = sandbox.Wrap(sb, spec.bin, spec.args)
	return spec
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
