package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/providers"
)

// now is time.Now, replaced in tests: a close stamps the row with it, and a
// test about the order that stamp produces cannot wait out a real second.
var now = time.Now

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
// defaults to "claude", which is the schema's own default and the answer for a
// caller that reached this over RPC without one. Path is the session's working
// directory when it lives in a git worktree; empty means the project's.
// The session takes the position after the project's last one, so it appends to
// the card list even once the user has dragged the others around.
// sandbox is whether this session runs confined ("on"/"off", empty to follow the
// provider's rung). It is written with the row rather than after it because the
// PTY reads it on the very first spawn: a second call would race the card the
// insert is about to put on screen, and lose that race silently — the session
// would open unconfined and stay that way until something respawned it.
func (s *Service) AddSession(
	projectID, sessionID, label, kind, path string, nextSeq int, sandbox string,
) error {
	return s.addSession(projectID, sessionID, label, kind, path, nextSeq, "", "", sandbox)
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
	// No sandbox answer: a delegated session is opened by another session, which
	// has nobody to ask, so it follows the provider's rung like every other
	// caller that cannot put the question on screen.
	return s.addSession(projectID, sessionID, label, kind, path, nextSeq, originID, originLabel, "")
}

// addSession is the one insert behind both entry points.
func (s *Service) addSession(
	projectID, sessionID, label, kind, path string, nextSeq int, originID, originLabel, sandbox string,
) error {
	if kind == "" {
		kind = providers.Claude
	}
	if sandbox != SessionConfined && sandbox != SessionUnconfined {
		sandbox = ""
	}
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO sessions (id, project_id, label, kind, path, origin_session_id, origin_label, sandbox, position)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, `+nextSessionPosition+`)`,
			sessionID, projectID, label, kind, path, originID, originLabel, sandbox, projectID,
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
	if err := s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
			return fmt.Errorf("delete session %q: %w", sessionID, err)
		}
		return setActiveSession(tx, projectID, activeID)
	}); err != nil {
		return err
	}
	s.sessionIsGone(sessionID)
	return nil
}

// CloseSession parks a session instead of deleting it: is_open flips to 0, which
// hides it from LoadState while keeping its row — and its provider session id —
// intact for a later resume. The project's active session moves to activeID (the
// neighbor the frontend picked).
//
// Every close lands here. DeleteSession is what the checkout's own removal uses,
// where the row must not outlive the directory it points at.
//
// closed_at is stamped in the same statement rather than after it: it is what
// orders the history the parked row exists to appear in, and a row parked
// without one sorts as if it had been closed before everything else.
func (s *Service) CloseSession(projectID, sessionID, activeID string) error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`UPDATE sessions SET is_open = 0, closed_at = ? WHERE id = ?`,
			now().Unix(), sessionID,
		); err != nil {
			return fmt.Errorf("close session %q: %w", sessionID, err)
		}
		return setActiveSession(tx, projectID, activeID)
	})
}

// ForgetSession deletes one parked session for good. It is the way out for the
// row whose checkout was removed behind lich's back: nothing can resume it, and
// PurgeWorktreeSessions never ran for it because the removal never went through
// the app.
//
// is_open = 0 is the whole guard: a session on screen is closed, never
// forgotten, so an id belonging to a live card matches nothing here. A row that
// was already gone is not an error, and only a delete that actually removed one
// reports the session gone.
func (s *Service) ForgetSession(sessionID string) error {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE id = ? AND is_open = 0`, sessionID)
	if err != nil {
		return fmt.Errorf("forget session %q: %w", sessionID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("forget session %q rows: %w", sessionID, err)
	}
	if n > 0 {
		s.sessionIsGone(sessionID)
	}
	return nil
}

// ReopenWorktreeSession resumes the session parked for the worktree at path,
// the last one closed there. Returns nil when nothing is parked at path — the
// caller then opens a brand-new session.
//
// The empty path is refused, and that guard is load-bearing for the same reason
// PurgeWorktreeSessions carries one: a project's own sessions are stored with no
// path, so an unguarded lookup would answer the worktree picker with a parked
// session that never lived in a worktree at all.
func (s *Service) ReopenWorktreeSession(projectID, path, newSessionID string) (*Session, error) {
	if path == "" {
		return nil, nil
	}
	return s.reopen(newSessionID,
		`WHERE project_id = ? AND path = ? AND is_open = 0 ORDER BY rowid DESC LIMIT 1`,
		projectID, path)
}

