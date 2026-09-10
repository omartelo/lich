package store

import (
	"encoding/json"
	"fmt"
)

// SetSessionMCPServers records the MCP servers a session's provider could reach
// at the spawn that just happened. The window divides a tool name against them
// (frontend/src/lib/session/tool-label.ts), and the row is what a page reload
// comes back to — the PTY is still running by then, so there is no second spawn
// to report it again.
//
// An empty list clears the row rather than parking "[]": a session that reached
// nothing and a session nobody has spawned read the same way, and both mean the
// card shows a tool name whole.
func (s *Service) SetSessionMCPServers(sessionID string, servers []string) error {
	return s.setSessionList(sessionID, "mcp_servers", servers)
}

// SetSessionSandboxLinks records the home paths this session's sandbox skipped
// for being symlinks, home-relative — what the card names as not mounted, so a
// ~/.gitconfig symlinked out of a dotfiles repository is an absence the session
// is told about rather than one it discovers by failing. Written by the spawn
// that resolved them, and read back on hydration for the reason above: the PTY
// outlives the page.
func (s *Service) SetSessionSandboxLinks(sessionID string, links []string) error {
	return s.setSessionList(sessionID, "sandbox_links", links)
}

// setSessionList writes a JSON array into one of the session's list columns.
// column is a literal from the two callers above and never anything a caller
// outside this file names.
func (s *Service) setSessionList(sessionID, column string, values []string) error {
	encoded := ""
	if len(values) > 0 {
		body, err := json.Marshal(values)
		if err != nil {
			return fmt.Errorf("encode %s for %q: %w", column, sessionID, err)
		}
		encoded = string(body)
	}
	if _, err := s.db.Exec(
		fmt.Sprintf(`UPDATE sessions SET %s = ? WHERE id = ?`, column), encoded, sessionID,
	); err != nil {
		return fmt.Errorf("set %s on %q: %w", column, sessionID, err)
	}
	return nil
}

// decodeStrings reads back what setSessionList wrote. A row lich cannot parse
// answers nil, which is the same answer an unspawned row gives.
func decodeStrings(encoded string) []string {
	if encoded == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(encoded), &values); err != nil {
		return nil
	}
	return values
}
