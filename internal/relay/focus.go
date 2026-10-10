package relay

// FocusEventName asks the window to open a session's card, the way a click on
// it does. Global like the other session events: the card it names may live in
// a project whose tab is not the one on screen.
const FocusEventName = "session-focus"

// FocusEvent is the payload of FocusEventName.
type FocusEvent struct {
	ID string `json:"id"`
}

// Focused says which session's card was brought up, so a caller that named it
// loosely can tell the person which one it was.
type Focused struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Project string `json:"project"`
}

// SetRaiseWindow wires how the relay brings lich's window to the front. Nil (a
// test that does not care, or lich serving without a window) leaves Focus
// opening the card wherever the window is. Called at startup.
func (s *Service) SetRaiseWindow(raise func()) {
	s.raiseWindow = raise
}

// Focus opens the card of the session target names and brings the window to
// the front, for an editor that just pointed the person at that session.
// target resolves the way Send's does.
func (s *Service) Focus(fromID, target, project string) (Focused, error) {
	dest, err := s.resolve(fromID, target, project)
	if err != nil {
		return Focused{}, err
	}
	if s.events != nil {
		s.events.Emit(FocusEventName, FocusEvent{ID: dest.ID})
	}
	if s.raiseWindow != nil {
		s.raiseWindow()
	}
	return Focused{ID: dest.ID, Label: dest.Peer.Label, Project: dest.Peer.Project}, nil
}
