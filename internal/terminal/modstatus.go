package terminal

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/omartelo/lich/internal/relay"
)

// modStatus answers a mod's read of its session's errands
// (docs/hooks/mod-status.md). It reads and never collects.
func (t *transport) modStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !t.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	session := r.URL.Query().Get("session_id")
	if session == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	t.plugins.note(session, r.Header.Get(pluginVersionHeader))
	t.mu.Lock()
	read := t.errandStatus
	t.mu.Unlock()
	status := relay.Status{Owed: []relay.OwedErrand{}, Open: []relay.OpenErrand{}, Ready: []relay.ReadyErrand{}}
	if read != nil {
		status = read(session)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		slog.Warn("mod: status lost on the way to the mod", "session", session, "err", err)
	}
}

func (t *transport) setErrandStatus(fn func(id string) relay.Status) {
	t.mu.Lock()
	t.errandStatus = fn
	t.mu.Unlock()
}

// SetErrandStatus wires what a mod's status read answers with: the relay's
// Status. Startup wiring, called once before any session reads.
func (s *Service) SetErrandStatus(fn func(id string) relay.Status) {
	if s.ws == nil {
		return
	}
	s.ws.setErrandStatus(fn)
}
