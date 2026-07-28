package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// CostLedger returns how far a session's cost accounting has read into one of
// its provider transcripts: the byte offset counted up to, the message id the
// last counted line belonged to, and what that stretch cost in USD.
//
// A session accumulates one ledger per transcript it has run through — `/clear`
// starts a new conversation, and its transcript gets its own row while the old
// one keeps what it already counted. A pair nothing has been counted for yet
// reads as offset 0 and cost 0, the correct starting point, so a missing row is
// not an error.
func (s *Service) CostLedger(sessionID, transcriptID string) (int64, string, float64, error) {
	var (
		offset      int64
		lastMessage string
		cost        float64
	)
	err := s.db.QueryRow(
		`SELECT byte_offset, last_message_id, cost_usd FROM session_costs
		 WHERE session_id = ? AND transcript_id = ?`,
		sessionID, transcriptID,
	).Scan(&offset, &lastMessage, &cost)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", 0, nil
	}
	if err != nil {
		return 0, "", 0, fmt.Errorf("read cost ledger for %q: %w", sessionID, err)
	}
	return offset, lastMessage, cost, nil
}

// SaveCostLedger records the position and total of one transcript's accounting.
// The SELECT ... WHERE EXISTS writes nothing for a session whose row is already
// gone (closed while its last turn was still being counted) instead of letting
// the foreign key raise: that is a race, not a failure, and it is the same
// silent miss SetProviderSession takes.
func (s *Service) SaveCostLedger(
	sessionID, transcriptID string, offset int64, lastMessage string, cost float64,
) error {
	_, err := s.db.Exec(
		`INSERT INTO session_costs (session_id, transcript_id, byte_offset, last_message_id, cost_usd)
		 SELECT ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM sessions WHERE id = ?)
		 ON CONFLICT(session_id, transcript_id) DO UPDATE SET
		     byte_offset = excluded.byte_offset,
		     last_message_id = excluded.last_message_id,
		     cost_usd = excluded.cost_usd`,
		sessionID, transcriptID, offset, lastMessage, cost, sessionID,
	)
	if err != nil {
		return fmt.Errorf("save cost ledger for %q: %w", sessionID, err)
	}
	return nil
}

// costRow is one session_costs row, carried across the delete/reinsert a
// parked session's resume performs (see ReopenWorktreeSession).
type costRow struct {
	transcriptID string
	offset       int64
	lastMessage  string
	cost         float64
}

// costLedgers reads every ledger a session holds.
func costLedgers(tx *sql.Tx, sessionID string) ([]costRow, error) {
	rows, err := tx.Query(
		`SELECT transcript_id, byte_offset, last_message_id, cost_usd
		   FROM session_costs WHERE session_id = ?`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("read cost ledgers for %q: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()

	var ledgers []costRow
	for rows.Next() {
		var row costRow
		if err := rows.Scan(&row.transcriptID, &row.offset, &row.lastMessage, &row.cost); err != nil {
			return nil, fmt.Errorf("scan cost ledger for %q: %w", sessionID, err)
		}
		ledgers = append(ledgers, row)
	}
	return ledgers, rows.Err()
}

// restoreCostLedgers re-keys ledgers onto a session id.
func restoreCostLedgers(tx *sql.Tx, sessionID string, ledgers []costRow) error {
	for _, row := range ledgers {
		if _, err := tx.Exec(
			`INSERT INTO session_costs (session_id, transcript_id, byte_offset, last_message_id, cost_usd)
			 VALUES (?, ?, ?, ?, ?)`,
			sessionID, row.transcriptID, row.offset, row.lastMessage, row.cost,
		); err != nil {
			return fmt.Errorf("restore cost ledger for %q: %w", sessionID, err)
		}
	}
	return nil
}

// SessionCost is what a session has cost so far: the sum of every transcript it
// has run through. Zero for a session that has counted nothing yet — whether
// that zero is worth showing is the caller's call, since an unpriced model must
// read as "no number", never as free.
func (s *Service) SessionCost(sessionID string) (float64, error) {
	var cost sql.NullFloat64
	err := s.db.QueryRow(
		`SELECT SUM(cost_usd) FROM session_costs WHERE session_id = ?`, sessionID,
	).Scan(&cost)
	if err != nil {
		return 0, fmt.Errorf("read session cost for %q: %w", sessionID, err)
	}
	return cost.Float64, nil
}
