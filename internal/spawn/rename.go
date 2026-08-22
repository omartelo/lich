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

// Rename gives a session the name on its card, the window's rename from outside
// the window. Like the window's, it makes the name the user's: the provider's
// ai-title never stomps a chosen name again (store.RenameSession).
//
// target is the session to rename, by either of the names it answers to; empty
// renames the caller's own, which is the one form an agent can reach without
// discovery — list_sessions shows it every session but itself.
//
// A name another session in that project already holds is refused rather than
// written, because two sessions under one label is the one thing `lich send`
// cannot resolve. The window has no such rule: it renames what the user is
// pointing at, and the user can see which card they meant.
func (s *Service) Rename(fromID, target, projectName, label string) (Renamed, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return Renamed{}, errors.New("a rename needs a name, and none was given")
	}

	projects, err := s.sessions.LoadState()
	if err != nil {
		return Renamed{}, fmt.Errorf("read the workspace: %w", err)
	}
	found, err := renameTarget(projects, fromID, target, projectName)
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

// renameTarget resolves the session a rename names: the one findSession finds,
// or the caller's own when the rename named none. A command line run outside a
// session has no own to fall back on, and is told so rather than handed the
// resolver's "no session named """.
func renameTarget(projects []store.Project, fromID, target, projectName string) (located, error) {
	if strings.TrimSpace(target) != "" {
		return findSession(projects, target, projectName)
	}
	if fromID == "" {
		return located{}, errors.New(
			"no session was named to rename, and this is not running in one — name the session",
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
