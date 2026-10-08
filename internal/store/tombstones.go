package store

import "fmt"

// ForgottenProviderSessions is the set of provider conversation ids lich once
// held a session for and deleted for good: the ones a listing of the
// conversations on disk must not offer back as started outside lich. The
// triggers of addProviderSessionTombstones keep it, so every path that deletes a
// row is covered. A row deleted before the table existed left no stone.
func (s *Service) ForgottenProviderSessions() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT provider_session_id FROM provider_session_tombstones`)
	if err != nil {
		return nil, fmt.Errorf("query provider session tombstones: %w", err)
	}
	defer rows.Close()
	forgotten := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan provider session tombstone: %w", err)
		}
		forgotten[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider session tombstones: %w", err)
	}
	return forgotten, nil
}
