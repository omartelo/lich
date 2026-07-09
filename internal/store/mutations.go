package store

import (
	"database/sql"
	"fmt"
)

// AddProject persists a newly opened project and marks it open. Reopening a
// previously closed project keeps its stored sessions, name and path intact —
// only is_open flips back to 1.
func (s *Service) AddProject(id, name, path string) error {
	_, err := s.db.Exec(
		`INSERT INTO projects (id, name, path, is_open) VALUES (?, ?, ?, 1)
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
func (s *Service) AddSession(projectID, sessionID, label string, nextSeq int) error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO sessions (id, project_id, label) VALUES (?, ?, ?)`,
			sessionID, projectID, label,
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

// RenameSession updates a session's display label.
func (s *Service) RenameSession(sessionID, label string) error {
	if _, err := s.db.Exec(`UPDATE sessions SET label = ? WHERE id = ?`, label, sessionID); err != nil {
		return fmt.Errorf("rename session: %w", err)
	}
	return nil
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
