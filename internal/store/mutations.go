package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/providers"
)

// nextSessionPosition is the position a newly inserted session takes: after the
// project's last one, so a new card appends to the list even once the user has
// dragged the others around. A subquery rather than a read-then-write, so the
// order can never be decided against a stale count.
const nextSessionPosition = `(SELECT COALESCE(MAX(position), -1) + 1 FROM sessions WHERE project_id = ?)`

// setActiveSessionSQL is shared by the transactional helper below and the
// standalone SetActiveSession, so the two can never drift apart.
const setActiveSessionSQL = `UPDATE projects SET active_session_id = ? WHERE id = ?`

// setActiveSession points a project at the session that should hold focus. Every
// mutation that adds, parks or removes a session ends by calling it inside the
// same transaction: the active id and the session set have to move together, or
// a reload restores a project focused on a session that is no longer there.
func setActiveSession(tx *sql.Tx, projectID, sessionID string) error {
	if _, err := tx.Exec(setActiveSessionSQL, sessionID, projectID); err != nil {
		return fmt.Errorf("set active session of %q: %w", projectID, err)
	}
	return nil
}

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
// can be reopened later with its session state restored. closed_seq is what
// orders the reopen menu; a counter rather than a timestamp, so two closes in
// the same second still order.
func (s *Service) CloseProject(id string) error {
	_, err := s.db.Exec(
		`UPDATE projects
		    SET is_open = 0,
		        closed_seq = (SELECT COALESCE(MAX(closed_seq), 0) + 1 FROM projects)
		  WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("close project %q: %w", id, err)
	}
	return nil
}

// DeleteProject removes a project and, through ON DELETE CASCADE, its sessions.
// Closing keeps a project for a later reopen; this is for the row that outlived
// the directory it points at, which nothing can reopen anymore.
//
// Its settings go by hand, because the settings table carries no foreign key and
// a project id is derived from its path: a directory recreated where a deleted
// one stood would otherwise inherit that project's provider, gh account and
// binary overrides with nothing in the UI saying where they came from. An empty
// id is refused for the same reason — it is the global scope, and deleting a
// project must never take every global setting with it.
//
// Both deletes are one transaction: the settings are only orphaned once the
// project they belong to is gone, so a failure between them would leave the row
// standing with its provider, gh account and binary overrides silently wiped.
func (s *Service) DeleteProject(id string) error {
	if id == globalScope {
		return fmt.Errorf("delete project: empty id")
	}
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM settings WHERE project_id = ?`, id); err != nil {
			return fmt.Errorf("delete project settings %q: %w", id, err)
		}
		if _, err := tx.Exec(`DELETE FROM projects WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete project %q: %w", id, err)
		}
		return nil
	})
}

// AddSession inserts a session, makes it the project's active one and records the
// project's next label counter — all atomically, mirroring the frontend reducer.
// Kind selects what the session's PTY runs (a provider id or "shell"); empty
// defaults to "claude" so older callers keep the original behavior. Path is the session's
// working directory when it lives in a git worktree; empty means the project's.
// The session takes the position after the project's last one, so it appends to
// the card list even once the user has dragged the others around.
func (s *Service) AddSession(projectID, sessionID, label, kind, path string, nextSeq int) error {
	return s.AddSessionFrom(projectID, sessionID, label, kind, path, nextSeq, "", "")
}

// AddSessionFrom is AddSession for a session opened by delegation: originID is
// the session that asked for it and originLabel what that session was called at
// the time. Both are written with the row rather than after it, so a card can
// never exist without the origin it was created with.
//
// It is a second entry point rather than two more arguments on AddSession
// because the window — the only caller reaching this over RPC — never has an
// origin to pass. Only internal/spawn does.
func (s *Service) AddSessionFrom(
	projectID, sessionID, label, kind, path string, nextSeq int, originID, originLabel string,
) error {
	if kind == "" {
		kind = providers.Claude
	}
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO sessions (id, project_id, label, kind, path, origin_session_id, origin_label, position)
			 VALUES (?, ?, ?, ?, ?, ?, ?, `+nextSessionPosition+`)`,
			sessionID, projectID, label, kind, path, originID, originLabel, projectID,
		); err != nil {
			return fmt.Errorf("insert session %q: %w", sessionID, err)
		}
		// Not setActiveSession: the label counter moves with the insert too, and
		// one statement is what keeps the pair atomic.
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
		return setActiveSession(tx, projectID, activeID)
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
		return setActiveSession(tx, projectID, activeID)
	})
}

