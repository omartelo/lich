// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// blankTranscriptDirs points the two transcript readers at empty directories, so
// a test for one of the token-only providers proves the probe reached it rather
// than tripping over a real ~/.claude on the machine running the suite.
func blankTranscriptDirs(t *testing.T) {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
}

// ompAssistant is one oh-my-pi assistant line: the shape the reader looks for,
// with the provider's own USD total on it.
func ompAssistant(id, model string, cost float64) string {
	return fmt.Sprintf(
		`{"type":"message","id":%q,"message":{"role":"assistant","provider":"rig","model":%q,`+
			`"usage":{"input":1000,"output":500,"cost":{"input":0,"output":0,"total":%v}}}}`,
		id, model, cost,
	)
}

// writeOMPSession plants an oh-my-pi conversation under a throwaway agent
// directory, in the sessions/<encoded-cwd>/<timestamp>_<id>.jsonl layout
// ompTranscriptPath globs for.
func writeOMPSession(t *testing.T, providerSessionID string, lines ...string) {
	t.Helper()
	blankTranscriptDirs(t)
	base := t.TempDir()
	dir := filepath.Join(base, "sessions", "-home-user-repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	path := filepath.Join(dir, "2026-09-03T12-00-00-000Z_"+providerSessionID+".jsonl")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// OMP_PROFILE wins outright over the override (see ompAgentDir), so a
	// profile set on the machine running the suite would send the glob elsewhere.
	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_CODING_AGENT_DIR", base)
}

// sessionRow is one row of a provider's own session table: its id, the parent
// that dispatched it (empty for a top-level conversation) and what it cost.
type sessionRow struct {
	id     string
	parent string
	cost   float64
}

// writeOpenCodeDB plants opencode's single database under a throwaway
// XDG_DATA_HOME, with the two columns the cost read touches.
func writeOpenCodeDB(t *testing.T, rows ...sessionRow) {
	t.Helper()
	blankTranscriptDirs(t)
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	// Proves the override took: on a platform that ignored it the writes below
	// would land beside the user's real conversations.
	path, ok := opencodeSessionDB()
	if !ok || !strings.HasPrefix(path, base) {
		t.Fatalf("opencodeSessionDB = %q, want one under %q", path, base)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSessionTable(t, path,
		`CREATE TABLE session (id TEXT PRIMARY KEY, parent_id TEXT, cost REAL NOT NULL DEFAULT 0)`,
		`INSERT INTO session (id, parent_id, cost) VALUES (?, ?, ?)`, rows,
	)
}

// writeCrushDB plants Crush's per-checkout database under cwd and returns that
// directory, which is what a session spawned there hands the cost read.
func writeCrushDB(t *testing.T, rows ...sessionRow) string {
	t.Helper()
	blankTranscriptDirs(t)
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cwd := t.TempDir()
	path, ok := crushSessionDB(cwd)
	if !ok {
		t.Fatal("crushSessionDB: want a path for a named checkout")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSessionTable(t, path,
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, parent_session_id TEXT, cost REAL NOT NULL DEFAULT 0)`,
		`INSERT INTO sessions (id, parent_session_id, cost) VALUES (?, ?, ?)`, rows,
	)
	return cwd
}

func writeSessionTable(t *testing.T, path, create, insert string, rows []sessionRow) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(create); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		var parent any
		if row.parent != "" {
			parent = row.parent
		}
		if _, err := db.Exec(insert, row.id, parent, row.cost); err != nil {
			t.Fatal(err)
		}
	}
}

// TestOMPCostSumsTheTurnsOMPPriced is the reader's whole contract: oh-my-pi
// writes a USD total on every assistant turn, so the session's cost is their
// sum, with no price table consulted.
func TestOMPCostSumsTheTurnsOMPPriced(t *testing.T) {
	writeOMPSession(t, "omp-1",
		`{"type":"session","id":"omp-1"}`,
		ompAssistant("a1", "rig/one", 0.0105),
		`{"type":"message","id":"t1","message":{"role":"toolResult","toolName":"bash"}}`,
		ompAssistant("a2", "rig/one", 0.25),
	)
	path, ok := ompTranscriptPath("omp-1")
	if !ok {
		t.Fatal("ompTranscriptPath: want the planted session file")
	}

	cost, miss, ok := ompTranscriptCost(path)

	if !ok || miss != costMissNone {
		t.Fatalf("ompTranscriptCost ok = %v, miss = %q", ok, miss)
	}
	if !nearly(cost, 0.2605) {
		t.Errorf("cost = %v, want 0.2605", cost)
	}
}

// TestOMPCostWithholdsAnUnpricedTurn carries the money rule into the reader: a
// turn oh-my-pi wrote without a total of its own would make the sum quietly too
// small, which is worse than showing nothing.
func TestOMPCostWithholdsAnUnpricedTurn(t *testing.T) {
	writeOMPSession(t, "omp-1",
		ompAssistant("a1", "rig/one", 0.5),
		`{"type":"message","id":"a2","message":{"role":"assistant","model":"rig/one","usage":{"input":9}}}`,
	)
	path, _ := ompTranscriptPath("omp-1")

	cost, miss, ok := ompTranscriptCost(path)

	if ok {
		t.Errorf("ompTranscriptCost = %v, want a miss", cost)
	}
	if miss != costMissUnpriced {
		t.Errorf("miss = %q, want %q", miss, costMissUnpriced)
	}
}

// TestOMPCostSkipsWhatIsNotABilledTurn pins the prescreen: the word "assistant"
// appears in prompts and tool output, and a line that merely contains it must
// not be counted, nor treated as a turn with no price.
func TestOMPCostSkipsWhatIsNotABilledTurn(t *testing.T) {
	writeOMPSession(t, "omp-1",
		`{"type":"message","id":"u1","message":{"role":"user","text":"you are an assistant"}}`,
		`not json at all, but it says "assistant"`,
		ompAssistant("a1", "rig/one", 0.125),
	)
	path, _ := ompTranscriptPath("omp-1")

	cost, miss, ok := ompTranscriptCost(path)

	if !ok || miss != costMissNone {
		t.Fatalf("ompTranscriptCost ok = %v, miss = %q", ok, miss)
	}
	if !nearly(cost, 0.125) {
		t.Errorf("cost = %v, want 0.125 — only the assistant turn is billable", cost)
	}
}

// TestOMPCostIgnoresALineStillBeingWritten is the file mid-append: the last line
// has no newline yet, so it is not a turn the reader can trust. Counting it
// would bill half a message.
func TestOMPCostIgnoresALineStillBeingWritten(t *testing.T) {
	writeOMPSession(t, "omp-1", ompAssistant("a1", "rig/one", 0.4))
	path, _ := ompTranscriptPath("omp-1")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(ompAssistant("a2", "rig/one", 99)[:40]); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	cost, _, ok := ompTranscriptCost(path)

	if !ok || !nearly(cost, 0.4) {
		t.Errorf("cost = %v (ok %v), want 0.4 — the half-written line waits for its newline", cost, ok)
	}
}

// TestOMPCostOnAMissingFile keeps the reader on the same "degrade, never break"
// contract as every other transcript read.
func TestOMPCostOnAMissingFile(t *testing.T) {
	cost, miss, ok := ompTranscriptCost(filepath.Join(t.TempDir(), "nope.jsonl"))
	if ok {
		t.Errorf("ompTranscriptCost = %v, want a miss", cost)
	}
	if miss != costMissUnread {
		t.Errorf("miss = %q, want %q", miss, costMissUnread)
	}
}

// TestOpenCodeCostWalksTheSubAgentChain is what the measurement found: opencode
// files each sub-agent as a session of its own and leaves the parent's cost its
// own turns alone, so reading the parent row alone under-reports the session.
func TestOpenCodeCostWalksTheSubAgentChain(t *testing.T) {
	writeOpenCodeDB(t,
		sessionRow{id: "ses_parent", cost: 0.021},
		sessionRow{id: "ses_child", parent: "ses_parent", cost: 0.0105},
		sessionRow{id: "ses_grandchild", parent: "ses_child", cost: 0.5},
		sessionRow{id: "ses_stranger", cost: 100},
	)
	path, _ := opencodeSessionDB()

	cost, ok := sessionDBCost(path, opencodeCostQuery, "ses_parent")

	if !ok {
		t.Fatal("sessionDBCost: want the parent's total")
	}
	if !nearly(cost, 0.5315) {
		t.Errorf("cost = %v, want 0.5315 — the parent and every sub-agent below it", cost)
	}
}

// TestCrushCostIsTheSessionRow is the other half of that measurement: Crush
// rolls a sub-agent's spend into the session that dispatched it, so summing the
// children again would bill them twice.
func TestCrushCostIsTheSessionRow(t *testing.T) {
	cwd := writeCrushDB(t,
		sessionRow{id: "crush-parent", cost: 0.0525},
		sessionRow{id: "crush-child", parent: "crush-parent", cost: 0.021},
	)
	path, _ := crushSessionDB(cwd)

	cost, ok := sessionDBCost(path, crushCostQuery, "crush-parent")

	if !ok || !nearly(cost, 0.0525) {
		t.Errorf("cost = %v (ok %v), want 0.0525 — Crush already rolled the child in", cost, ok)
	}
}

// TestSessionDBCostMisses covers every absence the read answers false for. Each
// is a "keep the last figure", never a zero: a database lich cannot read has not
// told it the session was free.
func TestSessionDBCostMisses(t *testing.T) {
	cwd := writeCrushDB(t, sessionRow{id: "crush-1", cost: 3})
	path, _ := crushSessionDB(cwd)
	tests := []struct {
		name, path, query, id string
	}{
		{"no conversation id", path, crushCostQuery, ""},
		{"a provider with no such database", path, costQueryFor(providers.Claude), "crush-1"},
		{"a database that is not there", filepath.Join(t.TempDir(), "crush.db"), crushCostQuery, "crush-1"},
		{"a row that is not there", path, crushCostQuery, "crush-gone"},
		{"a table that moved", path, `SELECT cost FROM nowhere WHERE id = ?`, "crush-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if cost, ok := sessionDBCost(tc.path, tc.query, tc.id); ok {
				t.Errorf("sessionDBCost = %v, want a miss", cost)
			}
		})
	}
}

// TestOpenCodeCostIsAbsentForAnUnknownConversation is why the sum is NULL-checked
// rather than coalesced: SUM over no rows is zero, and a zero in the money slot
// is a claim that the session was free.
func TestOpenCodeCostIsAbsentForAnUnknownConversation(t *testing.T) {
	writeOpenCodeDB(t, sessionRow{id: "ses_one", cost: 2})
	path, _ := opencodeSessionDB()

	if cost, ok := sessionDBCost(path, opencodeCostQuery, "ses_nothere"); ok {
		t.Errorf("sessionDBCost = %v, want a miss for a conversation opencode never ran", cost)
	}
}

// TestTheCostOnlyRungReportsWithoutAWindow is the feature: each of the three
// providers that record a turn's spend and no window to take it against gets a
// usage event carrying the cost, with the context fields zeroed — which is what
// tells the footer to drop the ring instead of the whole readout.
func TestTheCostOnlyRungReportsWithoutAWindow(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want float64
		// plant returns the session's spawn directory, which only Crush needs.
		plant func(t *testing.T) string
		kind  string
	}{
		{
			name: "oh-my-pi", id: "omp-1", want: 0.2605, kind: providers.OMP,
			plant: func(t *testing.T) string {
				writeOMPSession(t, "omp-1",
					ompAssistant("a1", "rig/one", 0.0105), ompAssistant("a2", "rig/one", 0.25))
				return ""
			},
		},
		{
			name: "opencode", id: "ses_one", want: 0.0315, kind: providers.OpenCode,
			plant: func(t *testing.T) string {
				writeOpenCodeDB(t,
					sessionRow{id: "ses_one", cost: 0.021},
					sessionRow{id: "ses_two", parent: "ses_one", cost: 0.0105})
				return ""
			},
		},
		{
			name: "Crush", id: "crush-1", want: 0.0525, kind: providers.Crush,
			plant: func(t *testing.T) string {
				return writeCrushDB(t, sessionRow{id: "crush-1", cost: 0.0525})
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cwd := tc.plant(t)
			src, ok := usageSourceFor(tc.id, cwd)
			if !ok || src.kind != tc.kind {
				t.Fatalf("usageSourceFor = %+v (ok %v), want kind %q", src, ok, tc.kind)
			}
			svc := New(newCostStore(tc.id), nil, events.New())
			svc.prices = testRate
			svc.spawns.Store("s1", spawn{kind: tc.kind, cwd: cwd})

			got, ok := svc.sessionUsage("s1")

			if !ok {
				t.Fatal("sessionUsage: want ok — the cost stands without a context ring")
			}
			if got.CostUSD == nil || !nearly(*got.CostUSD, tc.want) {
				t.Fatalf("CostUSD = %v, want %v", got.CostUSD, tc.want)
			}
			if got.Window != 0 || got.Percent != 0 || got.Tokens != 0 || got.Model != "" {
				t.Errorf("context fields = %+v, want zeroed — no window was reported", got)
			}
		})
	}
}

// TestTheCostOnlyRungStaysSilentWithoutACost is the other side of that gate: with
// nothing to report at all, no event goes out. An event of zeroes would blank a
// figure the last turn earned.
func TestTheCostOnlyRungStaysSilentWithoutACost(t *testing.T) {
	writeOMPSession(t, "omp-1", ompAssistant("a1", "rig/one", 0.5))
	// The readout is off, which is the flag's whole job on a subscription.
	svc := New(stubBins{providerSession: "omp-1"}, nil, events.New())
	svc.prices = testRate

	if got, ok := svc.sessionUsage("s1"); ok {
		t.Errorf("sessionUsage = %+v, want no event when there is neither a window nor a cost", got)
	}
}

// TestAnUnpricedOMPTurnIsSpokenOnTheFooter carries the reader's miss out to the
// event: the number is standing gone, so the corner says so rather than reading
// as zero spend.
func TestAnUnpricedOMPTurnIsSpokenOnTheFooter(t *testing.T) {
	writeOMPSession(t, "omp-1",
		`{"type":"message","id":"a1","message":{"role":"assistant","model":"rig/one","usage":{"input":9}}}`)
	svc := New(newCostStore("omp-1"), nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok — the marked absence is itself the readout")
	}
	if got.CostUSD != nil {
		t.Errorf("CostUSD = %v, want absent", *got.CostUSD)
	}
	if got.CostMiss != string(costMissUnpriced) {
		t.Errorf("CostMiss = %q, want %q", got.CostMiss, costMissUnpriced)
	}
}

// TestCrushWithoutASpawnDirectoryIsNotFound is the one provider the probe cannot
// reach from the id alone: Crush keeps a database per checkout, so a session
// whose directory lich cannot name has nothing to ask.
func TestCrushWithoutASpawnDirectoryIsNotFound(t *testing.T) {
	writeCrushDB(t, sessionRow{id: "crush-1", cost: 3})

	if src, ok := usageSourceFor("crush-1", ""); ok {
		t.Errorf("usageSourceFor = %+v, want a miss without a checkout to look in", src)
	}
}

// TestTheCostOnlyRungSurvivesAStoreFailure keeps the new rung on the same
// contract the transcript readers are held to: a ledger row that cannot be
// written withholds the money and says nothing on screen, because the next turn
// heals it.
func TestTheCostOnlyRungSurvivesAStoreFailure(t *testing.T) {
	writeOMPSession(t, "omp-1", ompAssistant("a1", "rig/one", 0.5))
	store := newCostStore("omp-1")
	store.saveLedgerErr = errors.New("the database is gone")
	svc := New(store, nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if ok {
		t.Fatalf("sessionUsage = %+v, want no event: no window, and no cost to stand alone", got)
	}
	src, _ := usageSourceFor("omp-1", "")
	cost, miss, ok := svc.sessionCost("s1", src)
	if ok {
		t.Errorf("sessionCost = %v, want a miss", cost)
	}
	if miss.spoken() {
		t.Errorf("miss = %q, want one the footer stays quiet about", miss)
	}
}

// TestAProviderWithNoRecordsIsPricedByNobody is the default arm: Antigravity and
// Cursor CLI file their conversations somewhere no reader here looks, and a kind
// that reaches the cost switch anyway must read as "nothing to show" rather than
// as free.
func TestAProviderWithNoRecordsIsPricedByNobody(t *testing.T) {
	svc := New(newCostStore("agy-1"), nil, events.New())
	svc.prices = testRate

	cost, miss, ok := svc.sessionCost("s1", usageSource{kind: providers.Antigravity, id: "agy-1"})

	if ok {
		t.Errorf("sessionCost = %v, want a miss", cost)
	}
	if miss != costMissNone {
		t.Errorf("miss = %q, want the unspoken one", miss)
	}
}
