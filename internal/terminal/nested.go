package terminal

import "log/slog"

// An agent CLI that a session's agent runs as a tool (`claude -p`,
// `cursor-agent -p`) inherits the session's LICH_* environment, and Claude Code
// and Cursor run the plugin's hooks inside it, so its reports arrive naming the
// host card. Left alone, its SessionEnd ended the host's turn and the relay
// closed every errand delivered there. The conversation id a report carries is
// what tells the two apart (docs/hooks/session-state.md, Nested agent CLIs).

// fromBoundConversation reports whether a state report belongs to the
// conversation the session is bound to. A report without an id comes from a
// plugin older than the field and is taken as before, and so is any report
// before the first session-start binds one.
//
// A store that cannot answer lets the report through: dropping the host's own
// state on a failed read loses its turn, which is the bug this guards against.
func (s *Service) fromBoundConversation(id, providerSessionID string) bool {
	if providerSessionID == "" {
		return true
	}
	bound, err := s.store.ProviderSession(id)
	if err != nil {
		slog.Warn("terminal: read provider session", "session", id, "err", err)
		return true
	}
	return bound == "" || bound == providerSessionID
}

// nestedStart reports whether a session-start is a nested CLI's rather than
// the session's own. Both name a conversation other than the bound one; what
// differs is when. `/clear`, `/resume` and a new conversation start at the
// prompt, after the turn before them ended, while a nested CLI is run by a tool
// call, inside the host's turn. A turn opened by a nested report cannot hide a
// legitimate rebind, because a report naming another conversation never opens
// one (fromBoundConversation).
func (s *Service) nestedStart(id, providerSessionID string) (bool, error) {
	bound, err := s.store.ProviderSession(id)
	if err != nil {
		return false, err
	}
	return bound != "" && bound != providerSessionID && s.turns.busy(id), nil
}
