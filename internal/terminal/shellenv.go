package terminal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// shellEnvTimeout caps the rc dump so a hung interactive shell can't block
// startup; overshooting just loses that launch's resolved vars, never the app.
const shellEnvTimeout = 5 * time.Second

// shellEnvSentinel fences the env dump off from rc chatter (greetings, MOTD,
// job-control warnings). Matched by last occurrence so rc that echoes it loses.
const shellEnvSentinel = "__LICH_SHELL_ENV__"

var (
	// resolving admits one login-shell resolution at a time, and refuses rather
	// than queues: the caller is a button press, and a second one waiting out the
	// first would pay the same shellEnvTimeout twice over for the same answer.
	resolving sync.Mutex

	parkedMu sync.Mutex
	// parkedReader is the reader an earlier resolution abandoned, still blocked
	// on its pty; it closes when that read finally returns. Nil once nothing is
	// outstanding.
	parkedReader <-chan struct{}
)

// shellDumpParked reports a reader an earlier resolution left behind, and
// forgets one that has since been collected. A pty read already blocked in the
// kernel cannot be interrupted (see runShellDump), so a second resolution
// started over the first leaks a second fd, goroutine and zombie child — and
// would be asking the same shell that did not answer the first time. Refusing
// keeps that cost at one outstanding reader, whatever the button is pressed.
//
// Read lazily rather than tracked: nothing tells lich when whatever holds that
// pty finally lets go, so the next attempt is the only place to look.
func shellDumpParked() bool {
	parkedMu.Lock()
	defer parkedMu.Unlock()
	if parkedReader == nil {
		return false
	}
	select {
	case <-parkedReader:
		parkedReader = nil
		return false
	default:
		return true
	}
}

func noteParkedReader(collected <-chan struct{}) {
	parkedMu.Lock()
	defer parkedMu.Unlock()
	parkedReader = collected
}

// ResolveShellEnv augments base with the variables a login+interactive shell
// exports. lich is launched from a GUI, so its environment is the graphical
// session's — it never sourced .zshrc/.bashrc/config.fish/.profile, where users
// commonly export things like an MCP server's auth token. A "shell" session
// hides this because the shell we spawn sources those rc files itself; a provider
// spawned directly (claude/codex/...) does not, so its ${VAR} expansions in
// .mcp.json come up empty. We run the user's shell the way a terminal emulator
// does — attached to a pty, see runShellDump — and merge its environment over
// base.
//
// SHELL unset (normal Windows: cmd.exe has no rc) or any failure returns base
// unchanged — the resolution is best-effort, never load-bearing.
func ResolveShellEnv(base []string) []string {
	env, err := ReresolveShellEnv(base)
	if err != nil {
		slog.Warn("terminal: shell env resolution yielded nothing, using launch env", "err", err)
		return base
	}
	return env
}

// ReresolveShellEnv is ResolveShellEnv with the failure returned rather than
// logged, and it is what a user-pressed re-check runs
// (internal/providers.Service.RefreshPath). Boot has nowhere to show a failure
// and carries on with the launch env; a re-check does, and must not hand back a
// re-scan of the PATH lich booted with as if it were fresh.
//
// The bound is the same one boot pays — shellEnvTimeout and the quiet window in
// runShellDump — so a login shell sitting on a prompt costs the button what it
// costs a launch, and no more.
func ReresolveShellEnv(base []string) ([]string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Normal on Windows: cmd.exe has no rc, so no PATH there was ever
		// resolved from a login shell. Handing base back re-pins what lich
		// launched with, which leaves a re-check re-scanning the process PATH —
		// the whole answer on that machine, not a stale half of one.
		return base, nil
	}

	if !resolving.TryLock() {
		return nil, fmt.Errorf("%s: a resolution is already running", shell)
	}
	defer resolving.Unlock()
	if shellDumpParked() {
		return nil, fmt.Errorf("%s: the last resolution's reader is still parked on its pty", shell)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shellEnvTimeout)
	defer cancel()

	// -l -i so both login profiles (bash/zsh) and interactive rc (fish's
	// config.fish is interactive-only) run; `env` is external, so the command is
	// identical across shells.
	out, parked, err := runShellDump(ctx, shell, "echo "+shellEnvSentinel+"; env", base)
	if parked != nil {
		noteParkedReader(parked)
	}

	extra := parseShellEnvDump(shellEnvSentinel, out)
	if extra == nil {
		if err == nil {
			err = errors.New("the shell printed no environment")
		}
		return nil, fmt.Errorf("%s: %w (%s)", shell, err, shellDumpDigest(out))
	}
	return mergeEnv(base, extra), nil
}

