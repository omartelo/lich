package cli

import (
	"strings"
	"testing"
	"time"
)

// costBody is a report the fake lich hands back: two projects, one session in
// them that lich could not price, and money off both rungs.
const costBody = `{"projects":[` +
	`{"project":"lich","sessions":3,"unpriced":1,"source":"mixed","costUsd":4.3125},` +
	`{"project":"revu","sessions":1,"unpriced":0,"source":"reported","costUsd":0.5}],` +
	`"sessions":4,"unpriced":1,"priced":1,"reported":2,"source":"mixed",` +
	`"costUsd":4.8125,"readout":true}`

// TestCostPrintsTheTableAndWhatItLeavesOut is the contract of the whole
// command: the money never appears without the count of what is missing from
// it, because a sum over sessions lich could not price is a lower bound and a
// reader has no other way to know.
func TestCostPrintsTheTableAndWhatItLeavesOut(t *testing.T) {
	f := newFakeLich(t, costBody)

	code, stdout, stderr := run(t, f, "cost")

	if code != 0 {
		t.Fatalf("exit = %d (%s)", code, stderr)
	}
	for _, want := range []string{
		"project\tsessions\tunpriced\tsource\tcost",
		"lich\t3\t1\tmixed\t$4.31",
		"revu\t1\t0\treported\t$0.50",
		"total\t4\t1\tmixed\t$4.81",
		"1 priced by lich, 2 reported by their provider.",
		"Lower bound: 1 unpriced of 4 sessions",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want it to carry %q", stdout, want)
		}
	}
	if call := f.only(t); call.method != "store.CostTotals" {
		t.Errorf("method = %q, want store.CostTotals", call.method)
	}
}

// A total with nothing missing says so rather than staying silent: "no warning"
// and "nothing to warn about" are the two readings an empty line leaves open.
func TestACompleteTotalSaysItIsComplete(t *testing.T) {
	f := newFakeLich(t, `{"projects":[{"project":"lich","sessions":2,"unpriced":0,"source":"priced","costUsd":1}],`+
		`"sessions":2,"unpriced":0,"priced":2,"source":"priced","costUsd":1,"readout":true}`)

	_, stdout, _ := run(t, f, "cost")

	if !strings.Contains(stdout, "Complete: every session in this total is priced.") {
		t.Errorf("stdout = %q, want the complete line", stdout)
	}
	if strings.Contains(stdout, "Lower bound") {
		t.Errorf("stdout = %q, want no lower-bound warning", stdout)
	}
}

// With the readout off nothing has ever been counted, so a $0.00 would read as
// a machine that spent nothing. The line says which it is, and where to change
// it.
func TestCostSaysWhenTheReadoutIsOff(t *testing.T) {
	f := newFakeLich(t, `{"projects":[{"project":"lich","sessions":2,"unpriced":2,"costUsd":0}],`+
		`"sessions":2,"unpriced":2,"costUsd":0,"readout":false}`)

	_, stdout, _ := run(t, f, "cost")

	if !strings.Contains(stdout, "The cost readout is off") {
		t.Errorf("stdout = %q, want the readout to be named", stdout)
	}
}

func TestCostReportsAnEmptyWorkspace(t *testing.T) {
	f := newFakeLich(t, `{"projects":[],"sessions":0,"unpriced":0,"costUsd":0,"readout":true}`)

	code, stdout, _ := run(t, f, "cost")

	if code != 0 || strings.TrimSpace(stdout) != "No sessions matched." {
		t.Errorf("cost over nothing = %d %q", code, stdout)
	}
}

// The filters ride to the store as they were typed; --since arrives as the
// unix second the window opens at, which is the only form the query can use.
func TestCostPassesItsFilters(t *testing.T) {
	f := newFakeLich(t, costBody)

	if code, _, stderr := run(t, f, "cost", "--project", "lich", "--provider", "codex", "--since", "7d"); code != 0 {
		t.Fatalf("exit = %d (%s)", code, stderr)
	}

	args := f.only(t).args
	if len(args) != 3 || args[0] != "lich" || args[1] != "codex" {
		t.Fatalf("args = %#v, want the project and provider through", args)
	}
	from, ok := args[2].(float64)
	if !ok {
		t.Fatalf("since = %#v, want a unix second", args[2])
	}
	want := time.Now().Add(-7 * 24 * time.Hour).Unix()
	if diff := int64(from) - want; diff < -5 || diff > 5 {
		t.Errorf("since = %d, want about %d (7d ago)", int64(from), want)
	}
}

