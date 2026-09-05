// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// TestLastTurnSaidReadsTheClaudeTranscript proves the whole read the panel rides
// on: session id → recorded provider session → transcript → the closing words.
func TestLastTurnSaidReadsTheClaudeTranscript(t *testing.T) {
	body := strings.Join([]string{
		`{"type":"user","message":{"content":"run the suite"}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"On it."}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"Three failures in parser_test.go."}]}}`,
	}, "\n") + "\n"
	writeTranscript(t, "uuid-abc", body)
	svc := New(stubBins{providerSession: "uuid-abc"}, nil, events.New())

	got, err := svc.LastTurnSaid("s1")
	if err != nil {
		t.Fatalf("LastTurnSaid: %v", err)
	}
	if got.Text != "Three failures in parser_test.go." {
		t.Errorf("LastTurnSaid = %q, want the last assistant text", got.Text)
	}
}

// TestLastTurnSaidMissesAreSilent pins that every absence reads the same empty
// answer rather than an error the panel would have to render: the feature is a
// convenience beside the diff, and a session lich cannot read simply shows none.
func TestLastTurnSaidMissesAreSilent(t *testing.T) {
	writeTranscript(t, "uuid-abc", `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n")

	tests := []struct {
		name  string
		store stubBins
	}{
		{"no provider session reported yet", stubBins{}},
		{"the transcript does not exist", stubBins{providerSession: "uuid-nothing"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := New(tc.store, nil, events.New())
			got, err := svc.LastTurnSaid("s1")
			if err != nil {
				t.Fatalf("LastTurnSaid: %v", err)
			}
			if got.Text != "" {
				t.Errorf("LastTurnSaid = %q, want empty", got.Text)
			}
		})
	}
}

// TestSaidReadersPickTheLastProse walks every JSONL format lich reads. Each case
// carries the noise its provider actually writes — tool output, thinking, a
// sub-agent — so a reader that matched too widely would fail here rather than in
// front of a user.
func TestSaidReadersPickTheLastProse(t *testing.T) {
	tests := []struct {
		name string
		read turnReader
		body []string
		want string
	}{
		{
			name: "claude skips sub-agent conversations",
			read: claudeTurn,
			body: []string{
				`{"type":"assistant","message":{"content":[{"type":"text","text":"parent speaks"}]}}`,
				`{"type":"assistant","isSidechain":true,"message":{"content":[{"type":"text","text":"sub-agent speaks"}]}}`,
			},
			want: "parent speaks",
		},
		{
			name: "codex reads output_text off assistant items",
			read: codexTurn,
			body: []string{
				`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"go"}]}}`,
				`{"type":"response_item","payload":{"type":"reasoning","summary":[]}}`,
				`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"All green."}]}}`,
				`{"type":"event_msg","payload":{"type":"token_count"}}`,
			},
			want: "All green.",
		},
		{
			name: "omp ignores the turns that are only thinking and tools",
			read: ompTurn,
			body: []string{
				`{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"x"},{"type":"toolCall","name":"bash"}]}}`,
				`{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"y"},{"type":"text","text":"Done, 12 files."}]}}`,
				`{"type":"message","message":{"role":"toolResult","content":[{"type":"text","text":"exit 0"}]}}`,
			},
			want: "Done, 12 files.",
		},
		{
			name: "kiro reads text past the blocks whose payload is an object",
			read: kiroTurn,
			body: []string{
				`{"kind":"Prompt","version":1,"data":{"content":[{"kind":"text","data":"make a note"}]}}`,
				`{"kind":"AssistantMessage","version":1,"data":{"content":[{"kind":"thinking","data":{"text":"z","signature":null}},{"kind":"text","data":"Created note.txt."},{"kind":"toolUse","data":{"name":"fs_write"}}]}}`,
				`{"kind":"ToolResults","version":1,"data":{"content":[]}}`,
			},
			want: "Created note.txt.",
		},
		{
			name: "antigravity takes PLANNER_RESPONSE, never the tool output beside it",
			read: antigravityTurn,
			body: []string{
				`{"type":"PLANNER_RESPONSE","source":"MODEL","content":"Opened the PR."}`,
				`{"type":"GENERIC","source":"MODEL","content":"The command exited with code 1."}`,
				`{"type":"PLANNER_RESPONSE","source":"MODEL","content":""}`,
			},
			want: "Opened the PR.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "transcript.jsonl")
			if err := os.WriteFile(path, []byte(strings.Join(tc.body, "\n")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := lastSaidInTail(path, tc.read); got != tc.want {
				t.Errorf("lastSaidInTail = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLastSaidInTailSkipsAHalfLine pins that a tail cut mid-line — the ordinary
// state of a bounded read — costs the cut line and nothing else.
func TestLastSaidInTailSkipsAHalfLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := `ontent":[{"type":"text","text":"cut in half"}]}}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"whole line"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lastSaidInTail(path, claudeTurn); got != "whole line" {
		t.Errorf("lastSaidInTail = %q, want the whole line", got)
	}
}

