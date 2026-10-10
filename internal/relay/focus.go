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

// SetRaiseWindow wires how the relay brings lich's window to the front, or
// opens one on the session's card when none is open. Nil (a test that does not
// care) leaves Focus opening the card wherever the window is. Called at startup.
func (s *Service) SetRaiseWindow(raise func(sessionID string)) {
	s.raiseWindow = raise
}

// FocusOptions is one Focus call, an object for the reason SendOptions is one.
type FocusOptions struct {
	From    string `json:"from"`
	Target  string `json:"target"`
	Project string `json:"project"`
}

// Focus opens the card of the session Target names and brings the window to
// the front, for an editor that just pointed the person at that session.
// Target resolves the way Send's does.
func (s *Service) Focus(opts FocusOptions) (Focused, error) {
	fromID, target, project := opts.From, opts.Target, opts.Project
	dest, err := s.resolve(fromID, target, project)
	if err != nil {
		return Focused{}, err
	}
	if s.events != nil {
		s.events.Emit(FocusEventName, FocusEvent{ID: dest.ID})
	}
	if s.raiseWindow != nil {
		s.raiseWindow(dest.ID)
	}
	return Focused{ID: dest.ID, Label: dest.Peer.Label, Project: dest.Peer.Project}, nil
}
