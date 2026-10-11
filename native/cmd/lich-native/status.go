package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// The states a card draws an indicator for. Every other reported state (the
// contract's "idle", lich's own "interrupted", one from a newer plugin) maps to
// statusNone, which clears the indicator rather than stranding a stale one.
const (
	statusNone       = ""
	statusBusy       = "busy"
	statusDone       = "done"
	statusWaiting    = "waiting"
	statusCompacting = "compacting"
)

const statusEventName = "session-status"

// badgePriority picks a project's one badge: "waiting" blocks the user, "busy"
// and "compacting" are still running, "done" is the leftover.
var badgePriority = []string{statusWaiting, statusBusy, statusCompacting, statusDone}

// statusEvent is the session-status payload (internal/terminal statusEvent).
type statusEvent struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Tool   string `json:"tool,omitempty"`
	Detail string `json:"detail,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// sessionTool is the tool a session's turn is running, in its harness's words.
type sessionTool struct {
	Name   string
	Detail string
}

// pendingStatus is one row of the notification queue.
type pendingStatus struct {
	ID     string
	Status string
}

type statusEntry struct {
	status string
	// reason leaves with "waiting": no other state carries one.
	reason string
	// since is stamped on the state's transition alone: the hook repeats a
	// state while a turn runs, and a session has been waiting since it started.
	since time.Time
	// seen only matters to "done", the one state that persists with nothing
	// running; busy and waiting stay live whether or not anyone looked.
	seen bool
	// dismissed is apart from seen because a dismissed "waiting" still blocks:
	// it leaves the queue but keeps drawing its card.
	dismissed bool
	// reported never goes back to false: idle and interrupted map to no status
	// yet still prove the provider reports at all.
	reported bool
	tool     *sessionTool
}

// statusStore keeps the last reported status of every session, ported from
// the web's session-status-store.ts and session-tool-store.ts. It is not safe
// for concurrent use: the model guards it with its own mutex.
type statusStore struct {
	entries map[string]*statusEntry
	// order keeps the queue in first-seen order, as the web's Map does.
	order []string
}

func newStatusStore() *statusStore {
	return &statusStore{entries: map[string]*statusEntry{}}
}

func (s *statusStore) entryOf(id string) *statusEntry {
	e, ok := s.entries[id]
	if !ok {
		e = &statusEntry{}
		s.entries[id] = e
		s.order = append(s.order, id)
	}
	return e
}

// apply folds one /events message into the store at now. Events other than
// session-status are ignored. changed reports whether anything a card draws
// moved; a payload that does not decode or names no session is an error.
func (s *statusStore) apply(ev lichclient.Event, now time.Time) (changed bool, err error) {
	if ev.Name != statusEventName {
		return false, nil
	}
	var p statusEvent
	if err := json.Unmarshal(ev.Data, &p); err != nil {
		return false, fmt.Errorf("%s: decode: %w", statusEventName, err)
	}
	if p.ID == "" {
		return false, errors.New(statusEventName + ": payload has no session id")
	}
	e := s.entryOf(p.ID)
	toolChanged := e.applyTool(p)
	return e.applyState(p, now) || toolChanged, nil
}

func (e *statusEntry) applyState(p statusEvent, now time.Time) bool {
	next := renderedStatus(p.State)
	reason := ""
	if next == statusWaiting {
		reason = p.Reason
	}
	first := !e.reported
	e.reported = true
	// A repeat state is no news unless its question changed: a second
	// permission prompt inside one turn repeats "waiting" with a new reason.
	if e.status == next && e.reason == reason {
		return first
	}
	if e.status != next {
		e.since = now
	}
	e.status = next
	e.reason = reason
	e.seen = false
	e.dismissed = false
	return true
}

// applyTool reads every report, repeats included, since a repeat "busy" is
// what carries a new tool. Leaving "busy" clears it, never "idle" alone: Codex
// has no SessionEnd and would hold a dead tool name. A "busy" naming no tool is
// the PostToolUse gap between two tools and changes nothing.
func (e *statusEntry) applyTool(p statusEvent) bool {
	if p.State != statusBusy {
		changed := e.tool != nil
		e.tool = nil
		return changed
	}
	if p.Tool == "" {
		return false
	}
	next := sessionTool{Name: p.Tool, Detail: p.Detail}
	if e.tool != nil && *e.tool == next {
		return false
	}
	e.tool = &next
	return true
}

func renderedStatus(state string) string {
	switch state {
	case statusBusy, statusDone, statusWaiting, statusCompacting:
		return state
	}
	return statusNone
}

// status is the session's rendered state, statusNone when it has none.
func (s *statusStore) status(id string) string {
	if e, ok := s.entries[id]; ok {
		return e.status
	}
	return statusNone
}

// tool is what the session's turn is running, false when nothing.
func (s *statusStore) tool(id string) (sessionTool, bool) {
	e, ok := s.entries[id]
	if !ok || e.tool == nil {
		return sessionTool{}, false
	}
	return *e.tool, true
}

// reason is what a waiting session is blocked on, "" when unsaid.
func (s *statusStore) reason(id string) string {
	if e, ok := s.entries[id]; ok {
		return e.reason
	}
	return ""
}

// reported is whether the session ever reported a state, idle included.
func (s *statusStore) reported(id string) bool {
	e, ok := s.entries[id]
	return ok && e.reported
}

// unread is a finished turn nobody has looked at since.
func (s *statusStore) unread(id string) bool {
	e, ok := s.entries[id]
	return ok && e.status == statusDone && !e.seen
}

// age is how long the session has been in a live state at now. "done" has no
// age: nothing is accruing and its number would climb for hours. A clock that
// jumped backwards reads as zero.
func (s *statusStore) age(id string, now time.Time) (time.Duration, bool) {
	e, ok := s.entries[id]
	if !ok || !isRunning(e.status) {
		return 0, false
	}
	return max(0, now.Sub(e.since)), true
}

// markSeen records that the session's status was read. It answers true when
// that read a finished turn: the caller then takes down the backend's unread
// mark, the one edge only the window can see.
func (s *statusStore) markSeen(id string) bool {
	e, ok := s.entries[id]
	if !ok || e.seen {
		return false
	}
	e.seen = true
	return e.status == statusDone
}

// dismiss takes the session out of the queue until its next report.
// Dismissing a finished turn is reading it, so it answers as markSeen does.
func (s *statusStore) dismiss(id string) bool {
	e, ok := s.entries[id]
	if !ok {
		return false
	}
	e.dismissed = true
	return s.markSeen(id)
}

// restoreUnread seeds the sessions the database says hold an unread finished
// turn (store.Session.Unread). A session a report already spoke for is left
// alone: hydration lands after /events opens, so that report is newer.
func (s *statusStore) restoreUnread(ids []string) {
	for _, id := range ids {
		e := s.entryOf(id)
		if e.status != statusNone {
			continue
		}
		e.status = statusDone
		e.reported = true
		e.seen = false
		e.dismissed = false
	}
}

// pending is the notification queue: sessions blocked waiting, or holding an
// unread finished turn, minus dismissed ones, in first-seen order.
func (s *statusStore) pending() []pendingStatus {
	var out []pendingStatus
	for _, id := range s.order {
		e := s.entries[id]
		if e.status != statusWaiting && e.status != statusDone {
			continue
		}
		if e.dismissed || (e.status == statusDone && e.seen) {
			continue
		}
		out = append(out, pendingStatus{ID: id, Status: e.status})
	}
	return out
}

// projectStatus reduces a project's sessions to the one status its tab badges,
// statusNone when there is nothing to say. A read "done" says nothing.
func (s *statusStore) projectStatus(ids []string) string {
	live := map[string]bool{}
	for _, id := range ids {
		e, ok := s.entries[id]
		if !ok || e.status == statusNone || (e.status == statusDone && e.seen) {
			continue
		}
		live[e.status] = true
	}
	for _, status := range badgePriority {
		if live[status] {
			return status
		}
	}
	return statusNone
}

// running returns the sessions among ids that still hold a turn, read or not.
func (s *statusStore) running(ids []string) []string {
	var out []string
	for _, id := range ids {
		if isRunning(s.status(id)) {
			out = append(out, id)
		}
	}
	return out
}

func isRunning(status string) bool {
	return status == statusBusy || status == statusWaiting || status == statusCompacting
}