// shellDumpDigestLines and shellDumpDigestWidth bound what a failed dump puts in
// the log: enough of the shell's own words to say why it never reached the
// sentinel, never the whole rc chatter.
const (
	shellDumpDigestLines = 3
	shellDumpDigestWidth = 120
)

// shellDumpDigest describes a dump the parse did not recognise, and is the only
// place its content is ever read for a human. A dump that failed to parse was
// never recognised as an environment, so a line that assigns one is reported by
// its key alone — its value was never ours to log. What is left is the rc file's
// own output: the error, greeting or prompt that says where the shell stopped.
func shellDumpDigest(out string) string {
	var lines []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if key, _, ok := strings.Cut(line, "="); ok && validEnvKey(key) {
			line = key + "=..."
		}
		if len(line) > shellDumpDigestWidth {
			line = line[:shellDumpDigestWidth] + "..."
		}
		lines = append(lines, line)
		if len(lines) == shellDumpDigestLines {
			break
		}
	}
	if len(lines) == 0 {
		return "printed nothing"
	}
	return fmt.Sprintf("%d bytes, starting: %s", len(out), strings.Join(lines, " | "))
}

// parseShellEnvDump returns the KEY=VALUE lines env printed after the last
// sentinel, or nil when the sentinel is absent (shell died before env ran). It
// is line-based, so a value spanning a newline loses its tail; switch the dump
// to `env -0` and split on NUL if that ever bites.
func parseShellEnvDump(sentinel, out string) []string {
	_, dump, ok := strings.CutLast(out, sentinel)
	if !ok {
		return nil
	}
	var kv []string
	for line := range strings.SplitSeq(dump, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if key, _, ok := strings.Cut(line, "="); ok && validEnvKey(key) {
			kv = append(kv, line)
		}
	}
	return kv
}

// validEnvKey rejects a multi-line value's continuation line (which may still
// contain '=') by holding lines to a POSIX variable name.
func validEnvKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// mergeEnv layers extra over base, extra winning on collision: the shell's rc is
// what defines the fuller PATH and the auth vars this whole dance exists to
// recover, so its values must beat the launch env's.
func mergeEnv(base, extra []string) []string {
	slot := make(map[string]int, len(base))
	merged := make([]string, len(base))
	copy(merged, base)
	for i, kv := range base {
		if key, _, ok := strings.Cut(kv, "="); ok {
			slot[key] = i
		}
	}
	for _, kv := range extra {
		key, _, _ := strings.Cut(kv, "=")
		if i, ok := slot[key]; ok {
			merged[i] = kv
			continue
		}
		slot[key] = len(merged)
		merged = append(merged, kv)
	}
	return merged
}

// PinPath copies env's PATH into lich's own process. ResolveShellEnv's merge
// only reaches what lich hands a child as cmd.Env, and that is not what
// resolves a bare binary name: exec.LookPath — and exec.Command, which calls it
// — reads the process PATH, never cmd.Env. So a GUI launch that never sourced
// the rc files fails to find anything the rc files put on PATH: provider
// detection reports codex missing, the spawn cannot start it, and the Chromium
// search misses a browser outside the system prefix. macOS is where this bites
// hardest — a Finder-launched .app gets launchd's bare PATH, without Homebrew's
// prefix — but a .desktop launch is the same shape.
func PinPath(env []string) {
	for _, kv := range env {
		v, ok := strings.CutPrefix(kv, "PATH=")
		if !ok {
			continue
		}
		if err := os.Setenv("PATH", v); err != nil {
			slog.Warn("terminal: pin resolved PATH", "err", err)
		}
		return
	}
}