// A window nothing can be made of stops before the call: a report over the
// wrong stretch of time is a wrong number, and money is what this prints.
func TestCostRefusesAWindowItCannotRead(t *testing.T) {
	for _, window := range []string{"last week", "7", "-3d", "0h", "d"} {
		f := newFakeLich(t, costBody)

		code, _, stderr := run(t, f, "cost", "--since", window)

		if code != 1 {
			t.Errorf("--since %q exited %d, want 1", window, code)
		}
		if !strings.Contains(stderr, "--since takes a window") {
			t.Errorf("--since %q said %q, want the accepted spellings", window, stderr)
		}
		if len(f.calls) != 0 {
			t.Errorf("--since %q still called the store: %+v", window, f.calls)
		}
	}
}

// --json is the whole report — totals, the two rung tallies and the readout
// flag included — so a script piping this anywhere gets the exclusion and the
// arithmetic behind the money along with it.
func TestCostEmitsJSON(t *testing.T) {
	f := newFakeLich(t, costBody)

	code, stdout, _ := run(t, f, "cost", "--json")

	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{
		`"unpriced":1`, `"priced":1`, `"reported":2`, `"source":"mixed"`,
		`"costUsd":4.8125`, `"readout":true`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want it to carry %q", stdout, want)
		}
	}
}

// --csv carries the unpriced and source columns beside the money, which is how
// the bound and whose arithmetic it is survive leaving lich. The exact figure
// goes out, not the two places the table rounds to.
func TestCostEmitsCSV(t *testing.T) {
	f := newFakeLich(t, costBody)

	code, stdout, _ := run(t, f, "cost", "--csv")

	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	want := "project,sessions,unpriced,source,cost_usd\n" +
		"lich,3,1,mixed,4.312500\nrevu,1,0,reported,0.500000\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// Two formats is no format: there is one stdout and the answer would be half
// of each.
func TestCostRefusesTwoFormatsAtOnce(t *testing.T) {
	f := newFakeLich(t, costBody)

	code, _, stderr := run(t, f, "cost", "--json", "--csv")

	if code != 1 || !strings.Contains(stderr, "usage: lich cost") {
		t.Errorf("cost --json --csv = %d %q, want the usage line", code, stderr)
	}
}

// A total on one rung says so in the source column and adds no line under the
// table: the split line exists to warn that two kinds of arithmetic were summed,
// and here they were not.
func TestCostSplitsTheArithmeticOnlyWhenItIsMixed(t *testing.T) {
	f := newFakeLich(t, `{"projects":[{"project":"lich","sessions":2,"unpriced":0,"source":"priced","costUsd":1}],`+
		`"sessions":2,"unpriced":0,"priced":2,"source":"priced","costUsd":1,"readout":true}`)

	_, stdout, _ := run(t, f, "cost")

	if !strings.Contains(stdout, "lich\t2\t0\tpriced\t$1.00") {
		t.Errorf("stdout = %q, want the row on the priced rung", stdout)
	}
	if strings.Contains(stdout, "reported by their provider") {
		t.Errorf("stdout = %q, want no split line for a total on one rung", stdout)
	}
}

// A row with no money to attribute reads as an absence, not as a blank cell a
// reader has to guess at — and no rung is claimed for it.
func TestCostMarksARowWithNoRung(t *testing.T) {
	f := newFakeLich(t, `{"projects":[{"project":"lich","sessions":2,"unpriced":2,"costUsd":0}],`+
		`"sessions":2,"unpriced":2,"costUsd":0,"readout":true}`)

	_, stdout, _ := run(t, f, "cost")

	if !strings.Contains(stdout, "lich\t2\t2\t—\t$0.00") {
		t.Errorf("stdout = %q, want the row to mark its missing rung", stdout)
	}
}
