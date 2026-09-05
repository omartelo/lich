package terminal

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/providers"
)

// Start spawns the binary for session id under project projectID — the user's
// shell when kind is "shell", otherwise the provider binary for that kind
// resolved from the project's settings (falling back to the provider's default
// on $PATH) — attached to a new PTY sized to cols x rows and rooted at cwd, then
// streams its output to the frontend. An empty cwd defaults to the user's home
// directory. Starting a session that is already running is a no-op.
//
// A non-empty resume is a provider conversation id to reopen, spelled for the
// session's kind by resumeArgs, which the frontend passes after the user
// accepted the prompt to continue the session this card ran before the last
// restart. The prompt is only raised for a conversation ResumeAvailable still
// finds, so an id the provider no longer knows normally never reaches here; one
// that slips through (pruned between the check and the spawn) fails in the PTY
// like any other bad invocation — the user sees the provider's own error.
//
// fork turns that resume into a branch: the provider copies the conversation
// into a new one of its own and leaves the original alone, so the card that ran
// it stays where it is with its history intact. Only the three kinds
// providers.SupportsFork names can do it, and only alongside a resume id — the
// conversation being branched is the one that id points at. The copy's own id
// is the provider's to assign, and reaches lich through this session's
// session-start report like any other.
//
// name is what the session answers to in its provider's peer roster (Claude
// Code's `/list-agents`), passed by the frontend so the roster names the card
// the user sees. Only Claude Code has a roster; every other kind ignores it.
//
// setup is passed once, by the flow that just created this session's worktree:
// it runs the project's worktree setup script (.lich/setup-worktree.sh, see
// project.SetupScript) in the PTY before the provider, so a fresh checkout
// installs its dependencies in view. A respawn or resume never sets it. The
// script runs in the session's own environment, so it reads the same
// LICH_WORKTREE_PORT the provider will.
func (s *Service) Start(
	id, projectID, cwd, kind, resume, name string, fork, setup bool, cols, rows int,
) error {
	// Refused before the spawn rather than dropped in resumeArgs: a fork the
	// provider cannot spell would reopen the parent's own conversation, and two
	// cards on one conversation is exactly what a fork promises not to do.
	if fork && !providers.SupportsFork(kind) {
		return fmt.Errorf("%s cannot fork a conversation — it only resumes one", kind)
	}
	if fork && resume == "" {
		return fmt.Errorf("a fork needs the conversation to branch, and no resume id was given")
	}
	sess, cwd, err := s.spawnSession(id, projectID, cwd, kind, resume, name, fork, setup, cols, rows)
	if err != nil || sess == nil {
		return err
	}
	// Emitted outside s.mu: Emit blocks on a stalled /events client, which
	// would freeze every session's I/O. Both are unconditional so a respawn
	// overwrites whatever the previous PTY left in the frontend's stores.
	s.hub.Emit(cwdEventName, cwdEvent{ID: id, Cwd: cwd})
	s.hub.Emit(agentEventName, agentEvent{ID: id, Agent: ""})
	s.hub.Emit(sandboxEventName, sandboxEvent{ID: id, Confined: sess.confined})
	// Bound to the effective cwd rather than the requested one, and outside
	// s.mu: track queues a warm-up of this checkout's snapshot index, and the
	// first one on a large repository is measured in seconds.
	s.snaps.track(id, cwd)
	go watchCwd(id, sess.pty.Pid(), cwd, sess.done, s.hub)
	return nil
}