// TestSaidForRoutesByProvider walks the dispatch itself: the same source struct
// with a different kind must reach a reader that understands that provider's
// format, and Kiro must reach the `.jsonl` rather than the `.json` its source
// carries — the one arm that transforms the path it is given.
func TestSaidForRoutesByProvider(t *testing.T) {
	tests := []struct {
		kind string
		// name of the file to plant; the source's path is `file` unless the
		// provider reads a different one beside it.
		file string
		path string
		line string
	}{
		{
			kind: providers.Claude, file: "t.jsonl", path: "t.jsonl",
			line: `{"type":"assistant","message":{"content":[{"type":"text","text":"claude here"}]}}`,
		},
		{
			kind: providers.Codex, file: "t.jsonl", path: "t.jsonl",
			line: `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"codex here"}]}}`,
		},
		{
			kind: providers.OMP, file: "t.jsonl", path: "t.jsonl",
			line: `{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"omp here"}]}}`,
		},
		{
			kind: providers.Antigravity, file: "t.jsonl", path: "t.jsonl",
			line: `{"type":"PLANNER_RESPONSE","source":"MODEL","content":"antigravity here"}`,
		},
		{
			// The source carries the metadata file usage.go reads; the turns are
			// in the transcript beside it.
			kind: providers.Kiro, file: "t.jsonl", path: "t.json",
			line: `{"kind":"AssistantMessage","version":1,"data":{"content":[{"kind":"text","data":"kiro here"}]}}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.line+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			src := usageSource{kind: tc.kind, path: filepath.Join(dir, tc.path), id: "x"}
			want := tc.kind + " here"
			if got := saidFor(src); got != want {
				t.Errorf("saidFor(%s) = %q, want %q", tc.kind, got, want)
			}
		})
	}
}

// TestSaidForReadsTheOpenCodeDatabase covers the one provider whose conversation
// is a database rather than a file, against the schema opencode writes: the role
// lives on the message row and the text on a part row beside it, so the join is
// what keeps a tool result from being read as the agent's own words.
func TestSaidForReadsTheOpenCodeDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT)`,
		`CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT)`,
		`INSERT INTO message VALUES ('m1','ses_1',1,'{"role":"user"}')`,
		`INSERT INTO message VALUES ('m2','ses_1',2,'{"role":"assistant"}')`,
		`INSERT INTO part VALUES ('p1','m1','ses_1',1,'{"type":"text","text":"do it"}')`,
		`INSERT INTO part VALUES ('p2','m2','ses_1',2,'{"type":"text","text":"earlier"}')`,
		`INSERT INTO part VALUES ('p3','m2','ses_1',3,'{"type":"text","text":"Shipped."}')`,
		`INSERT INTO part VALUES ('p4','m2','ses_1',4,'{"type":"tool","tool":"bash"}')`,
		`INSERT INTO part VALUES ('p5','m2','ses_other',9,'{"type":"text","text":"another session"}')`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	src := usageSource{kind: providers.OpenCode, path: path, id: "ses_1"}
	if got := saidFor(src); got != "Shipped." {
		t.Errorf("saidFor = %q, want the newest assistant text part", got)
	}
}

// TestSaidForIsEmptyForProvidersWithNoLastTurn pins the deliberate gap: neither
// Crush nor Cursor CLI reports a turn boundary, so neither is ever offered the
// panel this feeds, and reading their stores would be answering a question
// nobody can ask (docs/ceilings.md).
func TestSaidForIsEmptyForProvidersWithNoLastTurn(t *testing.T) {
	for _, kind := range []string{providers.Crush, providers.Cursor} {
		if got := saidFor(usageSource{kind: kind, path: "/nope", id: "x"}); got != "" {
			t.Errorf("saidFor(%s) = %q, want empty", kind, got)
		}
	}
}

// TestKiroTranscriptPathAnswersOnlyForItsMetadata pins that the swap is anchored
// on the file kiroSessionPath resolves, so no other provider's path can be
// turned into a Kiro transcript by suffix alone.
func TestKiroTranscriptPathAnswersOnlyForItsMetadata(t *testing.T) {
	if got := kiroTranscriptPath("/k/sessions/cli/abc.json"); got != "/k/sessions/cli/abc.jsonl" {
		t.Errorf("kiroTranscriptPath = %q, want the .jsonl beside it", got)
	}
	if got := kiroTranscriptPath("/c/rollout-abc.jsonl"); got != "" {
		t.Errorf("kiroTranscriptPath = %q, want empty for a non-metadata path", got)
	}
}

// TestCapRunesKeepsBothEnds pins that an over-long answer loses its middle, not
// its tail: what a report ends on — what is left, what it is blocked by — is
// worth as much as the recap it opens with. Counted in runes, so the cut can
// never land inside a character.
func TestCapRunesKeepsBothEnds(t *testing.T) {
	text := strings.Repeat("á", 100) + "TAIL"
	got := capRunes(text, 30)
	if !strings.HasPrefix(got, strings.Repeat("á", 20)) {
		t.Errorf("capRunes lost the head: %q", got)
	}
	if !strings.HasSuffix(got, "TAIL") {
		t.Errorf("capRunes lost the tail: %q", got)
	}
	if !strings.Contains(got, "[…]") {
		t.Errorf("capRunes = %q, want the cut marked", got)
	}
	if short := capRunes("short", 30); short != "short" {
		t.Errorf("capRunes cut a text under the cap: %q", short)
	}
}