// ReopenSession resumes one parked session by its own id — the history list's
// door, where the row was picked rather than looked up. Returns nil when no
// parked session has that id, which is what a row already resumed in another
// window looks like.
//
// Deliberately not scoped to a project: the row names its own, and that is the
// only answer that stays right for a session parked in a project the caller is
// not looking at — which is most of the history.
func (s *Service) ReopenSession(sessionID, newSessionID string) (*Session, error) {
	return s.reopen(newSessionID, `WHERE id = ? AND is_open = 0`, sessionID)
}

// reopen is the resume behind both doors: it finds one parked session with the
// given WHERE clause and re-adds it to the workspace under a fresh id
// (newSessionID), carrying over the old label, kind, path, provider session id,
// label_auto flag, model, entrypoint, sandbox and origin. The fresh id is
// deliberate: it makes the frontend treat the card as never-spawned, so its
// resume prompt fires and the provider conversation continues instead of
// starting cold.
func (s *Service) reopen(newSessionID, where string, args ...any) (*Session, error) {
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
		//
		// project_id is read rather than passed: reopening by id knows only the
		// session, and the row is what says where it belongs.
		var labelAuto int
		var projectID, model, entrypoint, sandbox string
		row := tx.QueryRow(
			`SELECT id, project_id, label, kind, path, provider_session_id, label_auto,
			        model, entrypoint, sandbox, origin_session_id, origin_label
			   FROM sessions `+where,
			args...,
		)
		if err := row.Scan(
			&old.ID, &projectID, &old.Label, &old.Kind, &old.Path, &old.ProviderSessionID,
			&labelAuto, &model, &entrypoint, &sandbox, &old.OriginSessionID, &old.OriginLabel,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil // nothing parked; caller creates a new session
			}
			return fmt.Errorf("find parked session: %w", err)
		}
		// The cost ledgers are read out before the row goes, because deleting it
		// cascades them away — and a resumed session that forgot what it spent
		// would restate its total from zero over the same conversation.
		ledgers, err := costLedgers(tx, old.ID)
		if err != nil {
			return err
		}
		// Hands-on time rides along for the same reason, one row over: the work
		// this session was worked through is the user's, and a resume that
		// dropped it would restart the clock on a session they have been at all
		// day.
		handsOn, err := handsOnOf(tx, old.ID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, old.ID); err != nil {
			return fmt.Errorf("drop parked session %q: %w", old.ID, err)
		}
		// closed_at is left to its default: the reinserted row is open again, and
		// a resume that carried the old stamp over would date the next close
		// before it happened.
		if _, err := tx.Exec(
			`INSERT INTO sessions
			   (id, project_id, label, kind, path, provider_session_id, label_auto,
			    model, entrypoint, sandbox, origin_session_id, origin_label, position)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, `+nextSessionPosition+`)`,
			newSessionID, projectID, old.Label, old.Kind, old.Path, old.ProviderSessionID, labelAuto,
			model, entrypoint, sandbox, old.OriginSessionID, old.OriginLabel, projectID,
		); err != nil {
			return fmt.Errorf("reinsert session %q: %w", newSessionID, err)
		}
		if err := restoreCostLedgers(tx, newSessionID, ledgers); err != nil {
			return err
		}
		if err := restoreHandsOn(tx, newSessionID, handsOn); err != nil {
			return err
		}
		if err := setActiveSession(tx, projectID, newSessionID); err != nil {
			return err
		}
		restored = &Session{
			ID:                newSessionID,
			Label:             old.Label,
			Kind:              old.Kind,
			Path:              old.Path,
			ProviderSessionID: old.ProviderSessionID,
			Entrypoint:        entrypoint,
			Sandbox:           sandbox,
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
	// Read before the delete, because what hangs off a session outside the
	// database is keyed by id and the rows are about to be gone.
	gone := s.sessionIDsAt(projectID, path)
	if _, err := s.db.Exec(
		`DELETE FROM sessions WHERE project_id = ? AND path = ?`, projectID, path,
	); err != nil {
		return fmt.Errorf("purge worktree sessions for %q: %w", path, err)
	}
	s.sessionIsGone(gone...)
	return nil
}

