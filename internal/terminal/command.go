package terminal

import "github.com/omartelo/lich/internal/providers"

// What a session's PTY runs: which binary, and with which arguments. Every
// decision here is pure and takes its inputs as parameters, so the spawn path
// (spawnSession) stays a sequence of calls rather than a nest of conditionals.

// defaultBin is the binary spawned when the session's kind is not a known
// provider and no custom path is set — a safety net; real sessions always carry
// a registered provider kind.
const defaultBin = providers.Claude

// KindShell marks a session that runs the user's shell instead of a provider.
const KindShell = "shell"

// resumeFlag is the Claude Code flag that reopens an existing session by id.
const resumeFlag = "--resume"

// resolveCommand picks the binary a session runs: the user's shell for "shell"
// sessions, otherwise the provider binary for the session's kind.
func resolveCommand(kind, bin, shellEnv string) string {
	if kind == KindShell {
		if shellEnv == "" {
			return defaultShell
		}
		return shellEnv
	}
	return resolveBin(kind, bin)
}

// resolveBin returns the configured binary, or the provider's default when it is
// empty (falling back to defaultBin for an unknown kind).
func resolveBin(kind, bin string) string {
	if bin != "" {
		return bin
	}
	if def := providers.DefaultBinary(kind); def != "" {
		return def
	}
	return defaultBin
}

// resumeArgs returns the arguments that reopen a Claude session, or nil when the
// session must start fresh. Resume is Claude-specific: "--resume" is Claude
// Code's flag, so a shell or any other provider never grows it (the frontend
// only ever passes a resume id for a claude session, but a stray one must not
// reach codex/opencode/crush either).
func resumeArgs(kind, resume string) []string {
	if kind != providers.Claude || resume == "" {
		return nil
	}
	return []string{resumeFlag, resume}
}
