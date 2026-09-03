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

// SaveCostLedger records the position and total of one transcript's accounting,
// stamped with the moment it was written — the only date a session's spend ever
// carries, and what CostTotals windows on.
// The SELECT ... WHERE EXISTS writes nothing for a session whose row is already
// gone (closed while its last turn was still being counted) instead of letting
// the foreign key raise: that is a race, not a failure, and it is the same
// silent miss SetProviderSession takes.
func (s *Service) SaveCostLedger(
	sessionID, transcriptID string, offset int64, lastMessage string, cost float64,
) error {
	_, err := s.db.Exec(
		`INSERT INTO session_costs
		     (session_id, transcript_id, byte_offset, last_message_id, cost_usd, updated_at)
		 SELECT ?, ?, ?, ?, ?, CAST(strftime('%s', 'now') AS INTEGER)
		 WHERE EXISTS (SELECT 1 FROM sessions WHERE id = ?)
		 ON CONFLICT(session_id, transcript_id) DO UPDATE SET
		     byte_offset = excluded.byte_offset,
		     last_message_id = excluded.last_message_id,
		     cost_usd = excluded.cost_usd,
		     updated_at = excluded.updated_at`,
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
	// When the row was last counted. Carried rather than re-stamped: a resume
	// moves old spend onto a new session id, and re-dating it would move that
	// money into today's window.
	updatedAt int64
}

// costLedgers reads every ledger a session holds.
func costLedgers(tx *sql.Tx, sessionID string) ([]costRow, error) {
	rows, err := tx.Query(
		`SELECT transcript_id, byte_offset, last_message_id, cost_usd, updated_at
		   FROM session_costs WHERE session_id = ?`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("read cost ledgers for %q: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()

	var ledgers []costRow
	for rows.Next() {
		var row costRow
		if err := rows.Scan(
			&row.transcriptID, &row.offset, &row.lastMessage, &row.cost, &row.updatedAt,
		); err != nil {
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
			`INSERT INTO session_costs
			     (session_id, transcript_id, byte_offset, last_message_id, cost_usd, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			sessionID, row.transcriptID, row.offset, row.lastMessage, row.cost, row.updatedAt,
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

// CostTotal is what one project's sessions have cost. Sessions is every session
// in the unit; Unpriced is how many of them carry no price at all, so the money
// covers Sessions-Unpriced of them.
type CostTotal struct {
	Project  string  `json:"project"`
	Sessions int     `json:"sessions"`
	Unpriced int     `json:"unpriced"`
	CostUSD  float64 `json:"costUsd"`
}

// CostReport is CostTotals' answer: a row per project, and the same three
// numbers summed across every row.
//
// The money is a **lower bound** whenever Unpriced is not zero. A session lich
// never priced holds no ledger row at all — its provider files its conversation
// where lich has no reader, a model in it has no price, or the readout was off
// while it ran — and nothing here can guess what it spent. Shipping that count
// beside the sum
// is the whole difference between "this is what it cost" and "this is what lich
// could add up", and a total that omitted it in silence would be the one
// failure mode that makes the number not worth reading.
//
// Why the row is missing is not in here on purpose: that is costMiss
// (internal/terminal/usage_cost.go), which is decided by the live scan and
// never persisted, so the ledger can only say that a session has no price —
// not which of the reasons it was.
//
// Readout is whether the cost readout is on at all. Off, no transcript is ever
// summed, so every session is unpriced and the total can only be zero — which a
// caller has to be able to say instead of printing a $0.00 that reads as free.
type CostReport struct {
	Projects []CostTotal `json:"projects"`
	Sessions int         `json:"sessions"`
	Unpriced int         `json:"unpriced"`
	CostUSD  float64     `json:"costUsd"`
	Readout  bool        `json:"readout"`
}

// CostTotals sums the ledger by project. project narrows it to one by name,
// case-insensitively; provider to one session kind; either empty means all of
// them. since, in unix seconds, keeps only the sessions active on or after it,
// and 0 keeps every session lich still remembers.
//
// A session's activity is its last counted turn, because that is the only date
// the ledger carries — the row is a running total with no per-turn history
// behind it. So a window selects *sessions active in it*, counted whole, rather
// than slicing each session's money by day: a session that ran through the
// boundary brings all of its cost with it. A session with nothing counted is
// dated by when it was parked, or by now while it is open, so an unpriced
// session that is still running is unpriced in every window ending today.
//
// Only sessions lich still holds a row for are in reach at all: a deleted
// session took its ledger with it (ON DELETE CASCADE) and is in no total,
// neither counted nor excluded.
func (s *Service) CostTotals(project, provider string, since int64) (CostReport, error) {
	// Read before the rows are open: the store holds a single connection, and a
	// second query while one is being walked waits on a connection its own
	// caller is holding.
	report := CostReport{Projects: []CostTotal{}, Readout: s.CostReadout()}
	rows, err := s.db.Query(
		`SELECT p.name,
		        COUNT(*),
		        SUM(CASE WHEN c.cost IS NULL THEN 1 ELSE 0 END),
		        COALESCE(SUM(c.cost), 0)
		   FROM sessions s
		   JOIN projects p ON p.id = s.project_id
		   LEFT JOIN (SELECT session_id, SUM(cost_usd) AS cost, MAX(updated_at) AS seen
		                FROM session_costs GROUP BY session_id) c ON c.session_id = s.id
		  WHERE (? = '' OR LOWER(p.name) = LOWER(?))
		    AND (? = '' OR s.kind = ?)
		    AND (? = 0 OR COALESCE(
		                    NULLIF(c.seen, 0),
		                    CASE WHEN s.is_open = 1
		                         THEN CAST(strftime('%s', 'now') AS INTEGER)
		                         ELSE s.closed_at END) >= ?)
		  GROUP BY p.id, p.name
		  ORDER BY COALESCE(SUM(c.cost), 0) DESC, p.name`,
		project, project, provider, provider, since, since,
	)
	if err != nil {
		return CostReport{}, fmt.Errorf("read cost totals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var row CostTotal
		if err := rows.Scan(&row.Project, &row.Sessions, &row.Unpriced, &row.CostUSD); err != nil {
			return CostReport{}, fmt.Errorf("scan cost total: %w", err)
		}
		report.Projects = append(report.Projects, row)
		report.Sessions += row.Sessions
		report.Unpriced += row.Unpriced
		report.CostUSD += row.CostUSD
	}
	if err := rows.Err(); err != nil {
		return CostReport{}, fmt.Errorf("iterate cost totals: %w", err)
	}
	// A name that matched nothing is refused rather than answered with a zero:
	// this is money, and a typo that reads as "that project cost nothing" is
	// worse than no answer at all.
	if project != "" && len(report.Projects) == 0 {
		return CostReport{}, fmt.Errorf("no sessions in a project named %q", project)
	}
	return report, nil
}
