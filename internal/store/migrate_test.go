package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// seedDB writes a database the way some other lich left it, without going
// through open.
func seedDB(t *testing.T, path, stmts string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(stmts); err != nil {
		t.Fatalf("seed db: %v", err)
	}
}

func userVersion(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return v
}

// The schema and the writes of the first lich that had a database (db18ee2a):
// three tables, no user_version.
const oldestWorkspace = `
CREATE TABLE projects (
    id                TEXT    PRIMARY KEY,
    name              TEXT    NOT NULL,
    path              TEXT    NOT NULL,
    is_open           INTEGER NOT NULL DEFAULT 1,
    next_seq          INTEGER NOT NULL DEFAULT 1,
    active_session_id TEXT    NOT NULL DEFAULT ''
);
CREATE TABLE sessions (
    id         TEXT NOT NULL PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    label      TEXT NOT NULL
);
CREATE INDEX idx_sessions_project ON sessions(project_id);
CREATE TABLE settings (
    key        TEXT NOT NULL,
    project_id TEXT NOT NULL DEFAULT '',
    value      TEXT NOT NULL,
    PRIMARY KEY (key, project_id)
);
INSERT INTO projects (id, name, path, is_open, next_seq, active_session_id)
     VALUES ('p1', 'alpha', '/src/alpha', 1, 3, 's2'),
            ('p2', 'beta', '/src/beta', 0, 1, '');
INSERT INTO sessions (id, project_id, label) VALUES ('s1', 'p1', 'Session 1'), ('s2', 'p1', 'Session 2');
INSERT INTO settings (key, project_id, value) VALUES ('claude.bin', '', '/opt/claude');`

func TestOpenUpgradesTheOldestWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oldest.db")
	seedDB(t, path, oldestWorkspace)

	svc, err := open(path)
	if err != nil {
		t.Fatalf("open oldest db: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	got, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("open projects = %+v, want only alpha", got)
	}
	p := got[0]
	if p.ID != "p1" || p.Name != "alpha" || p.Path != "/src/alpha" || p.NextSeq != 3 || p.ActiveSessionID != "s2" {
		t.Errorf("project = %+v", p)
	}
	if len(p.Sessions) != 2 || p.Sessions[0].Label != "Session 1" || p.Sessions[1].Label != "Session 2" {
		t.Fatalf("sessions = %+v", p.Sessions)
	}
	if s := p.Sessions[0]; s.Kind != "claude" || s.ProviderSessionID != "" {
		t.Errorf("upgraded session = %+v, want the column defaults", s)
	}
	if recent, _ := svc.RecentProjects(""); len(recent) != 1 || recent[0].ID != "p2" {
		t.Errorf("closed projects = %+v, want beta", recent)
	}
	if bin, _ := svc.GetSetting("claude.bin", ""); bin != "/opt/claude" {
		t.Errorf("claude.bin = %q, want /opt/claude", bin)
	}
	if v := userVersion(t, path); v != len(migrations) {
		t.Errorf("user_version = %d, want %d", v, len(migrations))
	}
}

func TestOpenRefusesANewerWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newer.db")
	svc, err := open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := svc.SetSetting("appearance.terminalTheme", "", "kept"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	_ = svc.Close()
	newer := len(migrations) + 1
	seedDB(t, path, `PRAGMA user_version = `+strconv.Itoa(newer))

	_, err = open(path)
	var refused *NewerSchemaError
	if !errors.As(err, &refused) {
		t.Fatalf("open newer db err = %v, want NewerSchemaError", err)
	}
	if refused.Found != newer || refused.Known != len(migrations) {
		t.Errorf("refusal = %+v", refused)
	}
	msg := err.Error()
	for _, want := range []string{"version " + strconv.Itoa(newer), "version " + strconv.Itoa(len(migrations)), "install"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal message %q does not say %q", msg, want)
		}
	}
	// Untouched: no migration ran, not even the legacy DELETE.
	if v := userVersion(t, path); v != newer {
		t.Errorf("user_version = %d, want %d", v, newer)
	}
	seedDB(t, path, `PRAGMA user_version = `+strconv.Itoa(len(migrations)))
	after, err := open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer after.Close()
	if got, _ := after.GetSetting("appearance.terminalTheme", ""); got != "kept" {
		t.Errorf("setting = %q, want the refused open to have written nothing", got)
	}
}

// A failed step leaves the database at the version before it, so the next
// launch retries that step rather than skipping it.
func TestOpenRollsBackAFailedMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failed.db")
	svc, err := open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_ = svc.Close()

	saved := migrations
	t.Cleanup(func() { migrations = saved })
	boom := errors.New("boom")
	migrations = append(append([]func(*sql.Tx) error{}, saved...), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE half_done (x INTEGER)`); err != nil {
			return err
		}
		return boom
	})

	if _, err := open(path); !errors.Is(err, boom) {
		t.Fatalf("open err = %v, want boom", err)
	}
	if v := userVersion(t, path); v != len(saved) {
		t.Errorf("user_version = %d, want %d", v, len(saved))
	}
	migrations = saved
	after, err := open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer after.Close()
	var n int
	if err := after.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'half_done'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("half_done tables = %d (err %v), want the step rolled back", n, err)
	}
}
