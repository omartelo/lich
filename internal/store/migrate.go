package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// migrations take a database from one schema version to the next: a database
// whose PRAGMA user_version is n has had migrations[:n] applied, so the number of
// entries is the version this binary writes. Append only. The index is all a
// database on disk remembers of a migration, so an entry that shipped is never
// edited, reordered or removed, and `schema` stays the version-1 shape: a column
// added later is an ALTER here, not a line there.
var migrations = []func(*sql.Tx) error{
	legacyMigrations,
}

// NewerSchemaError is an open refused because the database was written by a
// newer lich than this one. Opening it anyway would let this binary write its
// defaults over columns it does not know about, so it is not opened at all.
type NewerSchemaError struct {
	Found int
	Known int
}

func (e *NewerSchemaError) Error() string {
	return fmt.Sprintf(
		"the workspace database is at schema version %d, but this lich only knows up to version %d. "+
			"It was last opened by a newer lich: install that version (or a later one) to open it",
		e.Found, e.Known)
}

// migrate brings db up to len(migrations), one transaction per step, each step
// stamping the version it reached. A crash between steps leaves a database at an
// earlier version that the next launch carries on from.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > len(migrations) {
		return &NewerSchemaError{Found: version, Known: len(migrations)}
	}
	for ; version < len(migrations); version++ {
		if err := migrateStep(db, version); err != nil {
			return fmt.Errorf("migrate schema to version %d: %w", version+1, err)
		}
	}
	return nil
}

func migrateStep(db *sql.DB, from int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := migrations[from](tx); err != nil {
		return err
	}
	// PRAGMA takes no bound parameters; the value is an int this code computed.
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, from+1)); err != nil {
		return err
	}
	return tx.Commit()
}

// legacyMigrations is version 1: every database before user_version existed,
// and every new one. It creates what is missing and retries the ALTERs lich
// used to run on each launch, which is why it has to tolerate the errors an
// already-applied ALTER raises. SQLite has no ADD COLUMN IF NOT EXISTS and no
// RENAME COLUMN IF EXISTS; migrations after this one run exactly once and need
// no such matching.
//
// The rename/add pair covers all three shapes a database can be in: created
// fresh with provider_session_id (rename finds nothing, add is a duplicate),
// created with the old claude_session_id (rename carries the ids over, add is
// a duplicate), or predating the column entirely (rename finds nothing, add
// creates it).
func legacyMigrations(tx *sql.Tx) error {
	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	for _, stmt := range legacyAlters {
		if _, err := tx.Exec(stmt); err != nil && !migrationApplied(err) {
			return err
		}
	}
	// The terminal's own theme selection, from before one theme coloured both
	// surfaces. Nothing reads it, and a row left standing would hand whoever
	// gives the terminal a theme again a choice its user made under other rules.
	_, err := tx.Exec(`DELETE FROM settings WHERE key = 'appearance.terminalTheme'`)
	return err
}

var legacyAlters = []string{
	// 'claude' is providers.Claude spelled out: see the schema.
	`ALTER TABLE sessions ADD COLUMN kind TEXT NOT NULL DEFAULT 'claude'`,
	`ALTER TABLE sessions ADD COLUMN path TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions RENAME COLUMN claude_session_id TO provider_session_id`,
	`ALTER TABLE sessions ADD COLUMN provider_session_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN label_auto INTEGER NOT NULL DEFAULT 1`,
	`ALTER TABLE sessions ADD COLUMN is_open INTEGER NOT NULL DEFAULT 1`,
	`ALTER TABLE sessions ADD COLUMN position INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE sessions ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE sessions ADD COLUMN model TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN entrypoint TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN origin_session_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN origin_label TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN sandbox TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN closed_at INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE sessions ADD COLUMN scheduled_at INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE sessions ADD COLUMN scheduled_prompt TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN unread INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE sessions ADD COLUMN mcp_servers TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN parked_branch TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN fork_cost_offset REAL NOT NULL DEFAULT 0`,
	`ALTER TABLE sessions ADD COLUMN sandbox_links TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sessions ADD COLUMN run INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE projects ADD COLUMN position INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE projects ADD COLUMN closed_seq INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE session_costs ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE session_texts ADD COLUMN truncated INTEGER NOT NULL DEFAULT 0`,
}

// migrationApplied reports whether an ALTER TABLE failed because the migration
// had already been applied: the column exists, or the one to rename is gone.
func migrationApplied(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "no such column")
}