// sessionIDsAt lists the sessions of a project living at path — the live one
// and any parked leftovers alike. Only asked when something is listening for
// deleted sessions, so a store without that wiring runs the query it always
// ran. A failed read costs the cleanup, never the delete.
func (s *Service) sessionIDsAt(projectID, path string) []string {
	if s.sessionGone == nil {
		return nil
	}
	rows, err := s.db.Query(
		`SELECT id FROM sessions WHERE project_id = ? AND path = ?`, projectID, path,
	)
	if err != nil {
		slog.Warn("list worktree sessions", "path", path, "err", err)
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			slog.Warn("scan worktree session id", "path", path, "err", err)
			return ids
		}
		ids = append(ids, id)
	}
	return ids
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

// schedulePromptLimit bounds a scheduled prompt. It is typed into a TUI a
// character at a time when it comes due — the same delivery a relayed message
// gets, and the same reason that one is bounded (internal/relay, promptLimit):
// a megabyte of it is a hang, not a prompt. Checked here rather than at
// delivery so the person writing it is told while they can still shorten it.
const schedulePromptLimit = 8192

// SetSessionSchedule parks a prompt to be typed at a session later. at is unix
// seconds; 0, or an empty prompt, clears whatever was there — there is nothing
// else to cancel, because a session holds one scheduled prompt at a time and
// scheduling again replaces it.
//
// A session whose row is gone matches nothing and is not an error: the schedule
// belongs to the card, and a card that is gone has taken its schedule with it.
func (s *Service) SetSessionSchedule(sessionID string, at int64, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if len(prompt) > schedulePromptLimit {
		return fmt.Errorf("scheduled prompt is %d bytes, over the %d limit",
			len(prompt), schedulePromptLimit)
	}
	if at <= 0 || prompt == "" {
		at, prompt = 0, ""
	}
	if _, err := s.db.Exec(
		`UPDATE sessions SET scheduled_at = ?, scheduled_prompt = ? WHERE id = ?`,
		at, prompt, sessionID,
	); err != nil {
		return fmt.Errorf("set schedule on %q: %w", sessionID, err)
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

// Session sandbox answers, the row's own spelling. Empty is not one of them: it
// is the absence of an answer, and it means the provider's rung decides.
const (
	// SessionConfined runs this session in the sandbox whatever the rung says.
	SessionConfined = "on"
	// SessionUnconfined runs it on the machine whatever the rung says.
	SessionUnconfined = "off"
)

// SetSessionSandbox records whether one session runs confined, overriding the
// provider's rung for that session alone. It is written when a session is
// opened — the dialog's answer — and read on every later spawn, so a respawn
// after a reload and the resume of a parked worktree session are confined the
// same way the user opened them.
//
// Anything that is not one of the two answers clears the override rather than
// parking an unreadable value: a row that says neither is a row that follows
// the setting, which is the state every session starts in.
func (s *Service) SetSessionSandbox(sessionID, sandbox string) error {
	if sandbox != SessionConfined && sandbox != SessionUnconfined {
		sandbox = ""
	}
	if _, err := s.db.Exec(
		`UPDATE sessions SET sandbox = ? WHERE id = ?`, sandbox, sessionID,
	); err != nil {
		return fmt.Errorf("set sandbox on %q: %w", sessionID, err)
	}
	return nil
}

// SessionSandbox returns the answer recorded for a session, or "" when nobody
// decided for this one and the provider's rung stands. A read failure answers
// "" for SessionEntrypoint's reason: the row is an override, and deferring to
// the setting is the honest fallback.
func (s *Service) SessionSandbox(sessionID string) string {
	var sandbox sql.NullString
	if err := s.db.QueryRow(
		`SELECT sandbox FROM sessions WHERE id = ?`, sessionID,
	).Scan(&sandbox); err != nil {
		return ""
	}
	return sandbox.String
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
