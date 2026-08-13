package relay

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/omartelo/lich/internal/store"
)

// The roster: which sessions a caller may address, and how a name it typed
// becomes one of them. A session has two names and both reach it, so resolving
// one is a decision with an order to it — the relay never searches on its own,
// it asks here and gets a single session or an error saying why not.

// candidate is a resolved peer together with the session id behind it, which
// callers never see: they address a session by the label on its card.
type candidate struct {
	ID   string
	Peer Peer
}

// roster returns every live session except the caller's own.
func (s *Service) roster(fromID string) ([]candidate, error) {
	projects, err := s.sessions.LoadState()
	if err != nil {
		return nil, fmt.Errorf("read sessions: %w", err)
	}
	var found []candidate
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == fromID || !s.term.Live(sess.ID) {
				continue
			}
			cwd := sess.Path
			if cwd == "" {
				cwd = p.Path
			}
			found = append(found, candidate{
				ID: sess.ID,
				Peer: Peer{
					Label:   sess.Label,
					Name:    RosterName(cwd, sess.ID),
					Project: p.Name,
					Kind:    sess.Kind,
				},
			})
		}
	}
	return found, nil
}

// resolve finds the single live session named by target, optionally narrowed to
// one project. An ambiguous label is an error naming every match, because
// guessing which session a prompt lands in is the one mistake this feature must
// not make.
func (s *Service) resolve(fromID, target, project string) (candidate, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return candidate{}, fmt.Errorf("no target session given")
	}
	found, err := s.roster(fromID)
	if err != nil {
		return candidate{}, err
	}

	// The label first, then the roster name. Both address this session, and an
	// agent may have been handed either — a mention writes the roster name at
	// the prompt, list_sessions prints both. The label wins a tie because it is
	// the name the user chose and the one every message and error quotes.
	matches := matching(found, target, project, func(c candidate) string { return c.Peer.Label })
	if len(matches) == 0 {
		matches = matching(found, target, project, func(c candidate) string { return c.Peer.Name })
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return candidate{}, fmt.Errorf(
			"no live session named %q%s. %s",
			target, inProject(project), knownLabels(found),
		)
	default:
		return candidate{}, fmt.Errorf(
			"%q names %d live sessions (%s) — narrow it with the project",
			target, len(matches), projectsOf(matches),
		)
	}
}

// sessionOf finds one session in the workspace by id. The three ways there is
// no answer — no id at all, an unreadable store, an id lich has no record of —
// are one false here, because every caller treats them the same.
func (s *Service) sessionOf(id string) (store.Session, bool) {
	if id == "" {
		return store.Session{}, false
	}
	projects, err := s.sessions.LoadState()
	if err != nil {
		return store.Session{}, false
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				return sess, true
			}
		}
	}
	return store.Session{}, false
}

// labelOf names the sending session for the message it is about to deliver.
// An empty id is a caller with no session at all — the CLI run from a plain
// shell or a script — and stays empty, which compose words differently. A
// caller lich has no record of still gets to send: the receiving agent is told
// the sender is unknown rather than told nothing at all.
func (s *Service) labelOf(id string) string {
	if id == "" {
		return ""
	}
	if sess, ok := s.sessionOf(id); ok {
		return sess.Label
	}
	return "unknown"
}

// matching returns the candidates whose name — read out by naming — is target,
// narrowed to one project when one was given.
func matching(found []candidate, target, project string, naming func(candidate) string) []candidate {
	var matches []candidate
	for _, c := range found {
		if !strings.EqualFold(naming(c), target) {
			continue
		}
		if project != "" && !strings.EqualFold(c.Peer.Project, project) {
			continue
		}
		matches = append(matches, c)
	}
	return matches
}

// knownLabels names what is actually reachable, under both names, so a wrong
// name is one step from a right one rather than a guess.
func knownLabels(found []candidate) string {
	if len(found) == 0 {
		return "No other session is live."
	}
	names := make([]string, 0, len(found))
	for _, c := range found {
		names = append(names, fmt.Sprintf("%s (%s)", strconv.Quote(c.Peer.Label), c.Peer.Name))
	}
	return "Live sessions: " + strings.Join(names, ", ") + "."
}

func inProject(project string) string {
	if project == "" {
		return ""
	}
	return fmt.Sprintf(" in project %q", project)
}

func projectsOf(matches []candidate) string {
	names := make([]string, 0, len(matches))
	for _, c := range matches {
		names = append(names, c.Peer.Project)
	}
	return strings.Join(names, ", ")
}
