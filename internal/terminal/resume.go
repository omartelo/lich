package terminal

import "github.com/omartelo/lich/internal/providers"

// ResumeAvailable reports whether the conversation providerSessionID names can
// still be reopened, so the frontend only offers a resume it can honour.
//
// The session row keeps its provider session id for good, but a provider prunes
// its own transcripts, so the id outlives the conversation it points at. Resuming
// one then dies inside the PTY with the provider's error in place of a session —
// the shortcut Start's doc names. Antigravity is worse than that and the same
// question answers it: a `--conversation` it cannot find is dropped with a log
// line, so the session opens as a brand new conversation rather than failing. What the provider left on disk is what answers
// the question; this asks it for existence alone.
//
// cwd is the session's working directory, needed only by Crush: it keeps one
// database per checkout rather than one per machine.
//
// False for a provider with no resume wired (resumeArgs): there is nothing to
// offer where the CLI cannot reopen a conversation by id.
func (*Service) ResumeAvailable(kind, providerSessionID, cwd string) bool {
	if providerSessionID == "" {
		return false
	}
	switch kind {
	case providers.Claude:
		_, ok := claudeTranscriptPath(providerSessionID)
		return ok
	case providers.Codex:
		_, ok := codexTranscriptPath(providerSessionID)
		return ok
	case providers.Antigravity:
		_, ok := antigravityConversationPath(providerSessionID)
		return ok
	case providers.OMP:
		_, ok := ompTranscriptPath(providerSessionID)
		return ok
	case providers.OpenCode:
		path, ok := opencodeSessionDB()
		return ok && sessionRowExists(path, opencodeSessionTable, providerSessionID)
	case providers.Crush:
		path, ok := crushSessionDB(cwd)
		return ok && sessionRowExists(path, crushSessionTable, providerSessionID)
	}
	return false
}
