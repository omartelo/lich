package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTranscriptTextReadsWhatWasSaid pins the extraction the history index is
// built out of: every message either side said, oldest first, and nothing else.
// The tool call and its result are in the fixture because they are the bulk of a
// real transcript and the whole reason it is not indexed whole.
func TestTranscriptTextReadsWhatWasSaid(t *testing.T) {
	toolCall := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash",` +
		`"input":{"command":"grep -r worktree ."}}]}}`
	toolResult := `{"type":"user","message":{"content":[{"type":"tool_result",` +
		`"content":"internal/terminal/worktreeport.go:1"}]}}`
	plantTranscripts(t, map[string]string{
		"uuid-a": strings.Join([]string{
			userLine("port the worktree port hash"),
			toolCall,
			toolResult,
			assistantText("The worktree port is a hash, not a reservation."),
		}, "\n") + "\n",
	})

	got, cut := TranscriptText("uuid-a", "/home/user/repo")
	want := "port the worktree port hash\nThe worktree port is a hash, not a reservation.\n"
	if got != want {
		t.Errorf("TranscriptText = %q, want %q", got, want)
	}
	if cut {
		t.Error("a conversation of two lines reports the cap cut it")
	}
}

// TestTranscriptTextIsEmptyForWhatCannotBeRead covers the misses that all have
// to answer the same way, because the caller records the empty answer as "looked
// at, nothing there" and never asks again.
func TestTranscriptTextIsEmptyForWhatCannotBeRead(t *testing.T) {
	plantTranscripts(t, map[string]string{"uuid-a": userLine("a line") + "\n"})
	for _, id := range []string{"", "uuid-never-written"} {
		if got, _ := TranscriptText(id, "/home/user/repo"); got != "" {
			t.Errorf("TranscriptText(%q) = %q, want empty", id, got)
		}
	}
}

// TestTranscriptTextReadsTheOpencodeDatabase is the second mechanism: a provider
// with no JSONL to walk answers the same question by query, and the prose comes
// back in the order it was said with the tool parts left out.
func TestTranscriptTextReadsTheOpencodeDatabase(t *testing.T) {
	blankHarnessDirs(t)
	path, ok := opencodeSessionDB()
	if !ok {
		t.Fatal("opencodeSessionDB: want a path under the throwaway data home")
	}
	copyFile(t, opencodeMessageDB(t), path)

	got, _ := TranscriptText("ses_1", "")
	want := strings.Join([]string{
		"port the worktree hash",
		"An earlier worktree answer.",
		"The worktree port is a hash.",
	}, "\n")
	if got != want {
		t.Errorf("TranscriptText = %q, want %q", got, want)
	}
}

// TestTranscriptTextReadsTheCrushDatabase is the same mechanism keyed on the
// checkout instead of the machine: Crush files one database per directory, so
// the cwd is what decides which conversation answers.
func TestTranscriptTextReadsTheCrushDatabase(t *testing.T) {
	blankHarnessDirs(t)
	cwd := t.TempDir()
	path, ok := crushSessionDB(cwd)
	if !ok {
		t.Fatal("crushSessionDB: want a path for a named checkout")
	}
	copyFile(t, crushMessageDB(t), path)

	got, _ := TranscriptText("crush-1", cwd)
	want := strings.Join([]string{
		"port the worktree hash",
		"An earlier worktree answer.",
		"The worktree port is a hash.",
	}, "\n")
	if got != want {
		t.Errorf("TranscriptText = %q, want %q", got, want)
	}
	if got, _ := TranscriptText("crush-1", ""); got != "" {
		t.Error("TranscriptText with no cwd found a Crush conversation; it has no database to ask")
	}
}

// TestNewestBytesKeepsTheNewestWholeLines is the cap that bounds what one
// session's index holds. What it drops is the oldest end, and what it keeps is
// whole: a half sentence in the index is a search hit nobody can read.
func TestNewestBytesKeepsTheNewestWholeLines(t *testing.T) {
	text := "oldest line\nmiddle line\nnewest line\n"
	tests := []struct {
		name    string
		max     int
		want    string
		wantCut bool
	}{
		{name: "under the cap is untouched", max: len(text), want: text},
		{name: "drops the oldest lines", max: 24, want: "newest line\n", wantCut: true},
		{name: "cap of zero keeps nothing", max: 0, want: "", wantCut: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cut := newestBytes(text, tt.max)
			if got != tt.want {
				t.Errorf("newestBytes(%d) = %q, want %q", tt.max, got, tt.want)
			}
			if cut != tt.wantCut {
				t.Errorf("newestBytes(%d) cut = %v, want %v", tt.max, cut, tt.wantCut)
			}
		})
	}
}

// TestNewestBytesNeverCutsARune covers the one shape with no line break to cut
// at: a single message longer than the cap, in a script where a rune is more
// than one byte. A cut inside one would put invalid UTF-8 in the database.
func TestNewestBytesNeverCutsARune(t *testing.T) {
	text := strings.Repeat("é", 40)
	for max := 1; max <= len(text); max++ {
		got, _ := newestBytes(text, max)
		if !utf8.ValidString(got) {
			t.Fatalf("newestBytes(%d) = %q, which is not valid UTF-8", max, got)
		}
		if !strings.HasSuffix(text, got) {
			t.Fatalf("newestBytes(%d) = %q, which is not the end of the text", max, got)
		}
	}
}

// blankHarnessDirs points every harness lich resolves by environment variable at
// an empty directory, so the only conversation a test can find is the one it
// planted. It is this file's own rather than the one beside it: that one is
// Windows-tagged out for the PTY spawns around it, and nothing here is
// platform-specific.
func blankHarnessDirs(t *testing.T) {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

// copyFile plants a prepared provider database where its resolver expects to
// find it, creating the directories on the way.
func copyFile(t *testing.T, from, to string) {
	t.Helper()
	body, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTranscriptTextReportsTheCapCutting is the flag a history row draws: a
// conversation with more prose in it than the index holds keeps the newest of it
// and says so, because a search that reaches only part of a session must not
// read as a session with nothing in it.
//
// The fixture is a real transcript past the real cap rather than a stubbed one:
// what is under test is that the cap survives the walk, and a cap read off a
// variable would pass against a walk that never applied it.
func TestTranscriptTextReportsTheCapCutting(t *testing.T) {
	// One recognisable line at each end, so the answer proves which end was kept
	// rather than only how much of it there is.
	var body strings.Builder
	body.WriteString(userLine("the oldest thing anybody said") + "\n")
	filler := strings.Repeat("worktree ", 900)
	for body.Len() < indexTextBytes+(1<<20) {
		body.WriteString(assistantText(filler) + "\n")
	}
	body.WriteString(assistantText("the newest thing anybody said") + "\n")
	plantTranscripts(t, map[string]string{"uuid-big": body.String()})

	got, cut := TranscriptText("uuid-big", "")
	if !cut {
		t.Fatalf("a conversation of %d bytes of prose reports the cap left it whole", len(got))
	}
	if len(got) > indexTextBytes {
		t.Errorf("indexed %d bytes, want at most %d", len(got), indexTextBytes)
	}
	if !strings.Contains(got, "the newest thing anybody said") {
		t.Error("the newest words were dropped; the cap keeps that end")
	}
	if strings.Contains(got, "the oldest thing anybody said") {
		t.Error("the oldest words survived a cut that had to drop them")
	}
}
