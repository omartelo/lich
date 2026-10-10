package relay

import (
	"fmt"
	"log/slog"

	"github.com/omartelo/lich/internal/prompt"
	"github.com/omartelo/lich/internal/providers"
)

// AnnounceMerge tells every agent running in checkout that the pull request on
// its branch was merged from lich's own screen. An agent keeps notes on the work
// it did there, and without this the last thing they say about it is that it is
// waiting on a merge, which a later session reads back as true long after the
// checkout is gone.
//
// Only provider sessions hear it: a terminal card in the same checkout is a
// shell, where a typed notice would run as a command. A card with no process
// running has nobody to tell and is skipped, as a parked one is.
//
// Delivery runs in the background and a failure is only logged: the merge it
// reports already happened, and nothing the caller could do with the error would
// undo it. The error returned is the workspace failing to load, before anyone
// was told anything.
func (s *Service) AnnounceMerge(checkout string, number int, title, branch, base string) error {
	projects, err := s.sessions.LoadState()
	if err != nil {
		return fmt.Errorf("read sessions: %w", err)
	}
	lang := s.lang()
	message := mergeNotice(lang, number, title, branch, base)
	summary := fmt.Sprintf(prompt.For(lang).MergeSummary, number, base)
	note := &Notification{Status: NotifyCompleted, Summary: summary}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if !s.runsAgentIn(sess.ID, sess.Kind, sessionCwd(sess.Path, p.Path), checkout) {
				continue
			}
			go func(id, label string) {
				if err := s.deliver(id, message, note); err != nil {
					slog.Warn("relay: merge notice not delivered", "session", label, "err", err)
				}
			}(sess.ID, sess.Label)
		}
	}
	return nil
}

// runsAgentIn is whether a session is a live provider session whose checkout is
// the one given.
func (s *Service) runsAgentIn(id, kind, cwd, checkout string) bool {
	return cwd == checkout && providers.Known(kind) && s.term.Live(id)
}

// sessionCwd is the directory a session runs in: its worktree, or the project's
// own directory for a session that names none.
func sessionCwd(sessionPath, projectPath string) string {
	if sessionPath == "" {
		return projectPath
	}
	return sessionPath
}

// mergeNotice is what the agent reads. It says the work is done and asks for
// nothing beyond keeping its record straight: a notice that read as a task would
// start one (deleting the branch, pulling the base) in a checkout the user may
// be about to remove.
func mergeNotice(lang prompt.Lang, number int, title, branch, base string) string {
	return fmt.Sprintf(prompt.For(lang).MergeNotice, number, sanitize(title), branch, base)
}
