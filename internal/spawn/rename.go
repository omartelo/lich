package spawn

import (
	"errors"
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/store"
)

// RenamedEventName carries a session renamed outside the window, so the card
// takes the new name without a reload.
//
// It is the event the auto-applied ai-title already emits
// (terminal.titleEventName) rather than one of its own: the payload is the same
// pair and the window's answer to both is the same — relabel that card. A second
// name for one instruction would have bought a second handler saying it again.
const RenamedEventName = "session-title"

// renamedEvent is RenamedEventName's payload, the shape terminal's title event
// writes.
type renamedEvent struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Renamed is what a caller is told about the session it renamed. Previous is
// the name that is gone, which is the half the caller cannot look up afterwards.
type Renamed struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Label    string `json:"label"`
	Previous string `json:"previous"`
}

// RenameOptions is one Rename call, an object for the reason CloseOptions is
// one.
type RenameOptions struct {
	From    string `json:"from"`
	Target  string `json:"target"`
	Project string `json:"project"`
	Label   string `json:"label"`
}

// Rename gives a session the name on its card, the window's rename from outside
// the window. Like the window's, it makes the name the user's: the provider's
// ai-title never stomps a chosen name again (store.RenameSession).
//
// Target is the session to rename, by either of the names it answers to; empty
// renames the caller's own, which is the one form an agent can reach without
// discovery — list_sessions shows it every session but itself.
//
// A name another session in that project already holds is refused rather than
// written, because two sessions under one label is the one thing `lich send`
// cannot resolve. The window has no such rule: it renames what the user is
// pointing at, and the user can see which card they meant.
func (s *Service) Rename(opts RenameOptions) (Renamed, error) {
	fromID, target, projectName := opts.From, opts.Target, opts.Project
	label := strings.TrimSpace(opts.Label)
	if label == "" {
		return Renamed{}, errors.New("a rename needs a name, and none was given")
	}

	projects, err := s.sessions.LoadState()
	if err != nil {
		return Renamed{}, fmt.Errorf("read the workspace: %w", err)
	}
	found, err := targetOrOwn(projects, s.term.AgentName, fromID, target, projectName, "rename")
	if err != nil {
		return Renamed{}, err
	}
	if labelTaken(found.project, label, found.session.ID) {
		return Renamed{}, fmt.Errorf(
			"%q already names another session in %s, and two sessions under one name is the "+
				"one thing `lich send` cannot resolve",
			label, found.project.Name,
		)
	}

	if err := s.sessions.RenameSession(found.session.ID, label); err != nil {
		return Renamed{}, err
	}
	if s.events != nil {
		s.events.Emit(RenamedEventName, renamedEvent{ID: found.session.ID, Label: label})
	}
	return Renamed{
		ID:       found.session.ID,
		Project:  found.project.Name,
		Label:    label,
		Previous: found.session.Label,
	}, nil
}

// targetOrOwn resolves the session a rename or a filing names: the one
// findSession finds, or the caller's own when the call named none. A command
// line run outside a session has no own to fall back on, and is told so rather
// than handed the resolver's "no session named """. verb is what the call does,
// for that message.
func targetOrOwn(
	projects []store.Project, nameOf func(id string) string, fromID, target, projectName, verb string,
) (located, error) {
	if strings.TrimSpace(target) != "" {
		return findSession(projects, nameOf, target, projectName)
	}
	if fromID == "" {
		return located{}, fmt.Errorf(
			"no session was named to %s, and this is not running in one — name the session", verb,
		)
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == fromID {
				return located{project: p, session: sess}, nil
			}
		}
	}
	return located{}, fmt.Errorf("this session (%s) is not in the workspace anymore", fromID)
}
