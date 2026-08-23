package terminal

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const (
	modelOpus  = "claude-opus-4-8"           // native 1M window
	modelHaiku = "claude-haiku-4-5-20251001" // native 200k window (dated id)
	modelNew   = "claude-someday-9"          // unlisted → window inferred from tokens
)

// assistantLine builds one transcript JSONL entry for a model with the given
// input-side token counts, mirroring Claude's message.usage shape.
func assistantLine(model string, input, cacheRead, cacheCreate int) string {
	return `{"type":"assistant","message":{"model":"` + model + `","usage":{` +
		`"input_tokens":` + strconv.Itoa(input) +
		`,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) +
		`,"cache_creation_input_tokens":` + strconv.Itoa(cacheCreate) +
		`,"output_tokens":1975}}}`
}

// boundaryLine builds the system line a /compact writes: no assistant message,
// only the post-compaction count. trigger is "manual" or "auto" — both shapes
// are identical in a real transcript.
func boundaryLine(trigger string, preTokens, postTokens int) string {
	return `{"type":"system","subtype":"compact_boundary","isSidechain":false,` +
		`"compactMetadata":{"trigger":"` + trigger +
		`","preTokens":` + strconv.Itoa(preTokens) +
		`,"postTokens":` + strconv.Itoa(postTokens) + `}}`
}

func TestParseContextUsage(t *testing.T) {
	tests := []struct {
		name        string
		tail        string
		wantOK      bool
		wantTokens  int
		wantPercent int
	}{
		{
			name:        "a 1M model reads its context against 1M",
			tail:        assistantLine(modelOpus, 2, 64798, 0) + "\n",
			wantOK:      true,
			wantTokens:  64800,
			wantPercent: 6, // 64800 * 100 / 1_000_000
		},
		{
			name:        "a 200k model reads its context against 200k",
			tail:        assistantLine(modelHaiku, 2, 67673, 5550) + "\n",
			wantOK:      true,
			wantTokens:  73225,
			wantPercent: 36, // 73225 * 100 / 200000
		},
		{
			name: "picks the newest assistant line, not an earlier one",
			tail: assistantLine(modelHaiku, 1, 1000, 0) + "\n" +
				assistantLine(modelHaiku, 2, 40000, 0) + "\n",
			wantOK:      true,
			wantTokens:  40002,
			wantPercent: 20,
		},
		{
			// Contract change: an unlisted model takes the 1M window at any
			// token count. It used to infer the smallest window that fit, which
			// read a new 1M model as five times fuller than it was.
			name:        "an unlisted model defaults to the 1M window",
			tail:        assistantLine(modelNew, 2, 40000, 0) + "\n",
			wantOK:      true,
			wantTokens:  40002,
			wantPercent: 4, // 40002 * 100 / 1_000_000
		},
		{
			name:        "a small-window model past 200k widens to fit",
			tail:        assistantLine(modelHaiku, 0, 300000, 0) + "\n",
			wantOK:      true,
			wantTokens:  300000,
			wantPercent: 30, // 300000 * 100 / 1_000_000
		},
		{
			name:        "caps percent at 100 past a full window",
			tail:        assistantLine(modelOpus, 0, 1_200_000, 0) + "\n",
			wantOK:      true,
			wantTokens:  1_200_000,
			wantPercent: 100,
		},
		{
			name: "skips a sidechain assistant line for the main thread",
			tail: assistantLine(modelHaiku, 2, 40000, 0) + "\n" +
				`{"type":"assistant","isSidechain":true,"message":{"model":"` + modelHaiku + `","usage":{` +
				`"input_tokens":1,"cache_read_input_tokens":150000,"cache_creation_input_tokens":0}}}` + "\n",
			wantOK:      true,
			wantTokens:  40002,
			wantPercent: 20,
		},
		{
			name: "skips a leading partial line from a mid-line cut",
			tail: `he":{"usage":{"input_tokens":9}}}` + "\n" +
				assistantLine(modelHaiku, 2, 5000, 0) + "\n",
			wantOK:      true,
			wantTokens:  5002,
			wantPercent: 2,
		},
		{
			// A /compact writes no assistant line, so the newest count on the
			// thread is the boundary's, not the stale line before it.
			name: "a manual compaction reads the post-compaction count",
			tail: assistantLine(modelOpus, 2, 236_082, 0) + "\n" +
				boundaryLine("manual", 236_084, 12_642) + "\n" +
				`{"type":"user","isCompactSummary":true,"message":{"content":"summary"}}` + "\n",
			wantOK:      true,
			wantTokens:  12_642,
			wantPercent: 1, // 12642 * 100 / 1_000_000
		},
		{
			name: "an auto compaction reads the same way",
			tail: assistantLine(modelOpus, 2, 937_040, 0) + "\n" +
				boundaryLine("auto", 937_042, 25_402) + "\n",
			wantOK:      true,
			wantTokens:  25_402,
			wantPercent: 2,
		},
		{
			name: "the window still comes from the model of the line before the boundary",
			tail: assistantLine(modelHaiku, 2, 180_000, 0) + "\n" +
				boundaryLine("manual", 180_002, 40_000) + "\n",
			wantOK:      true,
			wantTokens:  40_000,
			wantPercent: 20, // 40000 * 100 / 200_000, the Haiku window
		},
		{
			name: "an assistant line after the boundary wins over it",
			tail: boundaryLine("manual", 236_084, 12_642) + "\n" +
				assistantLine(modelOpus, 2, 30_000, 0) + "\n",
			wantOK:      true,
			wantTokens:  30_002,
			wantPercent: 3,
		},
		{
			name:        "a boundary alone in the tail reads against the default window",
			tail:        boundaryLine("manual", 236_084, 12_642) + "\n",
			wantOK:      true,
			wantTokens:  12_642,
			wantPercent: 1,
		},
		{
			// Same rule as an assistant line: a sub-agent's compaction is not
			// the window the user sees.
			name: "skips a sidechain boundary for the main thread",
			tail: assistantLine(modelOpus, 2, 40_000, 0) + "\n" +
				`{"type":"system","subtype":"compact_boundary","isSidechain":true,` +
				`"compactMetadata":{"trigger":"manual","preTokens":9,"postTokens":1}}` + "\n",
			wantOK:      true,
			wantTokens:  40_002,
			wantPercent: 4,
		},
		{
			name:   "no assistant line yields not-ok",
			tail:   `{"type":"user","message":{"content":"hi"}}` + "\n",
			wantOK: false,
		},
		{
			name:   "an assistant line without usage is not-ok",
			tail:   `{"type":"assistant","message":{"model":"x"}}` + "\n",
			wantOK: false,
		},
		{
			name:   "empty tail is not-ok",
			tail:   "",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseContextUsage([]byte(tt.tail))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.tokens != tt.wantTokens {
				t.Errorf("tokens = %d, want %d", got.tokens, tt.wantTokens)
			}
			if got.percent != tt.wantPercent {
				t.Errorf("percent = %d, want %d", got.percent, tt.wantPercent)
			}
		})
	}
}

