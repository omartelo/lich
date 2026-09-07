package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// TurnRecord is one session's last finished turn: the two snapshot trees the
// turn ran between and when its window shut, in unix milliseconds — the unit
// the panel reads (internal/terminal.LastTurn).
//
// The trees are loose git objects in the session's own checkout, written by
// `git write-tree` against an index of lich's own and reachable from no ref. The
// row outlives the process that took them; it does not outlive a prune of the
// objects they name.
type TurnRecord struct {
	Before  string
	After   string
	EndedAt int64
}

// SaveTurnRecord files a session's last finished turn, replacing whatever was
// there. A record missing either tree deletes the row instead: a turn that lost
// a snapshot has no record, and leaving the previous one on disk would let a
// later launch answer with a turn this lich had already retracted.
//
// The SELECT ... WHERE EXISTS writes nothing for a session whose row is already
// gone — closed while its closing snapshot was still on the worker — for the
// same reason AddHandsOn does it: that is a race, not a failure.
func (s *Service) SaveTurnRecord(sessionID string, rec TurnRecord) error {
	if rec.Before == "" || rec.After == "" {
		if _, err := s.db.Exec(
			`DELETE FROM session_last_turn WHERE session_id = ?`, sessionID,
		); err != nil {
			return fmt.Errorf("clear last turn for %q: %w", sessionID, err)
		}
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO session_last_turn (session_id, before_tree, after_tree, ended_at)
		 SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM sessions WHERE id = ?)
		 ON CONFLICT(session_id) DO UPDATE SET
		     before_tree = excluded.before_tree,
		     after_tree  = excluded.after_tree,
		     ended_at    = excluded.ended_at`,
		sessionID, rec.Before, rec.After, rec.EndedAt, sessionID,
	)
	if err != nil {
		return fmt.Errorf("save last turn for %q: %w", sessionID, err)
	}
	return nil
}

// TurnRecord reads back a session's last finished turn. A session with no turn
// on record answers false rather than an error: nothing has finished there yet,
// which is an answer the panel already knows how to draw.
func (s *Service) TurnRecord(sessionID string) (TurnRecord, bool, error) {
	var rec TurnRecord
	err := s.db.QueryRow(
		`SELECT before_tree, after_tree, ended_at FROM session_last_turn WHERE session_id = ?`,
		sessionID,
	).Scan(&rec.Before, &rec.After, &rec.EndedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TurnRecord{}, false, nil
	}
	if err != nil {
		return TurnRecord{}, false, fmt.Errorf("read last turn for %q: %w", sessionID, err)
	}
	return rec, true, nil
}
