package store

import (
	"database/sql"
	"fmt"
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
		return fmt.Errorf("add project: %w", err)
	}
	return nil
}

// CloseProject marks a project closed without deleting it or its sessions, so it
// can be reopened later with its session state restored.
func (s *Service) CloseProject(id string) error {
	if _, err := s.db.Exec(`UPDATE projects SET is_open = 0 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("close project: %w", err)
	}
	return nil
}

// AddSession inserts a session, makes it the project's active one and records the
// project's next label counter — all atomically, mirroring the frontend reducer.
// Kind selects what the session's PTY runs ("claude" or "shell"); empty defaults
// to "claude" so older callers keep the original behavior. Path is the session's
// working directory when it lives in a git worktree; empty means the project's.
// The session takes the position after the project's last one, so it appends to
// the card list even once the user has dragged the others around.
func (s *Service) AddSession(projectID, sessionID, label, kind, path string, nextSeq int) error {
	if kind == "" {
		kind = "claude"
	}
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO sessions (id, project_id, label, kind, path, position)
			 VALUES (?, ?, ?, ?, ?,
			         (SELECT COALESCE(MAX(position), -1) + 1 FROM sessions WHERE project_id = ?))`,
			sessionID, projectID, label, kind, path, projectID,
		); err != nil {
			return fmt.Errorf("insert session: %w", err)
		}
		if _, err := tx.Exec(
			`UPDATE projects SET active_session_id = ?, next_seq = ? WHERE id = ?`,
			sessionID, nextSeq, projectID,
		); err != nil {
			return fmt.Errorf("update project counters: %w", err)
		}
		return nil
	})
}

// DeleteSession removes a session for good and sets the project's active session
// to activeID (the neighbor the frontend picked, or "" when none remain).
func (s *Service) DeleteSession(projectID, sessionID, activeID string) error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
		if _, err := tx.Exec(
			`UPDATE projects SET active_session_id = ? WHERE id = ?`,
			activeID, projectID,
		); err != nil {
			return fmt.Errorf("update active session: %w", err)
		}
		return nil
	})
}

// RenameSession updates a session's display label from an explicit user rename.
// It clears label_auto so the automatic ai-title (SetSessionTitle) never stomps
// a name the user chose.
func (s *Service) RenameSession(sessionID, label string) error {
	if _, err := s.db.Exec(
		`UPDATE sessions SET label = ?, label_auto = 0 WHERE id = ?`,
		label, sessionID,
	); err != nil {
		return fmt.Errorf("rename session: %w", err)
	}
	return nil
}

// SetSessionTitle sets a session's label from the Claude Code ai-title reported
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
		return false, fmt.Errorf("set session title: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("set session title rows: %w", err)
	}
	return n > 0, nil
}

// SetClaudeSession records the Claude Code session id running inside a lich
// session's PTY, reported by the SessionStart hook. A session whose row does not
// exist yet (the hook racing session persistence) matches nothing and is not an
// error — the id is simply dropped, which is acceptable for the features it
// backs. Re-reporting (e.g. after a resume) overwrites with the latest id.
func (s *Service) SetClaudeSession(sessionID, claudeSessionID string) error {
	if _, err := s.db.Exec(
		`UPDATE sessions SET claude_session_id = ? WHERE id = ?`,
		claudeSessionID, sessionID,
	); err != nil {
		return fmt.Errorf("set claude session: %w", err)
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
		return fmt.Errorf("set active session: %w", err)
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