func TestParseContextUsageEffort(t *testing.T) {
	// effort is a top-level field on the entry, beside type/isSidechain.
	withEffort := `{"type":"assistant","effort":"xhigh","message":{"model":"` + modelOpus +
		`","usage":{"input_tokens":2,"cache_read_input_tokens":20000,"cache_creation_input_tokens":0}}}`
	got, ok := parseContextUsage([]byte(withEffort + "\n"))
	if !ok || got.effort != "xhigh" {
		t.Fatalf("effort = %q ok = %v, want %q true", got.effort, ok, "xhigh")
	}
	// A line without the field parses with an empty effort.
	bare, ok := parseContextUsage([]byte(assistantLine(modelOpus, 2, 20000, 0) + "\n"))
	if !ok || bare.effort != "" {
		t.Errorf("effort = %q, want empty for a line that records none", bare.effort)
	}
}

func TestWindowForModel(t *testing.T) {
	tests := []struct {
		model  string
		tokens int
		want   int
	}{
		{modelOpus, 50_000, 1_000_000},
		{"claude-opus-5", 50_000, 1_000_000},
		{modelHaiku, 50_000, 200_000}, // dated id, matched by substring
		// Contract change: a model this build has never heard of now takes the
		// 1M window instead of the smallest one that fits, so a model released
		// later reads right without a new release.
		{modelNew, 50_000, 1_000_000},
		// A count past 200k disproves the Haiku guess — it widens.
		{modelHaiku, 500_000, 1_000_000},
	}
	for _, tt := range tests {
		if got := windowForModel(tt.model, tt.tokens); got != tt.want {
			t.Errorf("windowForModel(%q, %d) = %d, want %d", tt.model, tt.tokens, got, tt.want)
		}
	}
}

func TestReadTailReturnsLastBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	tail, ok := readTail(path, 4)
	if !ok {
		t.Fatal("readTail ok = false")
	}
	if string(tail) != "6789" {
		t.Errorf("tail = %q, want %q", tail, "6789")
	}
}

func TestReadTailWholeFileWhenSmaller(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	tail, ok := readTail(path, 4096)
	if !ok || string(tail) != "abc" {
		t.Errorf("tail = %q ok = %v, want %q true", tail, ok, "abc")
	}
}

func TestReadTailMissingFile(t *testing.T) {
	if _, ok := readTail(filepath.Join(t.TempDir(), "nope.jsonl"), 16); ok {
		t.Error("readTail on a missing file should be not-ok")
	}
}

// TestClaudeContextUsageEndToEnd drives the glob-by-UUID locate through a
// CLAUDE_CONFIG_DIR override, so no real ~/.claude is touched.
func TestClaudeContextUsageEndToEnd(t *testing.T) {
	base := t.TempDir()
	slug := filepath.Join(base, "projects", "-home-user-proj")
	if err := os.MkdirAll(slug, 0o755); err != nil {
		t.Fatal(err)
	}
	id := "abc123-uuid"
	if err := os.WriteFile(
		filepath.Join(slug, id+".jsonl"),
		[]byte(assistantLine(modelHaiku, 2, 20000, 0)+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", base)

	u, ok := claudeContextUsage(id)
	if !ok {
		t.Fatal("claudeContextUsage ok = false")
	}
	if u.tokens != 20002 || u.percent != 10 || u.model != modelHaiku {
		t.Errorf("usage = %+v, want tokens 20002 percent 10 model %q", u, modelHaiku)
	}

	if _, ok := claudeContextUsage("no-such-id"); ok {
		t.Error("an unknown id should be not-ok")
	}
}
