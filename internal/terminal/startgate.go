package terminal

import "github.com/omartelo/lich/internal/providers"

// startsAtPrompt is the providers whose session-start hook fires from the
// agent's own prompt, before its first turn and past any question the CLI asks
// first. Measured 2026-10-08: Claude Code and Cursor CLI ask whether a new
// folder is trusted and report only once it is; oh-my-pi and Kiro CLI ask
// nothing and report at start. Codex 0.161.0 reports on its first turn, even in
// a trusted folder sitting at its prompt; Antigravity on its first turn too,
// Crush on its first tool call, opencode when its first conversation is
// created. None of those four tells lich anything before work is given to it,
// and holding them for a report would leave them unable to take any.
var startsAtPrompt = map[string]bool{
	providers.Claude: true,
	providers.Cursor: true,
	providers.OMP:    true,
	providers.Kiro:   true,
}

// SetStartReports wires the question of whether a provider's sessions run
// lich's hooks at all. Without them no session-start ever arrives, so a session
// of that provider is not held back waiting for one (see Ready).
func (s *Service) SetStartReports(installed func(provider string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startReports = installed
}

// awaitsStart is whether Ready holds a session of kind until its session-start
// report arrives. Not under s.mu: the plugin check reads files and, for some
// providers, runs their CLI.
func (s *Service) awaitsStart(kind string) bool {
	s.mu.Lock()
	installed := s.startReports
	s.mu.Unlock()
	return startsAtPrompt[kind] && installed != nil && installed(kind)
}

// holdForStart records on a just-spawned session whether it waits for its
// session-start before being given work.
func (s *Service) holdForStart(id string, sess *session, awaits bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[id] == sess {
		sess.awaitsStart = awaits
	}
}

// markStarted records that a session's provider reported session-start, which
// releases the hold Ready keeps on it. A report for a session that is not
// running is dropped; the next spawn waits for its own.
func (s *Service) markStarted(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.started = true
	}
}
