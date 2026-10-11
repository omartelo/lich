package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// agentEvent and sandboxEvent are internal/terminal's session-agent and
// session-sandbox payloads.
type agentEvent struct {
	ID    string `json:"id"`
	Agent string `json:"agent"`
}

// sessionRelay is a session-relay payload (internal/relay RelayEvent): the
// request a session has open with another. Peer is "" when the other end is
// the lich command line; an empty direction clears the mark.
type sessionRelay struct {
	ID        string `json:"id"`
	Peer      string `json:"peer"`
	Direction string `json:"direction"`
}

type sandboxEvent struct {
	ID       string `json:"id"`
	Confined bool   `json:"confined"`
}

// Directions a relay mark runs (internal/relay DirectionOut, DirectionIn).
const (
	relayOut = "out"
	relayIn  = "in"
)

// applyStatus feeds a session-status report to the status store. An idle
// report is the provider CLI leaving the PTY, so the card drops its agent
// mark (session-agent-store.ts), and a report on the session being watched
// is read on arrival (project-events.tsx markSeenIfWatched).
func (m *model) applyStatus(ctx context.Context, ev lichclient.Event) error {
	var s struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(ev.Data, &s); err != nil {
		return err
	}
	m.mu.Lock()
	_, err := m.status.apply(ev, time.Now())
	if s.State == "idle" {
		// The CLI left: it can no longer answer a request either
		// (session-relay-store.ts).
		delete(m.agent, s.ID)
		delete(m.relay, s.ID)
	}
	read := m.focused && s.ID == m.active && m.status.markSeen(s.ID)
	m.mu.Unlock()
	if read {
		m.markRead(ctx, s.ID)
	}
	return err
}

// applyLive takes the live PTY reports the backend never replays: the
// provider CLI running in a session, and whether its spawn ran confined. It
// reports whether ev was one of them.
func (m *model) applyLive(ev lichclient.Event) (bool, error) {
	switch ev.Name {
	case "session-agent":
		var a agentEvent
		if err := json.Unmarshal(ev.Data, &a); err != nil {
			return true, err
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		// "" is the clear every spawn sends; an unknown kind is a provider
		// from a newer backend, and the card falls back to its own kind.
		if _, known := providerIcons[a.Agent]; known {
			m.agent[a.ID] = a.Agent
		} else {
			delete(m.agent, a.ID)
		}
	case "session-relay":
		var r sessionRelay
		if err := json.Unmarshal(ev.Data, &r); err != nil {
			return true, err
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if r.Direction == relayOut || r.Direction == relayIn {
			m.relay[r.ID] = r
		} else {
			delete(m.relay, r.ID)
		}
	case "session-sandbox":
		var s sandboxEvent
		if err := json.Unmarshal(ev.Data, &s); err != nil {
			return true, err
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.confined[s.ID] = s.Confined
	default:
		return false, nil
	}
	return true, nil
}

// setFocused follows the window's focus: a finished turn is read only while
// someone can see it, and coming back to the window reads the one on screen.
func (m *model) setFocused(ctx context.Context, focused bool) {
	m.mu.Lock()
	m.focused = focused
	if focused {
		m.stalePullRequests()
	}
	id := m.active
	read := focused && m.status.markSeen(id)
	m.mu.Unlock()
	if read {
		m.markRead(ctx, id)
	}
}

// markRead takes down the backend's unread mark of a turn this window just
// read, so a restart does not bring the ring back.
func (m *model) markRead(ctx context.Context, id string) {
	go func() {
		if err := m.client.SetSessionUnread(ctx, id, false); err != nil {
			log.Printf("mark %s read: %v", id, err)
		}
	}()
}

// setPinned flips a session's pin here, ahead of the backend, as the web's
// optimistic commit does. It reports false when no open session has that id.
func (m *model) setPinned(id string, pinned bool) bool {
	m.mu.Lock()
	p, i := m.sessionAt(id)
	if p != nil {
		p.Sessions[i].Pinned = pinned
	}
	m.mu.Unlock()
	return p != nil
}
