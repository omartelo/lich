package store

import (
	"testing"
	"time"
)

// costWorkspace opens a store with two projects and the sessions named, so a
// totals test can say who is billed where in one line.
func costWorkspace(t *testing.T) *Service {
	t.Helper()
	svc := newTestStore(t)
	for _, p := range []struct{ id, name string }{{"p1", "alpha"}, {"p2", "beta"}} {
		if err := svc.AddProject(p.id, p.name, "/tmp/"+p.name); err != nil {
			t.Fatalf("AddProject: %v", err)
		}
	}
	return svc
}

// bill adds a session of the given kind to a project and charges it.
func bill(t *testing.T, svc *Service, projectID, sessionID, kind string, cost float64) {
	t.Helper()
	if err := svc.AddSession(projectID, sessionID, sessionID, kind, "", 2, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.SaveCostLedger(sessionID, "uuid-"+sessionID, 10, "m1", cost); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}
}

// TestCostTotalsSumsPerProject is the unit the command reports in: every
// session a project holds, across every conversation each has run.
func TestCostTotalsSumsPerProject(t *testing.T) {
	svc := costWorkspace(t)
	bill(t, svc, "p1", "s1", "claude", 1.25)
	bill(t, svc, "p1", "s2", "codex", 0.75)
	bill(t, svc, "p2", "s3", "claude", 0.50)
	// A second conversation under the same session, the `/clear` case.
	if err := svc.SaveCostLedger("s1", "uuid-second", 10, "m2", 0.50); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}

	report, err := svc.CostTotals("", "", 0)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if len(report.Projects) != 2 {
		t.Fatalf("projects = %+v, want one row each", report.Projects)
	}
	// Ordered by spend, so the expensive project reads first.
	if got := report.Projects[0]; got.Project != "alpha" || got.Sessions != 2 || got.CostUSD != 2.50 {
		t.Errorf("first row = %+v, want alpha with 2 sessions at 2.50", got)
	}
	if got := report.Projects[1]; got.Project != "beta" || got.CostUSD != 0.50 {
		t.Errorf("second row = %+v, want beta at 0.50", got)
	}
	if report.Sessions != 3 || report.CostUSD != 3.00 {
		t.Errorf("total = %d sessions at %v, want 3 at 3.00", report.Sessions, report.CostUSD)
	}
}

// TestCostTotalsCountsWhatItCannotPrice is the contract the whole command turns
// on: a session with no ledger row is not silently dropped from the sum, it is
// counted beside it, which is what makes the total legible as a lower bound.
func TestCostTotalsCountsWhatItCannotPrice(t *testing.T) {
	svc := costWorkspace(t)
	bill(t, svc, "p1", "s1", "claude", 2.00)
	if err := svc.AddSession("p1", "s2", "unpriced", "crush", "", 3, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}

	report, err := svc.CostTotals("", "", 0)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	row := report.Projects[0]
	if row.Sessions != 2 || row.Unpriced != 1 || row.CostUSD != 2.00 {
		t.Errorf("row = %+v, want 2 sessions, 1 unpriced, 2.00 counted", row)
	}
	if report.Unpriced != 1 {
		t.Errorf("total unpriced = %d, want 1", report.Unpriced)
	}
}

// TestCostTotalsNarrowsByProjectAndProvider pins the two filters that come out
// of the same query. The project name is matched the way every other command
// matches one — case-insensitively.
func TestCostTotalsNarrowsByProjectAndProvider(t *testing.T) {
	svc := costWorkspace(t)
	bill(t, svc, "p1", "s1", "claude", 1.00)
	bill(t, svc, "p1", "s2", "codex", 2.00)
	bill(t, svc, "p2", "s3", "claude", 4.00)

	byProject, err := svc.CostTotals("ALPHA", "", 0)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if len(byProject.Projects) != 1 || byProject.CostUSD != 3.00 {
		t.Errorf("by project = %+v at %v, want alpha alone at 3.00", byProject.Projects, byProject.CostUSD)
	}

	byProvider, err := svc.CostTotals("", "claude", 0)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if byProvider.Sessions != 2 || byProvider.CostUSD != 5.00 {
		t.Errorf("by provider = %d sessions at %v, want 2 at 5.00", byProvider.Sessions, byProvider.CostUSD)
	}
}

