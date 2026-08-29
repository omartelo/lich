package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// AddHandsOn folds another stretch of hands-on time into a session's total, in
// whole seconds. The accumulator that measures it holds no absolute figure of
// its own (internal/terminal.handsOn), so this adds rather than sets: a lich
// that restarts resumes counting onto what the last one wrote instead of
// starting the session over at zero.
//
// The SELECT ... WHERE EXISTS writes nothing for a session whose row is already
// gone — closed while its last stretch was still being written — for the same
// reason SaveCostLedger does it: that is a race, not a failure.
func (s *Service) AddHandsOn(sessionID string, seconds int64) error {
	if seconds <= 0 {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO session_hands_on (session_id, seconds)
		 SELECT ?, ? WHERE EXISTS (SELECT 1 FROM sessions WHERE id = ?)
		 ON CONFLICT(session_id) DO UPDATE SET seconds = seconds + excluded.seconds`,
		sessionID, seconds, sessionID,
	)
	if err != nil {
		return fmt.Errorf("add hands-on time for %q: %w", sessionID, err)
	}
	return nil
}

// HandsOn is how long a session has been worked on, in whole seconds. A session
// nothing has been counted for yet answers 0, which is the right starting point,
// so a missing row is not an error.
func (s *Service) HandsOn(sessionID string) (int64, error) {
	var seconds int64
	err := s.db.QueryRow(
		`SELECT seconds FROM session_hands_on WHERE session_id = ?`, sessionID,
	).Scan(&seconds)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read hands-on time for %q: %w", sessionID, err)
	}
	return seconds, nil
}

// handsOnOf reads a session's total inside a transaction, for the delete and
// reinsert a parked session's resume performs (see reopen).
func handsOnOf(tx *sql.Tx, sessionID string) (int64, error) {
	var seconds int64
	err := tx.QueryRow(
		`SELECT seconds FROM session_hands_on WHERE session_id = ?`, sessionID,
	).Scan(&seconds)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read hands-on time for %q: %w", sessionID, err)
	}
	return seconds, nil
}

// restoreHandsOn re-keys a total onto a session id.
func restoreHandsOn(tx *sql.Tx, sessionID string, seconds int64) error {
	if seconds <= 0 {
		return nil
	}
	if _, err := tx.Exec(
		`INSERT INTO session_hands_on (session_id, seconds) VALUES (?, ?)`,
		sessionID, seconds,
	); err != nil {
		return fmt.Errorf("restore hands-on time for %q: %w", sessionID, err)
	}
	return nil
}
