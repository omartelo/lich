package store

import (
	"fmt"
	"os"
)

// WorktreePorts returns the dev-server port reserved for each checkout, keyed
// by path. A reservation is what makes the port a checkout owns survive a
// restart of lich, so the allocator can hand a colliding checkout a different
// number instead of both reading the same one off a hash.
//
// Reservations whose directory is gone — a removed worktree, a checkout deleted
// by hand, a crash between the two — are deleted on the way out. The allocator
// is this table's only reader, so purging here is the whole lifecycle: nothing
// else has to remember to release a port, and a stale row can never hold a
// number against a live checkout. A read failure yields an empty map, which
// degrades allocation to the hash rather than failing the spawn.
func (s *Service) WorktreePorts() map[string]int {
	rows, err := s.db.Query(`SELECT path, port FROM worktree_ports`)
	if err != nil {
		return map[string]int{}
	}
	defer func() { _ = rows.Close() }()

	ports := map[string]int{}
	var stale []string
	for rows.Next() {
		var path string
		var port int
		if err := rows.Scan(&path, &port); err != nil {
			return map[string]int{}
		}
		if _, err := os.Stat(path); err != nil {
			stale = append(stale, path)
			continue
		}
		ports[path] = port
	}
	if err := rows.Err(); err != nil {
		return map[string]int{}
	}
	// Closed before the deletes: this connection is the only one, and a write
	// issued while its rows are still open deadlocks against the busy timeout.
	_ = rows.Close()
	for _, path := range stale {
		if _, err := s.db.Exec(`DELETE FROM worktree_ports WHERE path = ?`, path); err != nil {
			// A row that outlives its directory costs a port number, not
			// correctness — the next read tries again.
			break
		}
	}
	return ports
}

// SetWorktreePort records port as the reservation for the checkout at path,
// replacing any earlier one.
func (s *Service) SetWorktreePort(path string, port int) error {
	_, err := s.db.Exec(
		`INSERT INTO worktree_ports (path, port) VALUES (?, ?)
		 ON CONFLICT(path) DO UPDATE SET port = excluded.port`,
		path, port,
	)
	if err != nil {
		return fmt.Errorf("reserve worktree port for %q: %w", path, err)
	}
	return nil
}
