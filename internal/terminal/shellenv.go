package terminal

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// shellEnvTimeout caps the rc dump so a hung interactive shell can't block
// startup; overshooting just loses that launch's resolved vars, never the app.
const shellEnvTimeout = 5 * time.Second

// shellEnvSentinel fences the env dump off from rc chatter (greetings, MOTD,
// job-control warnings). Matched by last occurrence so rc that echoes it loses.
const shellEnvSentinel = "__LICH_SHELL_ENV__"

// ResolveShellEnv augments base with the variables a login+interactive shell
// exports. lich is launched from a GUI, so its environment is the graphical
// session's — it never sourced .zshrc/.bashrc/config.fish/.profile, where users
// commonly export things like an MCP server's auth token. A "shell" session
// hides this because the shell we spawn sources those rc files itself; a provider
// spawned directly (claude/codex/...) does not, so its ${VAR} expansions in
// .mcp.json come up empty. We run the user's shell the way a terminal emulator
// does and merge its environment over base.
//
// SHELL unset (normal Windows: cmd.exe has no rc) or any failure returns base
// unchanged — the resolution is best-effort, never load-bearing.
func ResolveShellEnv(base []string) []string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return base
	}

	ctx, cancel := context.WithTimeout(context.Background(), shellEnvTimeout)
	defer cancel()

	// -l -i so both login profiles (bash/zsh) and interactive rc (fish's
	// config.fish is interactive-only) run; `env` is external, so the command is
	// identical across shells. Nil Stdin feeds EOF instead of a tty, so the
	// interactive shell never blocks on a read.
	cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c", "echo "+shellEnvSentinel+"; env")
	cmd.Env = base
	out, err := cmd.Output() // stderr, carrying rc warnings, is discarded

	extra := parseShellEnvDump(shellEnvSentinel, string(out))
	if extra == nil {
		slog.Warn("terminal: shell env resolution yielded nothing, using launch env", "shell", shell, "err", err)
		return base
	}
	return mergeEnv(base, extra)
}

// parseShellEnvDump returns the KEY=VALUE lines env printed after the last
// sentinel, or nil when the sentinel is absent (shell died before env ran). It
// is line-based, so a value spanning a newline loses its tail; switch the dump
// to `env -0` and split on NUL if that ever bites.
func parseShellEnvDump(sentinel, out string) []string {
	idx := strings.LastIndex(out, sentinel)
	if idx < 0 {
		return nil
	}
	var kv []string
	for line := range strings.SplitSeq(out[idx+len(sentinel):], "\n") {
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
