package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// A `TodoWrite` line as Claude Code files one, with the list encoded the way it
// was measured to encode it: a string holding the array.
func todoLine(items ...string) string {
	inner := strings.Join(items, ",")
	encoded := strings.ReplaceAll("["+inner+"]", `"`, `\"`)
	return `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"TodoWrite",` +
		`"input":{"todos":"` + encoded + `"}}]}}`
}

func todoItemJSON(status string) string {
	return `{"content":"a task","status":"` + status + `","activeForm":"doing a task"}`
}

// TestClaudeTodoReadsBothEncodings pins the pair of shapes the reader takes:
// the array the tool's schema describes, and the string Claude Code actually
// writes. A version that stops double-encoding must not stop the count.
func TestClaudeTodoReadsBothEncodings(t *testing.T) {
	tests := []struct {
		name string
		line string
		want todoList
		ok   bool
	}{
		{
			name: "list encoded as a string",
			line: todoLine(todoItemJSON("completed"), todoItemJSON("completed"), todoItemJSON("pending")),
			want: todoList{done: 2, total: 3},
			ok:   true,
		},
		{
			name: "list as a plain array",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"TodoWrite",` +
				`"input":{"todos":[{"status":"completed"},{"status":"in_progress"}]}}]}}`,
			want: todoList{done: 1, total: 2},
			ok:   true,
		},
		{
			name: "a status the reader has never seen is work left to do",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"TodoWrite",` +
				`"input":{"todos":[{"status":"blocked"},{"status":"completed"}]}}]}}`,
			want: todoList{done: 1, total: 2},
			ok:   true,
		},
		{
			name: "a sub-agent's list is not the session's",
			line: `{"type":"assistant","isSidechain":true,"message":{"content":[{"type":"tool_use",` +
				`"name":"TodoWrite","input":{"todos":[{"status":"completed"}]}}]}}`,
			ok: false,
		},
		{
			name: "another tool writes no list",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit",` +
				`"input":{"file_path":"a.go"}}]}}`,
			ok: false,
		},
		{
			name: "a cleared board is not a list",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"TodoWrite",` +
				`"input":{"todos":[]}}]}}`,
			ok: false,
		},
		{
			name: "a user turn carrying a bare string content",
			line: `{"type":"user","message":{"content":"write the tests"}}`,
			ok:   false,
		},
		{
			name: "a line cut in half",
			line: `ontent":[{"type":"tool_use","name":"TodoWrite"`,
			ok:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := claudeTodo([]byte(tc.line))
			if ok != tc.ok {
				t.Fatalf("claudeTodo ok = %v, want %v", ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("claudeTodo = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestTodoReaderForClaudeOnly pins the provider table: only Claude Code files a
// task list lich can read, and every other provider answers with nothing rather
// than with a reader that finds nothing.
func TestTodoReaderForClaudeOnly(t *testing.T) {
	if _, _, ok := todoReaderFor(usageSource{kind: providers.Claude, path: "/t.jsonl"}); !ok {
		t.Error("Claude should have a reader")
	}
	for _, kind := range []string{
		providers.Codex, providers.OMP, providers.Antigravity,
		providers.Kiro, providers.OpenCode, providers.Crush, providers.Cursor,
	} {
		if _, _, ok := todoReaderFor(usageSource{kind: kind, path: "/t.jsonl"}); ok {
			t.Errorf("%s should have no reader", kind)
		}
	}
}

// TestTodoReadReportsOnlyChanges pins what the event is worth emitting for: the
// first list, then a moved count, and nothing at all for a re-read that found
// no new line. A report lands on every tool call, so a re-emitted pair would
// re-render the card through a whole turn to say nothing.
func TestTodoReadReportsOnlyChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	write := func(lines ...string) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	src := usageSource{kind: providers.Claude, path: path, id: "x"}
	var cursors todoCursors

	write(todoLine(todoItemJSON("pending"), todoItemJSON("pending")))
	if list, changed := cursors.read("s1", src); !changed || list != (todoList{done: 0, total: 2}) {
		t.Fatalf("first read = %+v, changed %v; want 0 of 2, changed", list, changed)
	}
	if list, changed := cursors.read("s1", src); changed || list != (todoList{done: 0, total: 2}) {
		t.Errorf("re-read = %+v, changed %v; want the same pair, unchanged", list, changed)
	}

	// A turn's other tool calls move nothing, and the count has to survive them:
	// the reader answers with the list on record, not with an empty one.
	write(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{}}]}}`)
	if list, changed := cursors.read("s1", src); changed || list != (todoList{done: 0, total: 2}) {
		t.Errorf("after an unrelated tool = %+v, changed %v; want the same pair, unchanged", list, changed)
	}

	write(todoLine(todoItemJSON("completed"), todoItemJSON("pending")))
	if list, changed := cursors.read("s1", src); !changed || list != (todoList{done: 1, total: 2}) {
		t.Errorf("after progress = %+v, changed %v; want 1 of 2, changed", list, changed)
	}
}

// TestTodoReadWalksTheWholeFileOnce pins the seed: a list written further back
// than the tail the cursor starts at is still found, which is what a session
// resumed at launch depends on.
func TestTodoReadWalksTheWholeFileOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	filler := strings.Repeat(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{}}]}}`+"\n",
		searchTailBytes/90+40,
	)
	body := todoLine(todoItemJSON("completed"), todoItemJSON("pending"), todoItemJSON("pending")) + "\n" + filler
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	src := usageSource{kind: providers.Claude, path: path, id: "x"}
	list, changed := new(todoCursors).read("s1", src)
	if !changed || list != (todoList{done: 1, total: 3}) {
		t.Errorf("read = %+v, changed %v; want 1 of 3, changed", list, changed)
	}
}

// TestTodoReadRereadsAReplacedTranscript pins that a cursor whose file is no
// longer the file it was made against starts over instead of counting into
// bytes nobody read. Both ways that happens are here: the transcript swapped
// for another file (a forked conversation), and the same file rewritten shorter
// than the cursor had already reached.
func TestTodoReadRereadsAReplacedTranscript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	src := usageSource{kind: providers.Claude, path: path, id: "x"}
	var cursors todoCursors

	first := todoLine(todoItemJSON("completed"), todoItemJSON("completed"), todoItemJSON("pending"))
	if err := os.WriteFile(path, []byte(first+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, changed := cursors.read("s1", src); !changed {
		t.Fatal("first read should report a change")
	}

	forked := filepath.Join(dir, "forked.jsonl")
	body := todoLine(todoItemJSON("pending"), todoItemJSON("pending")) + "\n"
	if err := os.WriteFile(forked, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(forked, path); err != nil {
		t.Fatal(err)
	}
	if list, changed := cursors.read("s1", src); !changed || list != (todoList{done: 0, total: 2}) {
		t.Fatalf("after a fork = %+v, changed %v; want 0 of 2, changed", list, changed)
	}

	if err := os.WriteFile(path, []byte(todoLine(todoItemJSON("completed"))+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if list, changed := cursors.read("s1", src); !changed || list != (todoList{done: 1, total: 1}) {
		t.Errorf("after a shorter rewrite = %+v, changed %v; want 1 of 1, changed", list, changed)
	}
}
