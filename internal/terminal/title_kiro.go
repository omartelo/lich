package terminal

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"github.com/omartelo/lich/internal/providers"
)

// The Kiro CLI side of the session title, and the one provider lich reads a
// title for itself. The report every other harness sends is a hook script
// reading a transcript path off its own stdin, and Kiro passes one on none of
// its five events (docs/hooks/session-title.md). It does file a `title`,
// derived from the first prompt, into the session metadata transcript.go
// resolves (kiroSessionPath): the same `.json` the context readout is read
// from. So the turn's end is where lich goes and reads it.

// kiroMetadataTitle reads the title Kiro filed for one conversation. ok is
// false for every absence (no file yet, a file caught mid-write, metadata
// carrying no title), because a session whose name lich cannot read keeps the
// name lich gave it, which is silence rather than a failure.
func kiroMetadataTitle(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var meta struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", false
	}
	// Trimmed like the reported one is (parseSessionTitle), so both doors
	// apply the same string.
	title := strings.TrimSpace(meta.Title)
	return title, title != ""
}

// kiroTitle resolves the title Kiro filed for the conversation running in
// session id. ok is false when the session runs something else, when no
// conversation is linked to it yet, and on every absence above.
func (s *Service) kiroTitle(id string) (string, bool) {
	if s.kindOf(id) != providers.Kiro {
		return "", false
	}
	providerSessionID, err := s.store.ProviderSession(id)
	if err != nil || providerSessionID == "" {
		return "", false
	}
	path, ok := kiroSessionPath(providerSessionID)
	if !ok {
		return "", false
	}
	return kiroMetadataTitle(path)
}

// applyKiroTitle publishes that title down the same path a reported one takes
// (onTitle): the store's guarded write, which leaves a session the user renamed
// alone, and the event the card redraws on.
func (s *Service) applyKiroTitle(id string) {
	title, ok := s.kiroTitle(id)
	if !ok {
		return
	}
	if err := s.onTitle(id, title); err != nil {
		slog.Warn("terminal: apply kiro title", "session", id, "err", err)
	}
}
