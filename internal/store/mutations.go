package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/omartelo/lich/internal/providers"
)

// AddProject persists a newly opened project and marks it open. Reopening a
// previously closed project keeps its stored sessions, name, path and tab
// position intact — only is_open flips back to 1. A brand-new project takes the
// position after the last one, so it opens as the rightmost tab.
func (s *Service) AddProject(id, name, path string) error {
	_, err := s.db.Exec(
		`INSERT INTO projects (id, name, path, is_open, position)
		 VALUES (?, ?, ?, 1, (SELECT COALESCE(MAX(position), -1) + 1 FROM projects))
		 ON CONFLICT(id) DO UPDATE SET is_open = 1, name = excluded.name, path = excluded.path`,
		id, name, path,
	)
	if err != nil {
		return fmt.Errorf("add project %q: %w", id, err)
	}
	return nil
}

// CloseProject marks a project closed without deleting it or its sessions, so it
// can be reopened later with its session state restored.
func (s *Service) CloseProject(id string) error {
	if _, err := s.db.Exec(`UPDATE projects SET is_open = 0 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("close project %q: %w", id, err)
	}
	return nil
}

// AddSession inserts a session, makes it the project's active one and records the
// project's next label counter — all atomically, mirroring the frontend reducer.
// Kind selects what the session's PTY runs (a provider id or "shell"); empty
// defaults to "claude" so older callers keep the original behavior. Path is the session's
// working directory when it lives in a git worktree; empty means the project's.
// The session takes the position after the project's last one, so it appends to
// the card list even once the user has dragged the others around.
func (s *Service) AddSession(projectID, sessionID, label, kind, path string, nextSeq int) error {
	if kind == "" {
		kind = providers.Claude
	}
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO sessions (id, project_id, label, kind, path, position)
			 VALUES (?, ?, ?, ?, ?,
			         (SELECT COALESCE(MAX(position), -1) + 1 FROM sessions WHERE project_id = ?))`,
			sessionID, projectID, label, kind, path, projectID,
		); err != nil {
			return fmt.Errorf("insert session %q: %w", sessionID, err)
		}
		if _, err := tx.Exec(
			`UPDATE projects SET active_session_id = ?, next_seq = ? WHERE id = ?`,
			sessionID, nextSeq, projectID,
		); err != nil {
			return fmt.Errorf("update project %q counters: %w", projectID, err)
		}
		return nil
	})
}

// DeleteSession removes a session for good and sets the project's active session
// to activeID (the neighbor the frontend picked, or "" when none remain).
func (s *Service) DeleteSession(projectID, sessionID, activeID string) error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
			return fmt.Errorf("delete session %q: %w", sessionID, err)
		}
		if _, err := tx.Exec(
			`UPDATE projects SET active_session_id = ? WHERE id = ?`,
			activeID, projectID,
		); err != nil {
			return fmt.Errorf("update active session of %q: %w", projectID, err)
		}
		return nil
	})
}

// CloseSession parks a session instead of deleting it: is_open flips to 0, which
// hides it from LoadState while keeping its row — and its provider session id —
// intact for a later resume. The project's active session moves to activeID (the
// neighbor the frontend picked). The keep-the-worktree close uses this; a plain
// close still DeleteSessions for good.
func (s *Service) CloseSession(projectID, sessionID, activeID string) error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE sessions SET is_open = 0 WHERE id = ?`, sessionID); err != nil {
			return fmt.Errorf("close session %q: %w", sessionID, err)
		}
		if _, err := tx.Exec(
			`UPDATE projects SET active_session_id = ? WHERE id = ?`,
			activeID, projectID,
		); err != nil {
			return fmt.Errorf("update active session of %q: %w", projectID, err)
		}
		return nil
	})
}

// ReopenWorktreeSession resumes a parked worktree session. It finds the parked
// (is_open = 0) session for the worktree at path and re-adds it to the workspace
// under a fresh id (newSessionID), carrying over the old label, kind, provider
// session id and label_auto flag. The fresh id is deliberate: it makes the frontend treat the card
// as never-spawned, so its resume prompt fires and the provider conversation
// continues instead of starting cold. Returns nil when nothing is parked at path
// — the caller then opens a brand-new session.
func (s *Service) ReopenWorktreeSession(projectID, path, newSessionID string) (*Session, error) {
	var restored *Session
	err := s.tx(func(tx *sql.Tx) error {
		var old Session
		// label_auto rides along so a user rename survives the park/resume
		// cycle — reinserting without it would reset to 1 and let the ai-title
		// stomp the chosen name, breaking SetSessionTitle's contract.
		var labelAuto int
		row := tx.QueryRow(
			`SELECT id, label, kind, provider_session_id, label_auto
			   FROM sessions
			  WHERE project_id = ? AND path = ? AND is_open = 0
			  ORDER BY rowid DESC LIMIT 1`,
			projectID, path,
		)
		if err := row.Scan(&old.ID, &old.Label, &old.Kind, &old.ProviderSessionID, &labelAuto); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil // nothing parked here; caller creates a new session
			}
			return fmt.Errorf("find parked session for %q: %w", path, err)
		}
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, old.ID); err != nil {
			return fmt.Errorf("drop parked session %q: %w", old.ID, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO sessions (id, project_id, label, kind, path, provider_session_id, label_auto, position)
			 VALUES (?, ?, ?, ?, ?, ?, ?,
			         (SELECT COALESCE(MAX(position), -1) + 1 FROM sessions WHERE project_id = ?))`,
			newSessionID, projectID, old.Label, old.Kind, path, old.ProviderSessionID, labelAuto, projectID,
		); err != nil {
			return fmt.Errorf("reinsert session %q: %w", newSessionID, err)
		}
		if _, err := tx.Exec(
			`UPDATE projects SET active_session_id = ? WHERE id = ?`,
			newSessionID, projectID,
		); err != nil {
			return fmt.Errorf("activate reopened session on %q: %w", projectID, err)
		}
		restored = &Session{ID: newSessionID, Label: old.Label, Kind: old.Kind, Path: path, ProviderSessionID: old.ProviderSessionID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return restored, nil
}