// TestCostTotalsRefusesAProjectItCannotFind: a mistyped name answered with a
// zero would read as "that project cost nothing", which is the wrong answer to
// give about money.
func TestCostTotalsRefusesAProjectItCannotFind(t *testing.T) {
	svc := costWorkspace(t)
	bill(t, svc, "p1", "s1", "claude", 1.00)

	if _, err := svc.CostTotals("alfa", "", 0); err == nil {
		t.Fatal("CostTotals accepted a project name nothing matches")
	}
	// An unfiltered report over an empty workspace is not a failure — it is a
	// machine that has run nothing yet.
	empty, err := newTestStore(t).CostTotals("", "", 0)
	if err != nil {
		t.Fatalf("CostTotals on an empty workspace: %v", err)
	}
	if len(empty.Projects) != 0 || empty.Sessions != 0 {
		t.Errorf("empty report = %+v, want nothing", empty)
	}
}

// TestCostTotalsWindowsOnTheLastCountedTurn pins what --since selects: a
// session whose ledger last moved before the window is out of it, and the
// window takes the whole of what a session inside it has spent.
func TestCostTotalsWindowsOnTheLastCountedTurn(t *testing.T) {
	svc := costWorkspace(t)
	bill(t, svc, "p1", "s1", "claude", 1.00)
	bill(t, svc, "p1", "s2", "claude", 2.00)
	stale := time.Now().Add(-30 * 24 * time.Hour).Unix()
	if _, err := svc.db.Exec(
		`UPDATE session_costs SET updated_at = ? WHERE session_id = 's2'`, stale,
	); err != nil {
		t.Fatalf("age the ledger: %v", err)
	}

	week := time.Now().Add(-7 * 24 * time.Hour).Unix()
	report, err := svc.CostTotals("", "", week)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if report.Sessions != 1 || report.CostUSD != 1.00 {
		t.Errorf("window = %d sessions at %v, want the recent one alone at 1.00", report.Sessions, report.CostUSD)
	}

	all, err := svc.CostTotals("", "", 0)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if all.CostUSD != 3.00 {
		t.Errorf("no window = %v, want both at 3.00", all.CostUSD)
	}
}

// TestAnUnpricedSessionIsDatedByItsOwnLife: a session with no ledger has no
// counted turn to date it, and dropping it out of every window would hide the
// count that makes the total honest. An open one is unpriced now; a parked one
// is dated by when it was parked.
func TestAnUnpricedSessionIsDatedByItsOwnLife(t *testing.T) {
	svc := costWorkspace(t)
	bill(t, svc, "p1", "s1", "claude", 1.00)
	if err := svc.AddSession("p1", "live", "live", "crush", "", 3, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.AddSession("p1", "parked", "parked", "crush", "", 4, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	stale := time.Now().Add(-30 * 24 * time.Hour).Unix()
	if _, err := svc.db.Exec(
		`UPDATE sessions SET is_open = 0, closed_at = ? WHERE id = 'parked'`, stale,
	); err != nil {
		t.Fatalf("park the session: %v", err)
	}

	report, err := svc.CostTotals("", "", time.Now().Add(-7*24*time.Hour).Unix())
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if report.Sessions != 2 || report.Unpriced != 1 {
		t.Errorf("window = %d sessions with %d unpriced, want the live pair with one unpriced",
			report.Sessions, report.Unpriced)
	}
}

// TestCostTotalsCarriesTheReadoutFlag: with the readout off nothing is ever
// summed, so a zero is not a machine that spent nothing. The report says which.
func TestCostTotalsCarriesTheReadoutFlag(t *testing.T) {
	svc := costWorkspace(t)
	bill(t, svc, "p1", "s1", "claude", 1.00)

	off, err := svc.CostTotals("", "", 0)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if off.Readout {
		t.Error("readout reported on with the setting unset")
	}
	if err := svc.SetSetting(costReadoutKey, globalScope, "true"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	on, err := svc.CostTotals("", "", 0)
	if err != nil {
		t.Fatalf("CostTotals: %v", err)
	}
	if !on.Readout {
		t.Error("readout reported off with the setting on")
	}
}

// TestSavingALedgerStampsIt is what dates a session's spend at all: without the
// stamp every window is empty or everything is in it.
func TestSavingALedgerStampsIt(t *testing.T) {
	svc := newCostSession(t)
	before := time.Now().Unix()
	if err := svc.SaveCostLedger("s1", "uuid-a", 10, "m1", 1.00); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}

	var stamped int64
	if err := svc.db.QueryRow(
		`SELECT updated_at FROM session_costs WHERE session_id = 's1'`,
	).Scan(&stamped); err != nil {
		t.Fatalf("read the stamp: %v", err)
	}
	if stamped < before || stamped > time.Now().Unix()+1 {
		t.Errorf("stamp = %d, want it inside [%d, now]", stamped, before)
	}
}