// ReopenWorktreeSession resumes a parked worktree session. It finds the parked
// (is_open = 0) session for the worktree at path and re-adds it to the workspace
// under a fresh id (newSessionID), carrying over the old label, kind, provider
// session id, label_auto flag, model, entrypoint and origin. The fresh id is deliberate: it makes the frontend treat the card
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
		//
		// model and entrypoint ride along for the same reason, one rung lower:
		// both are spawn overrides their own doc comments promise survive every
		// later spawn of the session, and a reinsert that dropped them would put
		// the provider back on its default model and the terminal back on a bare
		// shell — silently, on the one path where the card keeps its identity but
		// not its id.
		var labelAuto int
		var model, entrypoint string
		row := tx.QueryRow(
			`SELECT id, label, kind, provider_session_id, label_auto, model, entrypoint,
			        origin_session_id, origin_label
			   FROM sessions
			  WHERE project_id = ? AND path = ? AND is_open = 0
			  ORDER BY rowid DESC LIMIT 1`,
			projectID, path,
		)
		if err := row.Scan(
			&old.ID, &old.Label, &old.Kind, &old.ProviderSessionID, &labelAuto, &model, &entrypoint,
			&old.OriginSessionID, &old.OriginLabel,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil // nothing parked here; caller creates a new session
			}
			return fmt.Errorf("find parked session for %q: %w", path, err)
		}
		// The cost ledgers are read out before the row goes, because deleting it
		// cascades them away — and a resumed session that forgot what it spent
		// would restate its total from zero over the same conversation.
		ledgers, err := costLedgers(tx, old.ID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, old.ID); err != nil {
			return fmt.Errorf("drop parked session %q: %w", old.ID, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO sessions
			   (id, project_id, label, kind, path, provider_session_id, label_auto,
			    model, entrypoint, origin_session_id, origin_label, position)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, `+nextSessionPosition+`)`,
			newSessionID, projectID, old.Label, old.Kind, path, old.ProviderSessionID, labelAuto,
			model, entrypoint, old.OriginSessionID, old.OriginLabel, projectID,
		); err != nil {
			return fmt.Errorf("reinsert session %q: %w", newSessionID, err)
		}
		if err := restoreCostLedgers(tx, newSessionID, ledgers); err != nil {
			return err
		}
		if err := setActiveSession(tx, projectID, newSessionID); err != nil {
			return err
		}
		restored = &Session{
			ID:                newSessionID,
			Label:             old.Label,
			Kind:              old.Kind,
			Path:              path,
			ProviderSessionID: old.ProviderSessionID,
			Entrypoint:        entrypoint,
			OriginSessionID:   old.OriginSessionID,
			OriginLabel:       old.OriginLabel,
		}
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

// SetSessionPinned pins or unpins a session — the flag the sidebar reads to
// hoist a card to the head of the list, and to withhold its close affordances.
// It leaves position untouched, so an unpinned session falls back to the slot
// the last drag gave it rather than landing wherever the pinned block ended.
func (s *Service) SetSessionPinned(sessionID string, pinned bool) error {
	if _, err := s.db.Exec(
		`UPDATE sessions SET pinned = ? WHERE id = ?`, pinned, sessionID,
	); err != nil {
		return fmt.Errorf("set session %q pinned: %w", sessionID, err)
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

// SetSessionModel records the model a session's provider was asked to run, so
// every later spawn of that session repeats the flag: a reload, a respawn, and
// the resume of a parked worktree session all go through the same path, and a
// model that only survived the first spawn would silently become the provider's
// default on the second.
//
// It is written once, right after the row is created. A session whose row is
// gone matches nothing and is not an error.
func (s *Service) SetSessionModel(sessionID, model string) error {
	if _, err := s.db.Exec(
		`UPDATE sessions SET model = ? WHERE id = ?`, model, sessionID,
	); err != nil {
		return fmt.Errorf("set model on %q: %w", sessionID, err)
	}
	return nil
}

// SessionModel returns the model recorded for a session, or "" for none — which
// is what a session opened from the window has, and what leaves the provider on
// its own default. A read failure answers "" for the same reason: the model is
// an override, and the provider's default is the safe thing to fall back to.
func (s *Service) SessionModel(sessionID string) string {
	var model sql.NullString
	if err := s.db.QueryRow(
		`SELECT model FROM sessions WHERE id = ?`, sessionID,
	).Scan(&model); err != nil {
		return ""
	}
	return model.String
}

// SetSessionEntrypoint records the command a terminal session opens into, so
// every later spawn of that session runs it again — a reload, a respawn, and the
// resume of a parked worktree session all go through the same path. An empty
// command clears it and leaves a plain shell.
//
// The kind clause is the guard that keeps this out of every other session: lich
// has one project-wide way to put something in front of a PTY already
// (.lich/setup-worktree.sh), and this is deliberately the opposite — one row,
// one shell. Written in SQL rather than checked in Go because that is the only
// spelling no future caller can skip: an RPC or an MCP tool that aims this at a
// provider row changes nothing instead of parking a setting nothing reads.
// 'shell' is terminal.KindShell, which SQL cannot interpolate — the same
// coupling the schema's 'claude' default already carries.
//
// A session whose row is gone, or whose kind is not shell, matches nothing and
// is not an error.
func (s *Service) SetSessionEntrypoint(sessionID, entrypoint string) error {
	if _, err := s.db.Exec(
		`UPDATE sessions SET entrypoint = ? WHERE id = ? AND kind = 'shell'`,
		strings.TrimSpace(entrypoint), sessionID,
	); err != nil {
		return fmt.Errorf("set entrypoint on %q: %w", sessionID, err)
	}
	return nil
}

// SessionEntrypoint returns the command recorded for a session, or "" for none —
// which is what every provider session has, and what leaves a terminal on a
// plain shell. A read failure answers "" for the same reason SessionModel does:
// the entrypoint is an override, and the bare shell is the safe fallback.
func (s *Service) SessionEntrypoint(sessionID string) string {
	var entrypoint sql.NullString
	if err := s.db.QueryRow(
		`SELECT entrypoint FROM sessions WHERE id = ?`, sessionID,
	).Scan(&entrypoint); err != nil {
		return ""
	}
	return entrypoint.String
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

// ProviderSession returns the provider conversation id recorded for a lich
// session, or "" when none has been reported yet (or the session is gone — a
// missing row is not an error). Pairs with SetProviderSession: the context-usage
// read locates a transcript by this id.
func (s *Service) ProviderSession(sessionID string) (string, error) {
	var id sql.NullString
	err := s.db.QueryRow(
		`SELECT provider_session_id FROM sessions WHERE id = ?`, sessionID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read provider session on %q: %w", sessionID, err)
	}
	return id.String, nil
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
	if _, err := s.db.Exec(setActiveSessionSQL, sessionID, projectID); err != nil {
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
