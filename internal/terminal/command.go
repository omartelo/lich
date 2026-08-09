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

// How each provider reopens an existing conversation by id: Claude Code takes a
// flag, Codex a subcommand.
const (
	claudeResumeFlag  = "--resume"
	codexResumeSubcmd = "resume"
)

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

// resumeArgs returns the arguments that reopen a provider conversation, or nil
// when the session must start fresh. Each provider spells resume its own way,
// and a provider with no spelling here never grows one — the frontend only
// passes a resume id for a kind it knows resumes, but a stray one must not reach
// a shell or opencode/crush either.
func resumeArgs(kind, resume string) []string {
	if resume == "" {
		return nil
	}
	switch kind {
	case providers.Claude:
		return []string{claudeResumeFlag, resume}
	case providers.Codex:
		return []string{codexResumeSubcmd, resume}
	}
	return nil
}