// PurgeWorktreeSessions deletes every session row for the worktree at path in a
// project — the live one and any parked leftovers alike — so removing a worktree
// never strands a hidden row that a later resume could resurrect against a
// checkout that no longer exists. The empty-path guard is load-bearing: a
// project's own sessions carry no path, so an unguarded delete would wipe them
// all. Idempotent — no matching rows is not an error.
func (s *Service) PurgeWorktreeSessions(projectID, path string) error {
	if path == "" {
		return nil
	}
	if _, err := s.db.Exec(
		`DELETE FROM sessions WHERE project_id = ? AND path = ?`, projectID, path,
	); err != nil {
		return fmt.Errorf("purge worktree sessions for %q: %w", path, err)
	}
	return nil
}

// RenameSession updates a session's display label from an explicit user rename.
// It clears label_auto so the automatic ai-title (SetSessionTitle) never stomps
// a name the user chose.
func (s *Service) RenameSession(sessionID, label string) error {
	if _, err := s.db.Exec(
		`UPDATE sessions SET label = ?, label_auto = 0 WHERE id = ?`,
		label, sessionID,
	); err != nil {
		return fmt.Errorf("rename session %q: %w", sessionID, err)
	}
	return nil
}

// SetSessionTitle sets a session's label from the provider's ai-title reported
// by the Stop hook, but only while the label is still automatic: a prior
// RenameSession clears label_auto and makes this a no-op, so a user's own name
// is never overwritten. Reports whether the label actually changed, so the
// caller only pushes a UI update when it did.
func (s *Service) SetSessionTitle(sessionID, title string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE sessions SET label = ? WHERE id = ? AND label_auto = 1`,
		title, sessionID,
	)
	if err != nil {
		return false, fmt.Errorf("set session %q title: %w", sessionID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("set session %q title rows: %w", sessionID, err)
	}
	return n > 0, nil
}

// SetProviderSession records the provider conversation id running inside a lich
// session's PTY, reported by the provider's session-start hook. A session whose
// row does not exist yet (the hook racing session persistence) matches nothing
// and is not an error — the id is simply dropped, which is acceptable for the
// features it backs. Re-reporting (e.g. after a resume) overwrites with the
// latest id.
func (s *Service) SetProviderSession(sessionID, providerSessionID string) error {
	if _, err := s.db.Exec(
		`UPDATE sessions SET provider_session_id = ? WHERE id = ?`,
		providerSessionID, sessionID,
	); err != nil {
		return fmt.Errorf("set provider session on %q: %w", sessionID, err)
	}
	return nil
}

// ReorderProjects records the tab order after a drag. It writes every project's
// position from the full list the frontend rendered, so the stored order is
// rewritten as a whole rather than patched around the moved tab.
func (s *Service) ReorderProjects(ids []string) error {
	return s.tx(func(tx *sql.Tx) error {
		for position, id := range ids {
			if _, err := tx.Exec(
				`UPDATE projects SET position = ? WHERE id = ?`, position, id,
			); err != nil {
				return fmt.Errorf("reorder projects: %w", err)
			}
		}
		return nil
	})
}

// ReorderSessions records a project's card order after a drag, writing every
// position from the full list. Scoped to the project so an id belonging to
// another project's list can never take a position in this one.
func (s *Service) ReorderSessions(projectID string, ids []string) error {
	return s.tx(func(tx *sql.Tx) error {
		for position, id := range ids {
			if _, err := tx.Exec(
				`UPDATE sessions SET position = ? WHERE id = ? AND project_id = ?`,
				position, id, projectID,
			); err != nil {
				return fmt.Errorf("reorder sessions: %w", err)
			}
		}
		return nil
	})
}

// SetActiveSession records which session is focused within a project.
func (s *Service) SetActiveSession(projectID, sessionID string) error {
	if _, err := s.db.Exec(
		`UPDATE projects SET active_session_id = ? WHERE id = ?`,
		sessionID, projectID,
	); err != nil {
		return fmt.Errorf("set active session %q on %q: %w", sessionID, projectID, err)
	}
	return nil
}

// tx runs fn inside a transaction, committing on success and rolling back on any
// error.
func (s *Service) tx(fn func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
