package terminal

import "github.com/omartelo/lich/internal/providers"

// ResumeAvailable reports whether the conversation providerSessionID names can
// still be reopened, so the frontend only offers a resume it can honour.
//
// The session row keeps its provider session id for good, but Claude Code prunes
// its own transcripts, so the id outlives the conversation it points at. Resuming
// one then dies inside the PTY with the provider's error in place of a session —
// the shortcut Start's doc names. The transcript on disk is what answers the
// question, and claudeContextUsage already reads it; this asks it for existence
// alone.
//
// False for every other provider: "--resume" is Claude Code's flag (resumeArgs),
// so there is nothing to offer anywhere else.
func (*Service) ResumeAvailable(kind, providerSessionID string) bool {
	if kind != providers.Claude || providerSessionID == "" {
		return false
	}
	_, ok := claudeTranscriptPath(providerSessionID)
	return ok
}