// spawnSession is the locked half of Start: dedupe, PTY spawn and session-map
// registration. A nil session with a nil error means id was already running.
// The returned cwd is the effective start directory (the input, or the
// resolved home when it was empty).
func (s *Service) spawnSession(
	id, projectID, cwd, kind, resume, name string, fork, setup bool, cols, rows int,
) (*session, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, running := s.sessions[id]; running {
		return nil, "", nil
	}
	// Whatever the previous provider left open under this id died with its PTY,
	// and the first report of the new one has to be read against its own turn.
	s.turns.forget(id)
	cols, rows = s.sizeFor(cols, rows)

	if cwd == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		cwd = home
	}

	// The MCP registration is withheld without a transport for the same reason
	// the hook coordinates are: the server it points at would have nothing to
	// talk to, so the session would carry tools that can only fail.
	mcpBin := ""
	if s.ws != nil {
		mcpBin = lichBin()
	}
	skipPermissions := s.store.SkipPermissions(kind, projectID, cwd)
	// The model is read from the row rather than passed in, so it survives every
	// spawn this session ever gets: the window's first view, a respawn after a
	// reload, and the resume of a parked worktree session all arrive here.
	spec := ptySpec{
		bin: resolveCommand(kind, s.store.ProviderBin(kind, projectID), userShell()),
		args: providerArgs(
			kind, name, resume, s.store.SessionModel(id), mcpBin, kiroPluginAgent(kind),
			fork, skipPermissions,
		),
		dir:  cwd,
		env:  s.sessionEnv(id, projectID, cwd),
		cols: cols,
		rows: rows,
	}
	// Before wrapSetup, so a fresh worktree terminal carrying both runs the
	// project's setup script first, then the entrypoint, then the shell.
	spec = wrapEntrypoint(spec, kind, s.store.SessionEntrypoint(id), runtime.GOOS)
	settingUp := false
	if setup {
		spec, settingUp = wrapSetup(spec, project.SetupScript(s.store.ProjectPath(projectID)), runtime.GOOS)
	}
	// Outermost, so the setup script and the entrypoint are confined with the
	// session they run in front of.
	inSandbox := confined(s.store, id, kind, projectID, cwd)
	creds := s.sandboxCredentials(projectID, cwd, inSandbox)
	spec = wrapSandbox(spec, kind, userHome(), sessionDropDir(s.dropDir, id, inSandbox), inSandbox, creds)
	p, err := startPTY(spec)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start pty for %q: %w", id, err)
	}

	box := newOutbox(func(data []byte) {
		if s.ws != nil && s.ws.send(id, data) {
			return
		}
		s.emitData(id, data)
	}, outboxDepth)
	out := newCoalescer(box.push, visibleFlushInterval, hiddenFlushInterval)
	sess := &session{
		pty:       p,
		out:       out,
		done:      make(chan struct{}),
		replay:    newReplayBuffer(replayCapBytes),
		outbox:    box,
		settingUp: settingUp,
		// Timed from the spawn, so a program that never writes anything still
		// becomes ready once: quiet is the signal, and silence from the start
		// is quiet too.
		lastOut:  time.Now(),
		confined: inSandbox,
	}
	s.sessions[id] = sess
	// Outside mu's protection by design (see Service.spawns), and stored with the
	// registration so no report can arrive for a session whose kind is unknown.
	s.spawns.Store(id, spawn{kind: kind, cwd: cwd})
	go s.stream(id, sess)
	return sess, cwd, nil
}

// providerKind resolves which provider mark a session-start report puts on a
// card: the kind lich spawned when that kind is a provider, else the reported
// one. A session that is not running — the report raced its own PTY's exit —
// falls back to the report, which is all that is left to answer from.
func (s *Service) providerKind(id, reported string) string {
	if kind := s.kindOf(id); providers.Known(kind) {
		return kind
	}
	return reported
}

// spawn is how one live session's PTY was started: the kind it runs and the
// directory it was rooted at. The cwd is the spawn directory and not the
// foreground one the watcher follows (cwd.go) — Crush keeps its database in the
// checkout it was started in, which a `cd` inside the session does not move.
type spawn struct {
	kind string
	cwd  string
}

// spawnOf is how a live session's PTY was started, or the zero spawn once it is
// gone. It never takes mu: a hook asks this on every report, and mu is held
// across a PTY spawn (see Service.spawns).
func (s *Service) spawnOf(id string) spawn {
	v, ok := s.spawns.Load(id)
	if !ok {
		return spawn{}
	}
	started, _ := v.(spawn)
	return started
}

// kindOf is what a live session's PTY was spawned to run, or "" once it is gone.
func (s *Service) kindOf(id string) string { return s.spawnOf(id).kind }

// closableState reports whether a state reported from inside a session of this
// kind is one that harness can also end. A state nothing ends is worse than no
// state: `busy` with no end-of-turn event behind it pins a spinner to the card
// for the rest of the session, which is wrong for far longer than it is right —
// the rule docs/adding-a-provider.md states, and the reason Crush's plugin
// registers two of the four reports rather than four.
//
// Only Cursor CLI is filtered, and only here rather than in what it registers,
// because lich does not own its registration: Cursor runs the plugin installed
// in Claude Code, which registers all of them. Of those, Cursor was measured
// (2026.08.11, hooks in its own format and in Claude Code's alike) to deliver
// `SessionStart`, `PreToolUse`, `PostToolUse` and `SessionEnd` and nothing else
// — no `UserPromptSubmit`, so a turn that calls no tool never begins, and no
// `Stop`, so one that does never ends. `idle` is the one state it can both
// report and mean, and it survives.
func closableState(kind, state string) bool {
	if kind != providers.Cursor {
		return true
	}
	return state == statusIdle
}
